package task

import (
	"context"
	"time"

	"github.com/quanxiang-cloud/dispatcher/internal/models"
	"github.com/quanxiang-cloud/dispatcher/internal/models/mysql"
	"github.com/quanxiang-cloud/dispatcher/internal/models/redis"
	"github.com/quanxiang-cloud/dispatcher/pkg/config"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/mysql2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/redis2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/time2"

	redisc "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Processor 处理器
type Processor struct {
	conf *config.Config

	reset   chan struct{}
	handOut handOut

	input chan *models.TaskLineHeap

	db          *gorm.DB
	redisClient *redisc.ClusterClient

	taskRepo     models.TaskRepo
	taskLineRepo models.TaskLineRepo
}

// NewProcessor new a
func NewProcessor(ctx context.Context, conf *config.Config) (*Processor, error) {
	db, err := mysql2.New(conf.Mysql, logger.Logger)
	if err != nil {
		return nil, err
	}
	client, err := redis2.NewClient(conf.Redis)
	if err != nil {
		return nil, err
	}
	p := &Processor{
		conf: conf,

		reset: make(chan struct{}, 0),
		input: make(chan *models.TaskLineHeap, 0),
		handOut: NewHTTPHandOut(&Config{
			Deadline:     time.Second * conf.HandOut.Deadline,
			DialTimeout:  time.Second * conf.HandOut.DialTimeout,
			MaxIdleConns: conf.HandOut.MaxIdleConns,
		}),

		db:           db,
		redisClient:  client,
		taskRepo:     mysql.NewTaskRepo(),
		taskLineRepo: redis.NewTaskLineRepo(client),
	}
	//keys, _ := p.taskLineRepo.Keys(ctx)
	//for k := range keys {
	//	fmt.Println(keys[k])
	//	heap, _ := p.taskLineRepo.GetHeap(ctx, 0)
	//	fmt.Println(heap)
	//	p.taskLineRepo.DeleteHeap(ctx, &models.TaskLineHeap{
	//		TaskID: keys[k],
	//	})
	//}
	p.process(ctx)
	go p.DO(ctx)
	go p.Monitor(ctx)

	return p, nil
}

const (
	silence     = 60 * 60
	lockTimeout = time.Duration(2) * time.Second
	deviation   = 1
	subTimeout  = 30 * time.Second
)

func (p *Processor) process(c context.Context) {
	for i := 0; i < p.conf.ProcessorNum; i++ {
		go func(ctx context.Context, ch chan *models.TaskLineHeap) {
			for {
				select {
				case heap := <-ch:
					ctx := logger.GenRequestID(c)
					err := p.dispatch(ctx, heap)
					if err != nil {
						logger.Logger.Errorw(err.Error(),
							zap.String("taskID", heap.TaskID),
							logger.STDRequestID(ctx))
					}
					// 解锁
					err = p.taskLineRepo.UnLock(ctx, heap.TaskID)
					if err != nil {
						logger.Logger.Errorw(err.Error(), logger.STDRequestID(ctx))
					}
				case <-c.Done():
					return
				}
			}
		}(c, p.input)
	}
}

// DO 事物处理
func (p *Processor) DO(c context.Context) {
A:
	for {
		ctx := logger.GenRequestID(c)
		// 获取最近任务
		heap, err := p.taskLineRepo.GetHeap(ctx, 0)
		if err != nil {
			logger.Logger.Errorw(err.Error(), logger.STDRequestID(ctx))
			continue
		}
		var next int64
		if heap == nil {
			next = silence
		} else {
			next = heap.TriggerTime - time2.NowUnix()
		}
		if next < 0 {
			next = 1
		}

		tick := time.Tick(time.Duration(next) * time.Second)
		select {
		case <-p.reset:
			if heap == nil {
				continue
			}
			next = heap.TriggerTime - time2.NowUnix()
			if next < 0 {

				continue
			}

			tick = time.Tick(time.Duration(next) * time.Second)
		case <-tick:

		case <-ctx.Done():

			return
		}

		var begin int64 = -1

	B:
		for {
			begin++

			// 抢占执行权
			heap1, err := p.taskLineRepo.GetHeap(ctx, begin)
			//heap1 := heap
			if err != nil {
				logger.Logger.Errorw(err.Error(), logger.STDRequestID(ctx))
				continue A
			}
			// 没有可以执行任务
			if heap1 == nil || heap1.TriggerTime-time2.NowUnix() > deviation {

				break B
			}

			ok, err := p.taskLineRepo.Lock(ctx, heap1.TaskID, heap1.TriggerTime, lockTimeout)
			if err != nil {

				logger.Logger.Errorw(err.Error(), logger.STDRequestID(ctx))
				continue B
			}
			// 抢占失败
			if !ok {
				continue A
			}

			// 从heap移除任务
			err = p.taskLineRepo.DeleteHeap(ctx, heap1)
			if err != nil {
				// 只能报个警告
				logger.Logger.Errorw(err.Error(), logger.STDRequestID(ctx))
			}
			// 分发任务
			p.input <- heap1
		}
	}
}

func (p *Processor) dispatch(ctx context.Context, taskHeap *models.TaskLineHeap) error {
	var fail = false
	defer func() {
		var msg = "dispatch success"
		if fail {
			msg = "dispatch fail"
			return
		}
		logger.Logger.Infow(msg, zap.String("taskID", taskHeap.TaskID),
			logger.STDRequestID(ctx))
	}()
	taskLine, err := p.taskLineRepo.Get(ctx, taskHeap.TaskID)
	if err != nil {
		return err
	}
	logger.Logger.Infow("dispatch task", zap.String("taskID", taskHeap.TaskID), logger.STDRequestID(ctx))

	// 分发任务
	err = p.handOut.HandOut(ctx, taskLine)

	if err != nil {
		fail = true
		logger.Logger.Errorw(err.Error(), zap.String("taskID", taskHeap.TaskID), logger.STDRequestID(ctx))
	}

	if fail && taskLine.HasRetry+1 < taskLine.Retry {
		err = p.retry(ctx, taskHeap, taskLine)
		if err == nil {
			return nil
		}
		logger.Logger.Errorw("retry fail", zap.String("Err", err.Error()), logger.STDRequestID(ctx))
	}

	if taskLine.TaskType == models.DisposableType {
		//  一次性任务 修改状态
		task := &models.Task{
			ID:        taskLine.TaskID,
			UpdatedAt: time2.NowUnix(),
			State:     models.FinishState,
		}
		if fail {
			task.State = models.FailedState
		}

		err = p.taskRepo.UpdateState(p.db, task)
		if err != nil {
			logger.Logger.Errorw(err.Error(), zap.String("taskID", taskHeap.TaskID), logger.STDRequestID(ctx))
		}
		err = p.taskLineRepo.Delete(ctx, taskLine)
		if err != nil {
			return err
		}
	} else if taskLine.TaskType == models.CronType || taskLine.TaskType == models.DelayCronType {
		if taskLine.HasRetry != 0 {
			taskLine.HasRetry = 0
			// 重置重试次数
			err := p.taskLineRepo.Create(ctx, taskLine)
			if err != nil {
				logger.Logger.Errorw(err.Error(), zap.String("taskID", taskHeap.TaskID), logger.STDRequestID(ctx))
			}
		}
		// 循环任务 计算下一次任务
		err = p.nextDo(ctx, taskLine)
		return err
	}
	return nil
}

func (p *Processor) nextDo(ctx context.Context, taskLine *models.TaskLine) error {
	ts, err := nextTime(taskLine.TaskType, taskLine.TimeBar)
	if err != nil {
		return err
	}

	heap := &models.TaskLineHeap{
		TaskID:      taskLine.TaskID,
		TriggerTime: ts,
	}

	err = p.taskLineRepo.CreateHeap(ctx, heap)
	if err != nil {
		return err
	}

	return p.redisClient.Publish(ctx, p.conf.SyncChannel, taskLine.TaskID).Err()
}

func (p *Processor) retry(ctx context.Context, taskHeap *models.TaskLineHeap, taskLine *models.TaskLine) error {
	// 尝试重试
	taskHeap.TriggerTime += taskLine.RetryDelay
	err := p.taskLineRepo.CreateHeap(ctx, taskHeap)
	if err != nil {
		return err
	}

	taskLine.HasRetry++

	err = p.taskLineRepo.Create(ctx, taskLine)
	if err != nil {
		logger.Logger.Errorw(err.Error(), zap.String("taskID", taskHeap.TaskID), logger.STDRequestID(ctx))
		// 手动事务回滚
		_ = p.taskLineRepo.DeleteHeap(ctx, taskHeap)
	}

	return p.redisClient.Publish(ctx, p.conf.SyncChannel, taskLine.TaskID).Err()
}

// Reset 重置
func (p *Processor) Reset(ctx context.Context) {
	p.reset <- struct{}{}
}

// Monitor 内置消息监听器
// 监听分布式消息通知
func (p *Processor) Monitor(c context.Context) error {
	ctx := logger.GenRequestID(c)
	pubsub := p.redisClient.Subscribe(ctx, p.conf.SyncChannel)
	_, err := pubsub.ReceiveTimeout(ctx, subTimeout)
	if err != nil {
		return err
	}
	for {
		select {
		case <-pubsub.Channel():
			p.Reset(ctx)
		case <-ctx.Done():
			return nil
		}
	}

}

// Close close
func (p *Processor) Close() error {
	close(p.input)
	close(p.reset)
	return nil
}

package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/quanxiang-cloud/dispatcher/internal/models"
	"github.com/quanxiang-cloud/dispatcher/internal/models/mysql"
	"github.com/quanxiang-cloud/dispatcher/internal/models/redis"
	"github.com/quanxiang-cloud/dispatcher/pkg/code"
	"github.com/quanxiang-cloud/dispatcher/pkg/config"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/error2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/id2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/mysql2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/redis2"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/time2"

	redisc "github.com/go-redis/redis/v8"
	"github.com/robfig/cron"

	"gorm.io/gorm"
)

// Task task
type Task interface {
	// 添加任务
	CreateTask(ctx context.Context, req *CreateTaskReq) (*CreateTaskReq, error)
	// UpdateTaskState 修改任务
	UpdateTask(ctx context.Context, req *UpdateTaskReq) (*UpdateTaskResp, error)
	// UpdateTaskState 修改任务状态
	UpdateTaskState(ctx context.Context, req *UpdateTaskStateReq) (*UpdateTaskStateResp, error)
}

// NewTask new task
func NewTask(conf *config.Config) (Task, error) {
	db, err := mysql2.New(conf.Mysql, logger.Logger)
	if err != nil {
		return nil, err
	}
	db.Logger.LogMode(4)
	client, err := redis2.NewClient(conf.Redis)
	if err != nil {
		return nil, err
	}

	return &task{
		conf: conf,

		db:           db,
		redisClient:  client,
		taskRepo:     mysql.NewTaskRepo(),
		taskLineRepo: redis.NewTaskLineRepo(client),
	}, nil
}

type task struct {
	conf *config.Config

	db          *gorm.DB
	redisClient *redisc.ClusterClient

	taskRepo     models.TaskRepo
	taskLineRepo models.TaskLineRepo
}

// CreateTaskReq 添加任务[参数]
type CreateTaskReq struct {
	Title      string `json:"title"`
	Describe   string `json:"describe"`
	Type       int    `json:"type"`
	TimeBar    string `json:"timeBar"`
	State      int    `json:"state"`
	Code       string `json:"code"`
	Retry      int    `json:"retry"`
	RetryDelay int64  `json:"retryDelay"`
}

// CreateTaskResp 添加任务[返回值]
type CreateTaskResp struct{}

func (t *task) CreateTask(ctx context.Context, req *CreateTaskReq) (*CreateTaskReq, error) {
	tx := t.db.Begin()
	task, err := t.taskRepo.GetWithCode(tx, req.Code)
	if err != nil {
		return nil, err
	}
	if task != nil {
		// 任务已经存在
		tx.Rollback()
		return nil, error2.NewError(code.TaskAlreadyExist)
	}

	task = &models.Task{
		ID:         id2.GenID(),
		Title:      req.Title,
		Describe:   req.Describe,
		Type:       models.TaskType(req.Type),
		TimeBar:    req.TimeBar,
		State:      models.TaskState(req.State),
		Code:       req.Code,
		Retry:      req.Retry,
		RetryDelay: req.RetryDelay,
		CreatedAt:  time2.NowUnix(),
	}
	err = t.taskRepo.Create(tx, task)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if task.State == models.OnState {
		// 加入执行队列
		err = t.joinImplementationPlan(ctx, task)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// 内部通知
	err = t.redisClient.Publish(ctx, t.conf.SyncChannel, task.ID).Err()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return nil, nil
}

func (t *task) joinImplementationPlan(ctx context.Context, task *models.Task) error {
	taskLine := &models.TaskLine{
		TaskID:     task.ID,
		TaskType:   task.Type,
		TimeBar:    task.TimeBar,
		Code:       task.Code,
		Retry:      task.Retry,
		RetryDelay: task.RetryDelay,
		CreatedAt:  time2.NowUnix(),
	}
	err := t.taskLineRepo.Create(ctx, taskLine)
	if err != nil {
		return err
	}
	err = t.joinMinHeap(ctx, task)
	if err == nil {
		return nil
	}
	// 逻辑事务
	err1 := t.taskLineRepo.Delete(ctx, taskLine)
	if err1 != nil {
		logger.Logger.Errorw(err1.Error(), logger.STDRequestID(ctx))
	}
	return err
}

func (t *task) joinMinHeap(ctx context.Context, task *models.Task) error {
	ts, err := nextTime(task.Type, task.TimeBar)
	if err != nil {
		return err
	}

	heap := &models.TaskLineHeap{
		TaskID:      task.ID,
		TriggerTime: ts,
	}

	err = t.taskLineRepo.CreateHeap(ctx, heap)
	if err != nil {
		return err
	}
	return nil
}

func (t *task) removeFromImplementationPlan(ctx context.Context, task *models.Task) error {
	err := t.removeFromMinHeap(ctx, task)
	if err != nil {
		return err
	}
	return t.taskLineRepo.Delete(ctx, &models.TaskLine{
		TaskID: task.ID,
	})
}

func (t *task) removeFromMinHeap(ctx context.Context, task *models.Task) error {
	heap := &models.TaskLineHeap{
		TaskID: task.ID,
	}
	return t.taskLineRepo.DeleteHeap(ctx, heap)
}

func nextTime(t models.TaskType, timebar string) (int64, error) {
	switch t {
	case models.CronType:
		p, err := cron.Parse(timebar)
		if err != nil {
			return 0, err
		}
		return p.Next(time.Now()).Unix(), nil
	case models.DisposableType:
		return time2.ISO8601ToUnix(timebar)
	case models.DelayCronType:
		cp := strings.Split(timebar, "@")
		if len(cp) != 2 {
			return 0, fmt.Errorf("invalid time format %s", timebar)
		}

		// 计算第一次执行时间
		next, err := time2.ISO8601ToUnix(cp[0])
		if err != nil {
			return 0, err
		}

		// 已经超过首次执行时间
		if next < time2.NowUnix() {
			p, err := cron.Parse(cp[1])
			if err != nil {
				return 0, err
			}
			return p.Next(time.Now()).Unix(), nil
		}
		return next, nil
	}
	return 0, error2.NewError(code.InvalidTaskType)
}

// UpdateTaskStateReq 修改任务状态[参数]
type UpdateTaskStateReq struct {
	TaskID string `json:"taskID"`
	Code   string `json:"code"`
	State  int    `json:"state"`
}

// UpdateTaskStateResp 修改任务状态[返回值]
type UpdateTaskStateResp struct{}

// UpdateTaskState 修改任务状态
func (t *task) UpdateTaskState(ctx context.Context, req *UpdateTaskStateReq) (*UpdateTaskStateResp, error) {

	var tk = new(models.Task)
	var err error = nil
	if req.TaskID != "" {
		tk, err = t.taskRepo.Get(t.db, req.TaskID)
	}
	if req.Code != "" {
		tk, err = t.taskRepo.GetWithCode(t.db, req.Code)
	}

	if err != nil {
		return nil, err
	}
	if tk == nil {
		return nil, nil
	}

	tx := t.db.Begin()
	switch req.State {
	case models.OnState:
		if tk.State == models.OnState {
			return nil, nil
		}
		err = t.updateTaskOn(ctx, tx, tk)
	case models.StandByState:
		if tk.State == models.StandByState {
			return nil, nil
		}
		err = t.updateTaskStandby(ctx, tx, tk)
	default:
		return nil, error2.NewError(code.InvalidTaskType)
	}
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tk.State = models.TaskState(req.State)
	tk.UpdatedAt = time2.NowUnix()
	err = t.taskRepo.UpdateState(tx, tk)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	err = t.redisClient.Publish(ctx, t.conf.SyncChannel, tk.ID).Err()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	return nil, err
}

func (t *task) updateTaskOn(ctx context.Context, tx *gorm.DB, task *models.Task) error {
	return t.joinImplementationPlan(ctx, task)
}

func (t *task) updateTaskStandby(ctx context.Context, tx *gorm.DB, task *models.Task) error {
	return t.removeFromImplementationPlan(ctx, task)
}

// UpdateTaskReq 修改任务[参数]
type UpdateTaskReq struct {
	TaskID     string `json:"taskID"`
	Code       string `json:"code"`
	Title      string `json:"title"`
	Describe   string `json:"describe"`
	Type       int    `json:"type"`
	TimeBar    string `json:"timeBar"`
	Retry      int    `json:"retry"`
	RetryDelay int    `json:"retryDelay"`
}

// UpdateTaskResp 修改任务[返回值]
type UpdateTaskResp struct{}

// UpdateTaskState 修改任务
func (t *task) UpdateTask(ctx context.Context, req *UpdateTaskReq) (*UpdateTaskResp, error) {
	task, err := t.taskRepo.Get(t.db, req.TaskID)
	if err != nil {
		return nil, err
	}
	if task.State == models.OnState {
		return nil, error2.NewError(code.ErrTaskState)
	}

	task.Code = req.Code
	task.Title = req.Title
	task.Describe = req.Describe
	task.Type = models.TaskType(req.Type)
	task.TimeBar = req.TimeBar
	task.Retry = req.Retry
	task.RetryDelay = int64(req.RetryDelay)

	err = t.taskRepo.Update(t.db, task)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

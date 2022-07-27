package models

import (
	"context"
	"time"
)

// TaskLine task lines
// redis
type TaskLine struct {
	TaskID string
	// TaskType cron类型 重入下一次执行时间
	TaskType TaskType

	// 冗余
	TimeBar    string
	Code       string
	Retry      int
	RetryDelay int64

	// HasRetry 重试次数
	HasRetry int

	CreatedAt int64
}

// TaskLineHeap task line heap
type TaskLineHeap struct {
	TaskID      string
	TriggerTime int64
}

// TaskLineRepo task line[存储服务]
type TaskLineRepo interface {
	Get(ctx context.Context, taskID string) (*TaskLine, error)

	Create(context.Context, *TaskLine) error

	CreateHeap(context.Context, *TaskLineHeap) error

	Delete(context.Context, *TaskLine) error

	DeleteHeap(context.Context, *TaskLineHeap) error

	// GetHeap 获取任务
	GetHeap(ctx context.Context, start int64) (*TaskLineHeap, error)

	// Lock 设置分布式锁
	Lock(ctx context.Context, key string, val interface{}, ttl time.Duration) (bool, error)

	// UnLock 解除分布式锁
	UnLock(ctx context.Context, key string) error

	Keys(ctx context.Context) ([]string, error)
}

package models

import "gorm.io/gorm"

// TaskType 任务类型
type TaskType int

const (
	// DisposableType 一次性
	DisposableType TaskType = iota + 1

	// CronType cron
	CronType

	// DelayCronType 延迟 CronType
	DelayCronType
)

// TaskState 任务状态
type TaskState int

const (
	// OnState 进行中
	OnState = iota + 1
	// StandByState stand by
	StandByState
	// FinishState finish
	FinishState
	// FailedState failed
	FailedState
)

const (
	// RetryDelay 默认延迟3秒
	RetryDelay = 3
)

// Task task
type Task struct {
	ID       string
	Title    string
	Describe string
	Type     TaskType
	// TimeBar 由type决定
	// DisposableType=1 ISO8601
	// DisposableType=2 cron 0 * * * * ?
	// DisposableType=3 ISO8601@cron 0 * * * * ?
	TimeBar string

	State TaskState

	// Code 任务唯一标志
	Code string

	Retry int
	// RetryDelay 延迟(秒)
	RetryDelay int64

	CreatedAt int64
	UpdatedAt int64
}

// TaskRepo Task[存储服务]
type TaskRepo interface {
	Create(*gorm.DB, *Task) error
	Update(*gorm.DB, *Task) error
	UpdateState(*gorm.DB, *Task) error
	GetWithCode(db *gorm.DB, code string) (*Task, error)
	Get(*gorm.DB, string) (*Task, error)
	Search(db *gorm.DB, title, code string, _t TaskType, state TaskState, isRetry bool, page, size int) ([]*Task, int64, error)
	Delete(*gorm.DB, *Task) error
}

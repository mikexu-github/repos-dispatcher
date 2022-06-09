package code

import "github.com/quanxiang-cloud/dispatcher/pkg/misc/error2"

func init() {
	error2.CodeTable = CodeTable
}

const (
	// TaskAlreadyExist 任务已经存在
	TaskAlreadyExist = 54090001
	// InvalidTaskType 无效任务类型
	InvalidTaskType = 54000002
	// InvalidTaskState 无效任务状态
	InvalidTaskState = 54000003
	// ErrTaskState 任务状态错误
	ErrTaskState = 54030003
)

// CodeTable 码表
var CodeTable = map[int64]string{
	TaskAlreadyExist: "task already exist.",
	InvalidTaskType:  "invalid task type.",
	InvalidTaskState: "invalid task state.",
	ErrTaskState:     "err task tate.",
}

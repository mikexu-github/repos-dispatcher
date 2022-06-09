package restful

import (
	"net/http"

	"github.com/quanxiang-cloud/dispatcher/internal/task"
	"github.com/quanxiang-cloud/dispatcher/pkg/config"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/resp"

	"github.com/gin-gonic/gin"
)

// Task gin task
type Task struct {
	task task.Task
}

// NewTask new task gin
func NewTask(conf *config.Config) (*Task, error) {
	t, err := task.NewTask(conf)
	if err != nil {
		return nil, err
	}
	return &Task{
		task: t,
	}, nil
}

// CreateTask 创建任务
func (t *Task) CreateTask(c *gin.Context) {
	req := &task.CreateTaskReq{}
	if err := c.ShouldBind(req); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	resp.Format(t.task.CreateTask(logger.CTXTransfer(c), req)).Context(c)
}

// UpdateTask 修改任务
func (t *Task) UpdateTask(c *gin.Context) {
	req := &task.UpdateTaskReq{}
	if err := c.ShouldBind(req); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	resp.Format(t.task.UpdateTask(logger.CTXTransfer(c), req)).Context(c)
}

// UpdateTaskState 修改任务状态
func (t *Task) UpdateTaskState(c *gin.Context) {
	req := &task.UpdateTaskStateReq{}
	if err := c.ShouldBind(req); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	resp.Format(t.task.UpdateTaskState(logger.CTXTransfer(c), req)).Context(c)
}

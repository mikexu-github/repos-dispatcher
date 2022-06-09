package mysql

import (
	"github.com/quanxiang-cloud/dispatcher/internal/models"

	"gorm.io/gorm"
)

type taskRepo struct {
}

// NewTaskRepo new task repo
func NewTaskRepo() models.TaskRepo {
	return &taskRepo{}
}

func (t *taskRepo) TableName() string {
	return "task"
}

func (t *taskRepo) Create(db *gorm.DB, task *models.Task) error {
	return db.Table(t.TableName()).Create(task).Error
}
func (t *taskRepo) Update(db *gorm.DB, task *models.Task) error {
	return db.Table(t.TableName()).Where("id = ?", task.ID).
		Updates(map[string]interface{}{
			"code":        task.Code,
			"title":       task.Title,
			"describe":    task.Describe,
			"type":        task.Type,
			"time_bar":    task.TimeBar,
			"retry":       task.Retry,
			"retry_delay": task.RetryDelay,
			"updated_at":  task.UpdatedAt,
		}).Error
}
func (t *taskRepo) UpdateState(db *gorm.DB, task *models.Task) error {
	return db.Table(t.TableName()).Where("id = ?", task.ID).
		Updates(map[string]interface{}{
			"state":      task.State,
			"updated_at": task.UpdatedAt,
		}).Error
}
func (t *taskRepo) GetWithCode(db *gorm.DB, code string) (*models.Task, error) {
	task := new(models.Task)
	err := db.Table(t.TableName()).
		Where("code = ?", code).
		Find(task).
		Error
	if err != nil {
		return nil, err
	}
	if task.ID == "" {
		return nil, nil
	}
	return task, nil
}
func (t *taskRepo) Get(db *gorm.DB, id string) (*models.Task, error) {
	task := new(models.Task)
	err := db.Table(t.TableName()).
		Where("id = ?", id).
		Find(task).
		Error
	if err != nil {
		return nil, err
	}
	if task.ID == "" {
		return nil, nil
	}
	return task, nil
}
func (t *taskRepo) Search(db *gorm.DB, title, code string, _t models.TaskType, state models.TaskState, isRetry bool, page, size int) ([]*models.Task, int64, error) {
	return nil, 0, nil
}
func (t *taskRepo) Delete(*gorm.DB, *models.Task) error {
	return nil
}

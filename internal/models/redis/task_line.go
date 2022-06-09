package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/quanxiang-cloud/dispatcher/internal/models"
)

type taskLineRepo struct {
	c *redis.ClusterClient
}

// NewTaskLineRepo new task line repo
func NewTaskLineRepo(c *redis.ClusterClient) models.TaskLineRepo {
	return &taskLineRepo{
		c: c,
	}
}

func (t *taskLineRepo) Key() string {
	return redisKey + ":line"
}

func (t *taskLineRepo) HeapKey() string {
	return redisKey + ":line:heap"
}

func (t *taskLineRepo) LockKey() string {
	return redisKey + ":line:lock:"
}

func (t *taskLineRepo) Get(ctx context.Context, taskID string) (*models.TaskLine, error) {
	entityByte, err := t.c.HGet(ctx, t.Key(),
		taskID).Bytes()
	if err != nil {
		return nil, err
	}

	entity := new(models.TaskLine)
	err = json.Unmarshal(entityByte, entity)

	return entity, err
}

func (t *taskLineRepo) Create(ctx context.Context, entity *models.TaskLine) error {
	entityJSON, err := json.Marshal(entity)
	if err != nil {
		return err
	}
	return t.c.HSet(ctx, t.Key(),
		entity.TaskID, entityJSON).Err()
}

func (t *taskLineRepo) CreateHeap(ctx context.Context, entity *models.TaskLineHeap) error {
	return t.c.ZAdd(ctx, t.HeapKey(), &redis.Z{
		Member: entity.TaskID,
		Score:  float64(entity.TriggerTime),
	}).Err()
}

func (t *taskLineRepo) Delete(ctx context.Context, entity *models.TaskLine) error {
	return t.c.HDel(ctx, t.Key(), entity.TaskID).Err()
}

func (t *taskLineRepo) DeleteHeap(ctx context.Context, entity *models.TaskLineHeap) error {
	return t.c.ZRem(ctx, t.HeapKey(), entity.TaskID).Err()
}

func (t *taskLineRepo) GetHeap(ctx context.Context, l int64) (*models.TaskLineHeap, error) {
	val := t.c.ZRangeWithScores(ctx, t.HeapKey(), l, l)
	if len(val.Val()) == 0 {
		return nil, nil
	}
	return &models.TaskLineHeap{
		TaskID:      val.Val()[0].Member.(string),
		TriggerTime: int64(val.Val()[0].Score),
	}, nil
}

// Lock 设置分布式锁
func (t *taskLineRepo) Lock(ctx context.Context, key string, val interface{}, ttl time.Duration) (bool, error) {
	return t.c.SetNX(ctx, t.LockKey()+key, val, ttl).Result()
}

// UnLock 解除分布式锁
func (t *taskLineRepo) UnLock(ctx context.Context, key string) error {
	return t.c.Del(ctx, t.LockKey()+key).Err()
}

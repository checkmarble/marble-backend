package repositories

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisExecutor struct {
	client *RedisClient
	orgId  uuid.UUID
}

func (client *RedisClient) NewExecutor(orgId uuid.UUID) *RedisExecutor {
	return &RedisExecutor{
		client: client,
		orgId:  orgId,
	}
}

func (exec *RedisExecutor) WithOrg(orgId uuid.UUID) *RedisExecutor {
	exec.orgId = orgId

	return exec
}

func (exec *RedisExecutor) Key(keys ...string) string {
	key := strings.Join(keys, ":")

	if exec.orgId == uuid.Nil {
		return key
	}
	return exec.orgId.String() + ":" + key
}

func (exec *RedisExecutor) Exec(f func(*redis.Client) error) error {
	return f(exec.client.client)
}

func (exec *RedisExecutor) Tx(ctx context.Context, f func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return exec.client.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		return f(p)
	})
}

func RedisQuery[T any](exec *RedisExecutor, cb func(*redis.Client) (T, error)) (T, error) {
	return cb(exec.client.client)
}

func RedisLoadModel[T any](ctx context.Context, exec *RedisExecutor, key string) (T, error) {
	dflt := *new(T)

	out, err := exec.client.client.Get(ctx, key).Result()
	if err != nil {
		return dflt, err
	}

	dec := json.NewDecoder(strings.NewReader(out))

	var model T

	if err := dec.Decode(&model); err != nil {
		return dflt, err
	}

	return model, nil
}

func (exec *RedisExecutor) SaveModel(ctx context.Context, key string, model any, ttl time.Duration) error {
	marshalled, err := json.Marshal(model)
	if err != nil {
		return err
	}

	return exec.client.client.Set(ctx, key, marshalled, ttl).Err()
}

func RedisLoadMap[T comparable](ctx context.Context, exec *RedisExecutor, key string) (T, error) {
	var model T

	cmd := exec.client.client.HGetAll(ctx, key)

	if cmd.Err() != nil {
		return model, cmd.Err()
	}
	if len(cmd.Val()) == 0 {
		return model, models.NotFoundError
	}

	if err := cmd.Scan(&model); err != nil {
		return model, err
	}

	return model, nil
}

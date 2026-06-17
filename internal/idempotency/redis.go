package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{client: client, ttl: ttl}
}

func NewRedisClient(addr string, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (s *RedisStore) Get(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	taskID, err := s.client.Get(context.Background(), redisKey(key)).Result()
	if err != nil {
		return "", false
	}
	return taskID, true
}

func (s *RedisStore) Set(key string, taskID string) {
	if key == "" {
		return
	}
	_ = s.client.Set(context.Background(), redisKey(key), taskID, s.ttl).Err()
}

func redisKey(key string) string {
	return "creator:idempotency:" + key
}

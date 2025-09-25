package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisUserCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisUserCache(client *redis.Client, ttl time.Duration) *RedisUserCache {
	return &RedisUserCache{client: client, ttl: ttl}
}

func (c *RedisUserCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil // cache miss
		}
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (c *RedisUserCache) Set(ctx context.Context, key string, val interface{}, ttlSecs int) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, time.Duration(ttlSecs)*time.Second).Err()
}

func (c *RedisUserCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

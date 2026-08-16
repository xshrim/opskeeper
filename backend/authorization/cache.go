package authorization

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ScopeCache interface {
	Get(context.Context, string) (ScopeFilter, bool, error)
	Set(context.Context, string, ScopeFilter, time.Duration) error
}

type RedisScopeCache struct {
	client *redis.Client
}

func NewRedisScopeCache(client *redis.Client) ScopeCache {
	return &RedisScopeCache{client: client}
}

func (c *RedisScopeCache) Get(ctx context.Context, key string) (ScopeFilter, bool, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return ScopeFilter{}, false, nil
	}
	if err != nil {
		return ScopeFilter{}, false, err
	}
	var filter ScopeFilter
	if err := json.Unmarshal([]byte(value), &filter); err != nil {
		return ScopeFilter{}, false, fmt.Errorf("decode authorization cache: %w", err)
	}
	return filter, true, nil
}

func (c *RedisScopeCache) Set(ctx context.Context, key string, filter ScopeFilter, ttl time.Duration) error {
	value, err := json.Marshal(filter)
	if err != nil {
		return fmt.Errorf("encode authorization cache: %w", err)
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

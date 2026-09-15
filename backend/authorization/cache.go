package authorization

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type MemoryScopeCache struct {
	mu    sync.RWMutex
	items map[string]memoryCacheItem
}
type memoryCacheItem struct {
	filter    ScopeFilter
	expiresAt time.Time
}

func NewMemoryScopeCache() ScopeCache {
	return &MemoryScopeCache{items: make(map[string]memoryCacheItem)}
}
func (c *MemoryScopeCache) Get(_ context.Context, key string) (ScopeFilter, bool, error) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return ScopeFilter{}, false, nil
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return ScopeFilter{}, false, nil
	}
	return item.filter, true, nil
}
func (c *MemoryScopeCache) Set(_ context.Context, key string, filter ScopeFilter, ttl time.Duration) error {
	c.mu.Lock()
	c.items[key] = memoryCacheItem{filter: filter, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
	return nil
}

type PostgresScopeCache struct{ pool *pgxpool.Pool }

func NewPostgresScopeCache(pool *pgxpool.Pool) ScopeCache { return &PostgresScopeCache{pool: pool} }
func (c *PostgresScopeCache) Get(ctx context.Context, key string) (ScopeFilter, bool, error) {
	var raw []byte
	err := c.pool.QueryRow(ctx, `SELECT value FROM cache_entries WHERE cache_key=$1 AND expires_at > now()`, key).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ScopeFilter{}, false, nil
		}
		return ScopeFilter{}, false, err
	}
	var filter ScopeFilter
	if err := json.Unmarshal(raw, &filter); err != nil {
		return ScopeFilter{}, false, fmt.Errorf("decode authorization cache: %w", err)
	}
	return filter, true, nil
}
func (c *PostgresScopeCache) Set(ctx context.Context, key string, filter ScopeFilter, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("authorization cache TTL must be positive")
	}
	raw, err := json.Marshal(filter)
	if err != nil {
		return fmt.Errorf("encode authorization cache: %w", err)
	}
	_, err = c.pool.Exec(ctx, `INSERT INTO cache_entries(cache_key,value,expires_at) VALUES($1,$2::jsonb,now()+($3 * interval '1 second')) ON CONFLICT(cache_key) DO UPDATE SET value=EXCLUDED.value,expires_at=EXCLUDED.expires_at`, key, raw, ttl.Seconds())
	return err
}

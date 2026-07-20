package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	fallbackMaxEntries = 10000
	fallbackSweepEvery = 1 * time.Minute

	unhealthyCooldown = 5 * time.Second
)

var rdb *redis.Client

var (
	clientInitialized  atomic.Bool
	redisHealthy       atomic.Bool
	unhealthyUntilNano atomic.Int64
)

type fallbackEntry struct {
	value     string
	expiresAt time.Time
}

var fallbackStore = struct {
	mu    sync.RWMutex
	items map[string]fallbackEntry
}{
	items: make(map[string]fallbackEntry),
}

var fallbackJanitorOnce sync.Once

func valueAsString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func setFallback(key string, value any, expiration time.Duration) {
	fallbackJanitorOnce.Do(startFallbackJanitor)

	if expiration <= 0 {
		expiration = time.Minute
	}

	now := time.Now()
	expiresAt := now.Add(expiration)

	fallbackStore.mu.Lock()
	if len(fallbackStore.items) >= fallbackMaxEntries {
		evictFallbackEntriesLocked(now)
	}
	if len(fallbackStore.items) >= fallbackMaxEntries {
		slog.Warn("fallback cache full, skipping in-memory cache set", "key", key, "max_entries", fallbackMaxEntries)
		fallbackStore.mu.Unlock()
		return
	}
	fallbackStore.items[key] = fallbackEntry{
		value:     valueAsString(value),
		expiresAt: expiresAt,
	}
	fallbackStore.mu.Unlock()
}

func getFallback(key string) (string, bool) {
	fallbackStore.mu.RLock()
	entry, ok := fallbackStore.items[key]
	fallbackStore.mu.RUnlock()
	if !ok {
		return "", false
	}

	if time.Now().After(entry.expiresAt) {
		fallbackStore.mu.Lock()
		delete(fallbackStore.items, key)
		fallbackStore.mu.Unlock()
		return "", false
	}

	return entry.value, true
}

func deleteFallback(key string) {
	fallbackStore.mu.Lock()
	delete(fallbackStore.items, key)
	fallbackStore.mu.Unlock()
}

func getFallbackBytes(key string) ([]byte, bool) {
	val, ok := getFallback(key)
	if !ok {
		return nil, false
	}
	return []byte(val), true
}

func startFallbackJanitor() {
	go func() {
		ticker := time.NewTicker(fallbackSweepEvery)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			fallbackStore.mu.Lock()
			evictFallbackEntriesLocked(now)
			fallbackStore.mu.Unlock()
		}
	}()
}

func evictFallbackEntriesLocked(now time.Time) {
	for key, entry := range fallbackStore.items {
		if now.After(entry.expiresAt) {
			delete(fallbackStore.items, key)
		}
	}
}

func RedisClient(addr string, password string, db int) error {
	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		clientInitialized.Store(false)
		redisHealthy.Store(false)
		return err
	}

	clientInitialized.Store(true)
	redisHealthy.Store(true)
	return nil
}

func redisReady() bool {
	if !clientInitialized.Load() {
		return false
	}
	if redisHealthy.Load() {
		return true
	}
	return time.Now().UnixNano() > unhealthyUntilNano.Load()
}

func markRedisHealthy() {
	redisHealthy.Store(true)
}

func markRedisUnhealthy() {
	redisHealthy.Store(false)
	unhealthyUntilNano.Store(time.Now().Add(unhealthyCooldown).UnixNano())
}

func SetCache(key string, value any, expiration time.Duration) error {
	if !redisReady() {
		setFallback(key, value, expiration)
		slog.Info("cache client is not healthy, skipping SetCache")
		return nil
	}

	ctx := context.Background()
	err := rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		markRedisUnhealthy()
		setFallback(key, value, expiration)
		return err
	}

	markRedisHealthy()
	deleteFallback(key)
	return nil
}

func GetCache(key string) (string, error) {
	if redisReady() {
		ctx := context.Background()
		val, err := rdb.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				markRedisHealthy()
				return "", nil
			}
			markRedisUnhealthy()
			return "", err
		}

		markRedisHealthy()
		deleteFallback(key)
		return val, nil
	}

	slog.Info("cache client is not healthy, trying in-memory fallback")

	if val, ok := getFallback(key); ok {
		return val, nil
	}

	return "", nil
}

func SetCacheBytes(key string, value []byte, expiration time.Duration) error {
	if !redisReady() {
		setFallback(key, value, expiration)
		slog.Info("cache client is not healthy, skipping SetCacheBytes")
		return nil
	}

	ctx := context.Background()
	err := rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		markRedisUnhealthy()
		setFallback(key, value, expiration)
		return err
	}

	markRedisHealthy()
	deleteFallback(key)
	return nil
}

func GetCacheBytes(key string) ([]byte, error) {
	if redisReady() {
		ctx := context.Background()
		val, err := rdb.Get(ctx, key).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				markRedisHealthy()
				return nil, nil
			}
			markRedisUnhealthy()
			return nil, err
		}

		markRedisHealthy()
		deleteFallback(key)
		return val, nil
	}

	slog.Info("cache client is not healthy, trying in-memory fallback")

	if val, ok := getFallbackBytes(key); ok {
		return val, nil
	}

	return nil, nil
}

func DeleteCache(key string) error {
	deleteFallback(key)

	if !redisReady() {
		slog.Info("cache client is not healthy, skipping DeleteCache")
		return nil
	}

	ctx := context.Background()
	if err := rdb.Del(ctx, key).Err(); err != nil {
		markRedisUnhealthy()
		return err
	}
	markRedisHealthy()
	return nil
}

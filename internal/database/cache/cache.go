package cache

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client
var clientInitialized bool

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
	fallbackStore.mu.Lock()
	fallbackStore.items[key] = fallbackEntry{
		value:     valueAsString(value),
		expiresAt: time.Now().Add(expiration),
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

func RedisClient(addr string, password string, db int) error {
	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		clientInitialized = false
		return err
	}

	clientInitialized = true
	return nil
}

func isHealthy() bool {
	if !clientInitialized {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return rdb.Ping(ctx).Err() == nil
}

func SetCache(key string, value any, expiration time.Duration) error {
	setFallback(key, value, expiration)

	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping SetCache")
		return nil
	}

	ctx := context.Background()
	err := rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetCache(key string) (string, error) {
	if isHealthy() {
		ctx := context.Background()
		val, err := rdb.Get(ctx, key).Result()
		if err == nil {
			return val, nil
		}

		if err != redis.Nil {
			slog.Debug("cache get failed, trying fallback", "Key", key, "Error", err.Error())
		}
	} else {
		slog.Info("cache client is not healthy, trying in-memory fallback")
	}

	if val, ok := getFallback(key); ok {
		return val, nil
	}

	if isHealthy() {
		return "", redis.Nil
	}

	return "", nil
}

func DeleteCache(key string) error {
	deleteFallback(key)

	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping DeleteCache")
		return nil
	}

	ctx := context.Background()
	return rdb.Del(ctx, key).Err()
}

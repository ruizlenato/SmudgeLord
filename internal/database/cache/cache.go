package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client
var clientInitialized bool

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
	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping GetCache")
		return "", nil
	}

	ctx := context.Background()
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func DeleteCache(key string) error {
	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping DeleteCache")
		return nil
	}

	ctx := context.Background()
	return rdb.Del(ctx, key).Err()
}

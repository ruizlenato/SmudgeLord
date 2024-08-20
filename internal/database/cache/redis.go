package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func RedisClient(addr string, password string, db int) error {
	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	return nil
}

func SetCache(key string, value interface{}, expiration time.Duration) error {
	ctx := context.Background()
	err := rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetCache(key string) (string, error) {
	ctx := context.Background()
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

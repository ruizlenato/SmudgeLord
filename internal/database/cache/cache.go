package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"
)

var client valkey.Client
var clientInitialized bool

func ValkeyClient(addr string) error {
	var err error
	client, err = valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
	})
	if err != nil {
		clientInitialized = false
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Do(ctx, client.B().Ping().Build()).Error()
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Do(ctx, client.B().Ping().Build()).Error()
	return err == nil
}

func SetCache(key string, value any, expiration time.Duration) error {
	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping SetCache")
		return nil
	}

	ctx := context.Background()

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		strValue = fmt.Sprintf("%v", v)
	}

	var cmd valkey.Completed
	if expiration > 0 {
		cmd = client.B().Set().Key(key).Value(strValue).ExSeconds(int64(expiration.Seconds())).Build()
	} else {
		cmd = client.B().Set().Key(key).Value(strValue).Build()
	}

	err := client.Do(ctx, cmd).Error()
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
	result := client.Do(ctx, client.B().Get().Key(key).Build())

	if result.Error() != nil {
		return "", result.Error()
	}

	return result.ToString()
}

func DeleteCache(key string) error {
	if !isHealthy() {
		slog.Info("cache client is not healthy, skipping DeleteCache")
		return nil
	}

	ctx := context.Background()
	err := client.Do(ctx, client.B().Del().Key(key).Build()).Error()
	return err
}

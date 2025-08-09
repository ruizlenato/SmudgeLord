package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

var client valkey.Client

func ValkeyClient(addr string) error {
	var err error
	client, err = valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
	})
	if err != nil {
		return err
	}

	ctx := context.Background()

	err = client.Do(ctx, client.B().Ping().Build()).Error()
	if err != nil {
		return err
	}

	return nil
}

func SetCache(key string, value any, expiration time.Duration) error {
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
	ctx := context.Background()
	result := client.Do(ctx, client.B().Get().Key(key).Build())

	if result.Error() != nil {
		return "", result.Error()
	}

	return result.ToString()
}

package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wangn-tech/campus-hub/internal/config"
)

func Open(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Password: cfg.Password, DB: cfg.Database})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

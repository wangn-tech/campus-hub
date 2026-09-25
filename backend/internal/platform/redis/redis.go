package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wangn-tech/campus-hub/internal/config"
)

func Open(cfg config.RedisConfig) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.Database,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}), nil
}

func Check(ctx context.Context, client *redis.Client) error { return client.Ping(ctx).Err() }

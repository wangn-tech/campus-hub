package realtime

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// presenceTTL keeps stale online markers from outliving a crashed instance; a
// live connection refreshes the marker every heartbeat.
const presenceTTL = 2 * time.Minute

// Presence records which users hold live connections so other instances can
// find them. Redis keys follow the WebSocket design document section 11.1.
type Presence interface {
	Connected(ctx context.Context, userUUID, connectionID string) error
	Disconnected(ctx context.Context, userUUID, connectionID string) error
}

// RedisPresence stores the connection set per user plus a short lived online
// flag; both expire so a killed instance does not leave users online forever.
type RedisPresence struct{ client *redis.Client }

func NewRedisPresence(client *redis.Client) *RedisPresence { return &RedisPresence{client: client} }

func (p *RedisPresence) Connected(ctx context.Context, userUUID, connectionID string) error {
	connectionsKey := "ws:user-connections:" + userUUID
	pipe := p.client.TxPipeline()
	pipe.SAdd(ctx, connectionsKey, connectionID)
	pipe.Expire(ctx, connectionsKey, presenceTTL)
	pipe.Set(ctx, "ws:online:"+userUUID, 1, presenceTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (p *RedisPresence) Disconnected(ctx context.Context, userUUID, connectionID string) error {
	connectionsKey := "ws:user-connections:" + userUUID
	if err := p.client.SRem(ctx, connectionsKey, connectionID).Err(); err != nil && err != redis.Nil {
		return err
	}
	// Drop the online flag only once the user has no connection left anywhere.
	remaining, err := p.client.SCard(ctx, connectionsKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	if remaining > 0 {
		return p.client.Expire(ctx, connectionsKey, presenceTTL).Err()
	}
	return p.client.Del(ctx, "ws:online:"+userUUID).Err()
}

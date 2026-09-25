package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/wangn-tech/campus-hub/internal/config"
)

func Open(cfg config.KafkaConfig) (*kgo.Client, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...), kgo.ClientID(cfg.ClientID)}
	switch cfg.ProducerAcks {
	case "all":
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	case "leader":
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
	case "none":
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()))
	default:
		return nil, fmt.Errorf("unsupported kafka producer_acks %q", cfg.ProducerAcks)
	}
	if cfg.SASLEnabled {
		opts = append(opts, kgo.SASL(plain.Auth{User: cfg.Username, Pass: cfg.Password}.AsMechanism()))
	}
	return kgo.NewClient(opts...)
}

func Check(ctx context.Context, client *kgo.Client) error { return client.Ping(ctx) }

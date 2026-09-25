package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/wangn-tech/campus-hub/internal/config"
)

// Topic defaults for clusters that have no topic management in place yet.
const (
	topicPartitions        = 3
	topicReplicationFactor = 1
)

// Client wraps the franz-go client so the rest of the application does not
// depend on the Kafka library directly.
type Client struct{ inner *kgo.Client }

// Message is one record to publish.
type Message struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
}

func Open(cfg config.KafkaConfig) (*Client, error) {
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
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{inner: client}, nil
}

func (c *Client) Check(ctx context.Context) error { return c.inner.Ping(ctx) }

func (c *Client) Close() { c.inner.Close() }

// EnsureTopics creates the given topics when they are missing. franz-go does
// not request auto topic creation, so producers depend on the topics already
// existing; creating them here keeps the relay working against a bare broker,
// while production clusters can pre-create the topics from the Kafka design
// document.
func (c *Client) EnsureTopics(ctx context.Context, topics ...string) error {
	if len(topics) == 0 {
		return nil
	}
	request := kmsg.NewPtrCreateTopicsRequest()
	request.TimeoutMillis = int32((10 * time.Second).Milliseconds())
	for _, topic := range topics {
		spec := kmsg.NewCreateTopicsRequestTopic()
		spec.Topic = topic
		spec.NumPartitions = topicPartitions
		spec.ReplicationFactor = topicReplicationFactor
		request.Topics = append(request.Topics, spec)
	}
	response, err := request.RequestWith(ctx, c.inner)
	if err != nil {
		return err
	}
	for _, topic := range response.Topics {
		if topic.ErrorCode == 0 || topic.ErrorCode == kerr.TopicAlreadyExists.Code {
			continue
		}
		return fmt.Errorf("create topic %s: %w", topic.Topic, kerr.ErrorForCode(topic.ErrorCode))
	}
	return nil
}

// Publish sends the messages and returns one error per message, nil when the
// broker acknowledged it. The order matches the input.
func (c *Client) Publish(ctx context.Context, messages []Message) []error {
	errs := make([]error, len(messages))
	if len(messages) == 0 {
		return errs
	}
	records := make([]*kgo.Record, 0, len(messages))
	for _, message := range messages {
		record := &kgo.Record{Topic: message.Topic, Key: []byte(message.Key), Value: message.Value}
		for key, value := range message.Headers {
			record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: []byte(value)})
		}
		records = append(records, record)
	}
	results := c.inner.ProduceSync(ctx, records...)
	for i := range results {
		if i < len(errs) {
			errs[i] = results[i].Err
		}
	}
	return errs
}

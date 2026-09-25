package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Consumer reads records from a consumer group and commits each offset only
// after the handler accepted the record, so a failure is redelivered instead of
// skipped.
type Consumer struct{ client *kgo.Client }

// Record is one consumed record.
type Record struct {
	Topic     string
	Key       string
	Value     []byte
	Headers   map[string]string
	Partition int32
	Offset    int64
}

// NewConsumer builds a consumer for the given group and topics. A new group
// starts at the beginning of the topics so events produced before the consumer
// existed are not lost; handlers therefore have to be idempotent.
func (c *Client) NewConsumer(group string, topics []string) (*Consumer, error) {
	opts := append([]kgo.Opt{}, c.opts...)
	opts = append(opts,
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client}, nil
}

// Run polls records until ctx is cancelled or the handler fails. A handler
// failure stops the loop so the caller can restart it and replay from the last
// committed offset.
func (c *Consumer) Run(ctx context.Context, handle func(context.Context, Record) error) error {
	defer c.client.Close()
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll kafka: %v", errs[0].Err)
		}
		var handleErr error
		fetches.EachRecord(func(record *kgo.Record) {
			if handleErr != nil {
				return
			}
			if err := handle(ctx, consumedRecord(record)); err != nil {
				handleErr = err
				return
			}
			if err := c.client.CommitRecords(ctx, record); err != nil {
				handleErr = fmt.Errorf("commit kafka offset: %w", err)
			}
		})
		if handleErr != nil {
			return handleErr
		}
	}
}

func consumedRecord(record *kgo.Record) Record {
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	return Record{
		Topic:     record.Topic,
		Key:       string(record.Key),
		Value:     record.Value,
		Headers:   headers,
		Partition: record.Partition,
		Offset:    record.Offset,
	}
}

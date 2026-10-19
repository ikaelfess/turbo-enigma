package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
)

const (
	initialRetryBackoff = 100 * time.Millisecond
	maxRetryBackoff     = 5 * time.Second
)

type Handler interface {
	Handle(ctx context.Context, event domain.OutboxEvent) error
}

type Consumer struct {
	client  *kgo.Client
	handler Handler
	logger  zerolog.Logger

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewConsumer returns a group consumer that commits an offset only after
// handler succeeds for that record. The caller must Start and then Stop it.
func NewConsumer(
	brokers []string,
	topic string,
	group string,
	handler Handler,
	logger zerolog.Logger,
) (*Consumer, error) {
	kotelService := kotel.NewKotel(
		kotel.WithTracer(kotel.NewTracer()),
		kotel.WithMeter(kotel.NewMeter()),
	)

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
		kgo.WithHooks(kotelService.Hooks()...),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka client: %w", err)
	}

	return &Consumer{
		client:  client,
		handler: handler,
		logger:  logger,
		done:    make(chan struct{}),
	}, nil
}

func RegisterConsumer(
	lifecycle fx.Lifecycle,
	cfg config.Config,
	handler Handler,
	logger zerolog.Logger,
) error {
	consumer, err := NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.KafkaConsumerGroup,
		handler,
		logger,
	)
	if err != nil {
		return err
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			consumer.Start()
			return nil
		},
		OnStop: consumer.Stop,
	})

	return nil
}

func (c *Consumer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		defer close(c.done)
		c.run(ctx)
	}()
}

// Stop lets the in-flight record finish and commit, then leaves the group.
func (c *Consumer) Stop(ctx context.Context) error {
	var err error
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
			select {
			case <-c.done:
			case <-ctx.Done():
				err = fmt.Errorf("wait for consumer loop: %w", ctx.Err())
			}
		}

		c.client.Close()
	})

	return err
}

func (c *Consumer) run(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			c.logger.Warn().Err(err).Str("topic", topic).Int32("partition", partition).
				Msg("kafka fetch error")
		})

		var processed []*kgo.Record
		fetches.EachRecord(func(record *kgo.Record) {
			if ctx.Err() != nil {
				return
			}

			if c.process(ctx, record) {
				processed = append(processed, record)
			}
		})

		if len(processed) == 0 {
			continue
		}

		// Detached so a stop signal does not drop the offsets of finished records.
		if err := c.client.CommitRecords(context.WithoutCancel(ctx), processed...); err != nil {
			c.logger.Error().Err(err).Msg("commit offsets")
		}
	}
}

// process reports whether the record is done and its offset may be committed.
func (c *Consumer) process(ctx context.Context, record *kgo.Record) bool {
	event, err := DecodeRecord(record)
	if err != nil {
		c.logger.Error().Err(err).
			Str("topic", record.Topic).
			Int32("partition", record.Partition).
			Int64("offset", record.Offset).
			Msg("skip undecodable record")

		return true
	}

	backoff := initialRetryBackoff
	for {
		// The attempt ignores a stop signal so an in-flight write finishes.
		err := c.handler.Handle(context.WithoutCancel(ctx), event)
		if err == nil {
			return true
		}

		c.logger.Error().Err(err).
			Str("event_id", event.ID.String()).
			Dur("retry_in", backoff).
			Msg("handle event, retrying")

		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, maxRetryBackoff)
	}
}

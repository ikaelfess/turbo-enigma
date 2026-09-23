package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

var _ usecase.EventPublisher = (*Producer)(nil)

type Producer struct {
	client *kgo.Client
	topic  string
}

func New(brokers []string, topic string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// The broker still decides. Compose leaves auto-creation off.
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka client: %w", err)
	}

	return &Producer{
		client: client,
		topic:  topic,
	}, nil
}

func (p *Producer) Close() {
	p.client.Close()
}

func NewProducer(lifecycle fx.Lifecycle, cfg config.Config) (*Producer, error) {
	producer, err := New(cfg.KafkaBrokers, cfg.KafkaTopic)
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			producer.Close()
			return nil
		},
	})

	return producer, nil
}

func (p *Producer) Publish(ctx context.Context, event domain.OutboxEvent) error {
	value, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(event.AggregateID.String()),
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(event.ID.String())},
			{Key: "event_type", Value: []byte(string(event.EventType))},
		},
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce outbox event: %w", err)
	}

	return nil
}

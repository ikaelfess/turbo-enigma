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

func NewProducer(lifecycle fx.Lifecycle, cfg config.Config) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka client: %w", err)
	}

	producer := &Producer{
		client: client,
		topic:  cfg.KafkaTopic,
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			client.Close()
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

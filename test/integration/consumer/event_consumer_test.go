package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/go-openapi/testify/v2/require"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/kafka"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
	"github.com/ikaelfess/transactional-outbox/test/integration/helpers"
)

const (
	waitTimeout = 60 * time.Second
	waitTick    = 100 * time.Millisecond
	stopTimeout = 30 * time.Second
)

var errInjectedWrite = errors.New("injected write failure")

func TestEventConsumer(t *testing.T) {
	helpers.WithDatabaseContainer(t, func(t *testing.T, connectionString string) {
		helpers.WithDatabaseTemplateManager(t, connectionString, func(t *testing.T, tm *pgdbtemplate.TemplateManager) {
			helpers.WithKafkaContainer(t, func(t *testing.T, brokers []string) {
				t.Run("record is consumed and stored", func(t *testing.T) {
					t.Parallel()

					helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
						topic := kafkaTopic(t)
						event := newEvent(100)
						publish(t, brokers, topic, event)

						startConsumer(t, brokers, topic, realHandler(db))

						requireStored(t, db, event)
					})
				})

				t.Run("duplicate record is stored once", func(t *testing.T) {
					t.Parallel()

					helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
						topic := kafkaTopic(t)
						first := newEvent(100)
						second := newEvent(200)
						publish(t, brokers, topic, first, first, second)

						startConsumer(t, brokers, topic, realHandler(db))

						requireStored(t, db, second)
						rows, err := helpers.FindAll[postgres.ConsumedEvent](t, db)
						require.NoError(t, err)
						require.Len(t, rows, 2)
					})
				})

				t.Run("undecodable record is skipped", func(t *testing.T) {
					t.Parallel()

					helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
						topic := kafkaTopic(t)
						event := newEvent(100)
						produceRaw(t, brokers, &kgo.Record{
							Topic: topic,
							Key:   []byte("not-a-uuid"),
							Value: []byte("{"),
						})
						publish(t, brokers, topic, event)

						startConsumer(t, brokers, topic, realHandler(db))

						requireStored(t, db, event)
						rows, err := helpers.FindAll[postgres.ConsumedEvent](t, db)
						require.NoError(t, err)
						require.Len(t, rows, 1)
					})
				})

				t.Run("failed write leaves the offset uncommitted", func(t *testing.T) {
					t.Parallel()

					helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
						topic := kafkaTopic(t)
						event := newEvent(100)
						publish(t, brokers, topic, event)

						failing := &flakyHandler{next: realHandler(db)}
						failing.fail.Store(true)
						first := startConsumer(t, brokers, topic, failing)

						require.Eventually(t, func() bool {
							return failing.calls.Load() >= 2
						}, waitTimeout, waitTick, "write retried in place")
						stopConsumer(t, first)

						rows, err := helpers.FindAll[postgres.ConsumedEvent](t, db)
						require.NoError(t, err)
						require.Empty(t, rows)

						startConsumer(t, brokers, topic, realHandler(db))

						requireStored(t, db, event)
					})
				})
			})
		})
	})
}

func realHandler(db *bun.DB) kafka.Handler {
	return usecase.NewConsumedEventUsecase(postgres.NewConsumedEventRepo(db))
}

type flakyHandler struct {
	next  kafka.Handler
	fail  atomic.Bool
	calls atomic.Int32
}

func (h *flakyHandler) Handle(ctx context.Context, event domain.OutboxEvent) error {
	h.calls.Add(1)
	if h.fail.Load() {
		return errInjectedWrite
	}

	return h.next.Handle(ctx, event)
}

func startConsumer(
	t *testing.T,
	brokers []string,
	topic string,
	handler kafka.Handler,
) *kafka.Consumer {
	t.Helper()

	consumer, err := kafka.NewConsumer(brokers, topic, topic+".group", handler, zerolog.Nop())
	require.NoError(t, err, "kafka consumer")
	consumer.Start()
	t.Cleanup(func() { _ = consumer.Stop(context.Background()) })

	return consumer
}

func stopConsumer(t *testing.T, consumer *kafka.Consumer) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), stopTimeout)
	defer cancel()
	require.NoError(t, consumer.Stop(ctx), "consumer stop")
}

func publish(t *testing.T, brokers []string, topic string, events ...domain.OutboxEvent) {
	t.Helper()

	producer, err := kafka.New(brokers, topic)
	require.NoError(t, err, "kafka producer")
	t.Cleanup(producer.Close)

	for _, event := range events {
		require.NoError(t, producer.Publish(t.Context(), event), "publish event")
	}
}

func produceRaw(t *testing.T, brokers []string, record *kgo.Record) {
	t.Helper()

	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.AllowAutoTopicCreation())
	require.NoError(t, err, "kafka client")
	t.Cleanup(client.Close)

	require.NoError(t, client.ProduceSync(t.Context(), record).FirstErr(), "produce raw record")
}

func requireStored(t *testing.T, db *bun.DB, event domain.OutboxEvent) {
	t.Helper()

	var row *postgres.ConsumedEvent
	require.Eventually(t, func() bool {
		found, err := helpers.FindBy[postgres.ConsumedEvent](t, db, "event_id = ?", event.ID)
		if err != nil {
			return false
		}

		row = found
		return true
	}, waitTimeout, waitTick, "event stored")

	require.Equal(t, event.AggregateID, row.AggregateID)
	require.Equal(t, string(event.EventType), row.EventType)
	require.JSONEq(t, mustJSON(t, event.Payload), string(row.Payload))
}

func newEvent(totalCents int64) domain.OutboxEvent {
	aggregateID := uuid.New()

	return domain.OutboxEvent{
		ID:          uuid.New(),
		AggregateID: aggregateID,
		EventType:   domain.OrderCreatedEventType,
		Payload: domain.OutboxEventPayload{
			ID:         aggregateID,
			TotalCents: totalCents,
			Items: []domain.OutboxEventItem{{
				ID:             uuid.New(),
				ItemName:       "Widget",
				Quantity:       1,
				UnitPriceCents: totalCents,
			}},
		},
	}
}

func kafkaTopic(t *testing.T) string {
	t.Helper()

	return strings.NewReplacer("/", ".", " ", "_").Replace(t.Name())
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	out, err := json.Marshal(v)
	require.NoError(t, err)

	return string(out)
}

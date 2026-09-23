package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/go-openapi/testify/v2/require"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/uptrace/bun"

	"github.com/ikaelfess/transactional-outbox/internal/adapters/kafka"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	riveradapter "github.com/ikaelfess/transactional-outbox/internal/adapters/river"
	"github.com/ikaelfess/transactional-outbox/internal/config"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
	"github.com/ikaelfess/transactional-outbox/test/integration/helpers"
)

const (
	consumeTimeout     = 5 * time.Second
	followUpConsume    = 500 * time.Millisecond
	unreachableTimeout = 5 * time.Second
	claimTTL           = 2 * time.Minute
	publishBatchSize   = 10

	bogusBroker = "127.0.0.1:1"
)

var errInjectedPublish = errors.New("injected publish failure")

func TestPublishBatch(t *testing.T) {
	helpers.WithDatabaseContainer(t, func(t *testing.T, connectionString string) {
		helpers.WithDatabaseTemplateManager(t, connectionString, func(t *testing.T, tm *pgdbtemplate.TemplateManager) {
			helpers.WithKafkaContainer(t, func(t *testing.T, brokers []string) {
				t.Run("happy path", func(t *testing.T) {
					t.Parallel()
					testHappyPath(t, tm, brokers)
				})

				t.Run("kafka unreachable", func(t *testing.T) {
					t.Parallel()
					testKafkaUnreachable(t, tm)
				})

				t.Run("partial batch", func(t *testing.T) {
					t.Parallel()
					testPartialBatch(t, tm, brokers)
				})

				t.Run("concurrent claim", func(t *testing.T) {
					t.Parallel()
					testConcurrentClaim(t, tm, brokers)
				})
			})
		})
	})
}

func testHappyPath(t *testing.T, tm *pgdbtemplate.TemplateManager, brokers []string) {
	helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
		events := seedOutboxEvents(t, db, 1)
		topic := kafkaTopic(t)
		worker := newPublisherWorker(t, db, newKafkaPublisher(t, brokers, topic), publishBatchSize)

		require.NoError(t, worker.Work(t.Context(), nil))

		requirePublishedAt(t, db, events[0].ID, true)
		records := consumeRecords(t, brokers, topic, 1)
		assertKafkaRecord(t, records[0], events[0])
	})
}

func testKafkaUnreachable(t *testing.T, tm *pgdbtemplate.TemplateManager) {
	helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
		events := seedOutboxEvents(t, db, 1)
		// This subtest never uses the shared container's brokers.
		worker := newPublisherWorker(
			t,
			db,
			newKafkaPublisher(t, []string{bogusBroker}, kafkaTopic(t)),
			publishBatchSize,
		)

		ctx, cancel := context.WithTimeout(t.Context(), unreachableTimeout)
		defer cancel()

		require.Error(t, worker.Work(ctx, nil))
		requirePublishedAt(t, db, events[0].ID, false)
	})
}

func testPartialBatch(t *testing.T, tm *pgdbtemplate.TemplateManager, brokers []string) {
	helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
		events := seedOutboxEvents(t, db, 2)
		topic := kafkaTopic(t)
		publisher := &failAfterNPublisher{
			publisher: newKafkaPublisher(t, brokers, topic),
			remain:    1,
		}
		worker := newPublisherWorker(t, db, publisher, publishBatchSize)

		require.Error(t, worker.Work(t.Context(), nil))

		requirePublishedAt(t, db, events[0].ID, true)
		requirePublishedAt(t, db, events[1].ID, false)

		records := consumeRecords(t, brokers, topic, 1)
		assertKafkaRecord(t, records[0], events[0])
	})
}

func testConcurrentClaim(t *testing.T, tm *pgdbtemplate.TemplateManager, brokers []string) {
	helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
		events := seedOutboxEvents(t, db, 2)
		topic := kafkaTopic(t)

		workers := make([]*riveradapter.OutboxEventPublisherWorker, 2)
		for i := range workers {
			workers[i] = newPublisherWorker(t, db, newKafkaPublisher(t, brokers, topic), 1)
		}

		start := make(chan struct{})
		errs := make([]error, len(workers))
		var wg sync.WaitGroup
		for i := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = workers[i].Work(t.Context(), nil)
			}()
		}
		close(start)
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		for _, event := range events {
			requirePublishedAt(t, db, event.ID, true)
		}

		records := consumeRecords(t, brokers, topic, len(events))
		byID := recordsByEventID(t, records)
		for _, event := range events {
			record, ok := byID[event.ID.String()]
			require.True(t, ok, "missing kafka record for %s", event.ID)
			assertKafkaRecord(t, record, event)
		}
	})
}

func newKafkaPublisher(t *testing.T, brokers []string, topic string) *kafka.Producer {
	t.Helper()

	producer, err := kafka.New(brokers, topic)
	require.NoError(t, err, "kafka producer")
	t.Cleanup(producer.Close)

	return producer
}

func newPublisherWorker(
	t *testing.T,
	db *bun.DB,
	publisher usecase.EventPublisher,
	batchSize int,
) *riveradapter.OutboxEventPublisherWorker {
	t.Helper()

	repo := postgres.NewPublisherOutboxEventRepo(db, config.Config{
		OutboxClaimTTL: claimTTL,
	})
	eventUsecase := usecase.NewOutboxEventUsecase(repo, publisher, batchSize)

	return riveradapter.NewOutboxEventPublisherWorker(eventUsecase)
}

func seedOutboxEvents(t *testing.T, db *bun.DB, count int) []domain.OutboxEvent {
	t.Helper()

	repo := postgres.NewOutboxEventRepo(db)
	createdAt := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	events := make([]domain.OutboxEvent, 0, count)

	for i := range count {
		aggregateID := uuid.New()
		event := domain.OutboxEvent{
			ID:          uuid.New(),
			AggregateID: aggregateID,
			EventType:   domain.OrderCreatedEventType,
			CreatedAt:   createdAt.Add(time.Duration(i) * time.Second),
			Payload: domain.OutboxEventPayload{
				ID:         aggregateID,
				TotalCents: int64((i + 1) * 100),
				Items: []domain.OutboxEventItem{{
					ID:             uuid.New(),
					ItemName:       "Widget",
					Quantity:       i + 1,
					UnitPriceCents: 100,
				}},
			},
		}

		created, err := repo.Create(t.Context(), event)
		require.NoError(t, err, "seed outbox event")
		require.Equal(t, event.ID, created.ID)
		events = append(events, event)
	}

	return events
}

func requirePublishedAt(t *testing.T, db *bun.DB, id uuid.UUID, published bool) {
	t.Helper()

	row, err := helpers.Find[postgres.OutboxEvent](t, db, id)
	require.NoError(t, err, "find outbox event")
	if published {
		require.NotNil(t, row.PublishedAt)
		return
	}

	require.Nil(t, row.PublishedAt)
}

type consumedRecord struct {
	key     string
	value   []byte
	headers map[string]string
}

func consumeRecords(t *testing.T, brokers []string, topic string, want int) []consumedRecord {
	t.Helper()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err, "kafka consumer")
	t.Cleanup(client.Close)

	deadline := time.Now().Add(consumeTimeout)
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()

	var records []consumedRecord
	for len(records) < want {
		fetches := client.PollFetches(ctx)
		records = append(records, collectRecords(fetches)...)
		if len(records) >= want {
			break
		}
		require.NoError(t, fetches.Err(), "consume %s", topic)
	}

	if remain := time.Until(deadline); remain > 0 {
		followCtx, followCancel := context.WithTimeout(ctx, min(remain, followUpConsume))
		defer followCancel()
		records = append(records, collectRecords(client.PollFetches(followCtx))...)
	}

	require.Len(t, records, want)
	return records
}

func collectRecords(fetches kgo.Fetches) []consumedRecord {
	records := make([]consumedRecord, 0)
	fetches.EachRecord(func(rec *kgo.Record) {
		headers := make(map[string]string, len(rec.Headers))
		for _, header := range rec.Headers {
			headers[header.Key] = string(header.Value)
		}

		records = append(records, consumedRecord{
			key:     string(rec.Key),
			value:   append([]byte(nil), rec.Value...),
			headers: headers,
		})
	})

	return records
}

func recordsByEventID(t *testing.T, records []consumedRecord) map[string]consumedRecord {
	t.Helper()

	byID := make(map[string]consumedRecord, len(records))
	for _, record := range records {
		eventID := record.headers["event_id"]
		_, exists := byID[eventID]
		require.False(t, exists, "duplicate kafka record for %s", eventID)
		byID[eventID] = record
	}

	return byID
}

func assertKafkaRecord(t *testing.T, record consumedRecord, event domain.OutboxEvent) {
	t.Helper()

	require.Equal(t, event.AggregateID.String(), record.key)
	require.Equal(t, event.ID.String(), record.headers["event_id"])
	require.Equal(t, string(event.EventType), record.headers["event_type"])

	var payload domain.OutboxEventPayload
	require.NoError(t, json.Unmarshal(record.value, &payload))
	require.Equal(t, event.Payload, payload)
}

func kafkaTopic(t *testing.T) string {
	t.Helper()

	return strings.NewReplacer("/", ".", " ", "_").Replace(t.Name())
}

type failAfterNPublisher struct {
	publisher usecase.EventPublisher
	remain    int
}

func (p *failAfterNPublisher) Publish(ctx context.Context, event domain.OutboxEvent) error {
	if p.remain == 0 {
		return errInjectedPublish
	}

	p.remain--
	return p.publisher.Publish(ctx, event)
}

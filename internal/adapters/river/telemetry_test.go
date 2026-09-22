package river

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/riverqueue/river/rivertype"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

func TestTelemetry_recordsSuccessfulJob(t *testing.T) {
	telemetry, recorder, logs := newTelemetryTest(t)
	job := publisherJob()

	var innerSpan trace.SpanContext
	err := telemetry.Work(context.Background(), job, func(ctx context.Context) error {
		innerSpan = trace.SpanFromContext(ctx).SpanContext()
		return nil
	})
	require.NoError(t, err)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, "OutboxEventPublisherJobArgs", span.Name())
	require.Equal(t, trace.SpanKindConsumer, span.SpanKind())
	require.Equal(t, codes.Ok, span.Status().Code)
	require.Equal(t, int64(42), attributeInt64(t, span, "river.job.id"))
	require.Equal(t, "OutboxEventPublisherJobArgs", attributeString(t, span, "river.job.kind"))
	require.True(t, innerSpan.IsValid())
	require.Equal(t, span.SpanContext().TraceID(), innerSpan.TraceID())
	require.Equal(t, span.SpanContext().SpanID(), innerSpan.SpanID())

	entry := decodeLog(t, logs)
	require.Equal(t, "info", entry["level"])
	require.Equal(t, "river job", entry["message"])
	require.EqualValues(t, 42, entry["job_id"])
	require.Equal(t, "OutboxEventPublisherJobArgs", entry["job_kind"])
	require.Equal(t, span.SpanContext().TraceID().String(), entry["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), entry["span_id"])
	require.IsType(t, float64(0), entry["duration_ms"])
	_, hasError := entry["error"]
	require.False(t, hasError)
}

func TestTelemetry_recordsReturnedError(t *testing.T) {
	telemetry, recorder, logs := newTelemetryTest(t)
	job := publisherJob()
	publishErr := errors.New("publish failed")

	err := telemetry.Work(context.Background(), job, func(context.Context) error {
		return publishErr
	})
	require.ErrorIs(t, err, publishErr)

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, codes.Error, span.Status().Code)
	require.Equal(t, "publish failed", span.Status().Description)
	require.True(t, hasExceptionEvent(span))

	entry := decodeLog(t, logs)
	require.Equal(t, "info", entry["level"])
	require.Equal(t, "river job", entry["message"])
	require.Equal(t, "publish failed", entry["error"])
	require.Equal(t, span.SpanContext().TraceID().String(), entry["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), entry["span_id"])
}

func TestTelemetry_recordsPanicAndRepanics(t *testing.T) {
	telemetry, recorder, logs := newTelemetryTest(t)
	job := publisherJob()

	require.PanicsWithValue(t, "boom", func() {
		_ = telemetry.Work(context.Background(), job, func(context.Context) error {
			panic("boom")
		})
	})

	spans := recorder.Ended()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, codes.Error, span.Status().Code)
	require.Equal(t, "boom", span.Status().Description)
	require.True(t, hasExceptionEvent(span))

	entry := decodeLog(t, logs)
	require.Equal(t, "error", entry["level"])
	require.Equal(t, "panic recovered", entry["message"])
	require.Equal(t, "boom", entry["error"])
	require.EqualValues(t, 42, entry["job_id"])
	require.Equal(t, "OutboxEventPublisherJobArgs", entry["job_kind"])
	require.Equal(t, span.SpanContext().TraceID().String(), entry["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), entry["span_id"])
}

func newTelemetryTest(t *testing.T) (*Telemetry, *tracetest.SpanRecorder, *bytes.Buffer) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
		otel.SetTracerProvider(nooptrace.NewTracerProvider())
	})

	logs := &bytes.Buffer{}
	return NewTelemetry(zerolog.New(logs)), recorder, logs
}

func publisherJob() *rivertype.JobRow {
	return &rivertype.JobRow{ID: 42, Kind: OutboxEventPublisherJobArgs{}.Kind()}
}

func attributeInt64(t *testing.T, span sdktrace.ReadOnlySpan, key string) int64 {
	t.Helper()

	for _, attr := range span.Attributes() {
		if attr.Key == attribute.Key(key) {
			return attr.Value.AsInt64()
		}
	}

	t.Fatalf("missing attribute %s", key)
	return 0
}

func attributeString(t *testing.T, span sdktrace.ReadOnlySpan, key string) string {
	t.Helper()

	for _, attr := range span.Attributes() {
		if attr.Key == attribute.Key(key) {
			return attr.Value.AsString()
		}
	}

	t.Fatalf("missing attribute %s", key)
	return ""
}

func hasExceptionEvent(span sdktrace.ReadOnlySpan) bool {
	for _, event := range span.Events() {
		if event.Name == "exception" {
			return true
		}
	}

	return false
}

func decodeLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	return entry
}

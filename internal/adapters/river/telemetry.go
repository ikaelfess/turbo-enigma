package river

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/ikaelfess/transactional-outbox/internal/adapters/river"

type Telemetry struct {
	river.MiddlewareDefaults

	logger zerolog.Logger
}

func NewTelemetry(logger zerolog.Logger) *Telemetry {
	return &Telemetry{logger: logger}
}

func (m *Telemetry) Work(
	ctx context.Context,
	job *rivertype.JobRow,
	doInner func(context.Context) error,
) error {
	ctx, span := otel.Tracer(tracerName).Start(
		ctx,
		job.Kind,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.Int64("river.job.id", job.ID),
			attribute.String("river.job.kind", job.Kind),
		),
	)
	start := time.Now()
	defer span.End()
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("%v", recovered)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			m.logPanic(span, job, start, err)
			panic(recovered)
		}
	}()

	err := doInner(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	m.logJob(span, job, start, err)
	return err
}

func (m *Telemetry) logJob(span trace.Span, job *rivertype.JobRow, start time.Time, err error) {
	event := m.jobEvent(zerolog.InfoLevel, span, job, start)
	if err != nil {
		event = event.Err(err)
	}

	event.Msg("river job")
}

func (m *Telemetry) logPanic(span trace.Span, job *rivertype.JobRow, start time.Time, err error) {
	m.jobEvent(zerolog.ErrorLevel, span, job, start).
		Err(err).
		Msg("panic recovered")
}

func (m *Telemetry) jobEvent(
	level zerolog.Level,
	span trace.Span,
	job *rivertype.JobRow,
	start time.Time,
) *zerolog.Event {
	event := m.logger.WithLevel(level).
		Int64("job_id", job.ID).
		Str("job_kind", job.Kind).
		Dur("duration_ms", time.Since(start))

	if sc := span.SpanContext(); sc.IsValid() {
		event = event.
			Str("trace_id", sc.TraceID().String()).
			Str("span_id", sc.SpanID().String())
	}

	return event
}

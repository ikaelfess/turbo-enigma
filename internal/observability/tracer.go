package observability

import (
	"context"
	"errors"
	"os"

	runtimemetric "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/fx"
)

// Telemetry is the process-wide trace and metric setup.
// Constructors that read the global providers depend on it so they run after setup.
type Telemetry struct{}

func NewTelemetry(lifecycle fx.Lifecycle, cfg LoggerConfig) (*Telemetry, error) {
	shutdown, err := setupTelemetry(context.Background(), cfg.ServiceName)
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{OnStop: shutdown})

	return &Telemetry{}, nil
}

func setupTelemetry(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		otel.SetTracerProvider(nooptrace.NewTracerProvider())
		otel.SetMeterProvider(noopmetric.NewMeterProvider())

		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithFromEnv(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) && !errors.Is(err, resource.ErrSchemaURLConflict) {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithTemporalitySelector(sdkmetric.DeltaTemporalitySelector),
		otlpmetrichttp.WithAggregationSelector(exponentialHistogramSelector),
	)
	if err != nil {
		return nil, errors.Join(err, tracerProvider.Shutdown(ctx))
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	if err := runtimemetric.Start(runtimemetric.WithMeterProvider(meterProvider)); err != nil {
		return nil, errors.Join(err, meterProvider.Shutdown(ctx), tracerProvider.Shutdown(ctx))
	}

	return func(stopCtx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(stopCtx), meterProvider.Shutdown(stopCtx))
	}, nil
}

func exponentialHistogramSelector(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	if kind == sdkmetric.InstrumentKindHistogram {
		return sdkmetric.AggregationBase2ExponentialHistogram{
			MaxSize:  160,
			MaxScale: 20,
		}
	}

	return sdkmetric.DefaultAggregationSelector(kind)
}

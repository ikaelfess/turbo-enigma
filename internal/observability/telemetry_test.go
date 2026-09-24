package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestSetupTelemetry_withoutEndpoint_doesNotRecordSpans(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := setupTelemetry(context.Background(), "api")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, shutdown(context.Background()))
	})

	require.False(t, spanRecords(t, "noop"))
}

func TestSetupTelemetry_withEndpoint_recordsSpans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_SERVICE_NAME", "from-env")

	shutdown, err := setupTelemetry(context.Background(), "api")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, shutdown(context.Background()))
	})

	require.True(t, spanRecords(t, "exported"))
}

func spanRecords(t *testing.T, name string) bool {
	t.Helper()

	_, span := otel.Tracer("test").Start(context.Background(), name)
	t.Cleanup(func() { span.End() })

	return span.IsRecording() && span.SpanContext().IsValid()
}

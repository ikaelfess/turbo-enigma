package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type requestIdKey struct{}

func requestIdFromContext(ctx context.Context) string {
	requestId, _ := ctx.Value(requestIdKey{}).(string)
	return requestId
}

func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestId := r.Header.Get("X-Request-ID")

			if requestId == "" {
				requestId = uuid.NewString()
			}

			w.Header().Set("X-Request-ID", requestId)
			ctx := context.WithValue(r.Context(), requestIdKey{}, requestId)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Recover() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					zerolog.Ctx(r.Context()).
						Error().
						Any("error", err).
						Msg("panic recovered")

					http.Error(
						w,
						http.StatusText(http.StatusInternalServerError),
						http.StatusInternalServerError,
					)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

var _ http.ResponseWriter = (*responseRecorder)(nil)

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}

	r.statusCode = statusCode
	r.wroteHeader = true

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.ResponseWriter.Write(body)
}

func Logger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestLogger := logger.With().
				Str("request_id", requestIdFromContext(r.Context())).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Logger()

			if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
				requestLogger = requestLogger.With().
					Str("trace_id", sc.TraceID().String()).
					Str("span_id", sc.SpanID().String()).
					Logger()
			}

			ctx := requestLogger.WithContext(r.Context())
			r = r.WithContext(ctx)

			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			zerolog.Ctx(ctx).
				Info().
				Int("status", recorder.statusCode).
				Dur("duration_ms", time.Duration(time.Since(start))).
				Msg("http request")
		})
	}
}

func Telemetry() func(http.Handler) http.Handler {
	return otelhttp.NewMiddleware("api",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health"
		}),
	)
}

# New Relic OTel signals vs this repo (`setupOTelSDK`)

**Checked:** 2026-09-24  
**Scope:** Map New Relic’s official Go getting-started `setupOTelSDK` (traces + metrics + logs) onto **this** transactional-outbox codebase’s current WIP observability wiring, and say how each signal should be used here.  
**Supersedes (signals only):** the “metrics and logs can stay off” stance in [`new-relic-instead-of-uptrace.md`](./new-relic-instead-of-uptrace.md). That note remains valid for exporter/backend swap; this note answers which signals to adopt from the example. **Out of scope:** implementing code changes.

## Recommendation (short)

**Adopt (traces):** finish what `tracer.go` started — register TraceContext+Baggage propagator, `otel.SetTracerProvider`, resource with `service.name` (and preferably `deployment.environment.name`), env-driven `otlptracehttp` to New Relic (`api-key` + regional OTLP endpoint), and shutdown on `fx.Lifecycle` `OnStop`. Existing `otelhttp` / `bunotel` / `otelriver` already create spans once the global provider is set; usecases do not need manual spans today.

**Optionally adopt (metrics):** only if you want Go runtime metrics in New Relic — add MeterProvider + `otlpmetrichttp` + `runtime.Start`, prefer delta temporality / exponential histograms per New Relic, and prefer the SDK default **60s** export interval (or `OTEL_METRIC_EXPORT_INTERVAL`) over the example’s hard-coded **3s**. This app records **no** custom meters today.

**Skip or defer (logs via example’s LoggerProvider):** keep **zerolog → stdout** as the app log API. `global.SetLoggerProvider` does **not** capture zerolog or River’s slog. Matching the example’s log half means bridging with `otelslog` (River) and/or a zerolog→OTel bridge; there is **no** official contrib `otelzerolog` on `main` as of this check. Prefer NR’s documented alternate of scraping stdout JSON if you want logs in New Relic without changing the logger API.

---

## Confirmed facts

### 0. Repo WIP (read 2026-09-24; do not assume the Uptrace note’s `telemetry.go`)

| Location | Current role |
| --- | --- |
| [`internal/observability/tracer.go`](../../internal/observability/tracer.go) | `otlptracehttp.New(ctx)` + `trace.WithBatcher` only. **No** resource, propagator, `SetTracerProvider`, shutdown, metrics, or logs |
| [`internal/observability/logger.go`](../../internal/observability/logger.go) | `zerolog` → `os.Stdout` with level + `service` field. **Not** OTel Logs |
| [`internal/observability/module.go`](../../internal/observability/module.go) | `fx.Provide(NewLogger)` only — tracer provider **not** wired into fx |
| Deleted `internal/observability/telemetry.go` | Was `uptrace.ConfigureOpentelemetry` with metrics/logs disabled ([prior note](./new-relic-instead-of-uptrace.md)) |
| [`internal/adapters/http/middleware.go`](../../internal/adapters/http/middleware.go) | `otelhttp` middleware; separate zerolog request logger; **manually** adds `trace_id` / `span_id` when span context is valid |
| [`internal/adapters/postgres/postgres.go`](../../internal/adapters/postgres/postgres.go) | `bunotel` + `otel.GetTracerProvider()`; `bunzerolog` for SQL logs |
| [`internal/adapters/river/river.go`](../../internal/adapters/river/river.go) | `otelriver` + `otel.GetTracerProvider()`; River `Logger` is `log/slog` JSON → stdout (separate from app zerolog) |
| [`internal/app/api.go`](../../internal/app/api.go) / [`outbox_event_publisher.go`](../../internal/app/outbox_event_publisher.go) | Service names `"api"` and `"outbox-event-publisher"` via `LoggerConfig.ServiceName` |
| [`internal/usecase/order.go`](../../internal/usecase/order.go), [`outbox_event.go`](../../internal/usecase/outbox_event.go) | No OTel imports; no manual spans or meters |
| [`go.mod`](../../go.mod) | Direct: `otel v1.46.0`, `otel/sdk v1.46.0`, `otlptracehttp v1.46.0`, `otelhttp v0.71.0`. **No** `otlpmetric*`, `otlplog*`, `otel/log`, or `contrib/instrumentation/runtime` |
| [`.env.example`](../../.env.example) | No `OTEL_*` / New Relic vars yet (Uptrace DSN removed in WIP) |

### 1. What the New Relic example actually does

Primary source (fetched 2026-09-24): [`getting-started-guides/go/otel.go`](https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/otel.go).

| Piece | Example behavior |
| --- | --- |
| Propagator | `otel.SetTextMapPropagator(TraceContext + Baggage)` |
| Traces | `otlptracehttp.New(ctx)` (no options) → `trace.NewTracerProvider(WithBatcher)` → `otel.SetTracerProvider` + shutdown |
| Metrics | `otlpmetrichttp.New(ctx)` → `metric.NewMeterProvider` + `PeriodicReader` with **`WithInterval(3*time.Second)`** → `otel.SetMeterProvider` + shutdown |
| Runtime metrics | `runtime.Start(runtime.WithMeterProvider(meterProvider))` |
| Logs | `otlploghttp.New(ctx)` → `log.NewLoggerProvider(WithProcessor(NewBatchProcessor))` → `global.SetLoggerProvider` + shutdown |
| App usage | [`fibonacci.go`](https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/fibonacci.go): `otel.Tracer` / `otel.Meter` / **`otelslog.NewLogger`** (bridge), not zerolog |
| Env (README) | `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS=api-key=…`, compression, `http/protobuf`, **`OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`** — [README](https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/README.md) |
| Example `go.mod` | Older than this repo (`otel v1.44.0`); includes `otelslog`, `runtime`, `otlpmetrichttp`, `otlploghttp`, `otel/log` — [go.mod](https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/go.mod) |

---

### 2. Piece-by-piece: fits this repo?

#### 2.1 TextMapPropagator (TraceContext + Baggage)

| | |
| --- | --- |
| **Example does** | Sets composite TraceContext + Baggage globally |
| **Fits this repo?** | **Yes — adopt.** `otelhttp` extracts/injects via the global propagator; without it, W3C `traceparent` propagation across HTTP (and any future cross-service carriers) will not work. `bunotel` / `otelriver` also rely on ambient context created by upstream instrumentation |
| **If never set** | Go OTel API default is a **no-op** composite propagator (`propagation.NewCompositeTextMapPropagator()` with no children) until `SetTextMapPropagator` — [v1.46.0 `internal/global/propagator.go`](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/internal/global/propagator.go). Spec env default `OTEL_PROPAGATORS=tracecontext,baggage` applies to **SDK autoconfiguration**, not to the bare Go global when you never set a propagator ([spec env vars](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)) |
| **This WIP** | Does **not** call `SetTextMapPropagator` |

#### 2.2 TracerProvider + `otlptracehttp` batcher

| | |
| --- | --- |
| **Example does** | Env-configured HTTP exporter, batcher, **`otel.SetTracerProvider`**, shutdown |
| **Fits this repo?** | **Yes — core path.** WIP already creates the provider shape in [`tracer.go`](../../internal/observability/tracer.go) but does **not** install it globally or shut it down |
| **Still missing vs example** | `otel.SetTracerProvider(tp)`; register `tp.Shutdown` (example’s joined shutdown) |
| **Still missing vs New Relic service entity** | Resource with required **`service.name`** ([NR resources](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-resources/#service)); endpoint + **`api-key`** header (license key) ([NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/)). Example relies on `OTEL_SERVICE_NAME` / `OTEL_RESOURCE_ATTRIBUTES` + exporter env rather than code `WithResource` |
| **SDK default resource** | `NewTracerProvider` without `WithResource` uses `resource.Default()`, which merges default `service.name` (`unknown_service:<exe>`), `OTEL_SERVICE_NAME` / `OTEL_RESOURCE_ATTRIBUTES`, and telemetry SDK attrs — [v1.46.0 provider.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/trace/provider.go), [env.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/resource/env.go). For this repo, prefer aligning `service.name` with `"api"` / `"outbox-event-publisher"` (code and/or `OTEL_SERVICE_NAME`) |
| **Why SetTracerProvider is mandatory here** | Call sites pass **`otel.GetTracerProvider()`** explicitly: [`postgres.go`](../../internal/adapters/postgres/postgres.go), [`river.go`](../../internal/adapters/river/river.go). `otelhttp` uses the global provider as well. A local `*trace.TracerProvider` that is never set globally is invisible to them |

#### 2.3 MeterProvider + `otlpmetrichttp` + `runtime.Start` (3s interval)

| | |
| --- | --- |
| **Example does** | Full metrics pipeline + runtime instrumentation; hard-codes **3s** reader interval |
| **Fits this repo?** | **Optional.** No custom `otel.Meter` / instruments in app code today (grep of `internal/`). Enabling metrics only exports what you register — today that would be **runtime metrics** if you call `runtime.Start` |
| **New Relic accepts OTLP metrics on same endpoint?** | **Yes.** Same base `OTEL_EXPORTER_OTLP_ENDPOINT` (e.g. `https://otlp.nr-data.net`); HTTP exporters append `/v1/metrics` ([NR OTLP endpoint URLs](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-endpoint-port-protocol); [spec](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.50.0/specification/protocol/exporter.md)) |
| **What `runtime.Start` emits** (contrib source) | `go.memory.used`, `go.memory.limit`, `go.memory.allocated`, `go.memory.allocations`, `go.memory.gc.goal`, `go.goroutine.count`, `go.processor.limit`, `go.config.gogc` — plus optional deprecated `runtime.go.*` when `OTEL_GO_X_DEPRECATED_RUNTIME_METRICS=true` — [`instrumentation/runtime/runtime.go` on main](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/instrumentation/runtime/runtime.go) |
| **3s vs defaults** | Example hard-codes 3s. Go SDK `PeriodicReader` default interval is **60s** (`defaultInterval`), overridable by `OTEL_METRIC_EXPORT_INTERVAL` — [v1.46.0 `periodic_reader.go`](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/metric/periodic_reader.go); [spec](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/). New Relic documents `OTEL_METRIC_EXPORT_*` for batching/rate limits but **does not** prescribe 3s ([NR OTLP payload batching](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-payload-size-rate-limits)) |
| **NR metric prefs** | Prefer **delta** temporality (`OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=delta`) and exponential histograms — [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#metric-aggregation-temporality); [metrics mapping](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-metrics/) |
| **Custom meters (if added later)** | `otel.Meter("…")` after `otel.SetMeterProvider`, or meter from the provider passed to `runtime.WithMeterProvider`. Do not invent product metrics in this note |

#### 2.4 LoggerProvider + `otlploghttp` + `global.SetLoggerProvider`

| | |
| --- | --- |
| **Example does** | Sets global OTel Logs provider; app logs via **`otelslog.NewLogger`** which defaults to `global.GetLoggerProvider()` — [otelslog handler](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/bridges/otelslog/v0.20.1/bridges/otelslog/handler.go) |
| **Fits this repo as-is?** | **No automatic attach.** App API is **zerolog** ([`logger.go`](../../internal/observability/logger.go)); River uses **slog → stdout** without `otelslog` |
| **Does `SetLoggerProvider` capture zerolog/slog?** | **No.** It only configures the OTel Logs API global. Package docs: returns no-op until set; bridges must call it — [`go.opentelemetry.io/otel/log/global` @ v0.22.0](https://pkg.go.dev/go.opentelemetry.io/otel/log/global) (`SetLoggerProvider` / `GetLoggerProvider`). Module still exists alongside otel v1.46 (logs API is a separate versioned module; package comments say it will move into `go.opentelemetry.io/otel` when stable) |
| **Official contrib bridges (main, 2026-09-24)** | Present: `otelslog`, `otelzap`, `otellogr`, `otellogrus` ([`versions.yaml` experimental-bridge](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/versions.yaml); `bridges/otelzerolog/go.mod` → **404** on main) |
| **`otelzerolog` status** | CHANGELOG once announced `go.opentelemetry.io/contrib/bridges/otelzerolog` (#5405), but it is **not** in current module sets and **not** on `main`. A historical pseudo-version exists on the module proxy; hook emits body/severity only and **does not** map zerolog fields ([zerolog#493](https://github.com/rs/zerolog/issues/493)). **Not** a currently maintained official path |
| **Third-party** | `github.com/agoda-com/opentelemetry-go/otelzerolog` @ v0.1.0 is real but targets **`github.com/agoda-com/opentelemetry-logs-go`**, not `go.opentelemetry.io/otel/log` — [module proxy / README](https://pkg.go.dev/github.com/agoda-com/opentelemetry-go/otelzerolog). Not an official contrib module |
| **Which option matches “use the example’s logger provider”?** | Wire **`otelslog`** (or another official bridge) so log records go through the OTel Logs SDK → `otlploghttp`. For **zerolog**, that implies a hook/bridge + `SetLoggerProvider`; there is no first-class supported contrib bridge on main today |

#### 2.5 Shutdown func joining errors ↔ fx

| | |
| --- | --- |
| **Example does** | Collects `Shutdown` funcs; on stop / error runs them with `errors.Join` |
| **This repo** | Uses `fx.Lifecycle` `OnStop` for HTTP server, DB, River, Kafka ([`api.go`](../../internal/app/api.go), [`postgres.go`](../../internal/adapters/postgres/postgres.go), [`river.go`](../../internal/adapters/river/river.go), etc.). Observability module currently has **no** OnStop for the tracer |
| **Mapping** | Provide the SDK provider(s) via fx; `OnStop: func(ctx context.Context) error { return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx), …) }` (same join pattern as the example). Order: flush telemetry before process exit; keep within `SHUTDOWN_TIMEOUT` |

---

### 3. How to use each signal **in this code**

#### Traces

- **Already instrumented (once global provider + propagator exist):**
  - HTTP: [`Telemetry()` → `otelhttp.NewMiddleware`](../../internal/adapters/http/middleware.go)
  - Postgres/Bun: [`bunotel.WithTracerProvider(otel.GetTracerProvider())`](../../internal/adapters/postgres/postgres.go)
  - River jobs: [`otelriver` + `otel.GetTracerProvider()`](../../internal/adapters/river/river.go)
- **Manual spans:** usecases ([`order.go`](../../internal/usecase/order.go), [`outbox_event.go`](../../internal/usecase/outbox_event.go)) have **no** tracer usage. Do not invent call sites; HTTP → DB / River worker spans already cover the main flows.
- **Stdout correlation today:** HTTP request logs already copy `trace_id` / `span_id` from span context into zerolog fields ([`middleware.go` Logger](../../internal/adapters/http/middleware.go)). That is **not** OTLP log export.

#### Metrics

- **Today:** none recorded in app code.
- **If enabling the example’s metrics half:** `runtime.Start` is the only instruments the example itself turns on (names above). Custom meters would be `otel.Meter(...)` after `SetMeterProvider` — not prescribed here.
- **For runtime metrics to attribute to a New Relic service entity:** same resource rules as traces (`service.name` required) — [NR resources](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-resources/#service); set delta temporality as above.

#### Logs

Keep **zerolog** as the application logging API unless you deliberately migrate.

| Option | What it means here | Matches example’s LoggerProvider? |
| --- | --- | --- |
| **(a) stdout only** | Keep [`NewLogger`](../../internal/observability/logger.go) and River slog JSON on stdout; **do not** set `LoggerProvider` / `otlploghttp`. Optionally scrape stdout with a Collector ([NR OTLP logs — via file/stdout](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-logs/)) | **No** — skips example’s log pipeline |
| **(b) Bridge into OTel Logs SDK** | Set LoggerProvider + `otlploghttp` like the example; route logs through a bridge. **River:** swap or wrap slog with [`otelslog`](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog) (this is what the NR example does). **App zerolog:** needs a hook that `Emit`s to `otel/log` (no official contrib module on main); historical contrib hook used `e.GetCtx()` so SDK can attach trace/span IDs from context ([sdk/log `logger.go`](https://github.com/open-telemetry/opentelemetry-go/blob/sdk/log/v0.22.0/sdk/log/logger.go) reads `trace.SpanContextFromContext`) | **Yes** — this is “use the example’s logger provider” |

Trace correlation for OTLP logs in New Relic needs `trace.id` / `span.id` on the log record ([NR logs mapping](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-logs/)). Bridges that pass the request/`Context` into `Emit` get that from the SDK; plain zerolog stdout fields (`trace_id` in JSON) help Collector/file scraping parsers, not the OTel Logs API by themselves.

---

### 4. New Relic ingest facts (per signal)

| Topic | Fact | Source |
| --- | --- | --- |
| Endpoint | Same regional base for traces, metrics, logs (US `https://otlp.nr-data.net`, EU/JP/FedRAMP variants); ports 443 / 4317 / 4318 | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/) |
| Paths (HTTP) | With `OTEL_EXPORTER_OTLP_ENDPOINT`, exporters append `v1/traces`, `v1/metrics`, `v1/logs`. Per-signal `*_TRACES_ENDPOINT` / `*_METRICS_ENDPOINT` / `*_LOGS_ENDPOINT` are used **as-is** (must include path) | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-endpoint-port-protocol); [spec exporter.md](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.50.0/specification/protocol/exporter.md) |
| Auth | Header **`api-key`** = ingest **license key** (same for all signals when using shared `OTEL_EXPORTER_OTLP_HEADERS`) | [NR OTLP api-key](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#api-key) |
| Protocol | Recommend **OTLP/HTTP binary protobuf** (`OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`) | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-endpoint-port-protocol) |
| TLS | Required (TLS 1.2+); `https` endpoints | [NR OTLP TLS](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-tls) |
| Compression | `gzip` or `zstd`; example uses `gzip` | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/); [Go README](https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/README.md) |
| Resource | **`service.name` required** for service entity; recommended `service.instance.id`, `telemetry.sdk.language`; prefer OTel `deployment.environment.name` (not listed as automatic NR entity tag) | [NR resources](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-resources/#service); [OTel deployment env](https://opentelemetry.io/docs/specs/semconv/resource/deployment-environment/) |
| Metrics-specific | Prefer delta temporality; exponential histograms recommended | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#metric-aggregation-temporality) |
| Logs-specific | OTLP LogRecords map to NR `Log`; `trace.id`/`span.id` from record fields; complex log attribute types limited | [NR OTLP logs](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-logs/) |
| `log/global` | Still the public API for `SetLoggerProvider` in `go.opentelemetry.io/otel/log/global` (e.g. v0.22.0); experimental until logs API stabilizes | [pkg.go.dev / module zip](https://pkg.go.dev/go.opentelemetry.io/otel/log/global) |

---

### 5. Env vars read by `otlp*http.New(ctx)` with no options

From OpenTelemetry **Protocol Exporter** spec ([v1.50.0](https://github.com/open-telemetry/opentelemetry-specification/blob/v1.50.0/specification/protocol/exporter.md)) and Go exporter `doc.go` at **v1.46.0** (this repo’s otel line).

**Shared / general**

| Variable | Role |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Base URL; HTTP exporters append `/v1/{traces\|metrics\|logs}` |
| `OTEL_EXPORTER_OTLP_HEADERS` | e.g. `api-key=<LICENSE_KEY>` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` (recommended by NR) |
| `OTEL_EXPORTER_OTLP_COMPRESSION` | `gzip` / `none` (spec); NR also documents `zstd` |
| `OTEL_EXPORTER_OTLP_TIMEOUT` | Export timeout |
| `OTEL_EXPORTER_OTLP_INSECURE` | Disable TLS (not for NR SaaS) |
| `OTEL_EXPORTER_OTLP_CERTIFICATE` / `CLIENT_CERTIFICATE` / `CLIENT_KEY` | TLS files |

**Per-signal overrides** (take precedence)

| Traces | Metrics | Logs |
| --- | --- | --- |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` |
| `OTEL_EXPORTER_OTLP_TRACES_HEADERS` | `OTEL_EXPORTER_OTLP_METRICS_HEADERS` | `OTEL_EXPORTER_OTLP_LOGS_HEADERS` |
| `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` | `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL` | `OTEL_EXPORTER_OTLP_LOGS_PROTOCOL` |
| `OTEL_EXPORTER_OTLP_TRACES_COMPRESSION` | `OTEL_EXPORTER_OTLP_METRICS_COMPRESSION` | `OTEL_EXPORTER_OTLP_LOGS_COMPRESSION` |
| `OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` | `OTEL_EXPORTER_OTLP_METRICS_TIMEOUT` | `OTEL_EXPORTER_OTLP_LOGS_TIMEOUT` |
| `OTEL_EXPORTER_OTLP_TRACES_INSECURE` | `OTEL_EXPORTER_OTLP_METRICS_INSECURE` | `OTEL_EXPORTER_OTLP_LOGS_INSECURE` |

**Also relevant (not exporter-only)**

| Variable | Role | Source |
| --- | --- | --- |
| `OTEL_SERVICE_NAME` | Sets `service.name` | [SDK env](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/); NR README |
| `OTEL_RESOURCE_ATTRIBUTES` | Extra resource attrs | same |
| `OTEL_METRIC_EXPORT_INTERVAL` / `OTEL_METRIC_EXPORT_TIMEOUT` | Periodic reader (default 60000 / 30000 ms) | [SDK env](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) |
| `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` | NR recommends `delta` | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#metric-aggregation-temporality); NR Go README |
| `OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION` | NR recommends `base2_exponential_bucket_histogram` | [NR OTLP](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#metric-histogram-aggregation) |
| `OTEL_BSP_*` / `OTEL_BLRP_*` | Span / log batch processors | [SDK env](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/); [NR batching](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/#configure-payload-size-rate-limits) |

Go `otlptracehttp` v1.46.0 documents these in package `doc.go` (defaults shown as `https://localhost:4318` for HTTP in that package’s docs).

---

## Recommendation (derived — not implemented)

1. **Traces — adopt the example’s trace half (+ gaps):** propagator, `SetTracerProvider`, resource/`OTEL_SERVICE_NAME` aligned to `"api"` / `"outbox-event-publisher"`, NR OTLP env, fx `OnStop` shutdown. Cross-link implementation details with [`new-relic-instead-of-uptrace.md`](./new-relic-instead-of-uptrace.md).
2. **Metrics — optional:** add only if runtime (or future custom) metrics are desired; use SDK/default interval or env, **not** the example’s 3s unless you have a reason; set NR delta + histogram env vars.
3. **Logs — prefer (a) for now:** zerolog + River slog stay on stdout; skip LoggerProvider unless you commit to `otelslog` (River) and an explicit zerolog bridge strategy. Do not expect `SetLoggerProvider` alone to ship existing logs to New Relic.

---

## Uncertainties

- Whether New Relic UI treats `deployment.environment.name` as an automatic entity tag (not listed in [NR resources entity-tag table](https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-resources/#service)) — **unconfirmed** (same as prior note).
- Exact APM UI gaps if metrics stay off (runtime pages, etc.) — **unconfirmed** beyond general entity/NRQL guidance.
- Why `bridges/otelzerolog` appears in an older CHANGELOG entry but is absent from `main` / `versions.yaml` (removed vs never released) — **unconfirmed**; only absence on main and module-set list is confirmed.
- Whether any **official** zerolog→`otel/log` bridge will return to contrib — **unconfirmed**.
- Full `otlploghttp` env-var `doc.go` text at a version pinned to this repo — **partially unconfirmed** (repo does not depend on it yet); shared `OTEL_EXPORTER_OTLP_*` / `*_LOGS_*` names are confirmed from the **spec** and NR docs.

---

## Primary URLs fetched / consulted (2026-09-24)

### New Relic

- https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-otlp/
- https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-metrics/
- https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-logs/
- https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-resources/
- https://docs.newrelic.com/docs/opentelemetry/best-practices/opentelemetry-best-practices-traces/
- https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/otel.go
- https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/README.md
- https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/go.mod
- https://github.com/newrelic/newrelic-opentelemetry-examples/blob/main/getting-started-guides/go/fibonacci.go

### OpenTelemetry (spec / docs / Go)

- https://github.com/open-telemetry/opentelemetry-specification/blob/v1.50.0/specification/protocol/exporter.md
- https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/
- https://opentelemetry.io/docs/languages/go/exporters/
- https://opentelemetry.io/docs/specs/semconv/resource/deployment-environment/
- https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/internal/global/propagator.go
- https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/trace/provider.go
- https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/metric/periodic_reader.go
- https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/resource/env.go
- https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/instrumentation/runtime/runtime.go
- https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/versions.yaml
- https://pkg.go.dev/go.opentelemetry.io/otel/log/global
- https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog
- Module proxy: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.46.0` `doc.go`; `…/otlpmetrichttp@v1.46.0` `doc.go`

### This repository (local)

- `internal/observability/{tracer,logger,module}.go`, `internal/adapters/{http/middleware,postgres/postgres,river/river}.go`, `internal/app/{api,outbox_event_publisher}.go`, `internal/usecase/{order,outbox_event}.go`, `go.mod`, `.env.example`
- Related note: [`docs/research/new-relic-instead-of-uptrace.md`](./new-relic-instead-of-uptrace.md)

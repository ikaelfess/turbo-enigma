# Prometheus and Grafana with this repo’s OTLP export

**Checked:** 2026-09-25  
**Scope:** How to send the **existing** `otlptracehttp` + `otlpmetrichttp` pipeline in [`internal/observability/telemetry.go`](../../internal/observability/telemetry.go) to **Prometheus** (metrics) and **Grafana** (dashboards), and where traces go, because Prometheus does not store them. **Out of scope:** changing application code, Compose, or env files.

## Recommendation (short)

Keep the Go services on OTLP/HTTP push. Point `OTEL_EXPORTER_OTLP_ENDPOINT` at an **OpenTelemetry Collector** (`otelcol-contrib`, pinned here to **v0.161.0**) on the Compose network. One base URL is enough: the Go HTTP exporters append `/v1/metrics` and `/v1/traces`.

On the metrics pipeline, use the Collector **Prometheus scrape exporter** (`prometheus`, beta). Its accumulator converts **delta** monotonic sums, explicit histograms, and exponential histograms to cumulative before scrape. Do **not** use `prometheusremotewrite` until delta has been converted: that exporter **drops** non-cumulative monotonic sums, histograms, and summaries. Prometheus scrapes the Collector; it does not scrape the apps.

Send traces from the same Collector OTLP receiver to **Grafana Tempo** (OTLP). Chart metrics with Grafana’s **Prometheus** datasource and traces with the **Tempo** datasource. Grafana documents **trace-to-metrics** as a Tempo datasource feature (link a span to a Prometheus query). This note does not define that query.

Leave the hard-coded delta selector and exponential-histogram selector in place for this path. `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` does **not** override `WithTemporalitySelector` at otel v1.46.0. Change that Go option only if you want cumulative at the source and to skip Collector-side conversion.

Do not add `go.opentelemetry.io/otel/exporters/prometheus`. That package is a pull exporter, cumulative only, and is not a dependency today.

---

## Confirmed facts

### 0. This repo (read 2026-09-25)

| Location | Current role |
| --- | --- |
| [`internal/observability/telemetry.go`](../../internal/observability/telemetry.go) | Propagator: W3C TraceContext + Baggage. Empty `OTEL_EXPORTER_OTLP_ENDPOINT` → noop tracer and meter, no export. Else: `resource.WithFromEnv` plus `service.name` from `LoggerConfig.ServiceName`; `otlptracehttp.New(ctx)` + `sdktrace.WithBatcher`; `otlpmetrichttp.New` with `sdkmetric.DeltaTemporalitySelector` and `AggregationBase2ExponentialHistogram{MaxSize: 160, MaxScale: 20}` for histograms; `sdkmetric.NewPeriodicReader` with no interval option; `runtimemetric.Start`; shutdown joins tracer and meter providers. No OTel logs pipeline |
| [`internal/observability/logger.go`](../../internal/observability/logger.go) | zerolog → stdout. Not OTel Logs |
| [`internal/app/api.go`](../../internal/app/api.go), [`outbox_event_publisher.go`](../../internal/app/outbox_event_publisher.go) | `service.name` `"api"` and `"outbox-event-publisher"` |
| [`internal/adapters/http/middleware.go`](../../internal/adapters/http/middleware.go) | `otelhttp.NewMiddleware` (no `WithMeterProvider`). otelhttp v0.71.0 defaults the meter to `otel.GetMeterProvider()` at middleware build ([`config.go`](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/net/http/otelhttp/v0.71.0/instrumentation/net/http/otelhttp/config.go)) and records `http.server.request.duration`, `http.server.request.body.size`, `http.server.response.body.size` |
| [`internal/adapters/postgres/postgres.go`](../../internal/adapters/postgres/postgres.go) | `bunotel` with `WithTracerProvider` only. `NewQueryHook` still calls `otel.Meter` when no meter option is set and records histogram `go.sql.query_timing` (unit `milliseconds`) ([bunotel v1.2.18](https://github.com/uptrace/bun/blob/v1.2.18/extra/bunotel/otel.go)) |
| [`internal/adapters/river/river.go`](../../internal/adapters/river/river.go) | `otelriver` with `TracerProvider` set and `MeterProvider` left nil, so the middleware uses `otel.GetMeterProvider()` and creates counters, gauges, and histograms (`river_*` plus `messaging.client.*` / `messaging.process.duration`) ([otelriver v0.12.0](https://github.com/riverqueue/rivercontrib/blob/v0.12.0/otelriver/middleware.go)) |
| [`internal/adapters/kafka/producer.go`](../../internal/adapters/kafka/producer.go) | `kotel.NewTracer()` + `kotel.NewMeter()`. `NewMeter` uses `otel.GetMeterProvider()` and records Int64 counters (`messaging.kafka.connects.count`, `…connect_errors.count`, `…disconnects.count`, `…write_errors.count`, `…write_bytes`, `…read_errors.count`, `…read_bytes`, `…produce_bytes`, `…produce_records`, `…fetch_bytes`, `…fetch_records`) |
| [`go.mod`](../../go.mod) | `otel` / `sdk` / `otlpmetrichttp` / `otlptracehttp` **v1.46.0**; `contrib/instrumentation/runtime` and `otelhttp` **v0.71.0**. **No** `go.opentelemetry.io/otel/exporters/prometheus` |
| [`compose.yml`](../../compose.yml) | Passes `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_COMPRESSION`, `OTEL_EXPORTER_OTLP_TIMEOUT`, `OTEL_RESOURCE_ATTRIBUTES` into Go services. Host ports: api **3000**, postgres **5432**, kafka-ui **8080**. Kafka listens on **9092** inside the Compose network and is **not** published to the host. No Prometheus, Grafana, Tempo, Mimir, or Collector service |
| [`.env.example`](../../.env.example) | Commented New Relic endpoint `https://otlp.nr-data.net`, `api-key` header, gzip, 10s timeout, `deployment.environment.name=local`. Export stays off until `OTEL_EXPORTER_OTLP_ENDPOINT` is set |

`setupTelemetry` checks **only** `OTEL_EXPORTER_OTLP_ENDPOINT`. A per-signal URL alone does not start export.

### 1. One OTLP/HTTP endpoint fans out; per-signal URLs do not

| Fact | Source |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` is a base URL. HTTP exporters append `v1/traces`, `v1/metrics`, `v1/logs`. Example: `http://collector:4318` → `http://collector:4318/v1/metrics` and `…/v1/traces` | [OTLP exporter spec](https://opentelemetry.io/docs/specs/otel/protocol/exporter/) |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` and `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` are used **as-is**. If the URL has no path, the path is `/`. They are **not** bases that get `/v1/metrics` appended | Same spec, examples 2 and 3 |
| Go `otlpmetrichttp` v1.46.0 matches that: base endpoint joins `DefaultMetricsPath` (`/v1/metrics`); `METRICS_ENDPOINT` sets `URLPath` to the URL path unchanged (or `/` if empty). `NewHTTPConfig` applies env, then options | [doc.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/doc.go), [envconfig.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf/envconfig.go), [options.go `NewHTTPConfig`](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf/options.go) |
| Go `otlptracehttp` v1.46.0: base URL appends `/v1/traces`; `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is the target URL as given | [otlptracehttp doc.go @ v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlptrace/otlptracehttp/doc.go) |
| Collector `otlpreceiver` (core **v0.161.0**, traces and metrics **stable**) accepts gRPC (default `:4317`) and HTTP (default `:4318`). HTTP paths default to `/v1/traces` and `/v1/metrics` | [otlpreceiver README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.161.0/receiver/otlpreceiver/README.md) |
| Prometheus’s own guide says `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:9090/api/v1/otlp` and claims the spec appends `/v1/metrics` to that per-signal variable | [Using Prometheus as your OpenTelemetry backend](https://prometheus.io/docs/guides/opentelemetry/) |

That Prometheus sentence contradicts the spec and this repo’s Go v1.46.0 exporter. For **this** exporter, the metrics URL that hits Prometheus’s OTLP route is either:

- base `OTEL_EXPORTER_OTLP_ENDPOINT=http://<prometheus>:9090/api/v1/otlp` (appends `/v1/metrics`), which also sends traces to `…/v1/traces` on the same host, or
- `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://<prometheus>:9090/api/v1/otlp/v1/metrics` (full path, used as-is).

Prometheus only serves metrics on `/api/v1/otlp/v1/metrics`. It is not a trace store. A shared base URL aimed at Prometheus therefore mis-routes traces. A Collector base URL does not.

`OTEL_TRACES_EXPORTER=none` in that Prometheus guide is SDK autoconfiguration. This repo calls `otlptracehttp.New` directly and does not read that variable ([trace exporter doc.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlptrace/otlptracehttp/doc.go)).

### 2. Temporality: code wins over the env var

| Fact | Source |
| --- | --- |
| `DeltaTemporalitySelector`: **delta** for Counter, Histogram, and ObservableCounter; **cumulative** for every other kind (including ObservableUpDownCounter and ObservableGauge) | [sdk/metric `reader.go` @ v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/metric/reader.go) |
| Exporter default if no selector is set is cumulative. Env `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` supports `cumulative`, `delta`, `lowmemory` (default `cumulative`). `doc.go` says `WithTemporalitySelector` overrides that env var | [otlpmetrichttp doc.go @ v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/doc.go) |
| Implementation order: `NewHTTPConfig` calls `ApplyHTTPEnvConfigs`, **then** the options passed to `New`. This repo passes `WithTemporalitySelector(DeltaTemporalitySelector)`, so the env var cannot switch this process to cumulative | [options.go `NewHTTPConfig`](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf/options.go) |
| `WithAggregationSelector` overrides `OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION` the same way. The env value `base2_exponential_bucket_histogram` selects `AggregationBase2ExponentialHistogram{MaxSize: 160, MaxScale: 20, NoMinMax: false}` — the same bounds this file hard-codes (`NoMinMax` is the zero value) | [doc.go](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/doc.go), [envconfig.go `withEnvAggPreference`](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf/envconfig.go) |
| `NewPeriodicReader` with no options uses **60s** interval and **30s** timeout, overridable by `OTEL_METRIC_EXPORT_INTERVAL` / `OTEL_METRIC_EXPORT_TIMEOUT` unless `WithInterval` / `WithTimeout` are set. This repo sets neither | [periodic_reader.go @ v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/metric/periodic_reader.go) |
| Compatibility spec: a cumulative monotonic sum becomes a Prometheus counter; a **delta** monotonic sum **may** be converted to cumulative or is otherwise not a counter. A cumulative histogram becomes a Prometheus histogram. A **delta** histogram **may** be aggregated to cumulative, or **must be dropped**. Same rule for exponential histograms | [Prometheus and OpenMetrics compatibility](https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/) |
| Collector `prometheus` exporter (scrape, **beta**, in core and contrib, **v0.161.0**) accumulates delta monotonic sums into cumulative. It **drops** non-monotonic delta sums. It accumulates delta explicit histograms and delta exponential histograms into cumulative, and **drops** a histogram point whose start timestamp is misaligned and not after the previous point | [prometheusexporter `accumulator.go` @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/exporter/prometheusexporter/accumulator.go) |
| Collector `prometheusremotewrite` (**beta**, v0.161.0): “Non-cumulative monotonic, histogram, and summary OTLP metrics are dropped.” | [prometheusremotewriteexporter README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/exporter/prometheusremotewriteexporter/README.md) |
| `deltatocumulative` processor (**alpha**, **contrib and k8s distributions only**, not core `otelcol`): converts all delta samples to cumulative in memory. Defaults: `max_stale` 5m, `max_streams` max int | [deltatocumulativeprocessor README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/processor/deltatocumulativeprocessor/README.md) |
| Prometheus OTLP receiver is **off** until `--web.enable-otlp-receiver`. It serves `POST /api/v1/otlp/v1/metrics`. Without a delta feature flag, delta OTLP is dropped. `--enable-feature=otlp-deltatocumulative` converts delta to cumulative (experimental; in-memory state is lost on restart, which resets counters). `--enable-feature=otlp-native-delta-ingestion` stores raw deltas (experimental, early; mutually exclusive with the conversion flag). `rate()` / `increase()` are for cumulative series and are wrong on native deltas | [OTLP guide](https://prometheus.io/docs/guides/opentelemetry/), [feature flags](https://prometheus.io/docs/prometheus/latest/feature_flags/), [CLI](https://prometheus.io/docs/prometheus/latest/command-line/prometheus/) |

What this implies for **this** process, with the selector left as it is:

| Instrument kind in this repo | Temporality actually exported |
| --- | --- |
| Runtime `Int64ObservableUpDownCounter` (`go.memory.used`, `go.memory.limit`, `go.memory.gc.goal`, `go.goroutine.count`, `go.processor.limit`, `go.config.gogc`) | **Cumulative** |
| Runtime `Int64ObservableCounter` (`go.memory.allocated`, `go.memory.allocations`) | **Delta** |
| Histograms (`go.sql.query_timing`, otelhttp `http.server.request.duration`, otelriver histograms) | **Delta** exponential (selector forces base2 exponential for `InstrumentKindHistogram`) |
| Sync counters (kotel, otelriver counters) | **Delta** |

**Config cannot fix the delta selector.** To export cumulative from the process, delete `WithTemporalitySelector(DeltaTemporalitySelector)` (env default is then cumulative) or pass `CumulativeTemporalitySelector`. To keep the New Relic-oriented delta code and still land in Prometheus, convert in the Collector scrape exporter (or the alpha `deltatocumulative` processor, required before remote write) or turn on Prometheus `--enable-feature=otlp-deltatocumulative`.

### 3. Exponential histograms

| Fact | Source |
| --- | --- |
| This repo forces base2 exponential histograms for every histogram: `MaxSize` 160, `MaxScale` 20 | [`telemetry.go`](../../internal/observability/telemetry.go) |
| OTel → Prometheus: a **cumulative** exponential histogram **must** become a Prometheus native histogram (integer counter flavor). `Scale` **> 8** should be downscaled into schema [-4, 8]; `Scale` **< -4** must be dropped. A **delta** exponential histogram may be converted to cumulative first, or **must be dropped** | [Compatibility spec, Exponential Histograms](https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/) |
| Prometheus text exposition does not carry native histograms; they should be dropped or may be converted to fixed buckets. Native histograms need a protobuf scrape | Same spec, “Differences between Prometheus formats” |
| Collector `prometheus` exporter: “OpenTelemetry exponential histograms are automatically converted to the Prometheus native histogram format.” Scrape them with Prometheus protobuf and a server that accepts native histograms. Exemplars on native histograms are **not** supported by this exporter | [prometheusexporter README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/exporter/prometheusexporter/README.md) |
| `prometheusremotewrite` can emit native histograms on the Remote Write 2.0 message (`io.prometheus.write.v2.Request`). That path is behind `exporter.prometheusremotewritexporter.enableSendingRW2` (the v0.161.0 README spells the gate `remotewrit`, without the second `e`) and the README marks PRW 2.0 support **in development**, not ready. Default message is Remote Write 1.0 `prometheus.WriteRequest` | [prometheusremotewriteexporter README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/exporter/prometheusremotewriteexporter/README.md) |

The v0.161.0 scrape-exporter README does not say what it does when `Scale` is 20. The spec says that point should be downscaled to schema ≤ 8. Whether this Collector build does that downscale is **unconfirmed** (see below).

### 4. Grafana

| Fact | Source |
| --- | --- |
| Prometheus datasource is built into Grafana. It queries PromQL (and any server that implements the Prometheus query API, including Mimir and Thanos). Exemplars are listed as a supported feature of the datasource | [Prometheus data source](https://grafana.com/docs/grafana/latest/datasources/prometheus/) |
| Tempo is Grafana’s trace store. The Tempo datasource searches traces, and documents **trace to metrics**: “Link spans to metrics queries in Prometheus.” Service graph needs a linked Prometheus datasource that already has service-graph metrics | [Tempo data source](https://grafana.com/docs/grafana/latest/datasources/tempo/) |
| Tempo’s distributor receives OTLP. Defaults: gRPC `localhost:4317`, HTTP `localhost:4318`. Bind an address if anything outside the Tempo container must reach it | [Tempo configuration, distributor](https://grafana.com/docs/tempo/latest/configuration/#distributor) |
| First-party image `grafana/otel-lgtm` (repo [grafana/docker-otel-lgtm](https://github.com/grafana/docker-otel-lgtm)) is a **dev/demo/test** bundle: Collector, Prometheus, Tempo, Loki, Pyroscope, Grafana, optional OBI. Apps use `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318` (`http/protobuf`). Grafana UI is port **3000**, user `admin` / `admin`. The README also publishes 4317, 4318, 9090 when port-forwarded. It is not a production stack | [README](https://github.com/grafana/docker-otel-lgtm/blob/main/README.md) |

`https://grafana.com/docs/grafana/latest/datasources/prometheus/configure/exemplars/` returned **404** on this check. Do not assume exemplar click-through from these exponential histograms: the Collector scrape exporter states it does not export exemplars for native histograms.

### 5. Name translation (so a Grafana user can find runtime series)

`runtimemetric.Start` at contrib **v0.71.0** registers only these instruments (scope `go.opentelemetry.io/contrib/instrumentation/runtime`). Kinds and units are the semconv helpers it calls (`semconv/v1.43.0/goconv`):

| OTel name | Kind | Unit | Attribute this `Start` sets |
| --- | --- | --- | --- |
| `go.memory.used` | Int64ObservableUpDownCounter | `By` | `go.memory.type` = `stack` or `other` |
| `go.memory.limit` | Int64ObservableUpDownCounter | `By` | only if a limit exists |
| `go.memory.allocated` | Int64ObservableCounter | `By` | |
| `go.memory.allocations` | Int64ObservableCounter | `{allocation}` | |
| `go.memory.gc.goal` | Int64ObservableUpDownCounter | `By` | |
| `go.goroutine.count` | Int64ObservableUpDownCounter | `{goroutine}` | |
| `go.processor.limit` | Int64ObservableUpDownCounter | `{thread}` | |
| `go.config.gogc` | Int64ObservableUpDownCounter | `%` | |

Sources: [runtime.go @ v0.71.0](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/runtime/v0.71.0/instrumentation/runtime/runtime.go), [goconv metric.go in otel v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/semconv/v1.43.0/goconv/metric.go).

Deprecated `runtime.go.*` / `runtime.uptime` are emitted only when `OTEL_GO_X_DEPRECATED_RUNTIME_METRICS=true` (same `runtime.go`).

Default Collector / Prometheus translation (`UnderscoreEscapingWithSuffixes`): replace `.` with `_`; append the unit word (`By` → `_bytes`, `%` → `_percent`); drop a unit wrapped in `{}`; append `_total` on a monotonic sum (counter). Resource `service.name` (and `service.namespace` if set) becomes the `job` label; `service.instance.id` becomes `instance`. Other resource attributes land on `target_info` unless promoted.

Sources: [translator README @ v0.161.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.161.0/pkg/translator/prometheus/README.md), [compatibility spec, Resource Attributes](https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/), [Prometheus OTLP guide, `target_info`](https://prometheus.io/docs/guides/opentelemetry/).

Under that default strategy the runtime series names are:

| OTel name | Prometheus name |
| --- | --- |
| `go.memory.used` | `go_memory_used_bytes` |
| `go.memory.limit` | `go_memory_limit_bytes` |
| `go.memory.allocated` | `go_memory_allocated_bytes_total` |
| `go.memory.allocations` | `go_memory_allocations_total` |
| `go.memory.gc.goal` | `go_memory_gc_goal_bytes` |
| `go.goroutine.count` | `go_goroutine_count` |
| `go.processor.limit` | `go_processor_limit` |
| `go.config.gogc` | `go_config_gogc_percent` |

`go.memory.type` becomes label `go_memory_type`. `job` is `api` or `outbox-event-publisher` when those `service.name` values are present and `service.namespace` is empty. No PromQL is specified here.

`http.server.request.duration` is a histogram, so the scrape will be a native histogram (and classic `_bucket` / `_count` / `_sum` only if the exporter also emits classic buckets). Exact classic series names depend on `translation_strategy` and whether Prometheus negotiates native histograms. Do not guess them.

### 6. Pull exporter this repo is not using

`go.opentelemetry.io/otel/exporters/prometheus` **v0.68.0** (module that requires `otel` v1.46.0) converts metrics to Prometheus exposition and implements `prometheus.Collector`. `New` builds its own `ManualReader` and the comment states it **only supports cumulative** temporality. It does not speak OTLP.

Source: [doc.go](https://github.com/open-telemetry/opentelemetry-go/blob/exporters/prometheus/v0.68.0/exporters/prometheus/doc.go), [exporter.go `New`](https://github.com/open-telemetry/opentelemetry-go/blob/exporters/prometheus/v0.68.0/exporters/prometheus/exporter.go), [pkg.go.dev v0.68.0](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/prometheus).

The metric SDK `WithReader` **appends** readers. Two readers on one `MeterProvider` are supported: an OTLP `PeriodicReader` and this Prometheus reader can both be registered. Each reader has its own temporality; the Prometheus reader stays cumulative even if the OTLP exporter stays delta.

Source: [sdk/metric `WithReader` @ v1.46.0](https://github.com/open-telemetry/opentelemetry-go/blob/v1.46.0/sdk/metric/config.go).

That design publishes a scrape port on each Go process. This repo does not do that, and the recommendation below does not add it.

---

## What this repo should do

1. Add Compose services; do not open a metrics port on `api` or `outbox_event_publisher`.
2. Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` for the Go services (Compose DNS). Leave the Go exporters option-less so they append `/v1/metrics` and `/v1/traces`. Keep `OTEL_EXPORTER_OTLP_ENDPOINT` non-empty or [`telemetry.go`](../../internal/observability/telemetry.go) installs noops.
3. Run **`otelcol-contrib` v0.161.0** (core `otelcol` has the Prometheus scrape exporter and the OTLP exporter; it does **not** include `deltatocumulative`). Receiver:

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
```

4. Metrics pipeline: exporter `prometheus` with `endpoint: 0.0.0.0:8889` (path `/metrics`). Do not bind 3000, 5432, 8080, or 9092. The accumulator is what accepts this repo’s delta counters and delta exponential histograms. Put `deltatocumulative` in front as well if you would rather depend on that processor’s contract than on the accumulator. It is alpha and stateful.
5. Use `prometheusremotewrite` only **after** `deltatocumulative`. Without that processor the remote-write exporter drops these delta histograms and delta counters. For a local Prometheus, scrape is the path that already converts.
6. Traces pipeline: `otlp` exporter to Tempo’s OTLP receiver (`tempo:4317` gRPC or `http://tempo:4318`). Do not send traces at Prometheus.
7. Prometheus: scrape `otel-collector:8889` with a protobuf protocol that accepts native histograms (the Collector README’s requirement). Do not enable `--web.enable-otlp-receiver` on this path. Publish Prometheus on host **9090** if you want the UI. 9090 is free on the host today; **9092 is Kafka inside the Compose network**, so do not give Prometheus or the Collector that port. A scrape of the Collector uses the exporter’s `job` / `instance` / `target_info` rules. `service.name` becomes `job` (`api`, `outbox-event-publisher`). `deployment.environment.name` from [`.env.example`](../../.env.example) stays on `target_info` unless you promote it with the exporter’s `resource_constant_labels`.
8. Grafana: Prometheus datasource at the Prometheus server, Tempo datasource at Tempo. Use the Tempo datasource’s trace-to-metrics setting when you want a span to open a Prometheus query; copy the query from a real series after the first scrape, do not invent it. Publish Grafana on host **3001** (container port 3000). Host **3000** is the api.
9. Optional single-container demo: `grafana/otel-lgtm` already runs Collector + Prometheus + Tempo + Grafana and accepts `OTEL_EXPORTER_OTLP_ENDPOINT` on 4318. Do not publish its Grafana on host 3000. Treat it as the upstream demo, not as this repo’s Compose layout. The image README does not document delta temporality or `MaxScale: 20`.
10. Direct-to-Prometheus (no Collector) is viable for **metrics only** if you set the full metrics URL (`http://prometheus:9090/api/v1/otlp/v1/metrics`), enable `--web.enable-otlp-receiver` and `--enable-feature=otlp-deltatocumulative`, and point `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` at Tempo’s **full** `/v1/traces` path. The base endpoint must still be set or this process will not export. The Prometheus guide’s per-signal URL without `/v1/metrics` is wrong for Go v1.46.0.

Leave logs on stdout. Nothing in this path ingests zerolog.

---

## Unconfirmed / out of scope

- Whether Collector `prometheus` exporter v0.161.0 downscales exponential histogram `Scale` 20 into Prometheus schema ≤ 8. The spec says it should. The exporter README says “automatically converted” and does not mention scale. Not verified in the conversion function.
- Exact classic-histogram series names (`_bucket`, `_sum`, `_count`) for `http.server.request.duration` and `go.sql.query_timing` after native-histogram conversion. Not specified here.
- Grafana exemplar drill-down from these series. The exemplars doc URL 404’d. The Collector scrape exporter does not support exemplars on native histograms.
- `otel-lgtm` Collector config (does it run `deltatocumulative`? which Prometheus flags?). The README lists the processes and the OTLP port; it does not show the pipeline YAML.
- Prometheus version number on the `latest` docs. Feature flags were read from `prometheus.io/docs/prometheus/latest/feature_flags/` on 2026-09-25 (same text as the 3.14 docs). The flag names are what to pass; the server version that first shipped each flag was not re-derived from the Git history.
- Changing [`telemetry.go`](../../internal/observability/telemetry.go), [`compose.yml`](../../compose.yml), or env files. This note only specifies the target.

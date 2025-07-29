# APM Server Daily Benchmark

The daily benchmark for APM Server is run via [`benchmarks.yml` workflow](https://github.com/elastic/apm-server/actions/workflows/benchmarks.yml).
The benchmark creates an EC deployment and runs [`apmbench`](../systemtest/cmd/apmbench) to send pre-recorded events to the APM Server deployment.
Once all benchmark runs have been completed, the daily benchmark results and metrics are pushed to the observability cluster using Elastic's [`gobench`](https://github.com/elastic/gobench).

## Metrics

The metrics for the benchmark are collected by the [`expvar`](../systemtest/benchtest/expvar) collector.
The collector periodically queries the server's [`/debug/vars` endpoint](https://pkg.go.dev/expvar) and aggregates the metrics.
There are many metrics that are collected, but for the daily benchmark, only a few important ones are shown on the dashboard.

### events/s

This metric refers to the median count of events processed by APM Server over the total duration of the benchmark run.
The active events count is reported by Metricbeat as [`libbeat.output.events.active`](https://www.elastic.co/docs/reference/beats/metricbeat/exported-fields-beat), which is collected periodically by the collector and summed up.

If this metric decreases by a significant margin, it means the new APM Server has a performance regression, and we should investigate why.
Some possible reasons could be addition of new feature or performance regression in Elasticsearch affecting APM Server throughput.

### max rss

This metric refers to the maximum resident set size (RSS), which is the maximum amount of memory the APM Server process is using throughout the benchmark.
The metric is taken from [`beat.memstats.rss`](https://www.elastic.co/docs/reference/beats/metricbeat/exported-fields-beat), reported by Metricbeat.

If this metric increases by a significant margin, it means that new code additions in APM Server is causing higher memory usage.
If that is unexpected, we should investigate and fix it.

### gc cycles

This metric represents the number of garbage collection (GC) cycles that has been completed by the Go runtime.
The metric is taken from the Go [`runtime.Memstats` struct](https://pkg.go.dev/runtime#MemStats) - `NumGC`.

If this metric increases by a significant margin, it means that new code additions in APM Server is causing Go GC to run more frequently.
If this is unexpected, it should be investigated since this could be a result of memory leaks.

### max goroutines

This metric represents the maximum number of goroutines running at once during the benchmark.
The metric is reported by Metricbeat as [`beat.runtime.goroutines`](https://www.elastic.co/docs/reference/beats/metricbeat/exported-fields-beat).

If this metric increases by a significant margin, we should investigate it since it could potentially be a result of goroutine leaks.

### max heap obj

This metric refers to the maximum bytes of heap objects allocated by the Go runtime.
The metric is taken from the Go [`runtime.Memstats` struct](https://pkg.go.dev/runtime#MemStats) - `HeapAlloc`.

If this metric increases by a significant margin, we should investigate it since it could potentially be a result of memory leaks.

### min indexers

This metric represents the minimum mean number of bulk indexers available for making bulk index requests to Elasticsearch across the benchmark.
The metric is derived from  [`output.elasticsearch.bulk_requests.available`](https://www.elastic.co/docs/reference/beats/metricbeat/exported-fields-beat) in Metricbeat.

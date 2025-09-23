---
title: Monitoring
---

## OpenTelemetry

When run as a server and configured accordingly, OPA will emit spans to an
[OpenTelemetry](https://opentelemetry.io/) collector via gRPC.

Each [REST API](./rest-api/) request sent to the server will start a span.
If processing the request involves policy evaluation, and that in turn uses
[`http.send`](./policy-reference/builtins/http), those HTTP clients will emit descendant spans.

Furthermore, spans exported for policy evaluation requests will contain an
attribute `opa.decision_id` of the evaluation's decision ID _if_ the server
has decision logging enabled.

See [the configuration documentation](./configuration/#distributed-tracing)
for all OpenTelemetry-related configurables.

## Prometheus

OPA exposes an HTTP endpoint that can be used to collect performance metrics
for all API calls. The Prometheus endpoint is enabled by default when you run
OPA as a server.

You can enable metric collection from OPA with the following `prometheus.yml` config:

```yaml
global:
  scrape_interval: 15s
scrape_configs:
- job_name: "opa"
  metrics_path: "/metrics"
  static_configs:
  - targets:
    - "localhost:8181"
```

The Prometheus endpoint exports Go runtime metrics as well as HTTP request latency metrics for all handlers (e.g., `v1/data`).

| Metric name                      | Metric type | Description                                                        | Status |
| -------------------------------- | ----------- | ------------------------------------------------------------------ | ------ |
| go_gc_duration_seconds           | summary     | A summary of the GC invocation durations.                          | STABLE |
| go_goroutines                    | gauge       | Number of goroutines that currently exist.                         | STABLE |
| go_info                          | gauge       | Information about the Go environment.                              | STABLE |
| go_memstats_alloc_bytes          | gauge       | Number of bytes allocated and still in use.                        | STABLE |
| go_memstats_alloc_bytes_total    | counter     | Total number of bytes allocated, even if freed.                    | STABLE |
| go_memstats_buck_hash_sys_bytes  | gauge       | Number of bytes used by the profiling bucket hash table.           | STABLE |
| go_memstats_frees_total          | counter     | Total number of frees.                                             | STABLE |
| go_memstats_gc_sys_bytes         | gauge       | Number of bytes used for garbage collection system metadata.       | STABLE |
| go_memstats_heap_alloc_bytes     | gauge       | Number of heap bytes allocated and still in use.                   | STABLE |
| go_memstats_heap_idle_bytes      | gauge       | Number of heap bytes waiting to be used.                           | STABLE |
| go_memstats_heap_inuse_bytes     | gauge       | Number of heap bytes that are in use.                              | STABLE |
| go_memstats_heap_objects         | gauge       | Number of allocated objects.                                       | STABLE |
| go_memstats_heap_released_bytes  | gauge       | Number of heap bytes released to OS.                               | STABLE |
| go_memstats_heap_sys_bytes       | gauge       | Number of heap bytes obtained from system.                         | STABLE |
| go_memstats_last_gc_time_seconds | gauge       | Number of seconds since 1970 of last garbage collection.           | STABLE |
| go_memstats_lookups_total        | counter     | Total number of pointer lookups.                                   | STABLE |
| go_memstats_mallocs_total        | counter     | Total number of mallocs.                                           | STABLE |
| go_memstats_mcache_inuse_bytes   | gauge       | Number of bytes in use by mcache structures.                       | STABLE |
| go_memstats_mcache_sys_bytes     | gauge       | Number of bytes used for mcache structures obtained from system.   | STABLE |
| go_memstats_mspan_inuse_bytes    | gauge       | Number of bytes in use by mspan structures.                        | STABLE |
| go_memstats_mspan_sys_bytes      | gauge       | Number of bytes used for mspan structures obtained from system.    | STABLE |
| go_memstats_next_gc_bytes        | gauge       | Number of heap bytes when next garbage collection will take place. | STABLE |
| go_memstats_other_sys_bytes      | gauge       | Number of bytes used for other system allocations.                 | STABLE |
| go_memstats_stack_inuse_bytes    | gauge       | Number of bytes in use by the stack allocator.                     | STABLE |
| go_memstats_stack_sys_bytes      | gauge       | Number of bytes obtained from system for stack allocator.          | STABLE |
| go_memstats_sys_bytes            | gauge       | Number of bytes obtained from system.                              | STABLE |
| go_threads                       | gauge       | Number of OS threads created.                                      | STABLE |
| http_request_duration_seconds    | histogram   | A histogram of duration for requests.                              | STABLE |

### Status Metrics

When Prometheus is enabled in the status plugin (see [Configuration](./configuration/#status)), the OPA instance's Prometheus endpoint also exposes these metrics:

| Metric name                    | Metric type | Description                                            | Status |
| ------------------------------ | ----------- | ------------------------------------------------------ | ------ |
| opa_info                       | gauge       | Information about the OPA environment.                 | STABLE |
| plugin_status_gauge            | gauge       | Number of plugins by name and status.                  | STABLE |
| bundle_loaded_counter          | counter     | Number of bundles loaded with success.                 | STABLE |
| bundle_failed_load_counter     | counter     | Number of bundles that failed to load.                 | STABLE |
| last_bundle_request            | gauge       | Last bundle request in UNIX nanoseconds.               | STABLE |
| last_success_bundle_activation | gauge       | Last successful bundle activation in UNIX nanoseconds. | STABLE |
| last_success_bundle_download   | gauge       | Last successful bundle download in UNIX nanoseconds.   | STABLE |
| last_success_bundle_request    | gauge       | Last successful bundle request in UNIX nanoseconds.    | STABLE |
| bundle_loading_duration_ns     | histogram   | A histogram of duration for bundle loading.            | STABLE |

## Operational Metrics

OPA exposes operational metrics for system monitoring.

### Server and Request Metrics

- `timer_server_handler_ns` - Total time to handle API request
- `timer_server_read_bytes_ns` - Time spent reading request body


### Bundle Management Metrics

- `timer_bundle_request_ns` - Time spent downloading bundles from remote servers
- `timer_rego_load_bundles_ns` - Time to load and activate bundles

Use these to detect slow bundle servers or compilation issues.

### Cache Performance Metrics

- `counter_server_query_cache_hit` - Query results served from cache
- `counter_rego_builtin_http_send_interquery_cache_hits` - HTTP responses served from inter-query cache
- `counter_rego_builtin_http_send_network_requests` - Actual network requests made
- `counter_rego_builtin_glob_interquery_value_cache_hits` - Glob pattern cache hits
- `counter_rego_builtin_regex_interquery_value_cache_hits` - Regex pattern cache hits

#### Evaluation Cache Metrics
- `counter_eval_op_virtual_cache_hit` - Virtual document cache hits
- `counter_eval_op_virtual_cache_miss` - Virtual document cache misses
- `counter_eval_op_base_cache_hit` - Base document cache hits
- `counter_eval_op_base_cache_miss` - Base document cache misses
- `counter_eval_op_comprehension_cache_hit` - Comprehension cache hits
- `counter_eval_op_comprehension_cache_miss` - Comprehension cache misses
- `counter_eval_op_comprehension_cache_build` - Comprehension cache builds
- `counter_eval_op_comprehension_cache_skip` - Comprehension cache skips

Higher cache hit ratios mean better performance.

### Disk Storage Metrics

- `timer_disk_read_ns` - Time to read from disk storage
- `timer_disk_write_ns` - Time to write to disk storage
- `timer_disk_commit_ns` - Time to commit disk transactions
- `counter_disk_read_keys` - Number of keys read from disk
- `counter_disk_read_bytes` - Bytes read from disk
- `counter_disk_written_keys` - Number of keys written to disk
- `counter_disk_deleted_keys` - Number of keys deleted from disk

### WASM Runtime Metrics

- `timer_wasm_pool_acquire_ns` - Time to acquire WASM instance from pool
- `timer_wasm_pool_release_ns` - Time to release WASM instance to pool
- `timer_wasm_vm_eval_ns` - Total WASM evaluation time
- `timer_wasm_vm_eval_prepare_input_ns` - Time preparing input for WASM
- `timer_wasm_vm_eval_call_ns` - Time calling WASM function
- `timer_wasm_vm_eval_execute_ns` - Time executing WASM code
- `timer_wasm_vm_eval_prepare_result_ns` - Time preparing WASM result

### Accessing Metrics

1. **Prometheus endpoint**: `/metrics` - HTTP GET endpoint that exports all metrics in Prometheus format
   - URL: `http://localhost:8181/metrics` (when OPA runs with default settings)
   - Format: Prometheus text format (compatible with Prometheus scrapers)
   - Includes: All timers, counters, histograms, plus Go runtime metrics
2. **Status API**: Includes subset of metrics in status reports
3. **Decision logs**: Can include request-level metrics
4. **Query API**: Add `?metrics=true` to policy evaluation requests

For a complete list of all available metrics, see [Metrics Registry](./metrics-registry).

For policy evaluation metrics, see [Policy Performance](./policy-performance#performance-metrics).

## Health Checks

OPA exposes a `/health` API endpoint that can be used to perform health checks.
See [Health API](./rest-api#health-api) for details.

## Status API

OPA provides a plugin which can push status to a remote service.
See [Status API](./management-status) for details.

---
title: Monitoring
kind: operations
weight: 30
restrictedtoc: true
---

## Monitoring

### Prometheus

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

| Metric name | Metric type | Description |
| --- | --- | --- |
| go_gc_duration_seconds | summary | A summary of the GC invocation durations. |
| go_goroutines | gauge | Number of goroutines that currently exist. |
| go_info | gauge | Information about the Go environment. |
| go_memstats_alloc_bytes | gauge | Number of bytes allocated and still in use. |
| go_memstats_alloc_bytes_total | counter | Total number of bytes allocated, even if freed. |
| go_memstats_buck_hash_sys_bytes | gauge | Number of bytes used by the profiling bucket hash table. |
| go_memstats_frees_total | counter | Total number of frees. |
| go_memstats_gc_cpu_fraction | gauge | The fraction of this program's available CPU time used by the GC since the program started. |
| go_memstats_gc_sys_bytes | gauge | Number of bytes used for garbage collection system metadata. |
| go_memstats_heap_alloc_bytes | gauge | Number of heap bytes allocated and still in use. |
| go_memstats_heap_idle_bytes | gauge | Number of heap bytes waiting to be used. |
| go_memstats_heap_inuse_bytes | gauge | Number of heap bytes that are in use. |
| go_memstats_heap_objects | gauge | Number of allocated objects. |
| go_memstats_heap_released_bytes | gauge | Number of heap bytes released to OS. |
| go_memstats_heap_sys_bytes | gauge | Number of heap bytes obtained from system. |
| go_memstats_last_gc_time_seconds | gauge | Number of seconds since 1970 of last garbage collection. |
| go_memstats_lookups_total | counter | Total number of pointer lookups. |
| go_memstats_mallocs_total | counter | Total number of mallocs. |
| go_memstats_mcache_inuse_bytes | gauge | Number of bytes in use by mcache structures. |
| go_memstats_mcache_sys_bytes | gauge | Number of bytes used for mcache structures obtained from system. |
| go_memstats_mspan_inuse_bytes | gauge | Number of bytes in use by mspan structures. |
| go_memstats_mspan_sys_bytes | gauge | Number of bytes used for mspan structures obtained from system. |
| go_memstats_next_gc_bytes | gauge | Number of heap bytes when next garbage collection will take place. |
| go_memstats_other_sys_bytes | gauge | Number of bytes used for other system allocations. |
| go_memstats_stack_inuse_bytes | gauge | Number of bytes in use by the stack allocator. |
| go_memstats_stack_sys_bytes | gauge | Number of bytes obtained from system for stack allocator. |
| go_memstats_sys_bytes | gauge | Number of bytes obtained from system. |
| go_threads | gauge | Number of OS threads created. |
| http_request_duration_seconds | histogram | A histogram of duration for requests. |

### Health Checks

OPA exposes a `/health` API endpoint that can be used to perform health checks.
See [Health API](../rest-api#health-api) for details.

### Status API

OPA provides a plugin which can push status to a remote service.
See [Status API](../management#status) for details.

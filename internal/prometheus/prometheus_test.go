// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// NOTE(an): Different go runtime metrics on 1.20.
// This can be removed when we drop support for go 1.19.
//go:build go1.20
// +build go1.20

package prometheus

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
)

func TestJSONSerialization(t *testing.T) {
	inner := metrics.New()
	logger := func(logger logging.Logger) loggerFunc {
		return func(attrs map[string]interface{}, f string, a ...interface{}) {
			logger.WithFields(attrs).Error(f, a...)
		}
	}(logging.NewNoOpLogger())

	prom := New(inner, logger)

	m := prom.All()
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	act := make(map[string]map[string]interface{}, len(m))
	err = json.Unmarshal(bs, &act)
	if err != nil {
		t.Fatal(err)
	}

	// NOTE(sr): "http_request_duration_seconds" only shows up after there has been a request
	exp := map[string][]string{
		"GAUGE": {
			"go_gc_heap_goal_bytes",
			"go_gc_heap_objects_objects",
			"go_gc_stack_starting_size_bytes",
			"go_gc_limiter_last_enabled_gc_cycle",
			"go_goroutines",
			"go_info",
			"go_memory_classes_heap_free_bytes",
			"go_memory_classes_heap_objects_bytes",
			"go_memory_classes_heap_released_bytes",
			"go_memory_classes_heap_stacks_bytes",
			"go_memory_classes_heap_unused_bytes",
			"go_memory_classes_metadata_mcache_free_bytes",
			"go_memory_classes_metadata_mcache_inuse_bytes",
			"go_memory_classes_metadata_mspan_free_bytes",
			"go_memory_classes_metadata_mspan_inuse_bytes",
			"go_memory_classes_metadata_other_bytes",
			"go_memory_classes_os_stacks_bytes",
			"go_memory_classes_other_bytes",
			"go_memory_classes_profiling_buckets_bytes",
			"go_memory_classes_total_bytes",
			"go_memstats_alloc_bytes",
			"go_memstats_buck_hash_sys_bytes",
			// "go_memstats_gc_cpu_fraction", // removed: https://github.com/prometheus/client_golang/issues/842#issuecomment-861812034
			"go_memstats_gc_sys_bytes",
			"go_memstats_heap_alloc_bytes",
			"go_memstats_heap_idle_bytes",
			"go_memstats_heap_inuse_bytes",
			"go_memstats_heap_objects",
			"go_memstats_heap_released_bytes",
			"go_memstats_heap_sys_bytes",
			"go_memstats_last_gc_time_seconds",
			"go_memstats_mcache_inuse_bytes",
			"go_memstats_mcache_sys_bytes",
			"go_memstats_mspan_inuse_bytes",
			"go_memstats_mspan_sys_bytes",
			"go_memstats_next_gc_bytes",
			"go_memstats_other_sys_bytes",
			"go_memstats_stack_inuse_bytes",
			"go_memstats_stack_sys_bytes",
			"go_memstats_sys_bytes",
			"go_sched_goroutines_goroutines",
			"go_sched_gomaxprocs_threads",
			"go_threads",
		},
		"COUNTER": {
			"go_gc_cycles_automatic_gc_cycles_total",
			"go_gc_cycles_forced_gc_cycles_total",
			"go_gc_cycles_total_gc_cycles_total",
			"go_gc_heap_allocs_bytes_total",
			"go_gc_heap_allocs_objects_total",
			"go_gc_heap_tiny_allocs_objects_total",
			"go_gc_heap_frees_bytes_total",
			"go_gc_heap_frees_objects_total",
			"go_cgo_go_to_c_calls_calls_total",
			"go_memstats_alloc_bytes_total",
			"go_memstats_lookups_total",
			"go_memstats_mallocs_total",
			"go_memstats_frees_total",
			"go_cpu_classes_idle_cpu_seconds_total",
			"go_cpu_classes_gc_mark_dedicated_cpu_seconds_total",
			"go_cpu_classes_scavenge_background_cpu_seconds_total",
			"go_cpu_classes_user_cpu_seconds_total",
			"go_cpu_classes_scavenge_assist_cpu_seconds_total",
			"go_cpu_classes_gc_mark_idle_cpu_seconds_total",
			"go_cpu_classes_scavenge_total_cpu_seconds_total",
			"go_cpu_classes_gc_mark_assist_cpu_seconds_total",
			"go_cpu_classes_total_cpu_seconds_total",
			"go_cpu_classes_gc_total_cpu_seconds_total",
			"go_sync_mutex_wait_total_seconds_total",
			"go_cpu_classes_gc_pause_cpu_seconds_total",
		},
		"SUMMARY": {
			"go_gc_duration_seconds",
		},
		"HISTOGRAM": {
			"go_gc_pauses_seconds",            // was: "go_gc_pauses_seconds_total"
			"go_gc_heap_allocs_by_size_bytes", // was: "go_gc_heap_allocs_by_size_bytes_total"
			"go_gc_heap_frees_by_size_bytes",  // was: "go_gc_heap_frees_by_size_bytes_total"
			"go_sched_latencies_seconds",
		},
	}
	found := 0
	for typ, es := range exp {
		for _, e := range es {
			a, ok := act[e]
			if !ok {
				t.Errorf("%v: metric missing", e)
				continue
			}
			if act, ok := a["type"].(string); !ok || act != typ {
				t.Errorf("%v: unexpected type: %v (expected %v)", e, act, typ)
				continue
			}
			found++
		}
	}
	if len(act) != found {
		t.Errorf("unexpected extra metrics, expected %d, got %d", found, len(act))
		for a, ty := range act {
			found := false
			for _, es := range exp {
				for _, e := range es {
					if a == e {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("unexpected metric: %v (type: %v)", a, ty)
			}
		}
	}
}

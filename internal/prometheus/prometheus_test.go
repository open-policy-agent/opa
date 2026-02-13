// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
//
// NOTE: Different go runtime metrics in pretty much
// every Go version. Let's only test these on latest.
//go:build go1.26

package prometheus

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
)

func TestJSONSerialization(t *testing.T) {
	inner := metrics.New()
	logger := func(logger logging.Logger) loggerFunc {
		return func(attrs map[string]any, f string, a ...any) {
			logger.WithFields(attrs).Error(f, a...)
		}
	}(logging.NewNoOpLogger())

	prom := New(inner, logger, []float64{1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1})

	m := prom.All()
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	act := make(map[string]map[string]any, len(m))
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
			"go_gc_gomemlimit_bytes", // BEGIN added in 1.21
			"go_gc_heap_live_bytes",
			"go_gc_gogc_percent",
			"go_gc_scan_globals_bytes",
			"go_gc_scan_heap_bytes",
			"go_gc_scan_stack_bytes",
			"go_gc_scan_total_bytes",
			"go_sched_goroutines_not_in_go_goroutines",
			"go_sched_goroutines_running_goroutines",
			"go_sched_threads_total_threads",
			"go_sched_goroutines_waiting_goroutines",
			"go_sched_goroutines_runnable_goroutines",
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
			"go_godebug_non_default_behavior_execerrdot_events_total", // BEGIN added in 1.21
			"go_godebug_non_default_behavior_gocachehash_events_total",
			"go_godebug_non_default_behavior_gocachetest_events_total",
			"go_godebug_non_default_behavior_gocacheverify_events_total",
			"go_godebug_non_default_behavior_http2client_events_total",
			"go_godebug_non_default_behavior_http2server_events_total",
			"go_godebug_non_default_behavior_installgoroot_events_total",
			// "go_godebug_non_default_behavior_jstmpllitinterp_events_total", // this one was removed in 1.23
			"go_godebug_non_default_behavior_panicnil_events_total",
			"go_godebug_non_default_behavior_randautoseed_events_total",
			"go_godebug_non_default_behavior_tarinsecurepath_events_total",
			"go_godebug_non_default_behavior_multipartmaxheaders_events_total",
			"go_godebug_non_default_behavior_multipartmaxparts_events_total",
			"go_godebug_non_default_behavior_multipathtcp_events_total",
			// "go_godebug_non_default_behavior_x509sha1_events_total", // removed in 1.24
			"go_godebug_non_default_behavior_x509usefallbackroots_events_total",
			"go_godebug_non_default_behavior_zipinsecurepath_events_total",
			"go_godebug_non_default_behavior_tlsmaxrsasize_events_total",
			"go_godebug_non_default_behavior_gotypesalias_events_total", // BEGIN added in 1.22
			"go_godebug_non_default_behavior_tlsunsafeekm_events_total",
			"go_godebug_non_default_behavior_httplaxcontentlength_events_total",
			"go_godebug_non_default_behavior_x509usepolicies_events_total",
			"go_godebug_non_default_behavior_tls10server_events_total",
			"go_godebug_non_default_behavior_httpmuxgo121_events_total",
			"go_godebug_non_default_behavior_tlsrsakex_events_total",
			"go_godebug_non_default_behavior_netedns0_events_total",           // added in 1.22.5
			"go_godebug_non_default_behavior_x509negativeserial_events_total", // added in 1.23.1 (or 1.23)
			"go_godebug_non_default_behavior_winsymlink_events_total",
			"go_godebug_non_default_behavior_x509keypairleaf_events_total",
			"go_godebug_non_default_behavior_winreadlinkvolume_events_total",
			"go_godebug_non_default_behavior_asynctimerchan_events_total",
			"go_godebug_non_default_behavior_httpservecontentkeepheaders_events_total",
			"go_godebug_non_default_behavior_tls3des_events_total",

			"go_godebug_non_default_behavior_randseednop_events_total",
			"go_godebug_non_default_behavior_x509rsacrt_events_total",
			"go_godebug_non_default_behavior_gotestjsonbuildtext_events_total",
			"go_godebug_non_default_behavior_rsa1024min_events_total",
			"go_godebug_non_default_behavior_allowmultiplevcs_events_total", // added in 1.24.6
			"go_godebug_non_default_behavior_embedfollowsymlinks_events_total",
			"go_godebug_non_default_behavior_containermaxprocs_events_total",
			"go_godebug_non_default_behavior_updatemaxprocs_events_total",
			"go_godebug_non_default_behavior_x509sha256skid_events_total",
			"go_godebug_non_default_behavior_tlssha1_events_total",           // here and above, added with 1.25.1
			"go_godebug_non_default_behavior_httpcookiemaxnum_events_total",  // go 1.25.2
			"go_godebug_non_default_behavior_urlmaxqueryparams_events_total", // go 1.25.6
			"go_godebug_non_default_behavior_cryptocustomrand_events_total",  // added in 1.25.7
			"go_godebug_non_default_behavior_urlstrictcolons_events_total",   // added in 1.25.7
			"go_gc_cleanups_queued_cleanups_total",
			"go_gc_cleanups_executed_cleanups_total",
			"go_gc_finalizers_queued_finalizers_total",
			"go_gc_finalizers_executed_finalizers_total",
			"go_sched_goroutines_created_goroutines_total",
		},
		"SUMMARY": {
			"go_gc_duration_seconds",
		},
		"HISTOGRAM": {
			"go_gc_pauses_seconds",            // was: "go_gc_pauses_seconds_total"
			"go_gc_heap_allocs_by_size_bytes", // was: "go_gc_heap_allocs_by_size_bytes_total"
			"go_gc_heap_frees_by_size_bytes",  // was: "go_gc_heap_frees_by_size_bytes_total"
			"go_sched_latencies_seconds",
			"go_sched_pauses_stopping_other_seconds", // BEGIN added in 1.22
			"go_sched_pauses_stopping_gc_seconds",
			"go_sched_pauses_total_gc_seconds",
			"go_sched_pauses_total_other_seconds",
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

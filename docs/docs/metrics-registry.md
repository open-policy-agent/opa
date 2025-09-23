# OPA Metrics Registry
<!-- This file is auto-generated. DO NOT EDIT. -->

Total metrics: **67**

## Summary
- **Timers**: 41 (measure duration in nanoseconds)
- **Counters**: 18 (track counts and accumulations)
- **Histograms**: 8 (track distributions)

## Metrics by Category

### Built-in Functions (6 metrics)

- **`counter_rego_builtin_glob_interquery_value_cache_hits`** - Glob pattern cache hits (count)
- **`timer_rego_builtin_http_send_ns`** - Total time spent in http.send() calls (nanoseconds) (`httpSendLatencyMetricKey`)
- **`counter_rego_builtin_http_send_interquery_cache_hits`** - HTTP responses served from inter-query cache (count)
- **`counter_rego_builtin_http_send_network_requests`** - Actual HTTP network requests made - cache misses (count)
- **`counter_rego_builtin_regex_interquery_value_cache_hits`** - Regex pattern cache hits (count)
- **`timer_rego_external_resolve_ns`** - Time to resolve external data references (nanoseconds) (`RegoExternalResolve`)

### Bundle Management (1 metric)

- **`timer_bundle_request_ns`** - Time to download bundle from remote server (nanoseconds) (`BundleRequest`)

### Caching (12 metrics)

- **`counter_eval_op_base_cache_hit`** - Base document cache hits (count)
- **`counter_eval_op_base_cache_miss`** - Base document cache misses (count)
- **`counter_eval_op_comprehension_cache_build`** - Comprehension cache builds (count)
- **`counter_eval_op_comprehension_cache_hit`** - Comprehension cache hits (count)
- **`counter_eval_op_comprehension_cache_miss`** - Comprehension cache misses (count)
- **`counter_eval_op_comprehension_cache_skip`** - Comprehension cache skips (count)
- **`counter_eval_op_virtual_cache_hit`** - Virtual document cache hits (count)
- **`counter_eval_op_virtual_cache_miss`** - Virtual document cache misses (count)
- **`counter_rego_builtin_glob_interquery_value_cache_hits`** - Glob pattern cache hits (count)
- **`counter_rego_builtin_http_send_interquery_cache_hits`** - HTTP responses served from inter-query cache (count)
- **`counter_rego_builtin_regex_interquery_value_cache_hits`** - Regex pattern cache hits (count)
- **`counter_server_query_cache_hit`** - Number of queries served from server cache (count) (`ServerQueryCacheHit`)

### Data Processing (2 metrics)

- **`timer_rego_data_parse_ns`** - Time to parse JSON/YAML data documents (nanoseconds) (`RegoDataParse`)
- **`timer_rego_input_parse_ns`** - Time to parse input document for query (nanoseconds) (`RegoInputParse`)

### Evaluation Operations (16 metrics)

- **`counter_eval_op_base_cache_hit`** - Base document cache hits (count)
- **`counter_eval_op_base_cache_miss`** - Base document cache misses (count)
- **`timer_eval_op_builtin_call_ns`** - Built-in function call time (nanoseconds)
- **`histogram_eval_op_builtin_call`** - Built-in function call time distribution (percentiles)
- **`counter_eval_op_comprehension_cache_build`** - Comprehension cache builds (count)
- **`counter_eval_op_comprehension_cache_hit`** - Comprehension cache hits (count)
- **`counter_eval_op_comprehension_cache_miss`** - Comprehension cache misses (count)
- **`counter_eval_op_comprehension_cache_skip`** - Comprehension cache skips (count)
- **`histogram_eval_op_plug`** - Plugging operation time distribution (percentiles)
- **`timer_eval_op_plug_ns`** - Plugging operation time (nanoseconds)
- **`timer_eval_op_resolve_ns`** - Reference resolution time (nanoseconds)
- **`histogram_eval_op_resolve`** - Reference resolution time distribution (percentiles)
- **`timer_eval_op_rule_index_ns`** - Rule indexing time (nanoseconds)
- **`histogram_eval_op_rule_index`** - Rule indexing time distribution (percentiles)
- **`counter_eval_op_virtual_cache_hit`** - Virtual document cache hits (count)
- **`counter_eval_op_virtual_cache_miss`** - Virtual document cache misses (count)

### Partial Evaluation (9 metrics)

- **`timer_partial_op_copy_propagation_ns`** - Copy propagation optimization time (nanoseconds)
- **`histogram_partial_op_copy_propagation`** - Copy propagation optimization distribution (percentiles)
- **`timer_partial_op_save_set_contains_ns`** - Set contains save time (nanoseconds)
- **`histogram_partial_op_save_set_contains`** - Set contains save time distribution (percentiles)
- **`timer_partial_op_save_set_contains_rec_ns`** - Recursive set contains save time (nanoseconds)
- **`histogram_partial_op_save_set_contains_rec`** - Recursive set contains save time distribution (percentiles)
- **`timer_partial_op_save_unify_ns`** - Unification save time (nanoseconds)
- **`histogram_partial_op_save_unify`** - Unification save time distribution (percentiles)
- **`timer_rego_partial_eval_ns`** - Time to partially evaluate policy (nanoseconds) (`RegoPartialEval`)

### Policy Compilation (10 metrics)

- **`timer_compile_eval_constraints_ns`** - Constraint evaluation time (nanoseconds) (`CompileEvalConstraints`)
- **`timer_compile_eval_mask_rule_ns`** - Mask rule evaluation time (nanoseconds) (`CompileEvalMaskRule`)
- **`timer_compile_extract_annotations_mask_ns`** - Mask annotation extraction time (nanoseconds) (`CompileExtractAnnotationsMask`)
- **`timer_compile_extract_annotations_unknowns_ns`** - Unknown annotation extraction time (nanoseconds) (`CompileExtractAnnotationsUnknowns`)
- **`timer_compile_prep_partial_ns`** - Partial evaluation preparation time (nanoseconds) (`CompilePrepPartial`)
- **`timer_compile_stage_check_imports_ns`** - Import checking stage time (nanoseconds)
- **`counter_compile_stage_comprehension_index_build`** - Number of comprehension indices built (count)
- **`timer_compile_translate_queries_ns`** - Query translation time (nanoseconds) (`CompileTranslateQueries`)
- **`timer_rego_module_compile_ns`** - Time to compile policy modules into evaluation form (nanoseconds) (`RegoModuleCompile`)
- **`timer_rego_module_parse_ns`** - Time to parse Rego policy modules (nanoseconds) (`RegoModuleParse`)

### Query Processing (3 metrics)

- **`timer_rego_query_compile_ns`** - Time to compile parsed query into evaluation form (nanoseconds) (`RegoQueryCompile`)
- **`timer_rego_query_eval_ns`** - Time to execute compiled query against data (nanoseconds) (`RegoQueryEval`)
- **`timer_rego_query_parse_ns`** - Time to parse query string into AST (nanoseconds) (`RegoQueryParse`)

### Server & SDK (4 metrics)

- **`timer_sdk_decision_eval_ns`** - Time to evaluate decision in SDK mode (nanoseconds) (`SDKDecisionEval`)
- **`timer_server_handler_ns`** - Total time to handle REST API request (nanoseconds) (`ServerHandler`)
- **`counter_server_query_cache_hit`** - Number of queries served from server cache (count) (`ServerQueryCacheHit`)
- **`timer_server_read_bytes_ns`** - Request body read time (nanoseconds)

### Storage & I/O (9 metrics)

- **`timer_disk_commit_ns`** - Disk commit operation time (nanoseconds)
- **`counter_disk_deleted_keys`** - Number of keys deleted from disk (count)
- **`timer_disk_read_ns`** - Disk read operation time (nanoseconds)
- **`counter_disk_read_bytes`** - Total bytes read from disk (bytes)
- **`counter_disk_read_keys`** - Number of keys read from disk (count)
- **`timer_disk_write_ns`** - Disk write operation time (nanoseconds)
- **`counter_disk_written_keys`** - Number of keys written to disk (count)
- **`timer_rego_load_bundles_ns`** - Time to load and activate bundles (nanoseconds) (`RegoLoadBundles`)
- **`timer_rego_load_files_ns`** - Time to load policy/data files from disk (nanoseconds) (`RegoLoadFiles`)

### WASM Runtime (7 metrics)

- **`timer_wasm_pool_acquire_ns`** - WASM instance acquisition time (nanoseconds)
- **`timer_wasm_pool_release_ns`** - WASM instance release time (nanoseconds)
- **`timer_wasm_vm_eval_ns`** - WASM evaluation time (nanoseconds)
- **`timer_wasm_vm_eval_call_ns`** - WASM function call time (nanoseconds)
- **`timer_wasm_vm_eval_execute_ns`** - WASM execution time (nanoseconds)
- **`timer_wasm_vm_eval_prepare_input_ns`** - WASM input preparation time (nanoseconds)
- **`timer_wasm_vm_eval_prepare_result_ns`** - WASM result preparation time (nanoseconds)

## Source Files

Metrics are defined across several files:

- **internal/wasm/sdk/internal/wasm/pool.go** (2 metrics) - WASM pool management
- **internal/wasm/sdk/internal/wasm/vm.go** (5 metrics) - WASM VM execution
- **v1/ast/compile.go** (2 metrics) - Compilation stage metrics
- **v1/metrics/metrics.go** (21 metrics) - Core metrics constants
- **v1/server/server.go** (1 metric) - Server operation metrics
- **v1/storage/disk/txn.go** (7 metrics) - Disk storage metrics
- **v1/topdown/http.go** (3 metrics) - HTTP built-in metrics
- **v1/topdown/instrumentation.go** (24 metrics) - Evaluation operation metrics

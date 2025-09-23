// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"
	"strings"
)

type MetricSource string

const (
	SourceMetrics         MetricSource = "v1/metrics/metrics.go"
	SourceInstrumentation MetricSource = "v1/topdown/instrumentation.go"
	SourceDiskStorage     MetricSource = "v1/storage/disk/txn.go"
	SourceHTTPBuiltin     MetricSource = "v1/topdown/http.go"
	SourceGlobBuiltin     MetricSource = "v1/topdown/glob.go"
	SourceRegexBuiltin    MetricSource = "v1/topdown/regex.go"
	SourceWASMPool        MetricSource = "internal/wasm/sdk/internal/wasm/pool.go"
	SourceWASMVM          MetricSource = "internal/wasm/sdk/internal/wasm/vm.go"
	SourceServer          MetricSource = "v1/server/server.go"
	SourceCompile         MetricSource = "v1/ast/compile.go"
)

type Metric struct {
	Name         string
	Type         string
	Source       MetricSource
	ConstantName string
	Description  string
}

var metricsRegistry = []Metric{
	// Core metrics from v1/metrics/metrics.go
	{Name: "bundle_request", Type: "timer", Source: SourceMetrics, ConstantName: "BundleRequest",
		Description: "Time to download bundle from remote server (nanoseconds)"},
	{Name: "server_handler", Type: "timer", Source: SourceMetrics, ConstantName: "ServerHandler",
		Description: "Total time to handle REST API request (nanoseconds)"},
	{Name: "server_query_cache_hit", Type: "counter", Source: SourceMetrics, ConstantName: "ServerQueryCacheHit",
		Description: "Number of queries served from server cache (count)"},
	{Name: "sdk_decision_eval", Type: "timer", Source: SourceMetrics, ConstantName: "SDKDecisionEval",
		Description: "Time to evaluate decision in SDK mode (nanoseconds)"},

	// Query evaluation metrics
	{Name: "rego_query_compile", Type: "timer", Source: SourceMetrics, ConstantName: "RegoQueryCompile",
		Description: "Time to compile parsed query into evaluation form (nanoseconds)"},
	{Name: "rego_query_eval", Type: "timer", Source: SourceMetrics, ConstantName: "RegoQueryEval",
		Description: "Time to execute compiled query against data (nanoseconds)"},
	{Name: "rego_query_parse", Type: "timer", Source: SourceMetrics, ConstantName: "RegoQueryParse",
		Description: "Time to parse query string into AST (nanoseconds)"},

	// Module and data metrics
	{Name: "rego_module_parse", Type: "timer", Source: SourceMetrics, ConstantName: "RegoModuleParse",
		Description: "Time to parse Rego policy modules (nanoseconds)"},
	{Name: "rego_module_compile", Type: "timer", Source: SourceMetrics, ConstantName: "RegoModuleCompile",
		Description: "Time to compile policy modules into evaluation form (nanoseconds)"},
	{Name: "rego_data_parse", Type: "timer", Source: SourceMetrics, ConstantName: "RegoDataParse",
		Description: "Time to parse JSON/YAML data documents (nanoseconds)"},
	{Name: "rego_input_parse", Type: "timer", Source: SourceMetrics, ConstantName: "RegoInputParse",
		Description: "Time to parse input document for query (nanoseconds)"},
	{Name: "rego_load_files", Type: "timer", Source: SourceMetrics, ConstantName: "RegoLoadFiles",
		Description: "Time to load policy/data files from disk (nanoseconds)"},
	{Name: "rego_load_bundles", Type: "timer", Source: SourceMetrics, ConstantName: "RegoLoadBundles",
		Description: "Time to load and activate bundles (nanoseconds)"},
	{Name: "rego_external_resolve", Type: "timer", Source: SourceMetrics, ConstantName: "RegoExternalResolve",
		Description: "Time to resolve external data references (nanoseconds)"},
	{Name: "rego_partial_eval", Type: "timer", Source: SourceMetrics, ConstantName: "RegoPartialEval",
		Description: "Time to partially evaluate policy (nanoseconds)"},

	// Compilation metrics
	{Name: "compile_prep_partial", Type: "timer", Source: SourceMetrics, ConstantName: "CompilePrepPartial",
		Description: "Partial evaluation preparation time (nanoseconds)"},
	{Name: "compile_eval_constraints", Type: "timer", Source: SourceMetrics, ConstantName: "CompileEvalConstraints",
		Description: "Constraint evaluation time (nanoseconds)"},
	{Name: "compile_translate_queries", Type: "timer", Source: SourceMetrics, ConstantName: "CompileTranslateQueries",
		Description: "Query translation time (nanoseconds)"},
	{Name: "compile_extract_annotations_unknowns", Type: "timer", Source: SourceMetrics, ConstantName: "CompileExtractAnnotationsUnknowns",
		Description: "Unknown annotation extraction time (nanoseconds)"},
	{Name: "compile_extract_annotations_mask", Type: "timer", Source: SourceMetrics, ConstantName: "CompileExtractAnnotationsMask",
		Description: "Mask annotation extraction time (nanoseconds)"},
	{Name: "compile_eval_mask_rule", Type: "timer", Source: SourceMetrics, ConstantName: "CompileEvalMaskRule",
		Description: "Mask rule evaluation time (nanoseconds)"},
	{Name: "compile_stage_check_imports", Type: "timer", Source: SourceCompile,
		Description: "Import checking stage time (nanoseconds)"},
	{Name: "compile_stage_comprehension_index_build", Type: "counter", Source: SourceCompile,
		Description: "Number of comprehension indices built (count)"},

	// HTTP built-in metrics
	{Name: "rego_builtin_http_send", Type: "timer", Source: SourceHTTPBuiltin, ConstantName: "httpSendLatencyMetricKey",
		Description: "Total time spent in http.send() calls (nanoseconds)"},
	{Name: "rego_builtin_http_send_interquery_cache_hits", Type: "counter", Source: SourceHTTPBuiltin,
		Description: "HTTP responses served from inter-query cache (count)"},
	{Name: "rego_builtin_http_send_network_requests", Type: "counter", Source: SourceHTTPBuiltin,
		Description: "Actual HTTP network requests made - cache misses (count)"},

	// Pattern matching built-ins
	{Name: "rego_builtin_glob_interquery_value_cache_hits", Type: "counter", Source: SourceGlobBuiltin,
		Description: "Glob pattern cache hits (count)"},
	{Name: "rego_builtin_regex_interquery_value_cache_hits", Type: "counter", Source: SourceRegexBuiltin,
		Description: "Regex pattern cache hits (count)"},

	// Evaluation operation metrics (timers + histograms)
	{Name: "eval_op_plug", Type: "timer", Source: SourceInstrumentation,
		Description: "Plugging operation time (nanoseconds)"},
	{Name: "eval_op_plug", Type: "histogram", Source: SourceInstrumentation,
		Description: "Plugging operation time distribution (percentiles)"},
	{Name: "eval_op_resolve", Type: "timer", Source: SourceInstrumentation,
		Description: "Reference resolution time (nanoseconds)"},
	{Name: "eval_op_resolve", Type: "histogram", Source: SourceInstrumentation,
		Description: "Reference resolution time distribution (percentiles)"},
	{Name: "eval_op_rule_index", Type: "timer", Source: SourceInstrumentation,
		Description: "Rule indexing time (nanoseconds)"},
	{Name: "eval_op_rule_index", Type: "histogram", Source: SourceInstrumentation,
		Description: "Rule indexing time distribution (percentiles)"},
	{Name: "eval_op_builtin_call", Type: "timer", Source: SourceInstrumentation,
		Description: "Built-in function call time (nanoseconds)"},
	{Name: "eval_op_builtin_call", Type: "histogram", Source: SourceInstrumentation,
		Description: "Built-in function call time distribution (percentiles)"},

	// Cache metrics
	{Name: "eval_op_virtual_cache_hit", Type: "counter", Source: SourceInstrumentation,
		Description: "Virtual document cache hits (count)"},
	{Name: "eval_op_virtual_cache_miss", Type: "counter", Source: SourceInstrumentation,
		Description: "Virtual document cache misses (count)"},
	{Name: "eval_op_base_cache_hit", Type: "counter", Source: SourceInstrumentation,
		Description: "Base document cache hits (count)"},
	{Name: "eval_op_base_cache_miss", Type: "counter", Source: SourceInstrumentation,
		Description: "Base document cache misses (count)"},
	{Name: "eval_op_comprehension_cache_skip", Type: "counter", Source: SourceInstrumentation,
		Description: "Comprehension cache skips (count)"},
	{Name: "eval_op_comprehension_cache_build", Type: "counter", Source: SourceInstrumentation,
		Description: "Comprehension cache builds (count)"},
	{Name: "eval_op_comprehension_cache_hit", Type: "counter", Source: SourceInstrumentation,
		Description: "Comprehension cache hits (count)"},
	{Name: "eval_op_comprehension_cache_miss", Type: "counter", Source: SourceInstrumentation,
		Description: "Comprehension cache misses (count)"},

	// Partial evaluation operations (timers + histograms)
	{Name: "partial_op_save_unify", Type: "timer", Source: SourceInstrumentation,
		Description: "Unification save time (nanoseconds)"},
	{Name: "partial_op_save_unify", Type: "histogram", Source: SourceInstrumentation,
		Description: "Unification save time distribution (percentiles)"},
	{Name: "partial_op_save_set_contains", Type: "timer", Source: SourceInstrumentation,
		Description: "Set contains save time (nanoseconds)"},
	{Name: "partial_op_save_set_contains", Type: "histogram", Source: SourceInstrumentation,
		Description: "Set contains save time distribution (percentiles)"},
	{Name: "partial_op_save_set_contains_rec", Type: "timer", Source: SourceInstrumentation,
		Description: "Recursive set contains save time (nanoseconds)"},
	{Name: "partial_op_save_set_contains_rec", Type: "histogram", Source: SourceInstrumentation,
		Description: "Recursive set contains save time distribution (percentiles)"},
	{Name: "partial_op_copy_propagation", Type: "timer", Source: SourceInstrumentation,
		Description: "Copy propagation optimization time (nanoseconds)"},
	{Name: "partial_op_copy_propagation", Type: "histogram", Source: SourceInstrumentation,
		Description: "Copy propagation optimization distribution (percentiles)"},

	// Disk storage metrics
	{Name: "disk_read", Type: "timer", Source: SourceDiskStorage,
		Description: "Disk read operation time (nanoseconds)"},
	{Name: "disk_write", Type: "timer", Source: SourceDiskStorage,
		Description: "Disk write operation time (nanoseconds)"},
	{Name: "disk_commit", Type: "timer", Source: SourceDiskStorage,
		Description: "Disk commit operation time (nanoseconds)"},
	{Name: "disk_read_bytes", Type: "counter", Source: SourceDiskStorage,
		Description: "Total bytes read from disk (bytes)"},
	{Name: "disk_read_keys", Type: "counter", Source: SourceDiskStorage,
		Description: "Number of keys read from disk (count)"},
	{Name: "disk_written_keys", Type: "counter", Source: SourceDiskStorage,
		Description: "Number of keys written to disk (count)"},
	{Name: "disk_deleted_keys", Type: "counter", Source: SourceDiskStorage,
		Description: "Number of keys deleted from disk (count)"},

	// WASM metrics
	{Name: "wasm_pool_acquire", Type: "timer", Source: SourceWASMPool,
		Description: "WASM instance acquisition time (nanoseconds)"},
	{Name: "wasm_pool_release", Type: "timer", Source: SourceWASMPool,
		Description: "WASM instance release time (nanoseconds)"},
	{Name: "wasm_vm_eval", Type: "timer", Source: SourceWASMVM,
		Description: "WASM evaluation time (nanoseconds)"},
	{Name: "wasm_vm_eval_prepare_input", Type: "timer", Source: SourceWASMVM,
		Description: "WASM input preparation time (nanoseconds)"},
	{Name: "wasm_vm_eval_call", Type: "timer", Source: SourceWASMVM,
		Description: "WASM function call time (nanoseconds)"},
	{Name: "wasm_vm_eval_execute", Type: "timer", Source: SourceWASMVM,
		Description: "WASM execution time (nanoseconds)"},
	{Name: "wasm_vm_eval_prepare_result", Type: "timer", Source: SourceWASMVM,
		Description: "WASM result preparation time (nanoseconds)"},

	// Server metrics
	{Name: "server_read_bytes", Type: "timer", Source: SourceServer,
		Description: "Request body read time (nanoseconds)"},
}

func (m Metric) formatMetricName() string {
	switch m.Type {
	case "timer":
		return fmt.Sprintf("timer_%s_ns", m.Name)
	case "counter":
		return "counter_" + m.Name
	case "histogram":
		return "histogram_" + m.Name
	default:
		return m.Name
	}
}

func groupByType(metrics []Metric) map[string][]Metric {
	groups := make(map[string][]Metric)
	for _, m := range metrics {
		groups[m.Type] = append(groups[m.Type], m)
	}
	return groups
}

func groupBySource(metrics []Metric) map[MetricSource][]Metric {
	groups := make(map[MetricSource][]Metric)
	for _, m := range metrics {
		groups[m.Source] = append(groups[m.Source], m)
	}
	return groups
}

func main() {
	fmt.Println("# OPA Metrics Registry")
	fmt.Println("<!-- This file is auto-generated. DO NOT EDIT. -->")
	fmt.Println()
	fmt.Printf("Total metrics: **%d**\n\n", len(metricsRegistry))

	byType := groupByType(metricsRegistry)

	fmt.Println("## Summary")
	fmt.Printf("- **Timers**: %d (measure duration in nanoseconds)\n", len(byType["timer"]))
	fmt.Printf("- **Counters**: %d (track counts and accumulations)\n", len(byType["counter"]))
	fmt.Printf("- **Histograms**: %d (track distributions)\n\n", len(byType["histogram"]))

	fmt.Println("## Metrics by Category")
	fmt.Println()

	categories := map[string][]string{
		"Query Processing":      {"rego_query_"},
		"Policy Compilation":    {"rego_module_", "compile_"},
		"Evaluation Operations": {"eval_op_"},
		"Partial Evaluation":    {"partial_op_", "rego_partial_eval"},
		"Caching":               {"cache_hit", "cache_miss", "cache_build", "cache_skip", "interquery"},
		"Storage & I/O":         {"disk_", "rego_load_"},
		"Built-in Functions":    {"http_send", "glob_interquery", "regex_interquery", "rego_external_resolve"},
		"WASM Runtime":          {"wasm_"},
		"Bundle Management":     {"bundle_"},
		"Data Processing":       {"rego_data_", "rego_input_"},
		"Server & SDK":          {"server_", "sdk_"},
	}

	categoryNames := make([]string, 0, len(categories))
	for name := range categories {
		categoryNames = append(categoryNames, name)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		patterns := categories[category]
		var categoryMetrics []Metric
		for _, m := range metricsRegistry {
			for _, pattern := range patterns {
				if strings.Contains(m.Name, pattern) {
					categoryMetrics = append(categoryMetrics, m)
					break
				}
			}
		}

		if len(categoryMetrics) > 0 {
			metricWord := "metrics"
			if len(categoryMetrics) == 1 {
				metricWord = "metric"
			}
			fmt.Printf("### %s (%d %s)\n\n", category, len(categoryMetrics), metricWord)

			sort.Slice(categoryMetrics, func(i, j int) bool {
				return categoryMetrics[i].Name < categoryMetrics[j].Name
			})

			for _, m := range categoryMetrics {
				fmt.Printf("- **`%s`** - %s", m.formatMetricName(), m.Description)
				if m.ConstantName != "" {
					fmt.Printf(" (`%s`)", m.ConstantName)
				}
				fmt.Printf("\n")
			}
			fmt.Println()
		}
	}

	fmt.Println("## Source Files")
	fmt.Println()
	fmt.Println("Metrics are defined across several files:")
	fmt.Println()

	sourceDescriptions := map[MetricSource]string{
		SourceMetrics:         "Core metrics constants",
		SourceInstrumentation: "Evaluation operation metrics",
		SourceDiskStorage:     "Disk storage metrics",
		SourceHTTPBuiltin:     "HTTP built-in metrics",
		SourceWASMPool:        "WASM pool management",
		SourceWASMVM:          "WASM VM execution",
		SourceCompile:         "Compilation stage metrics",
		SourceServer:          "Server operation metrics",
	}

	bySource := groupBySource(metricsRegistry)
	sourceFiles := make([]MetricSource, 0, len(sourceDescriptions))
	for source := range sourceDescriptions {
		sourceFiles = append(sourceFiles, source)
	}
	sort.Slice(sourceFiles, func(i, j int) bool {
		return string(sourceFiles[i]) < string(sourceFiles[j])
	})

	for _, source := range sourceFiles {
		desc := sourceDescriptions[source]
		if metrics, ok := bySource[source]; ok {
			metricWord := "metrics"
			if len(metrics) == 1 {
				metricWord = "metric"
			}
			fmt.Printf("- **%s** (%d %s) - %s\n", source, len(metrics), metricWord, desc)
		}
	}
}

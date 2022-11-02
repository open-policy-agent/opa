package metrics

import (
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestMetricsEndpoint(t *testing.T) {

	policy := `
	package test
	p = true
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	dr := struct {
		Result bool `json:"result"`
	}{}

	if err := testRuntime.GetDataWithInputTyped("test/p", nil, &dr); err != nil {
		t.Fatal(err)
	}

	if !dr.Result {
		t.Fatalf("Unexpected response: %+v", dr)
	}

	mr, err := http.Get(testRuntime.URL() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}

	defer mr.Body.Close()

	bs, err := io.ReadAll(mr.Body)
	if err != nil {
		t.Fatal(err)
	}

	str := string(bs)

	expected := []string{
		`http_request_duration_seconds_count{code="200",handler="v1/policies",method="put"} 1`,
		`http_request_duration_seconds_count{code="200",handler="v1/data",method="post"} 1`,
	}

	for _, exp := range expected {
		if !strings.Contains(str, exp) {
			t.Fatalf("Expected to find %q but got:\n\n%v", exp, str)
		}
	}
}

type response struct {
	Result  bool                   `json:"result"`
	Metrics map[string]interface{} `json:"metrics"`
}

func TestRequestWithInstrumentationV1DataAPI(t *testing.T) {

	policy := `
	package test
	p = true
	q = true
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	var resp response
	if err := testRuntime.GetDataWithInputTyped("test/p?instrument", nil, &resp); err != nil {
		t.Fatal(err)
	}

	if !resp.Result {
		t.Fatalf("Unexpected response: %+v", resp)
	}

	assertDataInstrumentationMetricsInMap(t, true, resp.Metrics)

	// run another request, this should re-use the compiled query
	var resp2 response
	if err := testRuntime.GetDataWithInputTyped("test/p?instrument", nil, &resp2); err != nil {
		t.Fatal(err)
	}

	if !resp2.Result {
		t.Fatalf("Unexpected response: %+v", resp2)
	}

	assertDataInstrumentationMetricsInMap(t, false, resp2.Metrics)

	// GET data endpoint
	var resp3 response
	if err := testRuntime.GetDataWithInputTyped("test/q?instrument", nil, &resp3); err != nil {
		t.Fatal(err)
	}

	if !resp.Result {
		t.Fatalf("Unexpected response: %+v", resp3)
	}

	assertDataInstrumentationMetricsInMap(t, true, resp3.Metrics)

	// 2nd GET data endpoint
	var resp4 response
	if err := testRuntime.GetDataWithInputTyped("test/q?instrument", nil, &resp4); err != nil {
		t.Fatal(err)
	}

	if !resp.Result {
		t.Fatalf("Unexpected response: %+v", resp4)
	}

	assertDataInstrumentationMetricsInMap(t, false, resp4.Metrics)
}

func TestRequestWithInstrumentationV1CompileAPI(t *testing.T) {

	policy := `
	package test
	p {input.x >= data.y}
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	var i interface{} = "{\"x\": 4}"
	req := types.CompileRequestV1{
		Query:    "data.test.p == true",
		Input:    &i,
		Unknowns: &[]string{"data.y"},
	}

	resp, err := testRuntime.CompileRequestWithInstrumentation(req)
	if err != nil {
		t.Fatal(err)
	}

	assertCompileInstrumentationMetricsInMap(t, true, resp.Metrics)
}

func assertCompileInstrumentationMetricsInMap(t *testing.T, includeCompile bool, metrics map[string]interface{}) {
	expectedKeys := []string{
		"histogram_eval_op_plug",
		"timer_eval_op_plug_ns",
		"timer_server_handler_ns",

		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_query_compile_stage_build_comprehension_index_ns",
		"timer_query_compile_stage_check_safety_ns",
		"timer_query_compile_stage_check_types_ns",
		"timer_query_compile_stage_check_undefined_funcs_ns",
		"timer_query_compile_stage_check_unsafe_builtins_ns",
		"timer_query_compile_stage_resolve_refs_ns",
		"timer_query_compile_stage_rewrite_comprehension_terms_ns",
		"timer_query_compile_stage_rewrite_dynamic_terms_ns",
		"timer_query_compile_stage_rewrite_expr_terms_ns",
		"timer_query_compile_stage_rewrite_local_vars_ns",
		"timer_query_compile_stage_rewrite_with_values_ns",
	}
	for _, key := range expectedKeys {
		if metrics[key] == nil {
			t.Errorf("Expected to find key %q in metrics response", key)
		}
	}
	if t.Failed() {
		t.Logf("metrics response: %v\n", metrics)
	}
}

func assertDataInstrumentationMetricsInMap(t *testing.T, includeCompile bool, metrics map[string]interface{}) {
	expectedKeys := []string{
		"counter_server_query_cache_hit",
		"counter_eval_op_virtual_cache_miss",
		"histogram_eval_op_plug",
		"timer_eval_op_plug_ns",
		"timer_rego_input_parse_ns",
		"timer_rego_query_eval_ns",
		"timer_server_handler_ns",
	}
	compileStageKeys := []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_query_compile_stage_build_comprehension_index_ns",
		"timer_query_compile_stage_check_safety_ns",
		"timer_query_compile_stage_check_types_ns",
		"timer_query_compile_stage_check_undefined_funcs_ns",
		"timer_query_compile_stage_check_unsafe_builtins_ns",
		"timer_query_compile_stage_resolve_refs_ns",
		"timer_query_compile_stage_rewrite_comprehension_terms_ns",
		"timer_query_compile_stage_rewrite_dynamic_terms_ns",
		"timer_query_compile_stage_rewrite_expr_terms_ns",
		"timer_query_compile_stage_rewrite_local_vars_ns",
		"timer_query_compile_stage_rewrite_to_capture_value_ns",
		"timer_query_compile_stage_rewrite_with_values_ns",
	}

	if includeCompile {
		expectedKeys = append(expectedKeys, compileStageKeys...)
	}

	for _, key := range expectedKeys {
		if metrics[key] == nil {
			t.Errorf("Expected to find key %q in metrics response", key)
		}
	}
	if !includeCompile {
		for _, key := range compileStageKeys {
			if metrics[key] != nil {
				t.Errorf("Expected NOT to find key %q in metrics response", key)
			}
		}
	}
	if t.Failed() {
		t.Logf("metrics response: %v\n", metrics)
	}
}

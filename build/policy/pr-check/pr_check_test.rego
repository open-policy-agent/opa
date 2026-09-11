package policy["pr-check_test"]

import data.policy["pr-check"] as pr_check

example_docs_changelist := [
	{"filename": "docs/docs/policy-reference/builtins/glob.mdx"},
	{"filename": "docs/docs/policy-reference/builtins/http.mdx"},
	{"filename": "docs/docs/policy-reference/builtins/regex.mdx"},
]

example_go_changelist := [
	{"filename": "cmd/build.go"},
	{"filename": "build/gen-run-go.sh"},
	{"filename": "v1/rego/testdata/ast.json"},
]

example_wasm_changelist := [
	{"filename": "wasm/src/regex.cc"},
	{"filename": "ast/errors.go"},
	{"filename": "v1/test/wasm/assets/test.js"},
]

example_docs_exception_changelist := [
	{"filename": "docs/bin/build-latest.sh"},
	{"filename": "docs/docs/envoy/_category_.yaml"},
]

mixed_bag_changelist := [
	{"filename": "wasm/Makefile"},
	{"filename": "v1/rego/testdata/ast.json"},
]

example_rego_changelist := [{"filename": "build/policy/pr-check/pr_check.rego"}]

example_all_checks_root_changelist := [{"filename": "Makefile"}]

example_docs_root_changelist := [{"filename": "netlify.toml"}]

example_docs_and_go_root_changelist := [
	{"filename": "builtin_metadata.json"},
	{"filename": "capabilities.json"},
]

example_gh_actions_changelist := [
	{"filename": "wasm/Makefile"},
	{"filename": "v1/rego/testdata/ast.json"},
	{"filename": ".github/workflows/pull-request.yaml"},
]

example_bench_ast_changelist := [{"filename": "v1/ast/parser.go"}]

example_bench_topdown_changelist := [{"filename": "v1/topdown/eval.go"}]

example_bench_rego_changelist := [{"filename": "v1/rego/rego.go"}]

example_bench_no_match_changelist := [{"filename": "cmd/build.go"}]

example_proto_changelist := [{"filename": "v1/ir/plan.proto"}]

example_buf_yaml_changelist := [{"filename": "buf.yaml"}]

example_workflow_only_changelist := [{"filename": ".github/workflows/pull-request.yaml"}]

test_run_docs_check_expect if {
	pr_check.changes.docs with input as example_docs_changelist
}

test_run_docs_checks_root_expect if {
	pr_check.changes.docs with input as example_docs_root_changelist
}

test_run_go_tests_expect if {
	pr_check.changes.go with input as example_go_changelist
}

test_run_wasm_tests_expect if {
	pr_check.changes.wasm with input as example_wasm_changelist
}

test_run_rego_tests_expect if {
	pr_check.changes.rego with input as example_rego_changelist
}

test_run_yaml_tests_expect if {
	pr_check.changes.yaml with input as example_gh_actions_changelist
}

test_run_docs_not_go_tests_expect if {
	pr_check.changes.docs with input as example_docs_exception_changelist
	not pr_check.changes.go with input as example_docs_exception_changelist
}

test_run_docs_and_go_tests_expect if {
	pr_check.changes.docs with input as example_docs_and_go_root_changelist
	pr_check.changes.go with input as example_docs_and_go_root_changelist
}

test_run_some_not_others_expect if {
	not pr_check.changes.docs with input as mixed_bag_changelist
	pr_check.changes.go with input as mixed_bag_changelist
	pr_check.changes.wasm with input as mixed_bag_changelist
	not pr_check.changes.yaml with input as mixed_bag_changelist
}

test_run_all_tests_expect if {
	pr_check.changes.docs with input as example_all_checks_root_changelist
	pr_check.changes.go with input as example_all_checks_root_changelist
	pr_check.changes.wasm with input as example_all_checks_root_changelist
	not pr_check.changes.yaml with input as example_all_checks_root_changelist
}

test_bench_compiler_related if {
	pr_check.changes.bench == {"./v1/topdown", "./v1/compile", "./v1/rego"} with input as example_bench_topdown_changelist
}

test_bench_no_match if {
	pr_check.changes.go with input as example_bench_no_match_changelist
	pr_check.changes.bench == set() with input as example_bench_no_match_changelist
}

test_run_proto_check_expect if {
	pr_check.changes.proto with input as example_proto_changelist
}

test_run_proto_check_on_buf_yaml if {
	pr_check.changes.proto with input as example_buf_yaml_changelist
}

test_run_no_proto_check_for_unrelated if {
	not pr_check.changes.proto with input as example_go_changelist
}

# A .proto outside the configured buf.yaml modules must NOT trigger the
# proto-check job — buf would silently ignore it and we'd ship false-green.
test_run_no_proto_check_for_stray_proto if {
	stray := [{"filename": "docs/examples/foo.proto"}]
	not pr_check.changes.proto with input as stray
}

# A change to a CI workflow file can affect any job, so every check must run.
test_workflow_change_triggers_all_checks if {
	pr_check.changes.docs with input as example_workflow_only_changelist
	pr_check.changes.go with input as example_workflow_only_changelist
	pr_check.changes.wasm with input as example_workflow_only_changelist
	pr_check.changes.rego with input as example_workflow_only_changelist
	pr_check.changes.yaml with input as example_workflow_only_changelist
	pr_check.changes.proto with input as example_workflow_only_changelist
}

# A .proto-only PR must still trigger the go-test job, since the
# Go-vs-proto consistency tests live in v1/bundle / v1/ir.
test_proto_only_pr_triggers_go_test if {
	pr_check.changes.go with input as example_proto_changelist
}

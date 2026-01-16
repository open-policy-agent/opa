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
}

test_run_all_tests_expect if {
	pr_check.changes.docs with input as example_all_checks_root_changelist
	pr_check.changes.go with input as example_all_checks_root_changelist
	pr_check.changes.wasm with input as example_all_checks_root_changelist
}

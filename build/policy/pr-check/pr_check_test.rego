package pr_check_test

import data.pr_check

example_docs_changelist := [
  {
    "filename": "docs/docs/policy-reference/builtins/glob.mdx"
  },
  {
    "filename": "docs/docs/policy-reference/builtins/http.mdx"
  },
  {
    "filename": "docs/docs/policy-reference/builtins/regex.mdx"
  },
]

example_go_changelist := [
  {
    "filename": "cmd/build.go"
  },
  {
    "filename": "build/gen-run-go.sh"
  },
  {
    "filename": "v1/rego/testdata/ast.json"
  }
]


example_wasm_changelist := [
  {
    "filename": "wasm/src/regex.cc"
  },
  {
    "filename": "ast/errors.go"
  },
  {
    "filename": "v1/test/wasm/assets/test.js"
  }
]

example_docs_exception_changelist := [
    {
        "filename": "docs/bin/build-latest.sh"
    }, 
    {
        "filename": "docs/docs/envoy/_category_.yaml"
    }
]

mixed_bag_changelist := [
    {
        "filename": "wasm/Makefile"
    }, 
    {
        "filename": "v1/rego/testdata/ast.json",
    }
]

example_root_changelist := [
  {
    "filename": "Makefile"
  },
  {
    "filename": "go.mod"
  },
  {
    "filename": "go.sum"
  }
]

test_run_docs_tests_expect if {
	pr_check.changes.docs with input as example_docs_changelist
}

test_run_go_tests_expect if {
	pr_check.changes.go with input as example_go_changelist
}

test_run_wasm_tests_expect if {
	pr_check.changes.wasm with input as example_wasm_changelist
}

test_run_docs_not_go_tests_expect if {
	pr_check.changes.docs with input as example_docs_exception_changelist
    not pr_check.changes.go with input as example_docs_exception_changelist
}

test_run_some_not_others_expect if {
	not pr_check.changes.docs with input as mixed_bag_changelist
    pr_check.changes.go with input as mixed_bag_changelist
    pr_check.changes.wasm with input as mixed_bag_changelist
}

test_run_all_tests_expect if {
	pr_check.changes.docs with input as example_root_changelist
    pr_check.changes.go with input as example_root_changelist
    pr_check.changes.wasm with input as example_root_changelist
}


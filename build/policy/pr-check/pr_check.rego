package policy["pr-check"]

changes.docs if {
	some changed_file in input
	startswith(changed_file.filename, "docs/")
}

changes.docs if {
	some changed_file in input
	changed_file.filename in docs_root_files
}

changes.docs if _workflows_changed

changes.go if {
	some changed_file in input
	endswith(changed_file.filename, ".go")
}

changes.go if {
	some changed_file in input
	strings.any_prefix_match(changed_file.filename, go_change_prefixes)
	strings.any_suffix_match(changed_file.filename, go_change_suffixes)
}

changes.go if {
	some changed_file in input
	changed_file.filename in go_root_files
}

# .proto changes also run go-test (consistency tests live there).
changes.go if changes.proto

changes.go if _workflows_changed

changes.wasm if {
	some changed_file in input
	strings.any_prefix_match(changed_file.filename, wasm_change_prefixes)
}

changes.wasm if {
	some changed_file in input
	changed_file.filename in rego_and_wasm_change_root_files
}

changes.wasm if _workflows_changed

changes.rego if {
	some changed_file in input
	endswith(changed_file.filename, ".rego")
}

changes.rego if {
	some changed_file in input
	changed_file.filename in rego_and_wasm_change_root_files
}

changes.rego if _workflows_changed

changes.yaml if {
	some changed_file in input
	strings.any_suffix_match(changed_file.filename, yaml_change_suffixes)
}

changes.yaml if _workflows_changed

changes.proto if {
	some changed_file in input
	strings.any_prefix_match(changed_file.filename, proto_change_prefixes)
	endswith(changed_file.filename, ".proto")
}

changes.proto if {
	some changed_file in input
	changed_file.filename == "buf.yaml"
}

changes.proto if _workflows_changed

changes.bench contains "./v1/ast" if {
	some changed_file in input
	startswith(changed_file.filename, "v1/ast/")
	endswith(changed_file.filename, ".go")
}

changes.bench contains "./v1/topdown" if {
	some changed_file in input
	startswith(changed_file.filename, "v1/topdown/")
	endswith(changed_file.filename, ".go")
}

changes.bench contains "./v1/rego" if {
	some changed_file in input
	startswith(changed_file.filename, "v1/rego/")
	endswith(changed_file.filename, ".go")
}

_workflows_changed if {
	some changed_file in input
	startswith(changed_file.filename, ".github/workflows/")
}

go_change_prefixes := [
	"build/",
	"capabilities/",
	"e2e/",
	"internal/",
	"v1/",
]

go_change_suffixes := [
	".mod",
	".sum",
	".json",
	".go-version",
	"Makefile",
	"Dockerfile",
	".sh",
	".yaml",
	".yml",
	".txtar",
]

wasm_change_prefixes := [
	"wasm/",
	"ast/",
	"internal/compiler/",
	"internal/planner/",
	"internal/wasm/",
	"test/wasm/",
	"test/cases/",
	"v1/ast/",
	"v1/test/cases/",
	"v1/test/wasm/",
	"v1/ir",
]

yaml_change_suffixes := [
	".yaml",
	".yml",
]

rego_and_wasm_change_root_files := ["Makefile"]

docs_root_files := [
	"builtin_metadata.json",
	"capabilities.json",
	"netlify.toml",
	"Makefile",
]

go_root_files := [
	".go-version",
	".golangci.yaml",
	".yamllint.yaml",
	"builtin_metadata.json",
	"capabilities.json",
	"Makefile",
	"Dockerfile",
	"go.mod",
	"go.sum",
	"main_windows.go",
	"main.go",
]

# Paths covered by buf.yaml.
proto_change_prefixes := [
	"v1/bundle/",
	"v1/ir/",
]

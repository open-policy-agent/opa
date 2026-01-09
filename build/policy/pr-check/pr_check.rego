package policy["pr-check"]

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

wasm_change_root_files := ["Makefile"]

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

changes["docs"] if {
	some changed_file in input
	startswith(changed_file.filename, "docs/")
} else if {
	some changed_file in input
	changed_file.filename in docs_root_files
}

changes["go"] if {
	some changed_file in input
	endswith(changed_file.filename, ".go")
} else if {
	some changed_file in input
	strings.any_prefix_match(changed_file.filename, go_change_prefixes)
	strings.any_suffix_match(changed_file.filename, go_change_suffixes)
} else if {
	some changed_file in input
	changed_file.filename in go_root_files
}

changes["wasm"] if {
	some changed_file in input
	strings.any_prefix_match(changed_file.filename, wasm_change_prefixes)
} else if {
	some changed_file in input
	changed_file.filename in wasm_change_root_files
}

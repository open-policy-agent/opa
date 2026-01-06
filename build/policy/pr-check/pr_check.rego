package policy["pr-check"]

go_change_prefixes := ["cmd/", "internal/", "v1/"]
go_change_suffixes := [
	".go",
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

wasm_change_suffixes := ["Makefile"]

docs_root_files := [
	"builtin_metadata.json",
	"capabilities.json",
	"netlify.toml",
	"Makefile",
]

changes["docs"] if {
	some e in input
	startswith(e.filename, "docs/")
} else if {
	some e in input
	some filename in docs_root_files
	e.filename == filename
}

changes["go"] if {
	some e in input
	not startswith(e.filename, "docs/")
	strings.any_prefix_match(e.filename, go_change_prefixes)
} else if {
	some e in input
	not startswith(e.filename, "docs/")
	strings.any_suffix_match(e.filename, go_change_suffixes)
}

changes["wasm"] if {
	some e in input
	strings.any_prefix_match(e.filename, wasm_change_prefixes)
} else if {
	some e in input
	strings.any_suffix_match(e.filename, wasm_change_suffixes)
}

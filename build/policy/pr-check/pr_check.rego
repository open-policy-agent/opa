package pr_check

go_change_suffixes := [
     ".go", 
     ".mod", 
     ".sum", 
     ".json", 
     ".go-version", 
     "Makefile", 
     ".sh", 
     ".yaml",
     ".yml"
    ]
go_change_prefixes := ["cmd/", "internal/", "v1/"]
wasm_change_patterns := [
     "Makefile",
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
     "v1/ir"
    ]

changes.root if {
    some e in input
    not contains(e.filename, "/")
}

changes.docs if changes.root

changes.docs if {
    some e in input 
    startswith(e.filename, "docs/")
}

changes.go if changes.root

changes.go if {
    some e in input
    not startswith(e.filename, "docs/")
    strings.any_suffix_match(e.filename, go_change_suffixes)
    strings.any_prefix_match(e.filename, go_change_prefixes)
}

changes.wasm if changes.root

changes.wasm if {
    some e in input
    strings.any_prefix_match(e.filename, wasm_change_patterns)
}
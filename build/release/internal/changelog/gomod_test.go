package changelog

import (
	"reflect"
	"testing"
)

func TestParseRequires_BlockAndSingleLine(t *testing.T) {
	content := `module example.com/foo

go 1.25.0

require github.com/single/dep v1.0.0

require (
	github.com/foo/bar v1.2.3
	github.com/baz/qux v0.5.0 // indirect
	example.com/y v2.0.0
)
`
	got := ParseRequires(content)
	want := map[string]string{
		"github.com/single/dep": "v1.0.0",
		"github.com/foo/bar":    "v1.2.3",
		"example.com/y":         "v2.0.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseRequires_IndirectExcluded(t *testing.T) {
	content := `require (
	github.com/direct v1.0.0
	github.com/indirect v0.1.0 // indirect
)
`
	got := ParseRequires(content)
	if _, ok := got["github.com/indirect"]; ok {
		t.Errorf("indirect dep should be excluded, got %+v", got)
	}
	if got["github.com/direct"] != "v1.0.0" {
		t.Errorf("direct dep missing/wrong: %+v", got)
	}
}

func TestParseRequires_SingleLineIndirectExcluded(t *testing.T) {
	content := `require github.com/x v1.0.0 // indirect`
	got := ParseRequires(content)
	if len(got) != 0 {
		t.Errorf("indirect single-line should be excluded, got %+v", got)
	}
}

func TestDiffRequires_AddRemoveBump(t *testing.T) {
	from := `require (
	github.com/stays v1.0.0
	github.com/bumps v1.0.0
	github.com/removed v1.0.0
)
`
	to := `require (
	github.com/stays v1.0.0
	github.com/bumps v1.5.0
	github.com/added v0.1.0
)
`
	got := DiffRequires(from, to)
	want := []ModuleChange{
		{Module: "github.com/added", NewVersion: "v0.1.0"},
		{Module: "github.com/bumps", OldVersion: "v1.0.0", NewVersion: "v1.5.0"},
		{Module: "github.com/removed", OldVersion: "v1.0.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiffRequires_NoChange(t *testing.T) {
	content := `require github.com/foo v1.0.0`
	if got := DiffRequires(content, content); len(got) != 0 {
		t.Errorf("expected no diff, got %+v", got)
	}
}

package changelog

import (
	"strings"
	"testing"
)

func TestDeriveArea_ExistingPrefixWins(t *testing.T) {
	cases := []struct {
		name     string
		subject  string
		labels   []string
		paths    []string
		wantArea string
		wantSubj string
	}{
		{
			name:     "simple ast prefix",
			subject:  "ast: Allow $ref in JSON schemas",
			wantArea: "ast",
			wantSubj: "Allow $ref in JSON schemas",
		},
		{
			name:     "build(deps) prefix preserved",
			subject:  "build(deps): bump foo from 1 to 2",
			wantArea: "build(deps)",
			wantSubj: "bump foo from 1 to 2",
		},
		{
			name:     "slash in prefix",
			subject:  "build/release: tweak",
			wantArea: "build/release",
			wantSubj: "tweak",
		},
		{
			name:     "plus and dash in prefix",
			subject:  "nightly+release-vuln-check: add links",
			wantArea: "nightly+release-vuln-check",
			wantSubj: "add links",
		},
		{
			name:     "second colon in subject is not a prefix",
			subject:  "ast: Fix X: edge case",
			wantArea: "ast",
			wantSubj: "Fix X: edge case",
		},
		{
			name:     "existing prefix beats label",
			subject:  "ast: Fix X",
			labels:   []string{"area/server"},
			wantArea: "ast",
			wantSubj: "Fix X",
		},
		{
			name:     "existing prefix beats path",
			subject:  "topdown: change",
			paths:    []string{"v1/server/handlers.go"},
			wantArea: "topdown",
			wantSubj: "change",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveArea(tc.subject, tc.labels, tc.paths)
			if got.Area != tc.wantArea || got.Subject != tc.wantSubj {
				t.Errorf("got %+v, want Area=%q Subject=%q", got, tc.wantArea, tc.wantSubj)
			}
			if got.Source != "existing prefix" {
				t.Errorf("source: got %q, want %q", got.Source, "existing prefix")
			}
		})
	}
}

// TestDeriveArea_LabelsAreIgnored pins the current no-op: areaFromLabels derives
// nothing, because the "area/X" convention it assumed is not one this repo
// actually uses. Paths still apply, and an explicit prefix still wins — only the
// label rule is off. See the TODO on areaFromLabels.
func TestDeriveArea_LabelsAreIgnored(t *testing.T) {
	cases := []struct {
		name     string
		subject  string
		labels   []string
		paths    []string
		wantArea string
	}{
		{name: "area/ label derives nothing", subject: "Wire up handler", labels: []string{"area/server"}, wantArea: ""},
		{name: "area: label derives nothing", subject: "Wire up handler", labels: []string{"area: rego"}, wantArea: ""},
		{name: "mixed labels derive nothing", subject: "x", labels: []string{"good first issue", "area/topdown"}, wantArea: ""},
		{name: "non-area labels derive nothing", subject: "x", labels: []string{"bug", "good first issue"}, wantArea: ""},
		{
			name:     "path still applies when labels are present",
			subject:  "Wire up handler",
			labels:   []string{"area/server"},
			paths:    []string{"v1/ast/parser.go"},
			wantArea: "ast",
		},
		{
			name:     "explicit prefix still wins over labels",
			subject:  "topdown: Wire up handler",
			labels:   []string{"area/server"},
			wantArea: "topdown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveArea(tc.subject, tc.labels, tc.paths)
			if got.Area != tc.wantArea {
				t.Errorf("got area %q, want %q (full result: %+v)", got.Area, tc.wantArea, got)
			}
			if strings.Contains(got.Source, "label") {
				t.Errorf("source should never mention a label while the rule is a no-op, got %q", got.Source)
			}
		})
	}
}

func TestDeriveArea_PathFallback(t *testing.T) {
	cases := []struct {
		name     string
		paths    []string
		wantArea string
	}{
		{name: "all v1/ast", paths: []string{"v1/ast/parser.go", "v1/ast/parser_test.go"}, wantArea: "ast"},
		{name: "docs/", paths: []string{"docs/docs/intro.md"}, wantArea: "docs"},
		{name: ".github/workflows", paths: []string{".github/workflows/ci.yml"}, wantArea: "gha"},
		{name: "build/release more specific than build", paths: []string{"build/release/main.go"}, wantArea: "build/release"},
		{name: "majority wins", paths: []string{"v1/ast/a.go", "v1/ast/b.go", "v1/topdown/c.go"}, wantArea: "ast"},
		{name: "no v1 catch-all: unlisted v1 subdir yields no area", paths: []string{"v1/exotic_thing.go"}, wantArea: ""},
		{name: "no internal catch-all", paths: []string{"internal/whatever/x.go"}, wantArea: ""},
		{name: "count tie broken by areaByPath order, not map order", paths: []string{"docs/a.md", "v1/ast/b.go"}, wantArea: "docs"},
		{name: "unmappable paths return empty", paths: []string{"README.md", "LICENSE"}, wantArea: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveArea("Subject without prefix", nil, tc.paths)
			if got.Area != tc.wantArea {
				t.Errorf("got area %q, want %q (full: %+v)", got.Area, tc.wantArea, got)
			}
			if tc.wantArea != "" && !strings.Contains(got.Source, "path") {
				t.Errorf("source should mention path, got %q", got.Source)
			}
		})
	}
}

func TestDeriveArea_NoSignals(t *testing.T) {
	got := DeriveArea("Just a subject", nil, nil)
	if got.Area != "" || got.Source != "" {
		t.Errorf("expected empty area+source, got %+v", got)
	}
	if got.Subject != "Just a subject" {
		t.Errorf("subject mutated: got %q", got.Subject)
	}
}

// TestDeriveArea_TieBreakIsDeterministic guards the golden tests: areaFromPaths
// used to pick the winner by iterating a map, so equal-count ties resolved
// differently between runs.
func TestDeriveArea_TieBreakIsDeterministic(t *testing.T) {
	paths := []string{"docs/a.md", "v1/ast/b.go", "v1/topdown/c.go", ".github/workflows/d.yml"}
	first := DeriveArea("Subject", nil, paths)
	for i := range 50 {
		got := DeriveArea("Subject", nil, paths)
		if got != first {
			t.Fatalf("iteration %d: got %+v, want %+v", i, got, first)
		}
	}
}

// TestDeriveArea_PrefixNotDuplicated covers the v1.17.0 regression: a subject
// whose prefix contains a comma was not recognized as a prefix, so the path
// heuristic stacked a second area on top of it.
func TestDeriveArea_PrefixNotDuplicated(t *testing.T) {
	for _, tc := range []struct {
		name           string
		subject        string
		paths          []string
		wantArea, want string
	}{
		{
			name:     "comma in prefix",
			subject:  "ast,storage/inmem: Add `inmem.NewFromASTObject`",
			paths:    []string{"v1/storage/inmem/inmem.go"},
			wantArea: "ast,storage/inmem",
			want:     "Add `inmem.NewFromASTObject`",
		},
		{
			name:     "parens in prefix",
			subject:  "docs(ecosystem): add OPA MCP",
			paths:    []string{"docs/x.md"},
			wantArea: "docs(ecosystem)",
			want:     "add OPA MCP",
		},
		{
			name:     "plus in prefix",
			subject:  "ast+rego+topdown: external rule source support",
			paths:    []string{"v1/ast/x.go"},
			wantArea: "ast+rego+topdown",
			want:     "external rule source support",
		},
		{
			name:     "colon mid-sentence is not a prefix",
			subject:  "Fix X: edge case",
			paths:    []string{"v1/ast/x.go"},
			wantArea: "ast",
			want:     "Fix X: edge case",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveArea(tc.subject, nil, tc.paths)
			if got.Area != tc.wantArea {
				t.Errorf("area = %q, want %q", got.Area, tc.wantArea)
			}
			if got.Subject != tc.want {
				t.Errorf("subject = %q, want %q", got.Subject, tc.want)
			}
		})
	}
}

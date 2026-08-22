package changelog

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

func TestTransform_AreaAndDependencyDetection(t *testing.T) {
	cases := []struct {
		name             string
		subject          string
		author           string
		labels           []string
		files            []string
		wantArea         string
		wantSubject      string
		wantSource       string // substring match
		wantIsDependency bool
	}{
		{
			name:        "existing prefix wins, subject stripped",
			subject:     "ast: Allow $ref",
			author:      "deeglaze",
			labels:      []string{"area/server"},
			wantArea:    "ast",
			wantSubject: "Allow $ref",
			wantSource:  "existing prefix",
		},
		{
			// Labels derive nothing for now (see the TODO on areaFromLabels), so
			// a label-only entry falls through to having no area at all.
			name:        "label alone yields no prefix",
			subject:     "Wire up handler",
			author:      "alice",
			labels:      []string{"area/server"},
			wantArea:    "",
			wantSubject: "Wire up handler",
			wantSource:  "",
		},
		{
			name:        "path heuristic when no prefix",
			subject:     "Refactor parser",
			author:      "alice",
			files:       []string{"v1/ast/parser.go", "v1/ast/parser_test.go"},
			wantArea:    "ast",
			wantSubject: "Refactor parser",
			wantSource:  "path",
		},
		{
			name:        "no signals leaves area empty",
			subject:     "Reword README",
			author:      "alice",
			files:       []string{"README.md"},
			wantArea:    "",
			wantSubject: "Reword README",
			wantSource:  "",
		},
		{
			name:             "dependabot author marks dependency",
			subject:          "Update foo to v2",
			author:           "dependabot[bot]",
			wantArea:         "",
			wantSubject:      "Update foo to v2",
			wantIsDependency: true,
		},
		{
			name:             "build(deps) prefix marks dependency and is preserved as area",
			subject:          "build(deps): Bump foo from 1 to 2",
			author:           "alice",
			wantArea:         "build(deps)",
			wantSubject:      "Bump foo from 1 to 2",
			wantSource:       "existing prefix",
			wantIsDependency: true,
		},
		{
			name:             "build: bump go marks dependency",
			subject:          "build: bump go 1.26.4",
			author:           "alice",
			wantArea:         "build",
			wantSubject:      "Bump go 1.26.4",
			wantSource:       "existing prefix",
			wantIsDependency: true,
		},
		{
			name:             "gha: bump action marks dependency",
			subject:          "gha: bump trivy-action",
			author:           "alice",
			wantArea:         "gha",
			wantSubject:      "Bump trivy-action",
			wantSource:       "existing prefix",
			wantIsDependency: true,
		},
		{
			name:             "build: refactor (no bump) is NOT a dependency",
			subject:          "build: simplify Dockerfile",
			author:           "alice",
			wantArea:         "build",
			wantSubject:      "Simplify Dockerfile",
			wantSource:       "existing prefix",
			wantIsDependency: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := &gh.PullRequest{Number: 1, Labels: tc.labels}
			entries := []Entry{{
				SHA:         "abcdef0123456789",
				Subject:     tc.subject,
				AuthorLogin: tc.author,
				PRs:         []*gh.PullRequest{pr},
				Files:       tc.files,
			}}

			logs := Transform(entries)

			e := entries[0]
			if e.Area != tc.wantArea {
				t.Errorf("Area: got %q, want %q", e.Area, tc.wantArea)
			}
			if e.Subject != tc.wantSubject {
				t.Errorf("Subject: got %q, want %q", e.Subject, tc.wantSubject)
			}
			if e.IsDependency != tc.wantIsDependency {
				t.Errorf("IsDependency: got %v, want %v", e.IsDependency, tc.wantIsDependency)
			}

			if len(logs) != 1 {
				t.Fatalf("expected 1 log, got %d", len(logs))
			}
			l := logs[0]
			if l.SHA != e.SHA || l.Area != e.Area || l.Subject != e.Subject || l.IsDependency != e.IsDependency {
				t.Errorf("log doesn't mirror entry: %+v vs %+v", l, e)
			}
			if tc.wantSource != "" && !strings.Contains(l.Source, tc.wantSource) {
				t.Errorf("Source: got %q, want substring %q", l.Source, tc.wantSource)
			}
			if tc.wantSource == "" && l.Source != "" {
				t.Errorf("Source should be empty, got %q", l.Source)
			}
		})
	}
}

func TestTransform_LogsOnePerEntryInOrder(t *testing.T) {
	entries := []Entry{
		{SHA: "111", Subject: "ast: a"},
		{SHA: "222", Subject: "server: b"},
		{SHA: "333", Subject: "docs: c"},
	}
	logs := Transform(entries)
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	for i, want := range []string{"111", "222", "333"} {
		if logs[i].SHA != want {
			t.Errorf("logs[%d].SHA: got %q, want %q", i, logs[i].SHA, want)
		}
	}
}

func TestCapitalizeSubject(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{name: "plain lowercase word", in: "use oras, not containerd", want: "Use oras, not containerd"},
		{name: "single word", in: "tweak", want: "Tweak"},
		{name: "already capitalized", in: "Add titles", want: "Add titles"},
		{name: "backtick start left alone", in: "`and`/`or` compilation", want: "`and`/`or` compilation"},
		{name: "dotted identifier left alone", in: "json.verify_schema validation", want: "json.verify_schema validation"},
		{name: "hyphenated tool name left alone", in: "golangci-lint bump to v2.12.2", want: "golangci-lint bump to v2.12.2"},
		{name: "underscored identifier left alone", in: "file_logger plugin enabled", want: "file_logger plugin enabled"},
		{name: "slashed path left alone", in: "not-body marshaling", want: "not-body marshaling"},
		{name: "empty subject", in: "", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := capitalizeSubject(tc.in); got != tc.want {
				t.Errorf("capitalizeSubject(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsDependency_DepsArea(t *testing.T) {
	for _, tc := range []struct {
		name, author, subject, area string
		want                        bool
	}{
		{name: "deps area", area: "deps", subject: "Bump wasmtime-go (v43 -> v44)", want: true},
		{name: "dependencies area", area: "dependencies", subject: "Bump x", want: true},
		{name: "build(deps) area", area: "build(deps)", subject: "bump foo", want: true},
		{name: "dependabot author", author: "dependabot[bot]", subject: "anything", want: true},
		{name: "build area without bump", area: "build", subject: "Use go install tool", want: false},
		{name: "unrelated area", area: "download", subject: "Use oras, not containerd", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDependency(tc.author, tc.subject, tc.area); got != tc.want {
				t.Errorf("isDependency(%q, %q, %q) = %v, want %v", tc.author, tc.subject, tc.area, got, tc.want)
			}
		})
	}
}

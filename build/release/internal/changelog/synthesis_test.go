package changelog

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

func TestSynthesizeMissingDeps_CoveredBySubject(t *testing.T) {
	entries := []Entry{
		{
			SHA: "abc1234", Subject: "Bump github.com/foo/bar from 1.0 to 2.0",
			Area: "build(deps)", IsDependency: true,
		},
	}
	changes := []ModuleChange{
		{Module: "github.com/foo/bar", OldVersion: "v1.0.0", NewVersion: "v2.0.0"},
	}
	out, logs := SynthesizeMissingDeps(entries, changes)
	if len(out) != 1 {
		t.Errorf("expected no synthetic entries, got %d", len(out))
	}
	if logs[0].Action != ActionCovered {
		t.Errorf("action: got %q, want covered", logs[0].Action)
	}
	if logs[0].CoveringSHA != "abc1234" {
		t.Errorf("CoveringSHA: got %q, want abc1234", logs[0].CoveringSHA)
	}
}

func TestSynthesizeMissingDeps_UncoveredSynthesized(t *testing.T) {
	entries := []Entry{
		{
			SHA: "1", Subject: "Bump github.com/foo from 1 to 2",
			Area: "build(deps)", IsDependency: true,
		},
	}
	changes := []ModuleChange{
		{Module: "github.com/foo", OldVersion: "v1", NewVersion: "v2"},
		{Module: "github.com/missing", OldVersion: "v0.1.0", NewVersion: "v0.2.0"},
	}
	out, logs := SynthesizeMissingDeps(entries, changes)
	if len(out) != 2 {
		t.Fatalf("expected 1 synthetic entry, got total %d", len(out))
	}
	syn := out[1]
	if !syn.IsSynthetic || !syn.IsDependency {
		t.Errorf("synthetic flags wrong: IsSynthetic=%v IsDependency=%v", syn.IsSynthetic, syn.IsDependency)
	}
	if syn.Area != "build(deps)" {
		t.Errorf("Area: got %q, want build(deps)", syn.Area)
	}
	if syn.Subject != "Bump github.com/missing from 0.1.0 to 0.2.0" {
		t.Errorf("Subject: got %q", syn.Subject)
	}
	if logs[1].Action != ActionSynthesized {
		t.Errorf("action: got %q, want synthesized", logs[1].Action)
	}
}

func TestSynthesizeMissingDeps_AddedRemovedSubjects(t *testing.T) {
	changes := []ModuleChange{
		{Module: "added", NewVersion: "v1"},
		{Module: "removed", OldVersion: "v1"},
	}
	out, _ := SynthesizeMissingDeps(nil, changes)
	if len(out) != 2 {
		t.Fatalf("expected 2 synthetic, got %d", len(out))
	}
	if out[0].Subject != "Add added 1" {
		t.Errorf("added subject wrong: %q", out[0].Subject)
	}
	if out[1].Subject != "Drop removed (was 1)" {
		t.Errorf("removed subject wrong: %q", out[1].Subject)
	}
}

func TestSynthesizeMissingDeps_NonDepEntriesCountForCoverage(t *testing.T) {
	// A non-dep entry that names the module DOES count as coverage. The
	// commits that add or remove a dependency are usually classified by the
	// code they touch ("runtime:", "download:"), so restricting coverage to
	// IsDependency entries rendered those changes twice. Every match is
	// reported as "[covered by <sha>]" in the synthesis log.
	entries := []Entry{
		{SHA: "abc", Subject: "Doc note: bump github.com/foo from 1 to 2"},
	}
	changes := []ModuleChange{
		{Module: "github.com/foo", OldVersion: "v1", NewVersion: "v2"},
	}
	out, logs := SynthesizeMissingDeps(entries, changes)
	if len(out) != 1 {
		t.Errorf("expected no synthesis, got %d entries", len(out))
	}
	if logs[0].Action != ActionCovered || logs[0].CoveringSHA != "abc" {
		t.Errorf("expected covered by abc, got %+v", logs[0])
	}
}

func TestSynthesizeMissingDeps_RendersBare(t *testing.T) {
	// Confirm the synthetic entry round-trips through Render with no link
	// or attribution (per prior art).
	entries, _ := SynthesizeMissingDeps(nil, []ModuleChange{
		{Module: "github.com/foo", OldVersion: "v1.0.0", NewVersion: "v2.0.0"},
	})
	out := Render(entries, "https://x")
	if !strings.Contains(out, "- build(deps): Bump github.com/foo from 1.0.0 to 2.0.0\n") {
		t.Errorf("bare bullet not present:\n%s", out)
	}
	// Synthetic entries must not get parens (link) or attribution.
	if strings.Contains(out, "authored by") {
		t.Errorf("synthetic entry should not have attribution:\n%s", out)
	}
}

// TestFeatureCommitWithDepBump exercises the full Filter→Synthesize→Render
// path for the case described in plan §0: a single commit that ships a
// feature AND bumps a dep. The commit entry must remain a normal feature
// bullet with full attribution; the dep bump must surface as a synthetic
// bare bullet under the "Dependency updates; notably:" parent.
func TestFeatureCommitWithDepBump(t *testing.T) {
	feature := Entry{
		SHA:         "abc1234",
		Subject:     "add endpoint and bump golang-jwt",
		Area:        "server",
		AuthorLogin: "alice",
		PRs:         []*gh.PullRequest{{Number: 100, URL: "https://github.com/example/x/pull/100"}},
		Files:       []string{"v1/server/handlers.go", "go.mod", "go.sum"},
		// Crucially IsDependency=false: the subject doesn't match any dep
		// marker even though the commit touched go.mod.
	}
	entries := []Entry{feature}

	kept, _ := FilterDependencies(entries)
	if len(kept) != 1 || kept[0].AuthorLogin != "alice" {
		t.Fatalf("feature commit must survive filter with attribution intact, got %+v", kept)
	}

	changes := []ModuleChange{
		{Module: "github.com/golang-jwt/jwt", OldVersion: "v4.5.0", NewVersion: "v5.0.0"},
	}
	withSynth, synthLogs := SynthesizeMissingDeps(kept, changes)
	if len(withSynth) != 2 {
		t.Fatalf("expected 1 feature + 1 synthesized = 2 entries, got %d", len(withSynth))
	}
	if synthLogs[0].Action != ActionSynthesized {
		t.Errorf("dep bump should be synthesized (not covered by feature commit), got %q", synthLogs[0].Action)
	}

	out := Render(withSynth, "https://github.com/example/x")
	wantFeature := "- server: add endpoint and bump golang-jwt ([#100](https://github.com/example/x/pull/100)) authored by @alice"
	if !strings.Contains(out, wantFeature) {
		t.Errorf("feature bullet missing/wrong, got:\n%s", out)
	}
	wantDepParent := "- Dependency updates; notably:"
	if !strings.Contains(out, wantDepParent) {
		t.Errorf("dependency parent missing, got:\n%s", out)
	}
	wantDepChild := "  - build(deps): Bump github.com/golang-jwt/jwt from 4.5.0 to 5.0.0"
	if !strings.Contains(out, wantDepChild) {
		t.Errorf("synthetic dep child missing, got:\n%s", out)
	}
}

// TestSubjectNamesModule covers the coverage-matching rules that stop a
// dependency change from being rendered twice — once as the commit that made it
// and once as a synthesized go.mod bullet.
func TestSubjectNamesModule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject string
		module  string
		want    bool
	}{
		{name: "final segment", subject: "Remove automaxprocs dependency", module: "go.uber.org/automaxprocs", want: true},
		{name: "host-stripped path", subject: "Remove direct x/net dependency", module: "golang.org/x/net", want: true},
		{name: "final segment with major suffix stripped", subject: "Bump wasmtime-go (v43 -> v44)", module: "github.com/bytecodealliance/wasmtime-go/v44", want: true},
		{name: "mentioned in passing", subject: "Use oras, not containerd", module: "github.com/containerd/containerd/v2", want: true},
		{name: "full module path", subject: "Drop github.com/vektah/gqlparser/v2 for now", module: "github.com/vektah/gqlparser/v2", want: true},
		{name: "case insensitive", subject: "remove AUTOMAXPROCS", module: "go.uber.org/automaxprocs", want: true},
		{name: "short final segment not matched alone", subject: "Improve net handling", module: "golang.org/x/net", want: false},
		{name: "unrelated subject", subject: "Fix parser panic", module: "google.golang.org/grpc", want: false},
		{name: "sibling module not matched", subject: "Use oras, not containerd", module: "github.com/containerd/errdefs", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := subjectNamesModule(tc.subject, tc.module); got != tc.want {
				t.Errorf("subjectNamesModule(%q, %q) = %v, want %v", tc.subject, tc.module, got, tc.want)
			}
		})
	}
}

// TestSynthesizeMissingDeps_CoveredByNonDepEntry asserts that a commit
// classified by the code it touches (not as dependency work) still suppresses
// the synthesized bullet for the module it removed.
func TestSynthesizeMissingDeps_CoveredByNonDepEntry(t *testing.T) {
	entries := []Entry{
		{SHA: "aaa1111", Subject: "Remove automaxprocs dependency", Area: "runtime"},
	}
	changes := []ModuleChange{
		{Module: "go.uber.org/automaxprocs", OldVersion: "v1.6.0"},
		{Module: "google.golang.org/grpc", OldVersion: "v1.80.0", NewVersion: "v1.81.0"},
	}
	got, logs := SynthesizeMissingDeps(entries, changes)
	if len(got) != 2 {
		t.Fatalf("expected 1 original + 1 synthesized entry, got %d: %+v", len(got), got)
	}
	if logs[0].Action != ActionCovered || logs[0].CoveringSHA != "aaa1111" {
		t.Errorf("automaxprocs: got %+v, want covered by aaa1111", logs[0])
	}
	if logs[1].Action != ActionSynthesized {
		t.Errorf("grpc: got %+v, want synthesized", logs[1])
	}
	if want := "Bump google.golang.org/grpc from 1.80.0 to 1.81.0"; got[1].Subject != want {
		t.Errorf("synthesized subject = %q, want %q (leading v must be stripped)", got[1].Subject, want)
	}
}

func TestSyntheticEntry_StripsVersionPrefix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change ModuleChange
		want   string
	}{
		{name: "bump", change: ModuleChange{Module: "m", OldVersion: "v1.9.0", NewVersion: "v1.10.1"}, want: "Bump m from 1.9.0 to 1.10.1"},
		{name: "add", change: ModuleChange{Module: "m", NewVersion: "v6.0.2"}, want: "Add m 6.0.2"},
		{name: "drop", change: ModuleChange{Module: "m", OldVersion: "v1.6.0"}, want: "Drop m (was 1.6.0)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := syntheticEntry(tc.change).Subject; got != tc.want {
				t.Errorf("subject = %q, want %q", got, tc.want)
			}
		})
	}
}

package changelog

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

func TestRenderAttribution(t *testing.T) {
	cases := []struct {
		name   string
		author string
		issue  *gh.Issue
		want   string
	}{
		{
			name:   "distinct people",
			author: "alice",
			issue:  &gh.Issue{ReporterLogin: "bob"},
			want:   "authored by @alice, reported by @bob",
		},
		{
			name:   "same person reported and authored",
			author: "alice",
			issue:  &gh.Issue{ReporterLogin: "alice"},
			want:   "reported and authored by @alice",
		},
		{
			name:   "no issue",
			author: "alice",
			issue:  nil,
			want:   "authored by @alice",
		},
		{
			name:   "no author no issue",
			author: "",
			issue:  nil,
			want:   "",
		},
		{
			name:   "no author with reporter",
			author: "",
			issue:  &gh.Issue{ReporterLogin: "bob"},
			want:   "reported by @bob",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := Entry{AuthorLogin: tc.author}
			if tc.issue != nil {
				e.Issues = []*gh.Issue{tc.issue}
			}
			got := renderAttribution(&e)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChooseLink_PriorityIssuePRCommit(t *testing.T) {
	repoURL := "https://github.com/open-policy-agent/opa"
	cases := []struct {
		name string
		e    Entry
		want string
	}{
		{
			name: "issue wins",
			e: Entry{
				SHA:    "abcdef0123456789",
				PRs:    []*gh.PullRequest{{Number: 100, URL: "https://github.com/open-policy-agent/opa/pull/100"}},
				Issues: []*gh.Issue{{Number: 42, URL: "https://github.com/open-policy-agent/opa/issues/42"}},
			},
			want: "[#42](https://github.com/open-policy-agent/opa/issues/42)",
		},
		{
			name: "multiple issues — all listed, sorted ascending",
			e: Entry{
				SHA: "abcdef0123456789",
				PRs: []*gh.PullRequest{{Number: 200, URL: "https://github.com/open-policy-agent/opa/pull/200"}},
				Issues: []*gh.Issue{
					{Number: 4242, URL: "https://github.com/open-policy-agent/opa/issues/4242"},
					{Number: 1111, URL: "https://github.com/open-policy-agent/opa/issues/1111"},
					{Number: 7777, URL: "https://github.com/open-policy-agent/opa/issues/7777"},
				},
			},
			want: "[#1111](https://github.com/open-policy-agent/opa/issues/1111), [#4242](https://github.com/open-policy-agent/opa/issues/4242), [#7777](https://github.com/open-policy-agent/opa/issues/7777)",
		},
		{
			name: "PR wins when no issue",
			e: Entry{
				SHA: "abcdef0123456789",
				PRs: []*gh.PullRequest{{Number: 100, URL: "https://github.com/open-policy-agent/opa/pull/100"}},
			},
			want: "[#100](https://github.com/open-policy-agent/opa/pull/100)",
		},
		{
			name: "commit fallback when no PR",
			e:    Entry{SHA: "abcdef0123456789"},
			want: "[`abcdef0`](https://github.com/open-policy-agent/opa/commit/abcdef0123456789)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseLink(&tc.e, repoURL)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRender_FullSection(t *testing.T) {
	repoURL := "https://github.com/open-policy-agent/opa"
	entries := []Entry{
		{
			SHA: "0000001", Subject: "ast: Allow $ref in allOf in JSON schemas",
			AuthorLogin: "deeglaze",
			PRs:         []*gh.PullRequest{{Number: 8581, URL: "https://github.com/open-policy-agent/opa/pull/8581"}},
			Issues:      []*gh.Issue{{Number: 6523, URL: "https://github.com/open-policy-agent/opa/issues/6523", ReporterLogin: "mosiac1"}},
		},
		{
			SHA: "0000002", Subject: "server: Wire in metadata for compile handler",
			AuthorLogin: "srenatus",
			PRs:         []*gh.PullRequest{{Number: 8650, URL: "https://github.com/open-policy-agent/opa/pull/8650"}},
		},
		{
			SHA: "0000003", Subject: "build: tweak GH actions",
			AuthorLogin: "frank",
		},
	}
	// Everything lands in one Miscellaneous list, sorted by subject —
	// issue-linked entries are not split into their own section.
	want := `### Miscellaneous

- ast: Allow $ref in allOf in JSON schemas ([#6523](https://github.com/open-policy-agent/opa/issues/6523)) authored by @deeglaze, reported by @mosiac1
- build: tweak GH actions ([` + "`0000003`" + `](https://github.com/open-policy-agent/opa/commit/0000003)) authored by @frank
- server: Wire in metadata for compile handler ([#8650](https://github.com/open-policy-agent/opa/pull/8650)) authored by @srenatus
`
	got := Render(entries, repoURL)
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

// TestRender_IssueLinkedEntryIsNotSeparated pins the taxonomy decision: an
// entry with a linked issue reads exactly like any other bullet, under the same
// heading. Only the link target differs (issue over PR).
func TestRender_IssueLinkedEntryIsNotSeparated(t *testing.T) {
	entries := []Entry{
		{
			SHA: "0000001", Subject: "ast: Fix X",
			AuthorLogin: "alice",
			PRs:         []*gh.PullRequest{{Number: 1}},
			Issues:      []*gh.Issue{{Number: 2, URL: "url2", ReporterLogin: "alice"}},
		},
	}
	got := Render(entries, "https://x")
	want := `### Miscellaneous

- ast: Fix X ([#2](url2)) reported and authored by @alice
`
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRender_OnlyMisc(t *testing.T) {
	entries := []Entry{
		{
			SHA: "0000002", Subject: "build: bump dep",
			AuthorLogin: "dependabot[bot]",
			PRs:         []*gh.PullRequest{{Number: 99, URL: "u99"}},
		},
	}
	got := Render(entries, "https://x")
	want := `### Miscellaneous

- build: bump dep ([#99](u99)) authored by @dependabot[bot]
`
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRender_EntriesSortedBySubject(t *testing.T) {
	entries := []Entry{
		{SHA: "1", Subject: "z: zebra", AuthorLogin: "a", PRs: []*gh.PullRequest{{Number: 1, URL: "u1"}}},
		{SHA: "2", Subject: "a: apple", AuthorLogin: "b", PRs: []*gh.PullRequest{{Number: 2, URL: "u2"}}},
		{SHA: "3", Subject: "m: mango", AuthorLogin: "c", PRs: []*gh.PullRequest{{Number: 3, URL: "u3"}}},
	}
	got := Render(entries, "https://x")
	want := `### Miscellaneous

- a: apple ([#2](u2)) authored by @b
- m: mango ([#3](u3)) authored by @c
- z: zebra ([#1](u1)) authored by @a
`
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRender_AreaPrefixApplied(t *testing.T) {
	// Subject is post-strip ("Allow $ref"), Area carries the prefix.
	entries := []Entry{
		{
			SHA:         "1",
			Subject:     "Allow $ref",
			Area:        "ast",
			AuthorLogin: "deeglaze",
			PRs:         []*gh.PullRequest{{Number: 8581, URL: "u8581"}},
		},
		{
			// No area set (no signal matched) — bullet renders as-is.
			SHA: "2", Subject: "Reword README",
			AuthorLogin: "alice",
			PRs:         []*gh.PullRequest{{Number: 9000, URL: "u9000"}},
		},
	}
	got := Render(entries, "https://x")
	want := `### Miscellaneous

- Reword README ([#9000](u9000)) authored by @alice
- ast: Allow $ref ([#8581](u8581)) authored by @deeglaze
`
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRender_DependencyGroup(t *testing.T) {
	// One regular Misc entry plus three deps; expect deps collapsed under
	// a "Dependency updates; notably:" parent bullet, sorted alphabetically
	// alongside the regular entry.
	entries := []Entry{
		{
			SHA: "1", Subject: "Generate JSON Schema for the bundle manifest",
			Area: "bundle", AuthorLogin: "sspaink",
			PRs: []*gh.PullRequest{{Number: 8661, URL: "u8661"}},
		},
		{
			SHA: "2", Subject: "Bump foo from 1 to 2",
			Area: "build(deps)", AuthorLogin: "dependabot[bot]", IsDependency: true,
			PRs: []*gh.PullRequest{{Number: 100, URL: "u100"}},
		},
		{
			SHA: "3", Subject: "bump go 1.26.4",
			Area: "build", AuthorLogin: "anderseknert", IsDependency: true,
			PRs: []*gh.PullRequest{{Number: 200, URL: "u200"}},
		},
		{
			SHA: "4", Subject: "bump trivy-action",
			Area: "gha", AuthorLogin: "srenatus", IsDependency: true,
			PRs: []*gh.PullRequest{{Number: 300, URL: "u300"}},
		},
	}
	got := Render(entries, "https://x")
	want := `### Miscellaneous

- Dependency updates; notably:
  - build(deps): Bump foo from 1 to 2 ([#100](u100)) authored by @dependabot[bot]
  - build: bump go 1.26.4 ([#200](u200)) authored by @anderseknert
  - gha: bump trivy-action ([#300](u300)) authored by @srenatus
- bundle: Generate JSON Schema for the bundle manifest ([#8661](u8661)) authored by @sspaink
`
	if got != want {
		t.Errorf("Render mismatch.\nGot:\n%s\nWant:\n%s", got, want)
	}
}

func TestRender_DependencyParentOnlyAppearsWithChildren(t *testing.T) {
	entries := []Entry{
		{
			SHA: "1", Subject: "Refactor X",
			Area: "ast", AuthorLogin: "alice",
			PRs: []*gh.PullRequest{{Number: 1, URL: "u1"}},
		},
	}
	got := Render(entries, "https://x")
	if strings.Contains(got, "Dependency updates") {
		t.Errorf("expected no dependency parent, got:\n%s", got)
	}
}

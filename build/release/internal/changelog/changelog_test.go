package changelog

import (
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

func TestNormalizeSubject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"squash-merge suffix stripped", "ast: Allow $ref in $ref (#8581)", "ast: Allow $ref in $ref"},
		{"no suffix", "ast: Allow $ref in $ref", "ast: Allow $ref in $ref"},
		{"only first line used", "ast: Refactor X (#1)\n\nLong body", "ast: Refactor X"},
		{"trailing whitespace before suffix", "fix: thing (#42)   ", "fix: thing"},
		{"multi-digit suffix", "build: bump go (#12345)", "build: bump go"},
		{"hash inside subject preserved", "ast: handle # in input (#10)", "ast: handle # in input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSubject(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEntryIssuePicksLowestNumbered(t *testing.T) {
	e := Entry{
		Issues: []*gh.Issue{
			{Number: 9000, URL: "u9000"},
			{Number: 42, URL: "u42"},
			{Number: 1234, URL: "u1234"},
		},
	}
	got := e.Issue()
	if got == nil || got.Number != 42 {
		t.Fatalf("expected lowest-numbered #42, got %+v", got)
	}
}

func TestEntryPRPicksFirst(t *testing.T) {
	e := Entry{
		PRs: []*gh.PullRequest{
			{Number: 100, URL: "u100"},
			{Number: 99, URL: "u99"},
		},
	}
	got := e.PR()
	if got == nil || got.Number != 100 {
		t.Fatalf("expected first PR #100, got %+v", got)
	}
}

func TestEntryNilSelectorsWhenEmpty(t *testing.T) {
	e := Entry{}
	if e.PR() != nil {
		t.Errorf("expected nil PR")
	}
	if e.Issue() != nil {
		t.Errorf("expected nil Issue")
	}
}

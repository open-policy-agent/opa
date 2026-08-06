package changelog

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

func TestAmbiguitiesReport_AllCategories(t *testing.T) {
	entries := []Entry{
		{
			SHA: "1111111aaaa", Subject: "build: direct push",
			AuthorLogin: "alice",
			// no PRs, no issues — surfaces under "no associated PR"
		},
		{
			SHA: "2222222bbbb", Subject: "ast: cherry-pick",
			AuthorLogin: "bob",
			PRs: []*gh.PullRequest{
				{Number: 100},
				{Number: 101},
				{Number: 102},
			},
			// surfaces under "multiple PRs" — chosen #100, alts #101, #102
		},
		{
			SHA: "3333333cccc", Subject: "server: feature",
			AuthorLogin: "carol",
			PRs:         []*gh.PullRequest{{Number: 200}},
			Issues: []*gh.Issue{
				{Number: 4242},
				{Number: 1111},
				{Number: 7777},
			},
			// surfaces under "PRs with multiple closing issues" — all three
			// are listed in the bullet AND in the report so the user is
			// nudged to double-check the linkage.
		},
		{
			SHA: "4444444dddd", Subject: "docs: clean entry",
			AuthorLogin: "dan",
			PRs:         []*gh.PullRequest{{Number: 300}},
			Issues:      []*gh.Issue{{Number: 50}},
			// no ambiguity — should not appear in the report
		},
		{
			SHA: "5555555eeee", Subject: "wip: unpushed kept",
			IsLocalOnly: true,
			Issues:      []*gh.Issue{{Number: 999}},
			// --include-local was on, so this stays in entries — should
			// surface under "included in the rendered changelog".
		},
	}
	dropped := []Entry{
		{
			SHA: "6666666ffff", Subject: "wip: no refs",
			IsLocalOnly: true,
		},
		{
			SHA: "7777777aaaa", Subject: "wip: with refs",
			IsLocalOnly: true,
			Issues:      []*gh.Issue{{Number: 42}, {Number: 7}},
		},
	}
	got := AmbiguitiesReport(entries, dropped)

	mustContain := []string{
		"Review checklist (please verify before committing):",
		"Commits with no associated PR",
		"1111111", "build: direct push",

		"Commits with multiple PRs",
		"2222222", "ast: cherry-pick", "chose #100", "alternatives: #101, #102",

		"PRs with multiple closing issues",
		"please verify the linkage is correct",
		"3333333", "server: feature", "PR #200", "issues: #4242, #1111, #7777",

		"Local-only commits dropped from the rendered changelog (2)",
		"--include-local",
		"6666666", "wip: no refs", "no references in commit message",
		"7777777", "wip: with refs", "references: #42, #7",

		"Local-only commits included in the rendered changelog (1)",
		"5555555", "wip: unpushed kept", "references: #999",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("expected report to contain %q\n--- report ---\n%s", s, got)
		}
	}
	for _, s := range []string{
		"docs: clean entry",
		// no "chose / alternatives" wording for multi-issue anymore
		"chose #1111",
		"alternatives: #4242",
	} {
		if strings.Contains(got, s) {
			t.Errorf("did not expect report to contain %q:\n%s", s, got)
		}
	}
	// dropped entries must not show up in the noPR section: that section is
	// for entries that survived to rendering with no PR link.
	noPRSlice := got
	if i := strings.Index(noPRSlice, "Commits with no associated PR"); i >= 0 {
		noPRSlice = noPRSlice[i:]
		if j := strings.Index(noPRSlice[1:], "\n\n"); j >= 0 {
			noPRSlice = noPRSlice[:j]
		}
	}
	for _, s := range []string{"wip: no refs", "wip: with refs", "wip: unpushed kept"} {
		if strings.Contains(noPRSlice, s) {
			t.Errorf("local-only entries should not appear in the noPR section:\n%s", got)
		}
	}
}

func TestAmbiguitiesReport_EmptyWhenAllResolved(t *testing.T) {
	entries := []Entry{
		{
			SHA: "1111111", Subject: "ast: ok",
			AuthorLogin: "alice",
			PRs:         []*gh.PullRequest{{Number: 1}},
			Issues:      []*gh.Issue{{Number: 2}},
		},
	}
	if got := AmbiguitiesReport(entries, nil); got != "" {
		t.Errorf("expected empty report, got: %s", got)
	}
}

func TestAmbiguitiesReport_OnlyDroppedLocal(t *testing.T) {
	dropped := []Entry{
		{SHA: "abc1234", Subject: "wip: unpushed", IsLocalOnly: true},
	}
	got := AmbiguitiesReport(nil, dropped)
	for _, s := range []string{
		"Review checklist",
		"Local-only commits dropped from the rendered changelog (1)",
		"--include-local",
		"abc1234", "wip: unpushed",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("expected report to contain %q\n--- report ---\n%s", s, got)
		}
	}
}

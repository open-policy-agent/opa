package changelog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

// fakeClient is a hand-written test double for gh.Client.
type fakeClient struct {
	commits map[string]*gh.Commit
	pulls   map[string][]*gh.PullRequest
	issues  map[int][]*gh.Issue
	// missing, when true for a sha, makes Commit/PullsForCommit return
	// gh.ErrCommitNotFound — the local-only fallback path.
	missing map[string]bool
}

func (f *fakeClient) Commit(_ context.Context, sha string) (*gh.Commit, error) {
	if f.missing[sha] {
		return nil, gh.ErrCommitNotFound
	}
	if c, ok := f.commits[sha]; ok {
		return c, nil
	}
	return nil, errors.New("commit not found: " + sha)
}

func (f *fakeClient) PullsForCommit(_ context.Context, sha string) ([]*gh.PullRequest, error) {
	if f.missing[sha] {
		return nil, gh.ErrCommitNotFound
	}
	return f.pulls[sha], nil
}

func (f *fakeClient) ClosingIssues(_ context.Context, prNumber int) ([]*gh.Issue, error) {
	return f.issues[prNumber], nil
}

func (*fakeClient) IssueURL(number int) string {
	return fmt.Sprintf("https://github.com/test/test/issues/%d", number)
}

// resolveTestCase exercises a single commit through Resolve and asserts shape.
type resolveTestCase struct {
	name string

	sha     string
	message string
	commit  *gh.Commit
	pulls   []*gh.PullRequest
	issues  []*gh.Issue // attached to the first PR (if any)

	wantSubject     string
	wantAuthorLogin string
	wantPRNumber    int // 0 = expect no PR
	wantIssueNumber int // 0 = expect no issue
}

func TestResolve_TableCases(t *testing.T) {
	cases := []resolveTestCase{
		{
			name:    "PR-body-only Fixes — issue surfaced via closingIssuesReferences",
			sha:     "aaaa111",
			message: "ast: Allow $ref in allOf in JSON schemas (#8581)",
			commit:  &gh.Commit{AuthorLogin: "deeglaze"},
			pulls:   []*gh.PullRequest{{Number: 8581, URL: "https://github.com/open-policy-agent/opa/pull/8581"}},
			issues: []*gh.Issue{
				{Number: 6523, URL: "https://github.com/open-policy-agent/opa/issues/6523", ReporterLogin: "mosiac1"},
			},
			wantSubject:     "ast: Allow $ref in allOf in JSON schemas",
			wantAuthorLogin: "deeglaze",
			wantPRNumber:    8581,
			wantIssueNumber: 6523,
		},
		{
			name:    "PR with no linked issue — falls back to PR link",
			sha:     "bbbb222",
			message: "server: Wire in metadata for compile handler (#8650)",
			commit:  &gh.Commit{AuthorLogin: "srenatus"},
			pulls:   []*gh.PullRequest{{Number: 8650, URL: "https://github.com/open-policy-agent/opa/pull/8650"}},
			issues:  nil,

			wantSubject:     "server: Wire in metadata for compile handler",
			wantAuthorLogin: "srenatus",
			wantPRNumber:    8650,
			wantIssueNumber: 0,
		},
		{
			name:    "PR with multiple closing issues — lowest-numbered chosen",
			sha:     "cccc333",
			message: "topdown: refactor (#9001)",
			commit:  &gh.Commit{AuthorLogin: "alice"},
			pulls:   []*gh.PullRequest{{Number: 9001}},
			issues: []*gh.Issue{
				{Number: 4242, ReporterLogin: "bob"},
				{Number: 1111, ReporterLogin: "carol"},
				{Number: 7777, ReporterLogin: "dan"},
			},
			wantSubject:     "topdown: refactor",
			wantAuthorLogin: "alice",
			wantPRNumber:    9001,
			wantIssueNumber: 1111,
		},
		{
			name:    "Commit with multiple PRs — first chosen, others preserved",
			sha:     "dddd444",
			message: "ast: cherry-pick fix (#5000)",
			commit:  &gh.Commit{AuthorLogin: "eve"},
			pulls: []*gh.PullRequest{
				{Number: 5000, URL: "u5000"},
				{Number: 5001, URL: "u5001"},
			},
			issues:          nil,
			wantSubject:     "ast: cherry-pick fix",
			wantAuthorLogin: "eve",
			wantPRNumber:    5000,
		},
		{
			name:            "Commit with no associated PR",
			sha:             "eeee555aaaa",
			message:         "build: tweak GH actions",
			commit:          &gh.Commit{AuthorLogin: "frank"},
			pulls:           nil,
			wantSubject:     "build: tweak GH actions",
			wantAuthorLogin: "frank",
		},
		{
			name:            "Dependabot-style commit",
			sha:             "ffff666",
			message:         "build(deps): bump pg in /e2e/api/compile/prisma in the e2e-prisma group (#8729)",
			commit:          &gh.Commit{AuthorLogin: "dependabot[bot]"},
			pulls:           []*gh.PullRequest{{Number: 8729}},
			issues:          nil,
			wantSubject:     "build(deps): bump pg in /e2e/api/compile/prisma in the e2e-prisma group",
			wantAuthorLogin: "dependabot[bot]",
			wantPRNumber:    8729,
		},
		{
			name:            "Commit with no `area:` prefix",
			sha:             "11119999",
			message:         "Fix typo in docs",
			commit:          &gh.Commit{AuthorLogin: "grace"},
			pulls:           []*gh.PullRequest{{Number: 7000}},
			wantSubject:     "Fix typo in docs",
			wantAuthorLogin: "grace",
			wantPRNumber:    7000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{
				commits: map[string]*gh.Commit{tc.sha: tc.commit},
				pulls:   map[string][]*gh.PullRequest{tc.sha: tc.pulls},
				issues:  map[int][]*gh.Issue{},
			}
			if len(tc.pulls) > 0 {
				fc.issues[tc.pulls[0].Number] = tc.issues
			}
			msg := func(sha string) (string, error) {
				if sha != tc.sha {
					return "", errors.New("unexpected sha: " + sha)
				}
				return tc.message, nil
			}

			entries, err := Resolve(context.Background(), fc, []string{tc.sha}, msg, nil, nil)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			e := entries[0]
			if e.Subject != tc.wantSubject {
				t.Errorf("subject: got %q, want %q", e.Subject, tc.wantSubject)
			}
			if e.AuthorLogin != tc.wantAuthorLogin {
				t.Errorf("author: got %q, want %q", e.AuthorLogin, tc.wantAuthorLogin)
			}
			if pr := e.PR(); tc.wantPRNumber == 0 {
				if pr != nil {
					t.Errorf("expected no PR, got %+v", pr)
				}
			} else {
				if pr == nil || pr.Number != tc.wantPRNumber {
					t.Errorf("PR: got %+v, want #%d", pr, tc.wantPRNumber)
				}
			}
			if iss := e.Issue(); tc.wantIssueNumber == 0 {
				if iss != nil {
					t.Errorf("expected no issue, got %+v", iss)
				}
			} else {
				if iss == nil || iss.Number != tc.wantIssueNumber {
					t.Errorf("issue: got %+v, want #%d", iss, tc.wantIssueNumber)
				}
			}
		})
	}
}

func TestResolve_ProgressCallback(t *testing.T) {
	shas := []string{"aaaa111", "bbbb222", "cccc333"}
	subjects := map[string]string{
		"aaaa111": "ast: one (#1)",
		"bbbb222": "server: two (#2)",
		"cccc333": "docs: three (#3)",
	}
	wantSubjects := map[string]string{
		"aaaa111": "ast: one",
		"bbbb222": "server: two",
		"cccc333": "docs: three",
	}
	fc := &fakeClient{
		commits: map[string]*gh.Commit{
			"aaaa111": {AuthorLogin: "a"},
			"bbbb222": {AuthorLogin: "b"},
			"cccc333": {AuthorLogin: "c"},
		},
		pulls:  map[string][]*gh.PullRequest{},
		issues: map[int][]*gh.Issue{},
	}
	msg := func(sha string) (string, error) { return subjects[sha], nil }

	type call struct {
		done, total  int
		sha, subject string
	}
	var calls []call
	progress := func(done, total int, sha, subject string) {
		calls = append(calls, call{done, total, sha, subject})
	}

	if _, err := Resolve(context.Background(), fc, shas, msg, progress, nil); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(calls) != len(shas) {
		t.Fatalf("got %d progress calls, want %d", len(calls), len(shas))
	}
	for i, c := range calls {
		if c.done != i {
			t.Errorf("call[%d]: done=%d, want %d", i, c.done, i)
		}
		if c.total != len(shas) {
			t.Errorf("call[%d]: total=%d, want %d", i, c.total, len(shas))
		}
		if c.sha != shas[i] {
			t.Errorf("call[%d]: sha=%q, want %q", i, c.sha, shas[i])
		}
		if c.subject != wantSubjects[c.sha] {
			t.Errorf("call[%d]: subject=%q, want %q (normalized)", i, c.subject, wantSubjects[c.sha])
		}
	}
}

func TestResolve_LocalOnlyCommit(t *testing.T) {
	cases := []struct {
		name        string
		message     string
		wantSubject string
		wantRefs    []int
	}{
		{
			name:        "no closing keyword — empty refs",
			message:     "ce25b2b: updated dependency transform\n\nWork in progress.\n",
			wantSubject: "ce25b2b: updated dependency transform",
			wantRefs:    nil,
		},
		{
			name:        "Fixes: #1234 trailer",
			message:     "ast: tweak something\n\nDetails.\n\nFixes: #1234\nSigned-off-by: Dev <dev@example.org>\n",
			wantSubject: "ast: tweak something",
			wantRefs:    []int{1234},
		},
		{
			name:        "Fixes #N without colon",
			message:     "server: change\n\nFixes #42\n",
			wantSubject: "server: change",
			wantRefs:    []int{42},
		},
		{
			name:        "multiple keywords (case-insensitive, dedup)",
			message:     "topdown: refactor\n\ncloses #1, Resolves: #2, fixes #1\n",
			wantSubject: "topdown: refactor",
			wantRefs:    []int{1, 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{
				missing: map[string]bool{"abc1234": true},
			}
			msg := func(sha string) (string, error) {
				if sha != "abc1234" {
					return "", errors.New("unexpected sha")
				}
				return tc.message, nil
			}
			entries, err := Resolve(context.Background(), fc, []string{"abc1234"}, msg, nil, nil)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(entries))
			}
			e := entries[0]
			if !e.IsLocalOnly {
				t.Errorf("expected IsLocalOnly=true")
			}
			if e.AuthorLogin != "" {
				t.Errorf("expected empty AuthorLogin, got %q", e.AuthorLogin)
			}
			if len(e.PRs) != 0 {
				t.Errorf("expected no PRs, got %d", len(e.PRs))
			}
			if e.Subject != tc.wantSubject {
				t.Errorf("subject: got %q, want %q", e.Subject, tc.wantSubject)
			}
			if len(e.Issues) != len(tc.wantRefs) {
				t.Fatalf("issues: got %d, want %d (%v)", len(e.Issues), len(tc.wantRefs), tc.wantRefs)
			}
			for i, want := range tc.wantRefs {
				if e.Issues[i].Number != want {
					t.Errorf("issue[%d]: got #%d, want #%d", i, e.Issues[i].Number, want)
				}
				if !strings.Contains(e.Issues[i].URL, fmt.Sprintf("/issues/%d", want)) {
					t.Errorf("issue[%d] URL %q does not contain /issues/%d", i, e.Issues[i].URL, want)
				}
			}
		})
	}
}

func TestResolve_ResolvedCallback(t *testing.T) {
	fc := &fakeClient{
		commits: map[string]*gh.Commit{"aaaa111": {AuthorLogin: "alice"}},
		pulls:   map[string][]*gh.PullRequest{"aaaa111": {{Number: 1}}},
		issues:  map[int][]*gh.Issue{},
		missing: map[string]bool{"bbbb222": true},
	}
	subjects := map[string]string{
		"aaaa111": "ast: pushed change (#1)",
		"bbbb222": "wip: local change\n\nFixes: #99\n",
	}
	msg := func(sha string) (string, error) { return subjects[sha], nil }

	var seen []bool
	resolved := func(e *Entry) { seen = append(seen, e.IsLocalOnly) }

	entries, err := Resolve(context.Background(), fc, []string{"aaaa111", "bbbb222"}, msg, nil, resolved)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(seen) != 2 || seen[0] != false || seen[1] != true {
		t.Errorf("ResolvedFunc IsLocalOnly sequence: got %v, want [false true]", seen)
	}
	if !entries[1].IsLocalOnly {
		t.Errorf("entry[1].IsLocalOnly: got false, want true")
	}
	if len(entries[1].Issues) != 1 || entries[1].Issues[0].Number != 99 {
		t.Errorf("entry[1].Issues: got %+v, want [#99]", entries[1].Issues)
	}
}

func TestDropLocalOnly(t *testing.T) {
	entries := []Entry{
		{SHA: "1"},
		{SHA: "2", IsLocalOnly: true},
		{SHA: "3"},
		{SHA: "4", IsLocalOnly: true},
	}
	kept, dropped := DropLocalOnly(entries)
	if len(kept) != 2 || kept[0].SHA != "1" || kept[1].SHA != "3" {
		t.Errorf("kept: got %+v, want [{1} {3}]", kept)
	}
	if len(dropped) != 2 || dropped[0].SHA != "2" || dropped[1].SHA != "4" {
		t.Errorf("dropped: got %+v, want [{2} {4}]", dropped)
	}
}

package fixture

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

// stubClient is the upstream client the Recorder wraps in these tests.
type stubClient struct {
	commits map[string]*gh.Commit
	pulls   map[string][]*gh.PullRequest
	issues  map[int][]*gh.Issue
	missing map[string]bool
}

func (s *stubClient) Commit(_ context.Context, sha string) (*gh.Commit, error) {
	if s.missing[sha] {
		return nil, gh.ErrCommitNotFound
	}
	c, ok := s.commits[sha]
	if !ok {
		return nil, errors.New("no such commit: " + sha)
	}
	return c, nil
}

func (s *stubClient) PullsForCommit(_ context.Context, sha string) ([]*gh.PullRequest, error) {
	if s.missing[sha] {
		return nil, gh.ErrCommitNotFound
	}
	return s.pulls[sha], nil
}

func (s *stubClient) ClosingIssues(_ context.Context, n int) ([]*gh.Issue, error) {
	return s.issues[n], nil
}

func (*stubClient) IssueURL(n int) string {
	return "https://github.com/test/test/issues/" + strconv.Itoa(n)
}

// TestRoundTrip records a run against a stub, writes it to disk, reads it back,
// and asserts the replay client answers identically — including the
// commit-not-found path that drives the local-only fallback.
func TestRoundTrip(t *testing.T) {
	ctx := t.Context()
	stub := &stubClient{
		commits: map[string]*gh.Commit{
			"aaa": {SHA: "aaa", AuthorLogin: "alice", Files: []string{"v1/ast/parser.go"}},
			"bbb": {SHA: "bbb", AuthorLogin: "dependabot[bot]", Files: []string{"go.mod"}},
		},
		pulls: map[string][]*gh.PullRequest{
			"aaa": {{Number: 10, URL: "u/10", Labels: []string{"area/ast"}}},
			"bbb": {{Number: 11, URL: "u/11"}},
		},
		issues: map[int][]*gh.Issue{
			10: {{Number: 5, URL: "u/i5", ReporterLogin: "bob"}},
			11: nil,
		},
		missing: map[string]bool{"ccc": true},
	}

	shas := []string{"aaa", "bbb", "ccc"}
	rec := NewRecorder(stub, Meta{Repo: "test/test", From: "v1.0.0", To: "v1.1.0"})
	msg := rec.WrapMessage(func(sha string) (string, error) { return "subject for " + sha + "\n", nil })

	for _, sha := range shas {
		if _, err := msg(sha); err != nil {
			t.Fatalf("message %s: %v", sha, err)
		}
		c, err := rec.Commit(ctx, sha)
		if sha == "ccc" {
			if !errors.Is(err, gh.ErrCommitNotFound) {
				t.Fatalf("commit ccc: want ErrCommitNotFound, got (%v, %v)", c, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("commit %s: %v", sha, err)
		}
		prs, err := rec.PullsForCommit(ctx, sha)
		if err != nil {
			t.Fatalf("pulls %s: %v", sha, err)
		}
		if _, err := rec.ClosingIssues(ctx, prs[0].Number); err != nil {
			t.Fatalf("closing issues %s: %v", sha, err)
		}
	}

	captured := rec.Recording()
	captured.SHAs = shas
	captured.FromGoMod = "module x\n\nrequire foo v1.0.0\n"
	captured.ToGoMod = "module x\n\nrequire foo v1.1.0\n"

	dir := filepath.Join(t.TempDir(), "v1.1.0")
	if err := Write(dir, captured); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !reflect.DeepEqual(loaded.Meta, captured.Meta) {
		t.Errorf("meta mismatch: got %+v, want %+v", loaded.Meta, captured.Meta)
	}
	if !reflect.DeepEqual(loaded.SHAs, shas) {
		t.Errorf("shas mismatch: got %v, want %v", loaded.SHAs, shas)
	}
	if loaded.FromGoMod != captured.FromGoMod || loaded.ToGoMod != captured.ToGoMod {
		t.Error("go.mod snapshots did not round-trip")
	}

	replayed := loaded.Client()
	for _, sha := range shas {
		wantMsg := captured.Messages[sha]
		gotMsg, err := loaded.Message(sha)
		if err != nil || gotMsg != wantMsg {
			t.Errorf("Message(%s) = (%q, %v), want (%q, nil)", sha, gotMsg, err, wantMsg)
		}

		gotCommit, err := replayed.Commit(ctx, sha)
		wantCommit, wantErr := stub.Commit(ctx, sha)
		if (err != nil) != (wantErr != nil) {
			t.Errorf("Commit(%s) error = %v, want %v", sha, err, wantErr)
		}
		if !reflect.DeepEqual(gotCommit, wantCommit) {
			t.Errorf("Commit(%s) = %+v, want %+v", sha, gotCommit, wantCommit)
		}

		gotPulls, err := replayed.PullsForCommit(ctx, sha)
		wantPulls, wantPullsErr := stub.PullsForCommit(ctx, sha)
		if (err != nil) != (wantPullsErr != nil) {
			t.Errorf("PullsForCommit(%s) error = %v, want %v", sha, err, wantPullsErr)
		}
		if !reflect.DeepEqual(gotPulls, wantPulls) {
			t.Errorf("PullsForCommit(%s) = %+v, want %+v", sha, gotPulls, wantPulls)
		}
	}

	if got := replayed.IssueURL(5); got != "https://github.com/test/test/issues/5" {
		t.Errorf("IssueURL(5) = %q", got)
	}
}

// TestReplayFailsLoudlyOnMissingRecords asserts a stale fixture errors rather
// than silently rendering an entry with no PR or issue.
func TestReplayFailsLoudlyOnMissingRecords(t *testing.T) {
	rec := &Recording{
		Meta:     Meta{Repo: "test/test"},
		Messages: map[string]string{},
		Commits:  map[string]commitRecord{},
		Pulls:    map[string][]*gh.PullRequest{},
		Closing:  map[int][]*gh.Issue{},
	}
	c := rec.Client()
	ctx := t.Context()

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"commit", func() error { _, err := c.Commit(ctx, "zzz"); return err }},
		{"pulls", func() error { _, err := c.PullsForCommit(ctx, "zzz"); return err }},
		{"closing issues", func() error { _, err := c.ClosingIssues(ctx, 99); return err }},
		{"message", func() error { _, err := rec.Message("zzz"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Error("expected an error for an unrecorded lookup, got nil")
			}
		})
	}
}

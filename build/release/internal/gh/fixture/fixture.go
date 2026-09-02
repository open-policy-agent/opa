// Package fixture records the GitHub and git responses a changelog run consumes
// (via the tool's --record flag) and replays them from disk for the golden tests.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

// Split by concern so a diff shows which kind of data drifted.
const (
	metaFile      = "meta.json"
	shasFile      = "shas.json"
	messagesFile  = "messages.json"
	commitsFile   = "commits.json"
	pullsFile     = "pulls.json"
	closingFile   = "closing.json"
	fromGoModFile = "go.mod.from"
	toGoModFile   = "go.mod.to"
)

type Meta struct {
	Repo string `json:"repo"`
	From string `json:"from"`
	To   string `json:"to"`
}

type commitRecord struct {
	Commit  *gh.Commit `json:"commit,omitempty"`
	Missing bool       `json:"missing,omitempty"`
}

// Recording is one changelog run's inputs.
type Recording struct {
	Meta     Meta
	SHAs     []string
	Messages map[string]string
	Commits  map[string]commitRecord
	Pulls    map[string][]*gh.PullRequest
	Closing  map[int][]*gh.Issue

	FromGoMod string
	ToGoMod   string
}

// Recorder is a pass-through gh.Client that captures every response.
type Recorder struct {
	inner gh.Client
	rec   *Recording
}

func NewRecorder(inner gh.Client, meta Meta) *Recorder {
	return &Recorder{
		inner: inner,
		rec: &Recording{
			Meta:     meta,
			Messages: map[string]string{},
			Commits:  map[string]commitRecord{},
			Pulls:    map[string][]*gh.PullRequest{},
			Closing:  map[int][]*gh.Issue{},
		},
	}
}

// Recording returns the capture so far. Callers set the git-sourced fields
// (SHAs, FromGoMod, ToGoMod) themselves.
func (r *Recorder) Recording() *Recording { return r.rec }

func (r *Recorder) Commit(ctx context.Context, sha string) (*gh.Commit, error) {
	c, err := r.inner.Commit(ctx, sha)
	switch {
	case err == nil:
		r.rec.Commits[sha] = commitRecord{Commit: c}
	case isNotFound(err):
		r.rec.Commits[sha] = commitRecord{Missing: true}
	}
	return c, err
}

func (r *Recorder) PullsForCommit(ctx context.Context, sha string) ([]*gh.PullRequest, error) {
	prs, err := r.inner.PullsForCommit(ctx, sha)
	if err == nil {
		r.rec.Pulls[sha] = prs
	}
	return prs, err
}

func (r *Recorder) ClosingIssues(ctx context.Context, prNumber int) ([]*gh.Issue, error) {
	issues, err := r.inner.ClosingIssues(ctx, prNumber)
	if err == nil {
		r.rec.Closing[prNumber] = issues
	}
	return issues, err
}

func (r *Recorder) IssueURL(number int) string { return r.inner.IssueURL(number) }

// WrapMessage captures commit messages alongside the API responses.
func (r *Recorder) WrapMessage(next func(string) (string, error)) func(string) (string, error) {
	return func(sha string) (string, error) {
		msg, err := next(sha)
		if err == nil {
			r.rec.Messages[sha] = msg
		}
		return msg, err
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, gh.ErrCommitNotFound)
}

func Write(dir string, rec *Recording) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	writes := []struct {
		name string
		val  any
	}{
		{metaFile, rec.Meta},
		{shasFile, rec.SHAs},
		{messagesFile, rec.Messages},
		{commitsFile, rec.Commits},
		{pullsFile, rec.Pulls},
		{closingFile, rec.Closing},
	}
	for _, w := range writes {
		if err := writeJSON(filepath.Join(dir, w.name), w.val); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, fromGoModFile), []byte(rec.FromGoMod), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, toGoModFile), []byte(rec.ToGoMod), 0o644)
}

func Read(dir string) (*Recording, error) {
	rec := &Recording{}
	reads := []struct {
		name string
		val  any
	}{
		{metaFile, &rec.Meta},
		{shasFile, &rec.SHAs},
		{messagesFile, &rec.Messages},
		{commitsFile, &rec.Commits},
		{pullsFile, &rec.Pulls},
		{closingFile, &rec.Closing},
	}
	for _, r := range reads {
		if err := readJSON(filepath.Join(dir, r.name), r.val); err != nil {
			return nil, err
		}
	}
	from, err := os.ReadFile(filepath.Join(dir, fromGoModFile))
	if err != nil {
		return nil, err
	}
	rec.FromGoMod = string(from)
	to, err := os.ReadFile(filepath.Join(dir, toGoModFile))
	if err != nil {
		return nil, err
	}
	rec.ToGoMod = string(to)
	return rec, nil
}

// Client answers from the recording. Unknown keys are errors, not empty results,
// so a stale fixture fails loudly.
func (rec *Recording) Client() gh.Client { return (*replay)(rec) }

func (rec *Recording) Message(sha string) (string, error) {
	msg, ok := rec.Messages[sha]
	if !ok {
		return "", fmt.Errorf("fixture has no commit message for %s", sha)
	}
	return msg, nil
}

type replay Recording

func (r *replay) Commit(_ context.Context, sha string) (*gh.Commit, error) {
	rec, ok := r.Commits[sha]
	if !ok {
		return nil, fmt.Errorf("fixture has no commit record for %s", sha)
	}
	if rec.Missing {
		return nil, gh.ErrCommitNotFound
	}
	return rec.Commit, nil
}

func (r *replay) PullsForCommit(_ context.Context, sha string) ([]*gh.PullRequest, error) {
	if rec, ok := r.Commits[sha]; ok && rec.Missing {
		return nil, gh.ErrCommitNotFound
	}
	prs, ok := r.Pulls[sha]
	if !ok {
		return nil, fmt.Errorf("fixture has no PR record for %s", sha)
	}
	return prs, nil
}

func (r *replay) ClosingIssues(_ context.Context, prNumber int) ([]*gh.Issue, error) {
	issues, ok := r.Closing[prNumber]
	if !ok {
		return nil, fmt.Errorf("fixture has no closing-issues record for PR %d", prNumber)
	}
	return issues, nil
}

func (r *replay) IssueURL(number int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", r.Meta.Repo, number)
}

func writeJSON(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(buf, '\n'), 0o644)
}

func readJSON(path string, v any) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

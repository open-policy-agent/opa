// Package gh wraps the GitHub API calls the release tooling needs.
package gh

import (
	"context"
	"errors"
)

// ErrCommitNotFound means the SHA is not on the remote — usually an unpushed
// commit. Callers fall back to parsing the local commit message.
var ErrCommitNotFound = errors.New("commit not found on remote")

type Commit struct {
	SHA         string
	AuthorLogin string
	Files       []string
}

type PullRequest struct {
	Number int
	URL    string
	Labels []string
}

type Issue struct {
	Number        int
	URL           string
	ReporterLogin string
}

type Client interface {
	Commit(ctx context.Context, sha string) (*Commit, error)
	// PullsForCommit returns PRs in the API's order; callers handle the >1 case.
	PullsForCommit(ctx context.Context, sha string) ([]*PullRequest, error)
	// ClosingIssues reads the PR's "Development" panel, which covers both
	// UI-linked issues and keyword-parsed Fixes/Closes/Resolves references.
	ClosingIssues(ctx context.Context, prNumber int) ([]*Issue, error)
	// IssueURL builds a URL for a reference of unknown kind; GitHub redirects
	// /issues/N to the PR view when N is a PR.
	IssueURL(number int) string
}

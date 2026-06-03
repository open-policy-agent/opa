// Package changelog turns a range of commits into a CHANGELOG.md section.
//
// Resolve queries GitHub; Generate runs everything after it and is pure.
package changelog

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

// Entry is the resolved view of a single commit. Ambiguity is preserved so the
// rendered output and the review summary consult the same data.
type Entry struct {
	SHA         string
	Subject     string
	AuthorLogin string

	PRs    []*gh.PullRequest
	Issues []*gh.Issue
	Files  []string

	Area         string
	IsDependency bool
	// IsSynthetic entries come from the go.mod diff, not a commit, so they
	// render as bare bullets with no link or attribution.
	IsSynthetic bool
	// IsLocalOnly entries have no remote metadata; their Issues come from
	// Fixes/Closes/Resolves trailers in the local commit message.
	IsLocalOnly bool
}

// PR returns the first associated PR, or nil.
func (e *Entry) PR() *gh.PullRequest {
	if len(e.PRs) == 0 {
		return nil
	}
	return e.PRs[0]
}

// Issue returns the lowest-numbered closing issue, or nil.
func (e *Entry) Issue() *gh.Issue {
	if len(e.Issues) == 0 {
		return nil
	}
	pick := e.Issues[0]
	for _, i := range e.Issues[1:] {
		if i.Number < pick.Number {
			pick = i
		}
	}
	return pick
}

// MessageFunc fetches the commit message for sha, injected so tests need no git
// repo.
type MessageFunc func(sha string) (string, error)

// ProgressFunc and ResolvedFunc are optional stderr-logging hooks, called before
// and after each commit is resolved.
type (
	ProgressFunc func(done, total int, sha, subject string)
	ResolvedFunc func(*Entry)
)

// Resolve builds Entries from shas, which should be newest-first.
func Resolve(ctx context.Context, client gh.Client, shas []string, msg MessageFunc, progress ProgressFunc, resolved ResolvedFunc) ([]Entry, error) {
	out := make([]Entry, 0, len(shas))
	for i, sha := range shas {
		raw, err := msg(sha)
		if err != nil {
			return nil, fmt.Errorf("read commit %s: %w", sha, err)
		}
		subject := normalizeSubject(raw)
		if progress != nil {
			progress(i, len(shas), sha, subject)
		}
		entry, err := resolveOne(ctx, client, sha, subject, raw)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
		if resolved != nil {
			resolved(&out[len(out)-1])
		}
	}
	return out, nil
}

func resolveOne(ctx context.Context, client gh.Client, sha, subject, rawMessage string) (Entry, error) {
	c, err := client.Commit(ctx, sha)
	if err != nil {
		if errors.Is(err, gh.ErrCommitNotFound) {
			return resolveLocalOnly(client, sha, subject, rawMessage), nil
		}
		return Entry{}, err
	}
	prs, err := client.PullsForCommit(ctx, sha)
	if err != nil {
		if errors.Is(err, gh.ErrCommitNotFound) {
			return resolveLocalOnly(client, sha, subject, rawMessage), nil
		}
		return Entry{}, err
	}
	e := Entry{
		SHA:         sha,
		Subject:     subject,
		AuthorLogin: c.AuthorLogin,
		PRs:         prs,
		Files:       c.Files,
	}
	if len(prs) > 0 {
		issues, err := client.ClosingIssues(ctx, prs[0].Number)
		if err != nil {
			return Entry{}, err
		}
		e.Issues = issues
	}
	return e, nil
}

// resolveLocalOnly leaves AuthorLogin empty: a local git author name is not a
// GitHub login.
func resolveLocalOnly(client gh.Client, sha, subject, rawMessage string) Entry {
	e := Entry{
		SHA:         sha,
		Subject:     subject,
		IsLocalOnly: true,
	}
	for _, n := range parseClosingRefs(rawMessage) {
		e.Issues = append(e.Issues, &gh.Issue{Number: n, URL: client.IssueURL(n)})
	}
	return e
}

var closingRefPattern = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+#(\d+)\b`)

// DropLocalOnly partitions entries by whether they exist on the remote,
// preserving order within each slice.
func DropLocalOnly(entries []Entry) (kept, dropped []Entry) {
	for i := range entries {
		if entries[i].IsLocalOnly {
			dropped = append(dropped, entries[i])
		} else {
			kept = append(kept, entries[i])
		}
	}
	return kept, dropped
}

// parseClosingRefs returns referenced numbers in source order, deduplicated.
func parseClosingRefs(msg string) []int {
	matches := closingRefPattern.FindAllStringSubmatch(msg, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(matches))
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

// trailingPRSuffix is the " (#NNN)" GitHub appends on squash-merge; the bullet
// emits its own link.
var trailingPRSuffix = regexp.MustCompile(`\s+\(#\d+\)\s*$`)

func normalizeSubject(commitMessage string) string {
	subj := commitMessage
	if i := strings.IndexByte(subj, '\n'); i >= 0 {
		subj = subj[:i]
	}
	subj = strings.TrimRight(subj, " \r\t")
	return trailingPRSuffix.ReplaceAllString(subj, "")
}

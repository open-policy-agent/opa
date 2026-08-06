package changelog

import (
	"regexp"
	"strings"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

// TransformLog is one line of provenance, printed to stderr for review.
type TransformLog struct {
	SHA     string
	Subject string
	Area    string
	// Source is how Area was determined: "existing prefix", `path "v1/ast/"`,
	// or "" when no area was set.
	Source       string
	IsDependency bool
}

// Transform derives an area, normalizes the subject and flags dependency bumps,
// in place. Returns one log per entry, in order.
func Transform(entries []Entry) []TransformLog {
	logs := make([]TransformLog, len(entries))
	for i := range entries {
		e := &entries[i]
		labels := allLabels(e.PRs)

		ar := DeriveArea(e.Subject, labels, e.Files)
		e.Subject = capitalizeSubject(ar.Subject)
		e.Area = ar.Area

		e.IsDependency = isDependency(e.AuthorLogin, e.Subject, e.Area)

		logs[i] = TransformLog{
			SHA:          e.SHA,
			Subject:      e.Subject,
			Area:         e.Area,
			Source:       ar.Source,
			IsDependency: e.IsDependency,
		}
	}
	return logs
}

// plainLowerWord deliberately excludes identifiers ("json.verify_schema"),
// hyphenated tool names ("golangci-lint") and anything starting with a
// backtick: capitalizing those changes a symbol rather than a sentence.
var plainLowerWord = regexp.MustCompile(`^[a-z]+(\s|$)`)

func capitalizeSubject(subject string) string {
	if !plainLowerWord.MatchString(subject) {
		return subject
	}
	return strings.ToUpper(subject[:1]) + subject[1:]
}

func isDependency(author, subject, area string) bool {
	if author == "dependabot[bot]" {
		return true
	}
	if strings.HasPrefix(area, "build(deps") {
		return true
	}
	if area == "deps" || area == "dependencies" {
		return true
	}
	lower := strings.ToLower(subject)
	switch {
	case strings.HasPrefix(lower, "build(deps"):
		return true
	case (area == "build" || area == "gha") && strings.Contains(lower, "bump"):
		return true
	case strings.HasPrefix(lower, "build:") && strings.Contains(lower, "bump"):
		return true
	case strings.HasPrefix(lower, "gha:") && strings.Contains(lower, "bump"):
		return true
	}
	return false
}

func allLabels(prs []*gh.PullRequest) []string {
	if len(prs) == 0 {
		return nil
	}
	out := make([]string, 0)
	for _, pr := range prs {
		out = append(out, pr.Labels...)
	}
	return out
}

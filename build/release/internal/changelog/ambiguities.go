package changelog

import (
	"fmt"
	"strings"
)

// AmbiguitiesReport summarizes what the maintainer should check by hand:
// multi-PR commits, multi-issue PRs, PR-less commits and local-only commits.
// Empty when there is nothing to report.
func AmbiguitiesReport(entries []Entry, droppedLocal []Entry) string {
	var multiPR, multiIssue, noPR, localKept []string

	for i := range entries {
		e := &entries[i]
		// No commit behind them, and the synthesis log already reports them.
		if e.IsSynthetic {
			continue
		}
		short := shortSHA(e.SHA)
		switch {
		case e.IsLocalOnly:
			localKept = append(localKept, formatLocalOnly(short, e))
		case len(e.PRs) == 0:
			noPR = append(noPR, fmt.Sprintf("- %s %q", short, e.Subject))
		case len(e.PRs) > 1:
			parts := make([]string, len(e.PRs))
			for j, pr := range e.PRs {
				parts[j] = fmt.Sprintf("#%d", pr.Number)
			}
			multiPR = append(multiPR, fmt.Sprintf("- %s %q — chose %s, alternatives: %s",
				short, e.Subject, parts[0], strings.Join(parts[1:], ", ")))
		}
		if len(e.Issues) > 1 && !e.IsLocalOnly {
			pr := e.PR()
			nums := make([]string, len(e.Issues))
			for j, iss := range e.Issues {
				nums[j] = fmt.Sprintf("#%d", iss.Number)
			}
			line := fmt.Sprintf("- %s %q", short, e.Subject)
			if pr != nil {
				line += fmt.Sprintf(" (PR #%d)", pr.Number)
			}
			line += " — issues: " + strings.Join(nums, ", ")
			multiIssue = append(multiIssue, line)
		}
	}

	localDropped := make([]string, 0, len(droppedLocal))
	for i := range droppedLocal {
		e := &droppedLocal[i]
		localDropped = append(localDropped, formatLocalOnly(shortSHA(e.SHA), e))
	}

	if len(multiPR) == 0 && len(multiIssue) == 0 && len(noPR) == 0 && len(localKept) == 0 && len(localDropped) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Review checklist (please verify before committing):\n\n")
	if len(localDropped) > 0 {
		fmt.Fprintf(&b, "Local-only commits dropped from the rendered changelog (%d) — pass --include-local to include them:\n", len(localDropped))
		for _, line := range localDropped {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(localKept) > 0 {
		fmt.Fprintf(&b, "Local-only commits included in the rendered changelog (%d) — push or remove before release:\n", len(localKept))
		for _, line := range localKept {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(noPR) > 0 {
		b.WriteString("Commits with no associated PR (using commit-SHA link):\n")
		for _, line := range noPR {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(multiPR) > 0 {
		b.WriteString("Commits with multiple PRs (first match used):\n")
		for _, line := range multiPR {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	if len(multiIssue) > 0 {
		b.WriteString("PRs with multiple closing issues (all listed in the bullet — please verify the linkage is correct):\n")
		for _, line := range multiIssue {
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func formatLocalOnly(short string, e *Entry) string {
	refs := make([]string, 0, len(e.Issues))
	for _, iss := range e.Issues {
		refs = append(refs, fmt.Sprintf("#%d", iss.Number))
	}
	line := fmt.Sprintf("- %s %q", short, e.Subject)
	if len(refs) > 0 {
		line += " — references: " + strings.Join(refs, ", ")
	} else {
		line += " — no references in commit message"
	}
	return line
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

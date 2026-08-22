package changelog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/build/release/internal/gh"
)

const dependencyParent = "Dependency updates; notably:"

// Render produces the section body: one "### Miscellaneous" list, sorted by
// subject, with dependency bumps nested under a parent bullet.
//
// There is deliberately no separate "### Fixes" section — the maintainer
// re-groups into topical sections by hand, so a machine-chosen split on "has a
// linked issue" only needs undoing first.
func Render(entries []Entry, repoURL string) string {
	var misc, deps []string
	for i := range entries {
		e := &entries[i]
		bullet := renderBullet(e, repoURL)
		if e.IsDependency {
			deps = append(deps, bullet)
		} else {
			misc = append(misc, bullet)
		}
	}
	sort.Strings(misc)
	sort.Strings(deps)

	if len(deps) > 0 {
		misc = append(misc, dependencyParent)
		sort.Strings(misc)
	}
	if len(misc) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Miscellaneous\n\n")
	for _, line := range misc {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteByte('\n')
		if line == dependencyParent {
			for _, child := range deps {
				b.WriteString("  - ")
				b.WriteString(child)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func renderBullet(e *Entry, repoURL string) string {
	subject := e.Subject
	if e.Area != "" {
		subject = e.Area + ": " + subject
	}
	if e.IsSynthetic {
		return subject
	}

	link := chooseLink(e, repoURL)
	attribution := renderAttribution(e)

	bullet := fmt.Sprintf("%s (%s)", subject, link)
	if attribution != "" {
		bullet += " " + attribution
	}
	return bullet
}

// chooseLink prefers issues over the PR over the commit. All closing issues are
// listed so none is dropped.
func chooseLink(e *Entry, repoURL string) string {
	if len(e.Issues) > 0 {
		sorted := append([]*gh.Issue(nil), e.Issues...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })
		parts := make([]string, len(sorted))
		for i, iss := range sorted {
			parts[i] = fmt.Sprintf("[#%d](%s)", iss.Number, iss.URL)
		}
		return strings.Join(parts, ", ")
	}
	if pr := e.PR(); pr != nil {
		return fmt.Sprintf("[#%d](%s)", pr.Number, pr.URL)
	}
	short := e.SHA
	if len(short) > 7 {
		short = short[:7]
	}
	return fmt.Sprintf("[`%s`](%s/commit/%s)", short, strings.TrimRight(repoURL, "/"), e.SHA)
}

func renderAttribution(e *Entry) string {
	author := e.AuthorLogin
	var reporter string
	if iss := e.Issue(); iss != nil {
		reporter = iss.ReporterLogin
	}
	switch {
	case author == "" && reporter == "":
		return ""
	case author != "" && reporter == "":
		return "authored by @" + author
	case author == "" && reporter != "":
		return "reported by @" + reporter
	case author == reporter:
		return "reported and authored by @" + author
	default:
		return fmt.Sprintf("authored by @%s, reported by @%s", author, reporter)
	}
}

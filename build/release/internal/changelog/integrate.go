package changelog

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var releaseTitle = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// subsectionHeading matches "### X" but not "#### X".
var subsectionHeading = regexp.MustCompile(`^###[ \t]+([^#].*?)[ \t]*$`)

// DropReleased removes the "## Unreleased" subsections that section already
// carries, returning their headings. A subsection matches on its heading or its
// body, since one of them is often edited for the release.
func DropReleased(existing, section string) (string, []string) {
	released := subsections(strings.Split(section, "\n"))
	headings := map[string]bool{}
	bodies := map[string]bool{}
	for _, s := range released {
		headings[s.title] = true
		if s.body != "" {
			bodies[s.body] = true
		}
	}

	lines := strings.Split(existing, "\n")
	sections := findHeadings(lines)
	i, ok := indexOfTitle(sections, unreleasedTitle)
	if !ok {
		return existing, nil
	}
	start, end := sections[i].line+1, sectionEnd(sections, i, len(lines))

	var dropped []string
	out := slices.Clone(lines[:start])
	next := start
	for _, s := range subsections(lines[start:end]) {
		if !headings[s.title] && !bodies[s.body] {
			continue
		}
		out = append(out, lines[next:start+s.from]...)
		next = start + s.to
		dropped = append(dropped, s.title)
	}
	if len(dropped) == 0 {
		return existing, nil
	}
	out = append(out, lines[next:]...)
	return join(out), dropped
}

type subsection struct {
	title, body string
	from, to    int
}

// subsections splits lines at "### " headings; text above the first is skipped.
func subsections(lines []string) []subsection {
	var out []subsection
	for i, l := range lines {
		m := subsectionHeading.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		if n := len(out); n > 0 {
			out[n-1].to = i
		}
		out = append(out, subsection{title: m[1], from: i, to: len(lines)})
	}
	for i := range out {
		out[i].body = strings.TrimSpace(strings.Join(lines[out[i].from+1:out[i].to], "\n"))
	}
	return out
}

// ExtractSection returns version's section from a CHANGELOG, heading included.
func ExtractSection(content, version string) (string, error) {
	lines := strings.Split(content, "\n")
	headings := findHeadings(lines)
	i, ok := indexOfTitle(headings, version)
	if !ok {
		return "", fmt.Errorf("no %q section", "## "+version)
	}
	section := lines[headings[i].line:sectionEnd(headings, i, len(lines))]
	return strings.Join(trimTrailingBlank(section), "\n") + "\n", nil
}

// InsertSection adds section above the first release older than version, so a
// patch for an older minor lands below the newer minors' sections.
func InsertSection(existing, version, section string) (string, error) {
	v, ok := parseRelease(version)
	if !ok {
		return "", fmt.Errorf("invalid release version %q, expected X.Y.Z", version)
	}

	lines := strings.Split(existing, "\n")
	headings := findHeadings(lines)
	if _, ok := indexOfTitle(headings, version); ok {
		return "", fmt.Errorf("%q: %w", "## "+version, ErrSectionExists)
	}

	at := len(lines)
	for _, h := range headings {
		if hv, ok := parseRelease(h.title); ok && less(hv, v) {
			at = h.line
			break
		}
	}
	return join(insertAt(lines, at, blockLines(section))), nil
}

func parseRelease(title string) ([3]int, bool) {
	m := releaseTitle.FindStringSubmatch(title)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

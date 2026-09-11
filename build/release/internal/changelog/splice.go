package changelog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const unreleasedTitle = "unreleased"

// ErrSectionExists lets callers tell "already done" apart from a real failure.
var ErrSectionExists = errors.New("section already present")

// sectionHeading excludes "#" after "##", so "### Miscellaneous" does not match.
var sectionHeading = regexp.MustCompile(`^##[ \t]+([^#].*?)[ \t]*$`)

// NormalizeVersion strips a leading "v"; CHANGELOG headings use the bare form.
func NormalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// Section prefixes body with its "## <version>" heading. Entries carry no date.
func Section(version, body string) string {
	if version == "" {
		return body
	}
	return "## " + version + "\n\n" + body
}

// Splice returns the new file contents. A "## Unreleased" heading is renamed to
// "## <version>" and the bullets appended to the end of that section, after any
// hand-written prose; otherwise a new section goes above the topmost release.
// An existing "## <version>" returns ErrSectionExists, so a re-run cannot
// duplicate it.
func Splice(existing, version, body string) (string, error) {
	if version == "" {
		return "", errors.New("version is required to splice a CHANGELOG section")
	}

	lines := strings.Split(existing, "\n")
	headings := findHeadings(lines)

	for _, h := range headings {
		if h.title == version {
			return "", fmt.Errorf("%q: %w", "## "+version, ErrSectionExists)
		}
	}

	block := blockLines(body)

	if i, ok := indexOfTitle(headings, unreleasedTitle); ok {
		lines[headings[i].line] = "## " + version
		if len(block) == 0 {
			return join(lines), nil
		}
		return join(insertAt(lines, sectionEnd(headings, i, len(lines)), block)), nil
	}

	at := len(lines)
	if len(headings) > 0 {
		at = headings[0].line
	}
	section := []string{"## " + version}
	if len(block) > 0 {
		section = append(section, "")
		section = append(section, block...)
	}
	return join(insertAt(lines, at, section)), nil
}

type heading struct {
	line  int
	title string
}

func findHeadings(lines []string) []heading {
	var out []heading
	for i, l := range lines {
		if m := sectionHeading.FindStringSubmatch(l); m != nil {
			out = append(out, heading{line: i, title: m[1]})
		}
	}
	return out
}

func indexOfTitle(headings []heading, title string) (int, bool) {
	for i, h := range headings {
		if strings.EqualFold(h.title, title) {
			return i, true
		}
	}
	return 0, false
}

// sectionEnd is one past the end of headings[i]'s section.
func sectionEnd(headings []heading, i, total int) int {
	if i+1 < len(headings) {
		return headings[i+1].line
	}
	return total
}

// blockLines strips surrounding blanks so insertAt controls the spacing.
func blockLines(body string) []string {
	trimmed := strings.Trim(body, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// insertAt normalizes to exactly one blank line either side of block.
func insertAt(lines []string, at int, block []string) []string {
	head := trimTrailingBlank(lines[:at])
	tail := trimLeadingBlank(lines[at:])

	out := make([]string, 0, len(head)+len(block)+len(tail)+2)
	out = append(out, head...)
	if len(out) > 0 {
		out = append(out, "")
	}
	out = append(out, block...)
	if len(tail) > 0 {
		out = append(out, "")
		out = append(out, tail...)
	}
	return out
}

func trimTrailingBlank(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

func trimLeadingBlank(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return lines[start:]
}

func join(lines []string) string {
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

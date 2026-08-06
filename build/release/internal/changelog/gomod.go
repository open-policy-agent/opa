package changelog

import (
	"sort"
	"strings"
)

type ModuleChange struct {
	Module     string
	OldVersion string // empty when added
	NewVersion string // empty when removed
}

// ParseRequires returns the direct requires as module→version. Not
// golang.org/x/mod/modfile: the syntax needed is small and stable, and this
// module has one dependency for a reason.
func ParseRequires(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(content, "\n")
	inBlock := false
	for _, raw := range lines {
		line := stripComment(raw)
		trimmed := strings.TrimSpace(line)
		hasIndirect := strings.Contains(commentOf(raw), "indirect")

		switch {
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case inBlock:
			if hasIndirect {
				continue
			}
			if mod, ver, ok := splitRequire(trimmed); ok {
				out[mod] = ver
			}
		case strings.HasPrefix(trimmed, "require ("):
			inBlock = true
		case strings.HasPrefix(trimmed, "require "):
			rest := strings.TrimPrefix(trimmed, "require ")
			if hasIndirect {
				continue
			}
			if mod, ver, ok := splitRequire(rest); ok {
				out[mod] = ver
			}
		}
	}
	return out
}

func stripComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

func commentOf(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[i+2:]
	}
	return ""
}

func splitRequire(s string) (string, string, bool) {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// DiffRequires reports direct-require changes, sorted by module path.
func DiffRequires(fromContent, toContent string) []ModuleChange {
	from := ParseRequires(fromContent)
	to := ParseRequires(toContent)

	seen := map[string]bool{}
	var out []ModuleChange
	for mod, oldV := range from {
		seen[mod] = true
		newV, ok := to[mod]
		switch {
		case !ok:
			out = append(out, ModuleChange{Module: mod, OldVersion: oldV})
		case newV != oldV:
			out = append(out, ModuleChange{Module: mod, OldVersion: oldV, NewVersion: newV})
		}
	}
	for mod, newV := range to {
		if seen[mod] {
			continue
		}
		out = append(out, ModuleChange{Module: mod, NewVersion: newV})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Module < out[j].Module })
	return out
}

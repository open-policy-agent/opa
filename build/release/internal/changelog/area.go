package changelog

import (
	"regexp"
	"strings"
)

type AreaResult struct {
	Area string
	// Subject has any leading "<Area>: " stripped, so Render can print
	// "<Area>: <Subject>" without duplication.
	Subject string
	Source  string
}

// existingPrefix matches any non-space, non-colon run followed by ": ". Matching
// that rather than an allow-list of punctuation is what keeps subjects like
// "ast,storage/inmem: Add X" from getting a second, path-derived prefix stacked
// on top. "Fix X: edge case" does not match: the colon is past a space.
var existingPrefix = regexp.MustCompile(`^([a-zA-Z][^\s:]*):\s+`)

// areaByPath order matters twice: matchPath takes the first match per file, and
// areaFromPaths breaks count ties by this order. Deliberately no "v1/" or
// "internal/" catch-all — "v1:" and "internal:" tell the reader nothing.
var areaByPath = []struct{ prefix, area string }{
	{".github/workflows/", "gha"},
	{".github/", "gha"},
	{"build/release/", "build/release"},
	{"build/", "build"},
	{"docs/", "docs"},
	{"e2e/", "e2e"},
	{"capabilities/", "capabilities"},
	{"wasm/", "wasm"},
	{"v1/ast/", "ast"},
	{"v1/bundle/", "bundle"},
	{"v1/cmd/", "cmd"},
	{"v1/compile/", "compile"},
	{"v1/debug/", "debug"},
	{"v1/download/", "download"},
	{"v1/format/", "format"},
	{"v1/ir/", "ir"},
	{"v1/loader/", "loader"},
	{"v1/logging/", "logging"},
	{"v1/metrics/", "metrics"},
	{"v1/plugins/", "plugins"},
	{"v1/repl/", "repl"},
	{"v1/runtime/", "runtime"},
	{"v1/sdk/", "sdk"},
	{"v1/server/", "server"},
	{"v1/storage/", "storage"},
	{"v1/tester/", "tester"},
	{"v1/test/", "test"},
	{"v1/topdown/", "topdown"},
	{"v1/types/", "types"},
	{"v1/util/", "util"},
	{"v1/version/", "version"},
}

// DeriveArea tries, in order: an existing "area: " prefix, PR labels (a no-op,
// see areaFromLabels), then the changed-path table.
func DeriveArea(subject string, labels, paths []string) AreaResult {
	if loc := existingPrefix.FindStringSubmatchIndex(subject); loc != nil {
		area := subject[loc[2]:loc[3]]
		rest := subject[loc[1]:]
		return AreaResult{Area: area, Subject: rest, Source: "existing prefix"}
	}
	if a, l := areaFromLabels(labels); a != "" {
		return AreaResult{Area: a, Subject: subject, Source: `label "` + l + `"`}
	}
	if a, p := areaFromPaths(paths); a != "" {
		return AreaResult{Area: a, Subject: subject, Source: `path "` + p + `"`}
	}
	return AreaResult{Subject: subject}
}

// TODO: derive an area from PR labels. The repo has no "area/X" convention to
// key off yet, so this is a no-op rather than a guess.
func areaFromLabels(_ []string) (area, source string) {
	return "", ""
}

// areaFromPaths picks the most common area, breaking ties by areaByPath order so
// the result does not depend on map iteration order.
func areaFromPaths(paths []string) (area, source string) {
	if len(paths) == 0 {
		return "", ""
	}
	counts := map[string]int{}
	prefixes := map[string]string{}
	for _, p := range paths {
		bestArea, bestPrefix := matchPath(p)
		if bestArea == "" {
			continue
		}
		counts[bestArea]++
		prefixes[bestArea] = bestPrefix
	}
	best := 0
	for _, c := range counts {
		best = max(best, c)
	}
	for _, r := range areaByPath {
		if counts[r.area] == best && best > 0 {
			return r.area, prefixes[r.area]
		}
	}
	return "", ""
}

func matchPath(p string) (area, prefix string) {
	for _, r := range areaByPath {
		if strings.HasPrefix(p, r.prefix) {
			return r.area, r.prefix
		}
	}
	return "", ""
}

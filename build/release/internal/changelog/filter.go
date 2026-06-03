package changelog

import (
	"regexp"
	"strings"
)

// FilterAction enumerates what a filter decided about an entry.
type FilterAction string

const (
	ActionKept    FilterAction = "kept"
	ActionDropped FilterAction = "dropped"
)

// FilterLog records one decision for stderr review.
type FilterLog struct {
	SHA     string
	Subject string
	Area    string
	Action  FilterAction
	Reason  string
}

// releaseMechanics matches commits that move the release process along rather
// than change OPA. No released CHANGELOG entry has ever listed one.
var releaseMechanics = regexp.MustCompile(`(?i)^(` +
	`release\s+v?\d+\.\d+\.\d+` + `|` +
	`prepare\s+v?\d+\.\d+\.\d+\s+development` + `|` +
	`integrate\s+v?\d+\.\d+\.\d+\s+patch\s+release` +
	`)\s*$`)

// FilterReleaseMechanics runs after Transform, so subjects have had any area
// prefix stripped.
func FilterReleaseMechanics(entries []Entry) ([]Entry, []FilterLog) {
	kept := make([]Entry, 0, len(entries))
	var logs []FilterLog
	for i := range entries {
		if releaseMechanics.MatchString(strings.TrimSpace(entries[i].Subject)) {
			logs = append(logs, FilterLog{
				SHA:     entries[i].SHA,
				Subject: entries[i].Subject,
				Area:    entries[i].Area,
				Action:  ActionDropped,
				Reason:  "release mechanics",
			})
			continue
		}
		kept = append(kept, entries[i])
	}
	return kept, logs
}

var noisePathPrefixes = []string{
	".github/",
	"e2e/",
	"docs/",
}

// FilterDependencies drops bot-authored dependency entries, whose aggregate
// subjects ("bump the dependencies group across 2 directories") carry less than
// SynthesizeMissingDeps produces from the go.mod diff, and dependency entries
// confined to noise paths. Human-authored dependency work passes through with
// attribution intact.
func FilterDependencies(entries []Entry) ([]Entry, []FilterLog) {
	kept := make([]Entry, 0, len(entries))
	logs := make([]FilterLog, 0, len(entries))
	for i := range entries {
		e := entries[i]
		log := FilterLog{
			SHA:     e.SHA,
			Subject: e.Subject,
			Area:    e.Area,
			Action:  ActionKept,
		}
		switch {
		case e.IsDependency && isBot(e.AuthorLogin):
			log.Action = ActionDropped
			log.Reason = "bot-authored dependency (covered by go.mod diff synthesis)"
			logs = append(logs, log)
			continue
		case e.IsDependency && filesAllInNoisePaths(e.Files):
			log.Action = ActionDropped
			log.Reason = "all changes under noise paths (" + strings.Join(noisePathPrefixes, ", ") + ")"
			logs = append(logs, log)
			continue
		}
		kept = append(kept, e)
		logs = append(logs, log)
	}
	return kept, logs
}

func isBot(login string) bool {
	return strings.HasSuffix(login, "[bot]")
}

// filesAllInNoisePaths is false for empty input: we don't drop entries we have
// no path data for.
func filesAllInNoisePaths(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !hasAnyPrefix(f, noisePathPrefixes) {
			return false
		}
	}
	return true
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

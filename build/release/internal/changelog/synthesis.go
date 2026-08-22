package changelog

import (
	"fmt"
	"regexp"
	"strings"
)

type SynthesisAction string

const (
	ActionCovered     SynthesisAction = "covered by existing entry"
	ActionSynthesized SynthesisAction = "synthesized"
)

type SynthesisLog struct {
	Module      string
	Change      ModuleChange
	Action      SynthesisAction
	CoveringSHA string
}

var (
	bumpModule  = regexp.MustCompile(`(?i)\bbump\s+(\S+)\s+from\s+\S+\s+to\s+\S+`)
	majorSuffix = regexp.MustCompile(`/v\d+$`)
)

// SynthesizeMissingDeps appends a bare entry for each go.mod require change no
// surviving commit already accounts for.
func SynthesizeMissingDeps(entries []Entry, changes []ModuleChange) ([]Entry, []SynthesisLog) {
	logs := make([]SynthesisLog, 0, len(changes))
	out := entries
	for _, ch := range changes {
		if sha, ok := coveringEntry(entries, ch.Module); ok {
			logs = append(logs, SynthesisLog{Module: ch.Module, Change: ch, Action: ActionCovered, CoveringSHA: sha})
			continue
		}
		out = append(out, syntheticEntry(ch))
		logs = append(logs, SynthesisLog{Module: ch.Module, Change: ch, Action: ActionSynthesized})
	}
	return out, logs
}

// coveringEntry looks at every entry, not just dependency ones: the commits that
// add or remove a dependency are usually classified by the code they touch
// ("runtime: Remove automaxprocs dependency"), and skipping those rendered such
// changes twice.
func coveringEntry(entries []Entry, module string) (string, bool) {
	for i := range entries {
		e := &entries[i]
		if e.IsDependency {
			if m := bumpModule.FindStringSubmatch(e.Subject); len(m) >= 2 && m[1] == module {
				return e.SHA, true
			}
		}
		if e.IsSynthetic {
			continue
		}
		if subjectNamesModule(e.Subject, module) {
			return e.SHA, true
		}
	}
	return "", false
}

// minModuleNameLen keeps short segments like "net" or "ini" from matching
// unrelated prose; the host-stripped form ("x/net") covers those cases.
const minModuleNameLen = 5

func subjectNamesModule(subject, module string) bool {
	lower := strings.ToLower(subject)
	base := majorSuffix.ReplaceAllString(module, "")

	candidates := []string{base}
	if i := strings.IndexByte(base, '/'); i >= 0 {
		candidates = append(candidates, base[i+1:])
	}
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		candidates = append(candidates, base[i+1:])
	}
	for _, c := range candidates {
		if len(c) >= minModuleNameLen && strings.Contains(lower, strings.ToLower(c)) {
			return true
		}
	}
	return false
}

func syntheticEntry(ch ModuleChange) Entry {
	trim := func(v string) string { return strings.TrimPrefix(v, "v") }
	var subject string
	switch {
	case ch.OldVersion == "":
		subject = fmt.Sprintf("Add %s %s", ch.Module, trim(ch.NewVersion))
	case ch.NewVersion == "":
		subject = fmt.Sprintf("Drop %s (was %s)", ch.Module, trim(ch.OldVersion))
	default:
		subject = fmt.Sprintf("Bump %s from %s to %s", ch.Module, trim(ch.OldVersion), trim(ch.NewVersion))
	}
	return Entry{
		Subject:      subject,
		Area:         "build(deps)",
		IsDependency: true,
		IsSynthetic:  true,
	}
}

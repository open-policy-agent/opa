// Command release is OPA's release-prep tool. See usage below, or the Makefile's
// release-prepare target, which sequences both subcommands.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/build/release/internal/artefacts"
	"github.com/open-policy-agent/opa/build/release/internal/changelog"
	"github.com/open-policy-agent/opa/build/release/internal/gh"
	"github.com/open-policy-agent/opa/build/release/internal/gh/fixture"
	"github.com/open-policy-agent/opa/build/release/internal/git"
)

const (
	defaultRepo       = "open-policy-agent/opa"
	unreleasedHeading = "Unreleased"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage(os.Stderr)
		return errors.New("subcommand required")
	}
	switch args[0] {
	case "changelog":
		return changelogCmd(args[1:])
	case "artefacts":
		return artefactsCmd(args[1:])
	case "dev":
		return devCmd(args[1:])
	case "-h", "--help", "help":
		usage(os.Stdout)
		return nil
	default:
		usage(os.Stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `release — OPA release prep tooling

Usage:
  release changelog [--version <X.Y.Z>] [--from <ref>] [--to <ref>] [--repo <owner/name>]
                    [--out <path> | --update <path>] [--include-local] [--record <dir>]
  release artefacts --version <X.Y.Z> [--repo-root <path>] [--dry-run] [--skip-generate]
  release dev --version <X.Y.Z> [--repo-root <path>] [--dry-run]
              [--allow-existing-unreleased]

Subcommands:
  changelog   Render the CHANGELOG section for a commit range.
  artefacts   Bump v1/version/version.go, snapshot capabilities, regenerate
              builtin_metadata.json and v1/ast/version_index.json.
  dev         Reopen development after a release: set version.go to
              '<X.Y.Z>-dev' and add an '## Unreleased' CHANGELOG heading.

Output modes:
  --out <path>     Write the rendered section standalone; '-' (default) is stdout.
                   With --version the section is prefixed with its '## <version>'
                   heading.
  --update <path>  Splice the section into an existing CHANGELOG.md in place.
                   Requires --version. A '## Unreleased' heading is renamed to
                   '## <version>' and the generated bullets are appended to the
                   end of that section, after any hand-written prose. With no
                   '## Unreleased' heading, a new section is inserted above the
                   topmost release. Re-running for a version that already has a
                   section is refused.

Authentication:
  GITHUB_TOKEN env var is read first; on miss, `+"`gh auth token`"+` is consulted.
  A token is effectively required: GitHub's GraphQL API (used to resolve PR to
  issue links) rejects unauthenticated requests with 401, and the REST half is
  limited to 60 requests/hour, which a release range exhausts in ~20 commits.
  The public_repo scope is enough; read:org is not needed.

  Transient failures (timeouts, 5xx, rate limiting) are retried up to 3 times
  with exponential backoff. Each retry is logged to stderr.

Local-only commits:
  Commits that don't yet exist on the remote are detected and logged to stderr.
  By default they are excluded from the rendered changelog. Pass --include-local
  to include them; the bullet's link target falls back to any "Fixes #N" /
  "Closes #N" / "Resolves #N" trailer in the local commit message.

Fixtures:
  --record <dir> captures every GitHub response and commit message from the run
  into <dir>, for replay by the hermetic golden tests. See
  internal/changelog/golden_test.go.

Artefacts:
  `+"`artefacts`"+` never touches git. It leaves the bumped and regenerated files in
  the working tree — new files show up as untracked in `+"`git status`"+`, which is
  unambiguous. It runs the three release-relevant generators directly rather
  than `+"`make generate`"+`, so a running docker daemon does not pull an unrelated
  opa.wasm rebuild into the release diff.
`)
}

func changelogCmd(args []string) error {
	fs := flag.NewFlagSet("changelog", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", "", "release version for the '## <version>' heading, e.g. 1.19.0 (a leading 'v' is stripped); required with --update")
	from := fs.String("from", "", "starting ref (exclusive); defaults to the latest tag reachable from HEAD")
	to := fs.String("to", "HEAD", "ending ref (inclusive)")
	repo := fs.String("repo", defaultRepo, "GitHub repo as owner/name")
	out := fs.String("out", "-", "output path for the rendered section; '-' for stdout")
	update := fs.String("update", "", "path to a CHANGELOG.md to splice the section into, in place; mutually exclusive with --out")
	includeLocal := fs.Bool("include-local", false, "include commits that exist only locally (not yet pushed) in the rendered changelog; off by default — they are still detected and logged to stderr")
	record := fs.String("record", "", "directory to record the run's GitHub and git responses into, as a replayable test fixture")
	if err := fs.Parse(args); err != nil {
		return err
	}

	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if *update != "" && explicit["out"] {
		return errors.New("--out and --update are mutually exclusive: --out writes a standalone section, --update edits a CHANGELOG in place")
	}

	ver := changelog.NormalizeVersion(*version)
	if *update != "" && ver == "" {
		return errors.New("--update requires --version")
	}

	if *from == "" {
		t, err := git.LatestTag()
		if err != nil {
			return fmt.Errorf("--from not given and could not discover latest tag: %w", err)
		}
		*from = t
		fmt.Fprintf(os.Stderr, "release: using --from=%s (latest tag)\n", t)
	}

	shas, err := git.CommitsBetween(*from, *to)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		fmt.Fprintf(os.Stderr, "release: no commits in %s..%s\n", *from, *to)
		return nil
	}

	client, err := gh.New(*repo, func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "release:   "+format+"\n", args...)
	})
	if err != nil {
		return err
	}
	repoURL := "https://github.com/" + strings.TrimRight(*repo, "/")

	msg := changelog.MessageFunc(git.CommitMessage)
	var recorder *fixture.Recorder
	if *record != "" {
		recorder = fixture.NewRecorder(client, fixture.Meta{Repo: *repo, From: *from, To: *to})
		client = recorder
		msg = recorder.WrapMessage(git.CommitMessage)
	}

	ctx := context.Background()
	fmt.Fprintf(os.Stderr, "release: resolving %d commit(s) in %s..%s via GitHub\n", len(shas), *from, *to)
	progress := func(done, total int, sha, subject string) {
		fmt.Fprintf(os.Stderr, "release: [%d/%d] %s %s\n", done+1, total, shortSHA(sha), subject)
	}
	resolved := func(e *changelog.Entry) {
		if !e.IsLocalOnly {
			return
		}
		refs := make([]string, 0, len(e.Issues))
		for _, iss := range e.Issues {
			refs = append(refs, fmt.Sprintf("#%d", iss.Number))
		}
		note := "no references in commit message"
		if len(refs) > 0 {
			note = "references: " + strings.Join(refs, ", ")
		}
		fmt.Fprintf(os.Stderr, "release:   [local only] %s %s — %s\n", shortSHA(e.SHA), e.Subject, note)
	}
	entries, err := changelog.Resolve(ctx, client, shas, msg, progress, resolved)
	if err != nil {
		return err
	}

	fromGoMod, err := git.FileAt(*from, "go.mod")
	if err != nil {
		return fmt.Errorf("read go.mod at %s: %w", *from, err)
	}
	toGoMod, err := git.FileAt(*to, "go.mod")
	if err != nil {
		return fmt.Errorf("read go.mod at %s: %w", *to, err)
	}

	res := changelog.Generate(entries, changelog.Options{
		RepoURL:      repoURL,
		IncludeLocal: *includeLocal,
		FromGoMod:    fromGoMod,
		ToGoMod:      toGoMod,
	})
	logTransforms(res.Transforms)
	logFilters(res.Filters)
	logSyntheses(res.Syntheses)

	if recorder != nil {
		rec := recorder.Recording()
		rec.SHAs = shas
		rec.FromGoMod = fromGoMod
		rec.ToGoMod = toGoMod
		if err := fixture.Write(*record, rec); err != nil {
			return fmt.Errorf("write fixture to %s: %w", *record, err)
		}
		fmt.Fprintf(os.Stderr, "release: recorded fixture to %s\n", *record)
	}

	if *update != "" {
		if err := spliceIntoFile(*update, ver, res.Body); err != nil {
			return err
		}
	} else if err := writeSection(*out, changelog.Section(ver, res.Body)); err != nil {
		return err
	}

	if res.Ambiguities != "" {
		fmt.Fprintln(os.Stderr)
		fmt.Fprint(os.Stderr, res.Ambiguities)
	}
	return nil
}

func spliceIntoFile(path, version, body string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	updated, err := changelog.Splice(string(existing), version, body)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Fprintf(os.Stderr, "release: spliced '## %s' into %s\n", version, path)
	return nil
}

func writeSection(out, section string) error {
	if out == "-" {
		if _, err := os.Stdout.WriteString(section); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "release: wrote CHANGELOG section to stdout (%d bytes)\n", len(section))
		return nil
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(section); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "release: wrote CHANGELOG section to %s (%d bytes)\n", out, len(section))
	return nil
}

func devCmd(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", "", "next release version, e.g. 1.19.1; version.go becomes '<version>-dev'")
	repoRoot := fs.String("repo-root", "", "OPA checkout to operate on; defaults to the enclosing repository's top level")
	dryRun := fs.Bool("dry-run", false, "report what would change without writing anything")
	allowExisting := fs.Bool("allow-existing-unreleased", false, "if CHANGELOG.md already has an '## Unreleased' heading, warn and leave it alone instead of failing; the version bump still happens")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" {
		return errors.New("--version is required")
	}
	ver, err := artefacts.ParseVersion(*version)
	if err != nil {
		return err
	}
	if strings.Contains(ver, "-") {
		return fmt.Errorf("--version %q already carries a pre-release suffix; pass the plain next version, e.g. 1.19.1", *version)
	}

	if *repoRoot == "" {
		root, err := git.RepoRoot()
		if err != nil {
			return fmt.Errorf("--repo-root not given and could not discover the repository root: %w", err)
		}
		*repoRoot = root
	}

	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "release: "+format+"\n", args...)
	}

	// Splice first: it is the step that fails (a '## Unreleased' heading already
	// present), and failing before version.go is touched keeps the tree clean.
	changelogPath := filepath.Join(*repoRoot, "CHANGELOG.md")
	existing, err := os.ReadFile(changelogPath)
	if err != nil {
		return fmt.Errorf("read CHANGELOG.md: %w", err)
	}
	// An empty body makes Splice insert a bare heading above the topmost release.
	updated, err := changelog.Splice(string(existing), unreleasedHeading, "")
	addHeading := true
	switch {
	case errors.Is(err, changelog.ErrSectionExists) && *allowExisting:
		logf("warning: CHANGELOG.md already has a '## %s' heading; leaving it as is", unreleasedHeading)
		addHeading = false
	case err != nil:
		return fmt.Errorf("CHANGELOG.md: %w", err)
	}

	devVersion := ver + "-dev"
	previous, changed, err := artefacts.BumpVersion(*repoRoot, devVersion, *dryRun, logf)
	if err != nil {
		return err
	}

	switch {
	case !addHeading:
	case *dryRun:
		logf("would add a '## %s' heading to CHANGELOG.md", unreleasedHeading)
	default:
		if err := os.WriteFile(changelogPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("write CHANGELOG.md: %w", err)
		}
		logf("added a '## %s' heading to CHANGELOG.md", unreleasedHeading)
	}

	fmt.Fprintln(os.Stderr)
	if *dryRun {
		fmt.Fprintln(os.Stderr, "Dry run — nothing was written. Re-run without --dry-run to apply.")
		return nil
	}
	fmt.Fprintln(os.Stderr, "Review checklist (please verify before committing):")
	if changed {
		fmt.Fprintf(os.Stderr, "- %s: Version %s -> %s\n", artefacts.VersionFile, previous, devVersion)
	}
	if addHeading {
		fmt.Fprintf(os.Stderr, "- CHANGELOG.md: '## %s' heading added\n", unreleasedHeading)
	} else {
		fmt.Fprintf(os.Stderr, "- CHANGELOG.md: unchanged, '## %s' heading was already there\n", unreleasedHeading)
	}
	return nil
}

func artefactsCmd(args []string) error {
	fs := flag.NewFlagSet("artefacts", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	version := fs.String("version", "", "release version, e.g. 1.19.0 (a leading 'v' is stripped)")
	repoRoot := fs.String("repo-root", "", "OPA checkout to operate on; defaults to the enclosing repository's top level")
	dryRun := fs.Bool("dry-run", false, "report what would change without writing anything or running any generator")
	skipGenerate := fs.Bool("skip-generate", false, "skip code generation; only bump version.go and snapshot capabilities")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" {
		return errors.New("--version is required")
	}

	if *repoRoot == "" {
		root, err := git.RepoRoot()
		if err != nil {
			return fmt.Errorf("--repo-root not given and could not discover the repository root: %w", err)
		}
		*repoRoot = root
	}

	res, err := artefacts.Prepare(artefacts.Options{
		RepoRoot:     *repoRoot,
		Version:      *version,
		DryRun:       *dryRun,
		SkipGenerate: *skipGenerate,
		Logf: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "release: "+format+"\n", args...)
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	if *dryRun {
		fmt.Fprintln(os.Stderr, "Dry run — nothing was written. Re-run without --dry-run to apply.")
		return nil
	}
	fmt.Fprintln(os.Stderr, "Review checklist (please verify before committing):")
	if res.VersionFileChanged {
		fmt.Fprintf(os.Stderr, "- %s: Version %s -> %s\n", artefacts.VersionFile, res.PreviousVersion, *version)
	}
	if res.CapabilitiesOverwritten {
		fmt.Fprintf(os.Stderr, "- %s already existed and was overwritten — check `git diff` on it\n", res.CapabilitiesPath)
	} else {
		fmt.Fprintf(os.Stderr, "- %s is new — it will show as untracked in `git status`\n", res.CapabilitiesPath)
	}
	for _, f := range res.Generated {
		fmt.Fprintf(os.Stderr, "- %s regenerated\n", f)
	}
	if *skipGenerate {
		fmt.Fprintf(os.Stderr, "- generation was skipped: %s and %s are stale\n", artefacts.MetadataFile, artefacts.VersionIndexFile)
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func logTransforms(logs []changelog.TransformLog) {
	if len(logs) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "release: transformations:\n")
	for _, l := range logs {
		marker := "—"
		if l.IsDependency {
			marker = "↳ Dependencies"
		}
		switch {
		case l.Area != "" && l.Source != "":
			fmt.Fprintf(os.Stderr, "release:   %s area=%-12q  via %-22s %s  %q\n",
				shortSHA(l.SHA), l.Area, l.Source, marker, l.Subject)
		default:
			fmt.Fprintf(os.Stderr, "release:   %s area=%-12q  %-26s %s  %q\n",
				shortSHA(l.SHA), "(none)", "no rule matched", marker, l.Subject)
		}
	}
}

// logFilters skips kept entries to keep stderr scannable.
func logFilters(logs []changelog.FilterLog) {
	type pair struct{ action, line string }
	var noisy []pair
	for _, l := range logs {
		if l.Action == changelog.ActionKept {
			continue
		}
		noisy = append(noisy, pair{
			action: string(l.Action),
			line: fmt.Sprintf("release:   %s area=%-12q  %-22s  %q\n",
				shortSHA(l.SHA), l.Area, l.Reason, l.Subject),
		})
	}
	if len(noisy) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "release: filters:\n")
	for _, n := range noisy {
		fmt.Fprintf(os.Stderr, "release: [%s]\n", n.action)
		fmt.Fprint(os.Stderr, n.line)
	}
}

// logSyntheses shows covered as well as synthesized, so the pairing is checkable.
func logSyntheses(logs []changelog.SynthesisLog) {
	if len(logs) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "release: go.mod require changes:\n")
	for _, l := range logs {
		ver := versionLabel(l.Change)
		switch l.Action {
		case changelog.ActionSynthesized:
			fmt.Fprintf(os.Stderr, "release:   [synthesized]   %s %s\n", l.Module, ver)
		case changelog.ActionCovered:
			fmt.Fprintf(os.Stderr, "release:   [covered by %s] %s %s\n", shortSHA(l.CoveringSHA), l.Module, ver)
		}
	}
}

func versionLabel(ch changelog.ModuleChange) string {
	switch {
	case ch.OldVersion == "":
		return "(added " + ch.NewVersion + ")"
	case ch.NewVersion == "":
		return "(removed " + ch.OldVersion + ")"
	default:
		return ch.OldVersion + " → " + ch.NewVersion
	}
}

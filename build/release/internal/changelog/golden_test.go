package changelog

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/gh/fixture"
)

// go test ./internal/changelog -update
var updateGolden = flag.Bool("update", false, "rewrite golden files from the fixtures")

const testdataDir = "../../testdata"

// The fixture data is invented, not recorded: real history covers only the
// branches it happens to contain, and naming real contributors in a test fixture
// is not worth doing.
var goldenCases = []struct {
	dir     string
	version string
}{
	{dir: "scenarios", version: "1.0.0"},
}

// Each fixture commit's SHA starts with its key here, so a golden diff points at
// the behaviour that moved. TestGoldenScenarioCoverage keeps the two in step.
var scenarioCoverage = map[string]string{
	"0001": "squash-merge (#N) suffix stripped; issue link; author != reporter",
	"0002": "issue link; author == reporter collapses to 'reported and authored by'",
	"0003": "PR with no linked issue falls back to the PR link",
	"0004": "PR closing several issues: all rendered, sorted; lowest drives the reporter",
	"0005": "commit associated with several PRs: first used, rest flagged",
	"0006": "commit with no PR at all: commit-SHA link",
	"0007": "local-only commit with a Fixes trailer: dropped, reference reported",
	"0008": "local-only commit with no trailer: dropped, no reference",
	"0009": "prefix containing a comma is recognised, so no second area is stacked on",
	"000a": "no prefix: area from changed paths, subject capitalized",
	"000b": "no prefix and unmappable paths: no area at all",
	"000c": "PR labels present but derive nothing (areaFromLabels is a no-op)",
	"000d": "subject starting with a backtick is left verbatim, not capitalized",
	"000e": "empty author login: bullet carries no attribution",
	"000f": "release mechanics 'Release vX.Y.Z' dropped",
	"0010": "release mechanics 'Prepare vX.Y.Z development' dropped",
	"0011": "release mechanics 'Integrate X.Y.Z patch release' dropped",
	"0012": "bot-authored aggregate dependency commit dropped",
	"0013": "dependency commit confined to noise paths (docs/) dropped",
	"0014": "human-authored dependency commit kept, and covers its go.mod bump",
	"0015": "non-dependency commit naming a module covers that go.mod removal",
}

// TestGolden is the regression baseline. Hermetic in both directions: no network
// and no git.
func TestGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.dir, func(t *testing.T) {
			dir := filepath.Join(testdataDir, tc.dir)

			rec, err := fixture.Read(dir)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			entries, err := Resolve(context.Background(), rec.Client(), rec.SHAs, rec.Message, nil, nil)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			res := Generate(entries, Options{
				RepoURL:   "https://github.com/" + rec.Meta.Repo,
				FromGoMod: rec.FromGoMod,
				ToGoMod:   rec.ToGoMod,
			})

			checkGolden(t, filepath.Join(dir, "expected.md"), Section(tc.version, res.Body))
			checkGolden(t, filepath.Join(dir, "expected-ambiguities.txt"), res.Ambiguities)
		})
	}
}

// TestGoldenSplice covers the shape --update actually runs against: a CHANGELOG
// with hand-written Unreleased prose already in it.
func TestGoldenSplice(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.dir, func(t *testing.T) {
			path := filepath.Join(testdataDir, tc.dir, "expected.md")
			section, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v (re-run with -update to create it)", path, err)
			}
			body := stripHeading(string(section))

			existing := "# Change Log\n\n## Unreleased\n\nHand-written prose.\n\n## 0.9.0\n\nOlder.\n"
			got, err := Splice(existing, tc.version, body)
			if err != nil {
				t.Fatalf("splice: %v", err)
			}
			checkGolden(t, filepath.Join(testdataDir, tc.dir, "expected-spliced.md"), got)
		})
	}
}

func stripHeading(section string) string {
	lines := strings.Split(section, "\n")
	if len(lines) == 0 || !sectionHeading.MatchString(lines[0]) {
		return section
	}
	return join(trimLeadingBlank(lines[1:]))
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated %s (%d bytes)", path, len(got))
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (re-run with -update to create it)", path, err)
	}
	if got != string(want) {
		t.Errorf("%s is out of date; re-run with -update to accept.\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func TestGoldenScenarioCoverage(t *testing.T) {
	rec, err := fixture.Read(filepath.Join(testdataDir, "scenarios"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	seen := make(map[string]bool, len(rec.SHAs))
	for _, sha := range rec.SHAs {
		if len(sha) != 40 {
			t.Errorf("sha %q is %d chars, want 40", sha, len(sha))
		}
		tag := sha[:4]
		seen[tag] = true
		if _, ok := scenarioCoverage[tag]; !ok {
			t.Errorf("commit %s has no scenarioCoverage entry for tag %q", sha, tag)
		}
	}
	for tag := range scenarioCoverage {
		if !seen[tag] {
			t.Errorf("scenarioCoverage describes tag %q, but no fixture commit uses it", tag)
		}
	}
}

// TestGoldenFixtureIsFictional trips if a re-record ever puts real names back in.
func TestGoldenFixtureIsFictional(t *testing.T) {
	rec, err := fixture.Read(filepath.Join(testdataDir, "scenarios"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// dependabot[bot] is an app, not a person, and isDependency matches it by
	// exact login.
	allowed := map[string]bool{
		"": true, "alice": true, "bob": true, "carol": true,
		"dave": true, "erin": true, "frank": true, "dependabot[bot]": true,
	}
	for sha, rc := range rec.Commits {
		if rc.Commit == nil {
			continue // local-only
		}
		if !allowed[rc.Commit.AuthorLogin] {
			t.Errorf("commit %s author %q is not one of the invented logins", sha, rc.Commit.AuthorLogin)
		}
	}
	for pr, issues := range rec.Closing {
		for _, iss := range issues {
			if !allowed[iss.ReporterLogin] {
				t.Errorf("PR %d issue %d reporter %q is not one of the invented logins", pr, iss.Number, iss.ReporterLogin)
			}
		}
	}
	if want := "example/example"; rec.Meta.Repo != want {
		t.Errorf("fixture repo = %q, want the placeholder %q", rec.Meta.Repo, want)
	}
}

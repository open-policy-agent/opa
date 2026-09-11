package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/build/release/internal/changelog"
)

func TestRun_Subcommands(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no args", args: nil, wantErr: "subcommand required"},
		{name: "unknown subcommand", args: []string{"nope"}, wantErr: `unknown subcommand "nope"`},
		{name: "help", args: []string{"--help"}},
		{name: "help -h", args: []string{"-h"}},
		{name: "help word", args: []string{"help"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestChangelogCmd_FlagValidation covers the checks that run before any git or
// GitHub access, so they need neither.
func TestChangelogCmd_FlagValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "update without version",
			args:    []string{"--update", "CHANGELOG.md"},
			wantErr: "--update requires --version",
		},
		{
			name:    "out and update together",
			args:    []string{"--update", "CHANGELOG.md", "--version", "1.2.3", "--out", "-"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: "flag provided but not defined",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := changelogCmd(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestChangelogCmd_EmptyRange exercises the CLI down to the git layer and back:
// an empty range returns before any GitHub call, so this needs no network.
func TestChangelogCmd_EmptyRange(t *testing.T) {
	dir := newGitRepo(t)
	t.Chdir(dir)

	target := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(target, []byte("# Change Log\n\n## 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := changelogCmd([]string{"--version", "1.2.3", "--from", "v0.1.0", "--to", "v0.1.0", "--update", target})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "1.2.3") {
		t.Errorf("an empty range must not touch the CHANGELOG, got:\n%s", got)
	}
}

func TestArtefactsCmd_FlagValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no version", args: nil, wantErr: "--version is required"},
		{
			name:    "invalid version",
			args:    []string{"--version", "latest", "--repo-root", "."},
			wantErr: "invalid version",
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: "flag provided but not defined",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := artefactsCmd(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestArtefactsCmd_DryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "v1", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "v1", "version", "version.go"),
		[]byte("package version\n\nvar Version = \"1.2.3-dev\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "capabilities.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := artefactsCmd([]string{"--version", "1.2.3", "--repo-root", root, "--dry-run"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), "1.2.3-dev") {
		t.Errorf("--dry-run modified version.go:\n%s", version)
	}
	if _, err := os.Stat(filepath.Join(root, "capabilities", "v1.2.3.json")); !os.IsNotExist(err) {
		t.Errorf("--dry-run created the capabilities snapshot: %v", err)
	}
}

// TestArtefactsCmd_RepoRootDefault asserts the default comes from git rather than
// the working directory, which for the make targets is build/release.
func TestArtefactsCmd_RepoRootDefault(t *testing.T) {
	root := newGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "v1", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "v1", "version", "version.go"),
		[]byte("package version\n\nvar Version = \"1.2.3-dev\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "capabilities.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "build", "release")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	if err := artefactsCmd([]string{"--version", "1.2.3", "--skip-generate"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "capabilities", "v1.2.3.json")); err != nil {
		t.Errorf("snapshot not written to the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, "capabilities")); !os.IsNotExist(err) {
		t.Error("snapshot was written relative to the working directory")
	}
}

func TestSpliceIntoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	original := "# Change Log\n\n## Unreleased\n\nProse.\n\n## 0.1.0\n\nOld.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	body := "### Miscellaneous\n\n- ast: Fix it ([#1](u)) authored by @alice\n"
	if err := spliceIntoFile(path, "1.2.3", body); err != nil {
		t.Fatalf("spliceIntoFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## 1.2.3", "Prose.", "- ast: Fix it", "## 0.1.0"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("result missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "## Unreleased") {
		t.Errorf("Unreleased heading was not renamed:\n%s", got)
	}

	// A second splice must fail and leave the file untouched.
	before := string(got)
	if err := spliceIntoFile(path, "1.2.3", body); err == nil {
		t.Error("expected the second splice to fail")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Error("a failed splice modified the file")
	}
}

func TestSpliceIntoFile_MissingFile(t *testing.T) {
	err := spliceIntoFile(filepath.Join(t.TempDir(), "nope.md"), "1.2.3", "body\n")
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("error = %v, want a read failure", err)
	}
}

func TestWriteSection(t *testing.T) {
	t.Run("to a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "section.md")
		if err := writeSection(path, "## 1.2.3\n\nbody\n"); err != nil {
			t.Fatalf("writeSection: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "## 1.2.3\n\nbody\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("to stdout", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stdout")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = f
		err = writeSection("-", "## 1.2.3\n")
		os.Stdout = orig
		f.Close()
		if err != nil {
			t.Fatalf("writeSection: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "## 1.2.3\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("unwritable path", func(t *testing.T) {
		if err := writeSection(filepath.Join(t.TempDir(), "no", "such", "dir", "x.md"), "body"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestShortSHA(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0123456789abcdef", "0123456"},
		{"0123456", "0123456"},
		{"012", "012"},
		{"", ""},
	} {
		if got := shortSHA(tc.in); got != tc.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// newGitRepo returns a repository with one commit tagged v0.1.0.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	run("config", "commit.gpgsign", "false")
	run("commit", "--allow-empty", "-m", "initial")
	run("tag", "v0.1.0")
	return dir
}

func TestVersionLabel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change changelog.ModuleChange
		want   string
	}{
		{name: "bump", change: changelog.ModuleChange{OldVersion: "v1.0.0", NewVersion: "v2.0.0"}, want: "v1.0.0 → v2.0.0"},
		{name: "added", change: changelog.ModuleChange{NewVersion: "v1.0.0"}, want: "(added v1.0.0)"},
		{name: "removed", change: changelog.ModuleChange{OldVersion: "v1.0.0"}, want: "(removed v1.0.0)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionLabel(tc.change); got != tc.want {
				t.Errorf("versionLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDecisionLogs covers the stderr review trail. It is display-only code, but
// it is what the maintainer checks the tool's choices against, so a silent
// early-return would matter.
func TestDecisionLogs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		emit  func()
		want  []string
		empty func()
	}{
		{
			name: "transforms",
			emit: func() {
				logTransforms([]changelog.TransformLog{
					{SHA: "abc1234def", Subject: "Fix it", Area: "ast", Source: "existing prefix"},
					{SHA: "beef567890", Subject: "Bump x", Area: "build(deps)", Source: `path "go.mod"`, IsDependency: true},
					{SHA: "cafe098765", Subject: "No area"},
				})
			},
			want:  []string{"abc1234", "ast", "existing prefix", "Dependencies", "no rule matched"},
			empty: func() { logTransforms(nil) },
		},
		{
			name: "filters",
			emit: func() {
				logFilters([]changelog.FilterLog{
					{SHA: "abc1234def", Subject: "Release v1.2.3", Action: changelog.ActionDropped, Reason: "release mechanics"},
					{SHA: "beef567890", Subject: "Kept", Action: changelog.ActionKept},
				})
			},
			want:  []string{"dropped", "release mechanics", "Release v1.2.3"},
			empty: func() { logFilters([]changelog.FilterLog{{Action: changelog.ActionKept}}) },
		},
		{
			name: "syntheses",
			emit: func() {
				logSyntheses([]changelog.SynthesisLog{
					{Module: "example.com/a", Change: changelog.ModuleChange{OldVersion: "v1.0.0", NewVersion: "v2.0.0"}, Action: changelog.ActionSynthesized},
					{Module: "example.com/b", Change: changelog.ModuleChange{OldVersion: "v1.0.0"}, Action: changelog.ActionCovered, CoveringSHA: "abc1234def"},
				})
			},
			want:  []string{"synthesized", "example.com/a", "covered by abc1234", "example.com/b"},
			empty: func() { logSyntheses(nil) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := captureStderr(t, tc.emit)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			if quiet := captureStderr(t, tc.empty); quiet != "" {
				t.Errorf("expected no output for nothing worth reporting, got:\n%s", quiet)
			}
		})
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stderr")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = f
	fn()
	os.Stderr = orig
	f.Close()

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// devRepo is a checkout as it looks just after a release: version.go on the
// released version, CHANGELOG.md with that version at the top and no Unreleased
// heading.
func devRepo(t *testing.T, released string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "v1", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "v1", "version", "version.go"),
		[]byte("package version\n\nvar Version = \""+released+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "# Change Log\n\n## " + released + "\n\nThe release.\n\n## 0.1.0\n\nOlder.\n"
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDevCmd(t *testing.T) {
	root := devRepo(t, "1.19.0")

	if err := devCmd([]string{"--version", "1.19.1", "--repo-root", root}); err != nil {
		t.Fatalf("devCmd: %v", err)
	}

	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), `var Version = "1.19.1-dev"`) {
		t.Errorf("version.go not moved to the next dev version:\n%s", version)
	}

	got, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Change Log\n\n## Unreleased\n\n## 1.19.0\n\nThe release.\n\n## 0.1.0\n\nOlder.\n"
	if string(got) != want {
		t.Errorf("CHANGELOG mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestDevCmd_RefusesExistingUnreleased(t *testing.T) {
	root := devRepo(t, "1.19.0")
	path := filepath.Join(root, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte("# Change Log\n\n## Unreleased\n\n## 1.19.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := devCmd([]string{"--version", "1.19.1", "--repo-root", root})
	if err == nil || !strings.Contains(err.Error(), "already present") {
		t.Fatalf("error = %v, want a refusal", err)
	}

	// The refusal must come before version.go is touched.
	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), `var Version = "1.19.0"`) {
		t.Errorf("version.go was modified despite the failure:\n%s", version)
	}
}

func TestDevCmd_DryRun(t *testing.T) {
	root := devRepo(t, "1.19.0")
	before, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}

	if err := devCmd([]string{"--version", "1.19.1", "--repo-root", root, "--dry-run"}); err != nil {
		t.Fatalf("devCmd: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Error("--dry-run modified the CHANGELOG")
	}
	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), `var Version = "1.19.0"`) {
		t.Errorf("--dry-run modified version.go:\n%s", version)
	}
}

func TestDevCmd_FlagValidation(t *testing.T) {
	root := devRepo(t, "1.19.0")
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no version", args: nil, wantErr: "--version is required"},
		{
			name:    "invalid version",
			args:    []string{"--version", "next", "--repo-root", root},
			wantErr: "invalid version",
		},
		{
			// The command appends -dev itself, so passing one produces 1.19.1-dev-dev.
			name:    "version already has a suffix",
			args:    []string{"--version", "1.19.1-dev", "--repo-root", root},
			wantErr: "already carries a pre-release suffix",
		},
		{
			name:    "missing CHANGELOG",
			args:    []string{"--version", "1.19.1", "--repo-root", t.TempDir()},
			wantErr: "read CHANGELOG.md",
		},
		{
			name:    "unknown flag",
			args:    []string{"--nope"},
			wantErr: "flag provided but not defined",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := devCmd(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestDevCmd_VPrefix mirrors the release side: a leading v is accepted.
func TestDevCmd_VPrefix(t *testing.T) {
	root := devRepo(t, "1.19.0")
	if err := devCmd([]string{"--version", "v1.19.1", "--repo-root", root}); err != nil {
		t.Fatalf("devCmd: %v", err)
	}
	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), `var Version = "1.19.1-dev"`) {
		t.Errorf("got:\n%s", version)
	}
}

func TestDevCmd_AllowExistingUnreleased(t *testing.T) {
	root := devRepo(t, "1.19.0")
	path := filepath.Join(root, "CHANGELOG.md")
	existing := "# Change Log\n\n## Unreleased\n\nHand-written note.\n\n## 1.19.0\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := devCmd([]string{"--version", "1.19.1", "--repo-root", root, "--allow-existing-unreleased"}); err != nil {
		t.Fatalf("devCmd: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != existing {
		t.Errorf("CHANGELOG was modified\n--- got ---\n%s\n--- want ---\n%s", got, existing)
	}

	// The version bump must still happen.
	version, err := os.ReadFile(filepath.Join(root, "v1", "version", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(version), `var Version = "1.19.1-dev"`) {
		t.Errorf("version.go was not bumped:\n%s", version)
	}
}

// TestDevCmd_AllowExistingUnreleasedStillAddsWhenMissing asserts the flag only
// suppresses the duplicate case, and is otherwise a no-op.
func TestDevCmd_AllowExistingUnreleasedStillAddsWhenMissing(t *testing.T) {
	root := devRepo(t, "1.19.0")

	if err := devCmd([]string{"--version", "1.19.1", "--repo-root", root, "--allow-existing-unreleased"}); err != nil {
		t.Fatalf("devCmd: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## Unreleased") {
		t.Errorf("heading not added:\n%s", got)
	}
}

// TestDevCmd_AllowExistingDoesNotMaskRealErrors keeps the flag narrow: it must
// tolerate only ErrSectionExists.
func TestDevCmd_AllowExistingDoesNotMaskRealErrors(t *testing.T) {
	root := devRepo(t, "1.19.0")
	if err := os.Remove(filepath.Join(root, "CHANGELOG.md")); err != nil {
		t.Fatal(err)
	}

	err := devCmd([]string{"--version", "1.19.1", "--repo-root", root, "--allow-existing-unreleased"})
	if err == nil || !strings.Contains(err.Error(), "read CHANGELOG.md") {
		t.Fatalf("error = %v, want a read failure", err)
	}
}

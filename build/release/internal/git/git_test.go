package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type repo struct {
	t   *testing.T
	dir string
}

// newRepo builds a real repository, since these functions are thin wrappers over
// the git CLI and there is nothing left to test once git is stubbed out.
func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir()}
	r.git("init", "--initial-branch=main")
	r.git("config", "user.name", "Test")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "commit.gpgsign", "false")
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *repo) commit(message string, files map[string]string) string {
	r.t.Helper()
	for name, content := range files {
		path := filepath.Join(r.dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			r.t.Fatal(err)
		}
		r.git("add", name)
	}
	if len(files) == 0 {
		r.git("commit", "--allow-empty", "-m", message)
	} else {
		r.git("commit", "-m", message)
	}
	return r.git("rev-parse", "HEAD")
}

// use makes r the process working directory for the test, which is where the
// package's git commands run.
func (r *repo) use() {
	r.t.Helper()
	r.t.Chdir(r.dir)
}

func TestRepoRoot(t *testing.T) {
	r := newRepo(t)
	r.commit("initial", nil)
	r.use()

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	// t.TempDir is under /var on macOS, which is a symlink to /private/var.
	want, err := filepath.EvalSymlinks(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err = filepath.EvalSymlinks(got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("RepoRoot() = %q, want %q", got, want)
	}
}

func TestRepoRoot_OutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := RepoRoot(); err == nil {
		t.Error("expected an error outside a repository")
	}
}

func TestLatestTag(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.git("tag", "v0.1.0")
	r.commit("second", nil)
	r.git("tag", "v0.2.0")
	r.commit("third", nil)
	r.use()

	got, err := LatestTag()
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if got != "v0.2.0" {
		t.Errorf("LatestTag() = %q, want v0.2.0", got)
	}
}

// TestLatestTag_OnlyReachable pins the caveat: the answer is the latest tag
// reachable from HEAD, not the latest tag in the repository. On a release branch
// cut from main those coincide; on a maintenance branch they need not.
func TestLatestTag_OnlyReachable(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.git("tag", "v0.1.0")

	r.git("checkout", "-b", "sidebranch")
	r.commit("side", nil)
	r.git("tag", "v0.9.0")
	r.git("checkout", "main")
	r.commit("second", nil)
	r.use()

	got, err := LatestTag()
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if got != "v0.1.0" {
		t.Errorf("LatestTag() = %q, want v0.1.0 — v0.9.0 is not reachable from HEAD", got)
	}
}

func TestLatestTag_NoTags(t *testing.T) {
	r := newRepo(t)
	r.commit("only", nil)
	r.use()

	if _, err := LatestTag(); err == nil {
		t.Error("expected an error when the repository has no tags")
	}
}

func TestCommitsBetween(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.git("tag", "v0.1.0")
	second := r.commit("second", nil)
	third := r.commit("third", nil)
	r.use()

	got, err := CommitsBetween("v0.1.0", "HEAD")
	if err != nil {
		t.Fatalf("CommitsBetween: %v", err)
	}
	want := []string{third, second}
	if len(got) != len(want) {
		t.Fatalf("got %d commits %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("commit %d = %s, want %s (newest first)", i, got[i], want[i])
		}
	}
}

func TestCommitsBetween_ExcludesMerges(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.git("tag", "v0.1.0")

	r.git("checkout", "-b", "feature")
	feature := r.commit("feature work", map[string]string{"feature.txt": "x"})
	r.git("checkout", "main")
	mainline := r.commit("mainline work", map[string]string{"main.txt": "y"})
	r.git("merge", "--no-ff", "-m", "Merge feature", "feature")
	r.use()

	got, err := CommitsBetween("v0.1.0", "HEAD")
	if err != nil {
		t.Fatalf("CommitsBetween: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d commits %v, want 2 (the merge commit must be excluded)", len(got), got)
	}
	for _, want := range []string{feature, mainline} {
		if !slices.Contains(got, want) {
			t.Errorf("%s missing from %v", want, got)
		}
	}
}

func TestCommitsBetween_EmptyRange(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.git("tag", "v0.1.0")
	r.use()

	got, err := CommitsBetween("v0.1.0", "v0.1.0")
	if err != nil {
		t.Fatalf("CommitsBetween: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want no commits", got)
	}
}

func TestCommitsBetween_UnknownRef(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.use()

	if _, err := CommitsBetween("v9.9.9", "HEAD"); err == nil {
		t.Error("expected an error for an unknown ref")
	}
}

func TestCommitMessage(t *testing.T) {
	r := newRepo(t)
	r.commit("ast: Fix the thing\n\nWith a body.\n\nFixes #123\n", nil)
	r.use()

	got, err := CommitMessage("HEAD")
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	for _, want := range []string{"ast: Fix the thing", "With a body.", "Fixes #123"} {
		if !strings.Contains(got, want) {
			t.Errorf("message %q does not contain %q", got, want)
		}
	}
}

func TestCommitMessage_UnknownRef(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.use()

	if _, err := CommitMessage("deadbeef"); err == nil {
		t.Error("expected an error for an unknown ref")
	}
}

func TestFileAt(t *testing.T) {
	r := newRepo(t)
	r.commit("add go.mod", map[string]string{"go.mod": "module x\n\nrequire y v1.0.0\n"})
	r.git("tag", "v0.1.0")
	r.commit("bump", map[string]string{"go.mod": "module x\n\nrequire y v2.0.0\n"})
	r.use()

	for _, tc := range []struct{ ref, want string }{
		{"v0.1.0", "require y v1.0.0"},
		{"HEAD", "require y v2.0.0"},
	} {
		got, err := FileAt(tc.ref, "go.mod")
		if err != nil {
			t.Fatalf("FileAt(%s): %v", tc.ref, err)
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("FileAt(%s) = %q, want it to contain %q", tc.ref, got, tc.want)
		}
	}
}

func TestFileAt_MissingPath(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.use()

	if _, err := FileAt("HEAD", "nope.txt"); err == nil {
		t.Error("expected an error for a path not in the tree")
	}
}

// TestRunIncludesStderr matters because git's diagnostics are the only clue when
// a ref is wrong, and they arrive on stderr.
func TestRunIncludesStderr(t *testing.T) {
	r := newRepo(t)
	r.commit("first", nil)
	r.use()

	_, err := CommitsBetween("v9.9.9", "HEAD")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("error %q does not mention the bad ref", err)
	}
}

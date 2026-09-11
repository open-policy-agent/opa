// Package git provides read-only access to the local repository.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// LatestTag returns the most recent semver tag reachable from HEAD.
func LatestTag() (string, error) {
	out, err := run("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RepoRoot is needed because the tool is normally invoked from build/release,
// which has its own go.mod, so the cwd is not the repository root.
func RepoRoot() (string, error) {
	out, err := run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitsBetween returns non-merge commits in from..to, newest first.
func CommitsBetween(from, to string) ([]string, error) {
	out, err := run("git", "log", "--format=%H", "--no-merges", from+".."+to)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// CommitMessage returns the subject and body for sha.
func CommitMessage(sha string) (string, error) {
	return run("git", "log", "--format=%B", "--max-count=1", sha)
}

// FileAt returns the contents of path at ref.
func FileAt(ref, path string) (string, error) {
	return run("git", "show", ref+":"+path)
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

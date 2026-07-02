package test

import (
	"os"
	"testing"
)

// Skip will skip this test on pull request CI runs.
// Used for slow test runners on GHA's darwin machines.
func Skip(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "" {
		return
	}
	t.Skip("skipped on Darwin, test runner is slow")
}

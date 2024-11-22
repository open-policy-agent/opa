//go:build !darwin
// +build !darwin

package test

import "testing"

// Skip will skip this test on pull request CI runs.
// Used for slow test runners on GHA's darwin machines.
func Skip(*testing.T) {}

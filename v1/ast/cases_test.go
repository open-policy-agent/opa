// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/test/conformance"
)

func caseRegoVersion(s string) (RegoVersion, error) {
	switch s {
	case "", "v1":
		return RegoV1, nil
	case "v0":
		return RegoV0, nil
	case "v0-compat-v1":
		return RegoV0CompatV1, nil
	}
	return RegoUndefined, fmt.Errorf("unknown rego_version %q", s)
}

func caseError(e *Error) conformance.Error {
	out := conformance.Error{Code: e.Code, Message: e.Message}
	if e.Location != nil {
		out.Module = e.Location.File
		out.Row = e.Location.Row
		out.Col = e.Location.Col
	}
	return out
}

// assertCaseErrors checks that every expected diagnostic was reported, and, for
// an exhaustive case, that nothing else was.
func assertCaseErrors(t *testing.T, filename string, want, got []conformance.Error, exhaustive bool) {
	t.Helper()

	missing, unexpected := conformance.MatchErrors(want, got, exhaustive)
	if len(missing) == 0 && len(unexpected) == 0 {
		return
	}

	var sb strings.Builder
	for _, e := range missing {
		fmt.Fprintf(&sb, "\n  missing:    %s", e)
	}
	for _, e := range unexpected {
		fmt.Fprintf(&sb, "\n  unexpected: %s", e)
	}
	fmt.Fprintf(&sb, "\n\nreported:")
	for _, e := range got {
		fmt.Fprintf(&sb, "\n  %s", e)
	}

	t.Fatalf("%s: diagnostics do not match:%s", filename, sb.String())
}

func indented(errs []conformance.Error) string {
	var sb strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&sb, "\n  %s", e)
	}
	return sb.String()
}

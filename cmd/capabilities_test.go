// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import "testing"

func TestCapabilitiesExitCode(t *testing.T) {

	t.Run("test with --versions", func(t *testing.T) {
		params := newCapabilitiesParams()
		params.showVersions = true
		_, err := doCapabilities(params)
		if err != nil {
			t.Fatal("expected success but got an error", err)
		}
	})

	t.Run("test with no arguments", func(t *testing.T) {
		params := newCapabilitiesParams()
		_, err := doCapabilities(params)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

}

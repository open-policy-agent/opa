// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"path"
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

func TestCapabilitiesNoArgs(t *testing.T) {
	t.Run("test with no arguments", func(t *testing.T) {
		_, err := doCapabilities(capabilitiesParams{})
		if err != nil {
			t.Fatal("expected success", err)
		}
	})
}

func TestCapabilitiesVersion(t *testing.T) {
	t.Run("test with version", func(t *testing.T) {
		params := capabilitiesParams{
			version: "v0.39.0",
		}
		_, err := doCapabilities(params)
		if err != nil {
			t.Fatal("expected success", err)
		}
	})
}

func TestCapabilitiesFile(t *testing.T) {
	t.Run("test with file", func(t *testing.T) {
		files := map[string]string{
			"test-capabilities.json": `
			{
				"builtins": [
					{
						"name": "plus",
						"infix": "+",
						"decl": {
							"type": "function",
							"args": [
								{
									"type": "number"
								},
								{
									"type": "number"
								}
							],
							"result": {
								"type": "number"
							}
						}
					}
				]
			}
			`,
		}

		test.WithTempFS(files, func(root string) {
			params := capabilitiesParams{
				file: path.Join(root, "test-capabilities.json"),
			}
			_, err := doCapabilities(params)

			if err != nil {
				t.Fatal("expected success", err)
			}
		})

	})
}

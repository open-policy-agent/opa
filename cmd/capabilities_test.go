// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"path"
	"slices"
	"sort"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util/test"
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

func TestCapabilitiesCurrent(t *testing.T) {
	tests := []struct {
		note              string
		v0Compatible      bool
		expFeatures       []string
		expFutureKeywords []string
	}{
		{
			note: "current",
			expFeatures: []string{
				ast.FeatureRegoV1,
			},
		},
		{
			note:         "current --v0-compatible",
			v0Compatible: true,
			expFeatures: []string{
				ast.FeatureRefHeadStringPrefixes,
				ast.FeatureRefHeads,
				ast.FeatureRegoV1Import,
			},
			expFutureKeywords: []string{
				"in",
				"every",
				"contains",
				"if",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			// These are sorted in the output
			sort.Strings(tc.expFutureKeywords)
			sort.Strings(tc.expFeatures)

			params := capabilitiesParams{
				showCurrent:  true,
				v0Compatible: tc.v0Compatible,
			}
			capsStr, err := doCapabilities(params)
			if err != nil {
				t.Fatal("expected success", err)
			}

			caps, err := ast.LoadCapabilitiesJSON(bytes.NewReader([]byte(capsStr)))
			if err != nil {
				t.Fatal("expected success", err)
			}

			if !slices.Equal(caps.Features, tc.expFeatures) {
				t.Errorf("expected features:\n\n%v\n\nbut got:\n\n%v", tc.expFeatures, caps.Features)
			}

			if !slices.Equal(caps.FutureKeywords, tc.expFutureKeywords) {
				t.Errorf("expected future keywords:\n\n%v\n\nbut got:\n\n%v", tc.expFutureKeywords, caps.FutureKeywords)
			}
		})
	}
}

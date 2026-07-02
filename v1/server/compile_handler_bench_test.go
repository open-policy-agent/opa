// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed testdata/bench_filters.rego
var benchRego []byte

//go:embed testdata/roles.json
var rolesJSON []byte

var roles = func() any {
	var roles any
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		panic(err)
	}
	return roles
}()

func BenchmarkCompileHandler(b *testing.B) {
	b.ReportAllocs()
	f := setup(b, string(benchRego), roles)

	input := map[string]any{
		"user": "caesar",
		"tenant": map[string]any{
			"id":   2,
			"name": "acmecorp",
		},
	}
	path := "filters/include"
	targets := []string{
		"application/vnd.opa.sql.postgresql+json",
		"application/vnd.opa.ucast.prisma+json",
	}

	for _, target := range targets {
		b.Run(strings.Split(target, "/")[1], func(b *testing.B) {
			// NB(sr): Unknowns are provided with the request: we don't want to benchmark the cache here
			// The percentile-recording tests below is making use of the unknowns cache.
			payload := map[string]any{
				"input":    input,
				"unknowns": []string{"input.tickets", "input.users"},
			}
			jsonData, err := json.Marshal(payload)
			if err != nil {
				b.Fatalf("Failed to marshal JSON: %v", err)
			}
			b.ResetTimer()

			for b.Loop() {
				req := httptest.NewRequest("POST", "/v1/compile/"+path, bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", target)

				if err := f.executeRequest(req, http.StatusOK, ""); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

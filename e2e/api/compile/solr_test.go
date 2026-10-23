// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/open-policy-agent/opa/v1/rego"
)

const solrCore = "fruit"

func solrDo(t *testing.T, method, u string, body any) []byte {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(bs)
	} else {
		r = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, u, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, u, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s: status %d: %s", method, u, resp.StatusCode, buf.String())
	}
	return buf.Bytes()
}

// setupSolr starts a Solr container with a "fruit" core, an explicit schema
// (so no field is tokenized or guessed), and the same four fruits the SQL e2e
// tests use. It returns the core's base URL.
func setupSolr(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, host, port, err := startContainer(ctx, containerConfig{
		image: "solr:10",
		cmd:   []string{"solr-precreate", solrCore},
		port:  "8983/tcp",
		waitFor: wait.ForHTTP("/solr/" + solrCore + "/admin/ping").
			WithPort("8983/tcp").
			WithStartupTimeout(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("failed to start solr: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	})
	core := fmt.Sprintf("http://%s:%s/solr/%s", host, port, solrCore)

	solrDo(t, http.MethodPost, core+"/schema", map[string]any{
		"add-field": []map[string]any{
			{"name": "name", "type": "string", "stored": true},
			{"name": "color", "type": "string", "stored": true},
			{"name": "price", "type": "pint", "stored": true},
			{"name": "tags", "type": "string", "multiValued": true, "stored": true},
			{"name": "origin.country", "type": "string", "stored": true},
		},
	})
	solrDo(t, http.MethodPost, core+"/update?commit=true", []map[string]any{
		{"id": "1", "name": "apple", "color": "green", "price": 10, "tags": []string{"fresh", "sweet"}, "origin.country": "ES"},
		{"id": "2", "name": "banana", "color": "yellow", "price": 20, "tags": []string{"sweet"}, "origin.country": "EC"},
		{"id": "3", "name": "cherry", "color": "red", "price": 11, "tags": []string{"fresh", "sour"}},
		{"id": "4", "name": "orange", "color": "orange", "tags": []string{}}, // no price, no tags
	})
	return core
}

// solrSchemas converts the core's schema with the script the docs offer for it, into
// the `schemas` annotation for input.fruits.
func solrSchemas(t *testing.T, core string) string {
	t.Helper()
	var resp any
	if err := json.Unmarshal(solrDo(t, http.MethodGet, core+"/schema", nil), &resp); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("../../../docs/static/filtering/solr_schema.rego")
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rego.New(
		rego.Module("solr_schema.rego", string(script)),
		rego.Query("data.solr.annotation"),
		rego.Input(resp),
	).Eval(t.Context())
	if err != nil || len(rs) != 1 {
		t.Fatalf("convert schema: %v %v", rs, err)
	}
	return strings.ReplaceAll(rs[0].Expressions[0].Value.(string), "input.doc", "input.fruits")
}

func solrIDs(t *testing.T, core, q string) []int {
	t.Helper()
	params := url.Values{
		"q":    {q},
		"fl":   {"id"},
		"sort": {"id asc"},
		"rows": {"100"},
		"wt":   {"json"},
	}
	var resp struct {
		Response struct {
			Docs []struct {
				ID string `json:"id"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(solrDo(t, http.MethodGet, core+"/select?"+params.Encode(), nil), &resp); err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, d := range resp.Response.Docs {
		id, err := strconv.Atoi(d.ID)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// TestCompileSolrE2E compiles policies to Lucene queries via the compile API,
// and runs them against a real Solr.
func TestCompileSolrE2E(t *testing.T) {
	if os.Getenv("DOCKER_RUNNING") != "1" {
		t.Skip("docker-dependant tests are not runnable")
	}
	opaURL := startOPA(t)

	core := setupSolr(t)
	schemas := solrSchemas(t, core)

	const (
		apple, banana, cherry, orange = 1, 2, 3, 4
		// `color` is the Solr field name; the policies call it `colour`.
		colourMapping = "color"
	)

	tests := []struct {
		name   string
		policy string
		exp    []int
	}{
		{name: "no conditions", policy: `include if true`, exp: []int{apple, banana, cherry, orange}},
		{name: "unconditional NO", policy: `include if false`},
		{name: "equality, field mapping", policy: `include if input.fruits.colour == input.fav_colour`, exp: []int{banana}},
		{name: "lt", policy: `include if input.fruits.price < 11`, exp: []int{apple}},
		{name: "gte", policy: `include if input.fruits.price >= 11`, exp: []int{banana, cherry}},
		{name: "gt, flipped operands", policy: `include if 11 < input.fruits.price`, exp: []int{banana}},
		{name: "string range", policy: `include if input.fruits.name >= "c"`, exp: []int{cherry, orange}},
		{name: "missing", policy: `include if not input.fruits.price != null`, exp: []int{orange}},
		{name: "not equal null", policy: `include if input.fruits.price != null`, exp: []int{apple, banana, cherry}},
		{name: "exists", policy: `include if input.fruits.price = _`, exp: []int{apple, banana, cherry}},
		{name: "in", policy: `include if input.fruits.name in {"apple", "cherry", "pineapple"}`, exp: []int{apple, cherry}},
		{name: "not in", policy: `include if not input.fruits.name in {"apple", "cherry", "pineapple", "orange"}`, exp: []int{banana}},
		{
			name: "conjunct, inequality",
			policy: `include if {
				input.fruits.name != "apple"
				input.fruits.name != "banana"
			}`,
			exp: []int{cherry, orange},
		},
		{
			name: "disjunct, equality",
			policy: `include if input.fruits.name == "apple"
				include if input.fruits.name == "banana"`,
			exp: []int{apple, banana},
		},
		{
			// Like Rego with the improved `not`: `price` is undefined for orange, so the comparison is, and its negation holds.
			name:   "not lt, missing field matches",
			policy: `include if not input.fruits.price < 12`,
			exp:    []int{banana, orange},
		},
		{
			// Like Rego: `price` is undefined for orange, so the comparison is too.
			name:   "neq, missing field does not match",
			policy: `include if input.fruits.price != 11`,
			exp:    []int{apple, banana},
		},
		{
			name:   "not eq, missing field matches",
			policy: `include if not input.fruits.price == 10`,
			exp:    []int{banana, cherry, orange},
		},
		{
			name: "mix of positive and negative",
			policy: `include if {
				input.fruits.price >= 10
				not input.fruits.name == "banana"
			}`,
			exp: []int{apple, cherry},
		},
		{name: "startswith", policy: `include if startswith(input.fruits.name, "a")`, exp: []int{apple}},
		{name: "endswith", policy: `include if endswith(input.fruits.name, "e")`, exp: []int{apple, orange}},
		{name: "contains", policy: `include if contains(input.fruits.name, "an")`, exp: []int{banana, orange}},
		{name: "not startswith", policy: `include if not startswith(input.fruits.name, "a")`, exp: []int{banana, cherry, orange}},
		// If the `*` weren't escaped, this would select apple.
		{name: "startswith, wildcard in value is literal", policy: `include if startswith(input.fruits.name, "a*")`},
		{name: "multi-valued: some, startswith", policy: `include if { some t in input.fruits.tags; startswith(t, "s") }`, exp: []int{apple, banana, cherry}},
		{name: "multi-valued: some, endswith", policy: `include if { some t in input.fruits.tags; endswith(t, "t") }`, exp: []int{apple, banana}},
		{name: "multi-valued: some, contains", policy: `include if { some t in input.fruits.tags; contains(t, "ou") }`, exp: []int{cherry}},
		{name: "any prefix", policy: `include if strings.any_prefix_match(input.fruits.name, ["ap", "ch"])`, exp: []int{apple, cherry}},
		{name: "multi-valued: any suffix of the array", policy: `include if strings.any_suffix_match(input.fruits.tags, {"sh", "ur"})`, exp: []int{apple, cherry}},
		{name: "multi-valued: every, none starts with", policy: `include if every t in input.fruits.tags { not startswith(t, "s") }`, exp: []int{orange}},
		{name: "nested field", policy: `include if input.fruits.origin.country == "EC"`, exp: []int{banana}},
		{name: "nested field, not", policy: `include if not input.fruits.origin.country == "EC"`, exp: []int{apple, cherry, orange}},
		{name: "multi-valued: contains", policy: `include if "sweet" in input.fruits.tags`, exp: []int{apple, banana}},
		{name: "multi-valued: some, eq", policy: `include if input.fruits.tags[_] == "fresh"`, exp: []int{apple, cherry}},
		{
			name:   "multi-valued: some, in",
			policy: `include if { some t in input.fruits.tags; t in {"sour", "sweet"} }`,
			exp:    []int{apple, banana, cherry},
		},
		{name: "multi-valued: not contains", policy: `include if not "sweet" in input.fruits.tags`, exp: []int{cherry, orange}},
		{
			// Relies on Solr not indexing an empty array: such a document has no `tags` field.
			name:   "multi-valued: count is zero",
			policy: `include if count(input.fruits.tags) == 0`,
			exp:    []int{orange},
		},
		{
			name:   "multi-valued: every, none is",
			policy: `include if every t in input.fruits.tags { t != "sweet" }`,
			exp:    []int{cherry, orange},
		},
		{
			// Relies on Solr accepting ranges in a field group: `tags:({* TO "sweet"} OR {"sweet" TO *})`.
			name:   "multi-valued: every, all are",
			policy: `include if every t in input.fruits.tags { t == "sweet" }`,
			exp:    []int{banana, orange},
		},
		{
			name:   "multi-valued: every, all in",
			policy: `include if every t in input.fruits.tags { t in {"sweet", "sour"} }`,
			exp:    []int{banana, orange},
		},
		{
			name:   "multi-valued: same values",
			policy: `include if {t | some t in input.fruits.tags} == {"sweet", "fresh"}`,
			exp:    []int{apple},
		},
		{
			name:   "multi-valued: every known, none of them in",
			policy: `include if every g in input.groups { not g in input.fruits.tags }`,
			exp:    []int{orange},
		},
		{name: "multi-valued: element range", policy: `include if { some t in input.fruits.tags; t >= "s" }`, exp: []int{apple, banana, cherry}},
		{
			name:   "multi-valued: every, none below",
			policy: `include if every t in input.fruits.tags { not t < "s" }`,
			exp:    []int{banana, orange},
		},
		{
			name:   "multi-valued: combined",
			policy: `include if { "fresh" in input.fruits.tags; input.fruits.price < 11 }`,
			exp:    []int{apple},
		},
		{
			// If the quote weren't escaped, this would select everything.
			name:   "quote in value",
			policy: `include if input.fruits.name == "apple\" OR name:*"`,
		},
		{
			name:   "wildcards in value are literal",
			policy: `include if input.fruits.name == "app*"`,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := fmt.Sprintf("# METADATA\n# scope: package\n%s\npackage filters%d\n%s", schemas, i, tt.policy)
			putPolicy(t, opaURL, fmt.Sprintf("solr%d.rego", i), policy)

			status, respBody := postCompile(t, opaURL, fmt.Sprintf("filters%d/include", i), "application/vnd.opa.lucene.solr+json", map[string]any{
				"input":    map[string]any{"fav_colour": "yellow", "groups": []string{"sweet", "sour"}},
				"unknowns": []string{"input.fruits"},
				"options": map[string]any{
					"targetSQLTableMappings": map[string]any{
						"lucene": map[string]any{"fruits": map[string]any{"colour": colourMapping}},
					},
				},
			})
			if status != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, status, respBody)
			}
			var body struct {
				Result struct {
					Query *string `json:"query"`
				} `json:"result"`
			}
			if err := json.Unmarshal(respBody, &body); err != nil {
				t.Fatal(err)
			}

			var act []int
			if q := body.Result.Query; q != nil { // nil: unconditional NO
				t.Log(*q)
				act = solrIDs(t, core, *q)
			}
			if diff := cmp.Diff(tt.exp, act); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

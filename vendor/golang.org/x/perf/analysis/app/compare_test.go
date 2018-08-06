// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/context"
	"golang.org/x/perf/storage"
	"golang.org/x/perf/storage/benchfmt"
)

func TestResultGroup(t *testing.T) {
	data := `key: value
BenchmarkName 1 ns/op
key: value2
BenchmarkName 1 ns/op`
	var results []*benchfmt.Result
	br := benchfmt.NewReader(strings.NewReader(data))
	g := &resultGroup{}
	for br.Next() {
		results = append(results, br.Result())
		g.add(br.Result())
	}
	if err := br.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if !reflect.DeepEqual(g.results, results) {
		t.Errorf("g.results = %#v, want %#v", g.results, results)
	}
	if want := map[string]valueSet{"key": {"value": 1, "value2": 1}}; !reflect.DeepEqual(g.LabelValues, want) {
		t.Errorf("g.LabelValues = %#v, want %#v", g.LabelValues, want)
	}
	groups := g.splitOn("key")
	if len(groups) != 2 {
		t.Fatalf("g.splitOn returned %d groups, want 2", len(groups))
	}
	for i, results := range [][]*benchfmt.Result{
		{results[0]},
		{results[1]},
	} {
		if !reflect.DeepEqual(groups[i].results, results) {
			t.Errorf("groups[%d].results = %#v, want %#v", i, groups[i].results, results)
		}
	}
}

// static responses for TestCompareQuery
var compareQueries = map[string]string{
	"one": `upload: 1
upload-part: 1
label: value
BenchmarkOne 1 5 ns/op
BenchmarkTwo 1 10 ns/op`,
	"two": `upload: 1
upload-part: 2
BenchmarkOne 1 10 ns/op
BenchmarkTwo 1 5 ns/op`,
	"onetwo": `upload: 1
upload-part: 1
label: value
BenchmarkOne 1 5 ns/op
BenchmarkTwo 1 10 ns/op
label:
upload-part: 2
BenchmarkOne 1 10 ns/op
BenchmarkTwo 1 5 ns/op`,
}

func TestCompareQuery(t *testing.T) {
	// TODO(quentin): This test seems too heavyweight; we are but shouldn't be also testing the storage client -> storage server interaction.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm = %v", err)
		}
		q := r.Form.Get("q")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, compareQueries[q])
	}))
	defer ts.Close()

	a := &App{StorageClient: &storage.Client{BaseURL: ts.URL}}

	for _, q := range []string{"one vs two", "onetwo"} {
		t.Run(q, func(t *testing.T) {
			data := a.compareQuery(context.Background(), q)
			if data.Error != "" {
				t.Fatalf("compareQuery failed: %s", data.Error)
			}
			if have := data.Q; have != q {
				t.Errorf("Q = %q, want %q", have, q)
			}
			if len(data.Groups) != 2 {
				t.Errorf("len(Groups) = %d, want 2", len(data.Groups))
			}
			if len(data.Benchstat) == 0 {
				t.Error("len(Benchstat) = 0, want >0")
			}
			if want := map[string]bool{"upload-part": true, "label": true}; !reflect.DeepEqual(data.Labels, want) {
				t.Errorf("Labels = %#v, want %#v", data.Labels, want)
			}
			if want := (benchfmt.Labels{"upload": "1"}); !reflect.DeepEqual(data.CommonLabels, want) {
				t.Errorf("CommonLabels = %#v, want %#v", data.CommonLabels, want)
			}
		})
	}
}

func TestAddToQuery(t *testing.T) {
	tests := []struct {
		query, add string
		want       string
	}{
		{"one", "two", "two | one"},
		{"pre | one vs two", "three", "three pre | one vs two"},
		{"four", "five six", `"five six" | four`},
		{"seven", `extra "fun"\problem`, `"extra \"fun\"\\problem" | seven`},
		{"eight", `ni\"ne`, `"ni\\\"ne" | eight`},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			if got := addToQuery(test.query, test.add); got != test.want {
				t.Errorf("addToQuery(%q, %q) = %q, want %q", test.query, test.add, got, test.want)
			}
		})
	}
}

func TestElideKeyValues(t *testing.T) {
	type sb map[string]bool
	tests := []struct {
		content string
		keys    sb
		want    string
	}{
		{"BenchmarkOne/key=1-1 1 ns/op", sb{"key": true}, "BenchmarkOne/key=*-1 1 ns/op"},
		{"BenchmarkOne/key=1-2 1 ns/op", sb{"other": true}, "BenchmarkOne/key=1-2 1 ns/op"},
		{"BenchmarkOne/key=1/key2=2-3 1 ns/op", sb{"key": true}, "BenchmarkOne/key=*/key2=2-3 1 ns/op"},
		{"BenchmarkOne/foo/bar-4 1 ns/op", sb{"sub1": true}, "BenchmarkOne/*/bar-4 1 ns/op"},
		{"BenchmarkOne/foo/bar-5 1 ns/op", sb{"gomaxprocs": true}, "BenchmarkOne/foo/bar-* 1 ns/op"},
		{"BenchmarkOne/foo/bar-6 1 ns/op", sb{"name": true}, "Benchmark*/foo/bar-6 1 ns/op"},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			have := elideKeyValues(test.content, test.keys)
			if have != test.want {
				t.Errorf("elideKeys(%q, %#v) = %q, want %q", test.content, map[string]bool(test.keys), have, test.want)
			}
		})
	}
}

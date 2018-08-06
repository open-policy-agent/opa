// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db_test

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/perf/internal/diff"
	"golang.org/x/perf/storage/benchfmt"
	. "golang.org/x/perf/storage/db"
	"golang.org/x/perf/storage/db/dbtest"
)

// Most of the db package is tested via the end-to-end-tests in perf/storage/app.

// TestUploadIDs verifies that NewUpload generates the correct sequence of upload IDs.
func TestUploadIDs(t *testing.T) {
	ctx := context.Background()

	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	defer SetNow(time.Time{})

	tests := []struct {
		sec int64
		id  string
	}{
		{0, "19700101.1"},
		{0, "19700101.2"},
		{86400, "19700102.1"},
		{86400, "19700102.2"},
		{86400, "19700102.3"},
		{86400, "19700102.4"},
		{86400, "19700102.5"},
		{86400, "19700102.6"},
		{86400, "19700102.7"},
		{86400, "19700102.8"},
		{86400, "19700102.9"},
		{86400, "19700102.10"},
		{86400, "19700102.11"},
	}
	for _, test := range tests {
		SetNow(time.Unix(test.sec, 0))
		u, err := db.NewUpload(ctx)
		if err != nil {
			t.Fatalf("NewUpload: %v", err)
		}
		if err := u.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if u.ID != test.id {
			t.Fatalf("u.ID = %q, want %q", u.ID, test.id)
		}
	}
}

// checkQueryResults performs a query on db and verifies that the
// results as printed by BenchmarkPrinter are equal to results.
func checkQueryResults(t *testing.T, db *DB, query, results string) {
	q := db.Query(query)
	defer q.Close()

	var buf bytes.Buffer
	bp := benchfmt.NewPrinter(&buf)

	for q.Next() {
		if err := bp.Print(q.Result()); err != nil {
			t.Fatalf("Print: %v", err)
		}
	}
	if err := q.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if diff := diff.Diff(buf.String(), results); diff != "" {
		t.Errorf("wrong results: (- have/+ want)\n%s", diff)
	}
}

// TestReplaceUpload verifies that the expected number of rows exist after replacing an upload.
func TestReplaceUpload(t *testing.T) {
	SetNow(time.Unix(0, 0))
	defer SetNow(time.Time{})
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	ctx := context.Background()

	labels := benchfmt.Labels{"key": "value"}

	u, err := db.NewUpload(ctx)
	if err != nil {
		t.Fatalf("NewUpload: %v", err)
	}
	labels["uploadid"] = u.ID
	for _, num := range []string{"1", "2"} {
		labels["num"] = num
		for _, num2 := range []int{1, 2} {
			if err := u.InsertRecord(&benchfmt.Result{
				labels,
				nil,
				1,
				fmt.Sprintf("BenchmarkName %d ns/op", num2),
			}); err != nil {
				t.Fatalf("InsertRecord: %v", err)
			}
			labels = labels.Copy()
		}
	}

	if err := u.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	checkQueryResults(t, db, "key:value",
		`key: value
num: 1
uploadid: 19700101.1
BenchmarkName 1 ns/op
BenchmarkName 2 ns/op
num: 2
BenchmarkName 1 ns/op
BenchmarkName 2 ns/op
`)

	labels["num"] = "3"

	for _, uploadid := range []string{u.ID, "new"} {
		u, err := db.ReplaceUpload(uploadid)
		if err != nil {
			t.Fatalf("ReplaceUpload: %v", err)
		}
		labels["uploadid"] = u.ID
		if err := u.InsertRecord(&benchfmt.Result{
			labels,
			nil,
			1,
			"BenchmarkName 3 ns/op",
		}); err != nil {
			t.Fatalf("InsertRecord: %v", err)
		}
		labels = labels.Copy()

		if err := u.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}

	checkQueryResults(t, db, "key:value",
		`key: value
num: 3
uploadid: 19700101.1
BenchmarkName 3 ns/op
uploadid: new
BenchmarkName 3 ns/op
`)
}

// TestNewUpload verifies that NewUpload and InsertRecord wrote the correct rows to the database.
func TestNewUpload(t *testing.T) {
	SetNow(time.Unix(0, 0))
	defer SetNow(time.Time{})
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	u, err := db.NewUpload(context.Background())
	if err != nil {
		t.Fatalf("NewUpload: %v", err)
	}

	br := benchfmt.NewReader(strings.NewReader(`
key: value
BenchmarkName 1 ns/op
BenchmarkName 2 ns/op
`))
	for br.Next() {
		if err := u.InsertRecord(br.Result()); err != nil {
			t.Fatalf("InsertRecord: %v", err)
		}
	}
	if err := br.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if err := u.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	rows, err := DBSQL(db).Query("SELECT UploadId, RecordId, Name, Value FROM RecordLabels")
	if err != nil {
		t.Fatalf("sql.Query: %v", err)
	}
	defer rows.Close()

	want := map[string]string{
		"key":  "value",
		"name": "Name",
	}

	i := 0

	for rows.Next() {
		var uploadid string
		var recordid int64
		var name, value string

		if err := rows.Scan(&uploadid, &recordid, &name, &value); err != nil {
			t.Fatalf("rows.Scan: %v", err)
		}
		if uploadid != "19700101.1" {
			t.Errorf("uploadid = %q, want %q", uploadid, "19700101.1")
		}
		if recordid != 0 {
			t.Errorf("recordid = %d, want 0", recordid)
		}
		if want[name] != value {
			t.Errorf("%s = %q, want %q", name, value, want[name])
		}
		i++
	}
	if i != len(want) {
		t.Errorf("have %d labels, want %d", i, len(want))
	}

	if err := rows.Err(); err != nil {
		t.Errorf("rows.Err: %v", err)
	}
}

func TestQuery(t *testing.T) {
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	u, err := db.NewUpload(context.Background())
	if err != nil {
		t.Fatalf("NewUpload: %v", err)
	}

	var allRecords []int

	for i := 0; i < 1024; i++ {
		allRecords = append(allRecords, i)
		r := &benchfmt.Result{Labels: make(map[string]string), NameLabels: make(map[string]string), Content: "BenchmarkName 1 ns/op"}
		r.Labels["upload"] = u.ID
		for j := uint(0); j < 10; j++ {
			r.Labels[fmt.Sprintf("label%d", j)] = fmt.Sprintf("%d", i/(1<<j))
		}
		r.NameLabels["name"] = "Name"
		if err := u.InsertRecord(r); err != nil {
			t.Fatalf("InsertRecord: %v", err)
		}
	}
	if err := u.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	tests := []struct {
		q    string
		want []int // nil means we want an error
	}{
		{"label0:0", []int{0}},
		{"label0:1 label0:1 label0<2 label0>0", []int{1}},
		{"label0>0 label0<2 label0:1 label0:1", []int{1}},
		{"label0<2 label0<1", []int{0}},
		{"label0>1021 label0>1022 label1:511", []int{1023}},
		{"label1:0", []int{0, 1}},
		{"label0:5 name:Name", []int{5}},
		{"label0:0 label0:5", []int{}},
		{"bogus query", nil},
		{"label1<2 label3:0", []int{0, 1, 2, 3}},
		{"label1>510 label1<52", []int{1022, 1023}},
		{"", allRecords},
		{"missing>", []int{}},
		{"label0>", allRecords},
		{"upload:" + u.ID, allRecords},
		{"upload:none", []int{}},
		{"upload>" + u.ID, []int{}},
		{"upload<" + u.ID, []int{}},
		{"label0:0 upload:" + u.ID, []int{0}},
	}
	for _, test := range tests {
		t.Run("query="+test.q, func(t *testing.T) {
			q := db.Query(test.q)
			if test.want == nil {
				if q.Next() {
					t.Fatal("Next() = true, want false")
				}
				if err := q.Err(); err == nil {
					t.Fatal("Err() = nil, want error")
				}
				return
			}
			defer func() {
				t.Logf("q.Debug: %s", q.Debug())
				if err := q.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}()
			var have []int
			for i := range test.want {
				if !q.Next() {
					t.Fatalf("#%d: Next() = false", i)
				}
				r := q.Result()
				n, err := strconv.Atoi(r.Labels["label0"])
				if err != nil {
					t.Fatalf("unexpected label0 value %q: %v", r.Labels["label0"], err)
				}
				have = append(have, n)
				if r.NameLabels["name"] != "Name" {
					t.Errorf("result[%d].name = %q, want %q", i, r.NameLabels["name"], "Name")
				}
			}
			for q.Next() {
				r := q.Result()
				t.Errorf("Next() = true, want false (got labels %v)", r.Labels)
			}
			if err := q.Err(); err != nil {
				t.Errorf("Err() = %v, want nil", err)
			}
			sort.Ints(have)
			if len(have) == 0 {
				have = []int{}
			}
			if !reflect.DeepEqual(have, test.want) {
				t.Errorf("label0[] = %v, want %v", have, test.want)
			}
		})
	}
}

// TestListUploads verifies that ListUploads returns the correct values.
func TestListUploads(t *testing.T) {
	SetNow(time.Unix(0, 0))
	defer SetNow(time.Time{})
	db, cleanup := dbtest.NewDB(t)
	defer cleanup()

	for i := -1; i < 9; i++ {
		u, err := db.NewUpload(context.Background())
		if err != nil {
			t.Fatalf("NewUpload: %v", err)
		}
		for j := 0; j <= i; j++ {
			labels := benchfmt.Labels{
				"key": "value",
				"i":   fmt.Sprintf("%d", i),
				"j":   fmt.Sprintf("%d", j),
			}
			if err := u.InsertRecord(&benchfmt.Result{
				labels,
				nil,
				1,
				fmt.Sprintf("BenchmarkName %d ns/op", j),
			}); err != nil {
				t.Fatalf("InsertRecord: %v", err)
			}
		}
		if err := u.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}

	type result struct {
		count int
		id    string
	}

	tests := []struct {
		query       string
		extraLabels []string
		limit       int
		want        []result
	}{
		{"", nil, 0, []result{{9, "19700101.10"}, {8, "19700101.9"}, {7, "19700101.8"}, {6, "19700101.7"}, {5, "19700101.6"}, {4, "19700101.5"}, {3, "19700101.4"}, {2, "19700101.3"}, {1, "19700101.2"}}},
		{"", nil, 2, []result{{9, "19700101.10"}, {8, "19700101.9"}}},
		{"j:5", nil, 0, []result{{1, "19700101.10"}, {1, "19700101.9"}, {1, "19700101.8"}, {1, "19700101.7"}}},
		{"i:5", nil, 0, []result{{6, "19700101.7"}}},
		{"i:5", []string{"i", "missing"}, 0, []result{{6, "19700101.7"}}},
		{"not:found", nil, 0, nil},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("query=%s/limit=%d", test.query, test.limit), func(t *testing.T) {
			r := db.ListUploads(test.query, test.extraLabels, test.limit)
			defer func() {
				t.Logf("r.Debug: %s", r.Debug())
				r.Close()
			}()
			var have []result
			for r.Next() {
				ui := r.Info()
				res := result{ui.Count, ui.UploadID}
				have = append(have, res)
				for k, v := range ui.LabelValues {
					switch k {
					case "i":
						uploadNum, err := strconv.Atoi(res.id[strings.LastIndex(res.id, ".")+1:])
						if err != nil {
							t.Fatalf("cannot parse upload ID %q", res.id)
						}
						if v != fmt.Sprintf("%d", uploadNum-2) {
							t.Errorf(`i = %q, want "%d"`, v, uploadNum-2)
						}
					default:
						t.Errorf("unexpected label %q", k)
					}
				}
			}
			if err := r.Err(); err != nil {
				t.Errorf("Err() = %v", err)
			}
			if !reflect.DeepEqual(have, test.want) {
				t.Errorf("results = %v, want %v", have, test.want)
			}
		})
	}
}

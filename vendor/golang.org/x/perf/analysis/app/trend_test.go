// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"testing"

	"github.com/aclements/go-gg/table"
	"golang.org/x/perf/internal/diff"
)

func TestTableToJS(t *testing.T) {
	in := table.TableFromStrings(
		[]string{"text", "num"},
		[][]string{
			{"hello", "15.1"},
			{"world", "20"},
		}, true)
	have := tableToJS(in, []column{{Name: "text"}, {Name: "num"}})
	want := `{cols: [{"id":"text","type":"string","label":"text"},
{"id":"num","type":"number","label":"num"}],
rows: [{c:[{v: "hello"}, {v: 15.1}]},
{c:[{v: "world"}, {v: 20}]}]}`
	if d := diff.Diff(string(have), want); d != "" {
		t.Errorf("tableToJS returned wrong JS (- have, + want):\n%s", d)
	}
}

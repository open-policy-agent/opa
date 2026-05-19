package ir

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMakeNumberRefStmtMarshalsBothKeys(t *testing.T) {
	stmt := &MakeNumberRefStmt{
		Index:  7,
		Target: 3,
	}
	stmt.SetLocation(2, 11, 5, "test.rego", "")

	bs, err := json.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(bs)

	for _, want := range []string{
		`"index":7`,
		`"Index":7`,
		`"target":3`,
		`"file":2`,
		`"row":11`,
		`"col":5`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n  got: %s", want, got)
		}
	}
}

func TestMakeNumberRefStmtUnmarshalAcceptsBothKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{
			name: "lowercase index only",
			in:   `{"file":0,"col":0,"row":0,"index":42,"target":1}`,
			want: 42,
		},
		{
			name: "uppercase Index only (legacy)",
			in:   `{"file":0,"col":0,"row":0,"Index":42,"target":1}`,
			want: 42,
		},
		{
			name: "both present, lowercase wins",
			in:   `{"file":0,"col":0,"row":0,"index":42,"Index":99,"target":1}`,
			want: 42,
		},
		{
			name: "both present in opposite order, lowercase still wins",
			in:   `{"file":0,"col":0,"row":0,"Index":99,"index":42,"target":1}`,
			want: 42,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stmt MakeNumberRefStmt
			if err := json.Unmarshal([]byte(tc.in), &stmt); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if stmt.Index != tc.want {
				t.Fatalf("Index = %d, want %d", stmt.Index, tc.want)
			}
		})
	}
}

func TestMakeNumberRefStmtRoundTrip(t *testing.T) {
	orig := &MakeNumberRefStmt{Index: 13, Target: 4}
	orig.SetLocation(1, 2, 3, "", "")

	bs, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got MakeNumberRefStmt
	if err := json.Unmarshal(bs, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Index != orig.Index || got.Target != orig.Target ||
		got.File != orig.File || got.Row != orig.Row || got.Col != orig.Col {
		t.Fatalf("round-trip mismatch\n  orig: %+v\n  got:  %+v", *orig, got)
	}
}

package ir

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestMakeNumberRefStmtMarshalsBothKeys(t *testing.T) {
	stmt := &MakeNumberRefStmt{
		Index:  7,
		Target: 3,
	}
	stmt.SetLocation(2, 11, 5, "test.rego", nil)

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

func TestLocationEndRowColSetAndMarshalled(t *testing.T) {
	cases := map[string]struct {
		text       []byte
		wantEndRow int
		wantEndCol int
	}{
		"single line": {
			text:       []byte("foo"),
			wantEndRow: 11,
			wantEndCol: 8,
		},
		"multi line": {
			text:       []byte("foo\nbar"),
			wantEndRow: 12,
			wantEndCol: 4,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stmt := &MakeNumberRefStmt{
				Index:  7,
				Target: 3,
			}
			stmt.SetLocation(2, 11, 5, "test.rego", tc.text)

			endRow, endCol := stmt.Location.End()
			if endRow != tc.wantEndRow {
				t.Errorf("expected end row %d, got %d", tc.wantEndRow, endRow)
			}
			if endCol != tc.wantEndCol {
				t.Errorf("expected end col %d, got %d", tc.wantEndCol, endCol)
			}

			bs, err := json.Marshal(stmt)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := string(bs)

			for _, want := range []string{
				fmt.Sprintf(`"end_row":%d`, tc.wantEndRow),
				fmt.Sprintf(`"end_col":%d`, tc.wantEndCol),
			} {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\n  got: %s", want, got)
				}
			}

			var roundTripped MakeNumberRefStmt
			if err := json.Unmarshal(bs, &roundTripped); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if roundTripped.EndRow != tc.wantEndRow {
				t.Errorf("round-tripped end row = %d, want %d", roundTripped.EndRow, tc.wantEndRow)
			}
			if roundTripped.EndCol != tc.wantEndCol {
				t.Errorf("round-tripped end col = %d, want %d", roundTripped.EndCol, tc.wantEndCol)
			}
		})
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
	orig.SetLocation(1, 2, 3, "", nil)

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

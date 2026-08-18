package location

import (
	"encoding/json"
	"testing"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

func TestLocationCompare(t *testing.T) {
	tests := []struct {
		a   string
		b   string
		exp int
	}{
		{
			a:   "",
			b:   "",
			exp: 0,
		},
		{
			a:   "",
			b:   `{"file": "a", "row": 1, "col": 1}`,
			exp: 1,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 1}`,
			b:   "",
			exp: -1,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 1}`,
			b:   `{"file": "a", "row": 1, "col": 1}`,
			exp: 0,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 1}`,
			b:   `{"file": "b", "row": 1, "col": 1}`,
			exp: -1,
		},
		{
			a:   `{"file": "b", "row": 1, "col": 1}`,
			b:   `{"file": "a", "row": 1, "col": 1}`,
			exp: 1,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 1}`,
			b:   `{"file": "a", "row": 2, "col": 1}`,
			exp: -1,
		},
		{
			a:   `{"file": "a", "row": 2, "col": 1}`,
			b:   `{"file": "a", "row": 1, "col": 1}`,
			exp: 1,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 1}`,
			b:   `{"file": "a", "row": 1, "col": 2}`,
			exp: -1,
		},
		{
			a:   `{"file": "a", "row": 1, "col": 2}`,
			b:   `{"file": "a", "row": 1, "col": 1}`,
			exp: 1,
		},
	}

	unmarshal := func(s string) *Location {
		if s != "" {
			var loc Location
			if err := util.Unmarshal([]byte(s), &loc); err != nil {
				t.Fatal(err)
			}
			return &loc
		}
		return nil
	}

	for _, tc := range tests {
		locA := unmarshal(tc.a)
		locB := unmarshal(tc.b)
		result := locA.Compare(locB)
		if tc.exp != result {
			t.Fatalf("Expected %v but got %v for %v.Compare(%v)", tc.exp, result, locA, locB)
		}
	}

	loc1 := &Location{File: "file1.rego", Row: 10, Col: 5}
	loc2 := loc1
	if loc1.Compare(loc2) != 0 {
		t.Fatalf("Expected loc1 to be equal to loc2 (pointer equality)")
	}
	loc1, loc2 = nil, nil
	if loc1.Compare(loc2) != 0 {
		t.Fatalf("Expected loc1 to be equal to loc2 (both nil)")
	}
}

func TestLocationMarshal(t *testing.T) {
	testCases := map[string]struct {
		loc     *Location
		options astJSON.Options
		exp     string
	}{
		"default json options": {
			loc: &Location{
				Text: []byte("text"),
				File: "file",
				Row:  1,
				Col:  1,
			},
			exp: `{"file":"file","row":1,"col":1}`,
		},
		"including text": {
			loc: &Location{
				Text: []byte("text"),
				File: "file",
				Row:  1,
				Col:  1,
			},
			options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocationText: true,
				},
			},
			exp: `{"file":"file","row":1,"col":1,"text":"dGV4dA=="}`,
		},
		"including text, but no text present": {
			loc: &Location{
				File: "file",
				Row:  1,
				Col:  1,
			},
			options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocationText: true,
				},
			},
			exp: `{"file":"file","row":1,"col":1}`,
		},
		"excluding file": {
			loc: &Location{
				File: "file",
				Row:  1,
				Col:  1,
			},
			options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					ExcludeLocationFile: true,
				},
			},
			exp: `{"row":1,"col":1}`,
		},
	}

	for id, tc := range testCases {
		t.Run(id, func(t *testing.T) {
			astJSON.SetOptions(tc.options)
			defer astJSON.SetOptions(astJSON.Defaults())

			bs, err := json.Marshal(tc.loc)
			if err != nil {
				t.Fatal(err)
			}
			if string(bs) != tc.exp {
				t.Fatalf("Expected %v but got %v", tc.exp, string(bs))
			}
		})
	}
}

func TestLocationUnmarshal(t *testing.T) {
	// Location has no custom unmarshaller on any Go version: decoding goes
	// through the struct tags, which means the ignored ("-") fields are not
	// populated and unknown keys are tolerated.
	in := `{"file":"p.rego","row":1,"col":2,"text":"dGVzdA==","tabs":[1],"unexpected":true}`

	var loc Location
	if err := util.UnmarshalJSON([]byte(in), &loc); err != nil {
		t.Fatal(err)
	}

	if exp, act := "p.rego", loc.File; exp != act {
		t.Errorf("Expected file %q but got %q", exp, act)
	}
	if exp, act := 1, loc.Row; exp != act {
		t.Errorf("Expected row %v but got %v", exp, act)
	}
	if exp, act := 2, loc.Col; exp != act {
		t.Errorf("Expected col %v but got %v", exp, act)
	}
	if loc.Text != nil {
		t.Errorf("Expected no text but got %q", string(loc.Text))
	}
	if loc.Tabs != nil {
		t.Errorf("Expected no tabs but got %v", loc.Tabs)
	}
}

func TestLocationString(t *testing.T) {
	tests := []struct {
		loc *Location
		exp string
	}{
		{
			loc: &Location{File: "file1.rego", Row: 10, Col: 5},
			exp: "file1.rego:10",
		},
		{
			loc: &Location{Row: 1, Col: 20},
			exp: "1:20",
		},
		{
			loc: &Location{Text: []byte("some text")},
			exp: "some text",
		}}

	for _, tc := range tests {
		str := tc.loc.String()
		if str != tc.exp {
			t.Fatalf("Expected %v but got %v for String()", tc.exp, str)
		}
	}
}

func TestLocationHasFile(t *testing.T) {
	tests := map[string]struct {
		loc *Location
		exp bool
	}{
		"nil receiver": {loc: nil, exp: false},
		"empty file":   {loc: &Location{Row: 1, Col: 1}, exp: false},
		"with file":    {loc: &Location{File: "x.rego", Row: 1, Col: 1}, exp: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tc.loc.HasFile(); got != tc.exp {
				t.Fatalf("Expected %v but got %v", tc.exp, got)
			}
		})
	}
}

func TestEndOf(t *testing.T) {
	tests := map[string]struct {
		row, col int
		text     []byte
		expRow   int
		expCol   int
	}{
		"single-line text": {
			text: []byte("false"), row: 3, col: 10,
			expRow: 3,
			expCol: 15,
		},
		"multi-line text": {
			text: []byte("a\nbc"), row: 5, col: 2,
			expRow: 6,
			expCol: 3,
		},
		"multi-byte runes count as one column each": {
			// "café" is 5 bytes but 4 runes; the scanner advances Col
			// per rune (see scanner.next), so EndOf must too.
			text: []byte("café"), row: 1, col: 1,
			expRow: 1,
			expCol: 5,
		},
		"multi-byte runes across a newline": {
			text: []byte("café\nñ"), row: 1, col: 1,
			expRow: 2,
			expCol: 2,
		},
		"single multi-byte rune": {
			text: []byte("é"), row: 1, col: 1,
			expRow: 1,
			expCol: 2,
		},
		"empty text": {
			text: nil, row: 4, col: 7,
			expRow: 4,
			expCol: 7,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			row, col := EndOf(tc.row, tc.col, tc.text)
			if row != tc.expRow || col != tc.expCol {
				t.Fatalf("Expected (%d, %d) but got (%d, %d)", tc.expRow, tc.expCol, row, col)
			}
		})
	}
}

func TestLocationEnd(t *testing.T) {
	tests := map[string]struct {
		loc    *Location
		expRow int
		expCol int
	}{
		"delegates to EndOf": {
			loc:    &Location{Text: []byte("a\nbc"), Row: 5, Col: 2},
			expRow: 6,
			expCol: 3,
		},
		"nil receiver": {
			loc:    nil,
			expRow: 0,
			expCol: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			row, col := tc.loc.End()
			if row != tc.expRow || col != tc.expCol {
				t.Fatalf("Expected (%d, %d) but got (%d, %d)", tc.expRow, tc.expCol, row, col)
			}
		})
	}
}

// Verify zero allocations for Location.AppendText.
func BenchmarkLocationAppendText(b *testing.B) {
	locs := []*Location{
		{File: "file1.rego", Row: 10, Col: 5},
		{Row: 1, Col: 20},
		{Text: []byte("some text")},
	}

	for _, loc := range locs {
		b.Run(loc.String(), func(b *testing.B) {
			buf := make([]byte, 0, loc.StringLength())
			for b.Loop() {
				if _, err := loc.AppendText(buf); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

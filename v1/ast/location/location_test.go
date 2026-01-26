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

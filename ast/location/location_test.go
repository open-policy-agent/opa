package location

import (
	"encoding/json"
	"testing"

	astJSON "github.com/open-policy-agent/opa/ast/json"
	"github.com/open-policy-agent/opa/util"
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
}

func TestLocationMarshal(t *testing.T) {
	testCases := map[string]struct {
		loc *Location
		exp string
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
				JSONOptions: astJSON.Options{
					MarshalOptions: astJSON.MarshalOptions{
						IncludeLocationText: true,
					},
				},
			},
			exp: `{"file":"file","row":1,"col":1,"text":"dGV4dA=="}`,
		},
		"excluding file": {
			loc: &Location{
				File: "file",
				Row:  1,
				Col:  1,
				JSONOptions: astJSON.Options{
					MarshalOptions: astJSON.MarshalOptions{
						ExcludeLocationFile: true,
					},
				},
			},
			exp: `{"row":1,"col":1}`,
		},
	}

	for id, tc := range testCases {
		t.Run(id, func(t *testing.T) {
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

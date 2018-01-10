package statereadonly

import "testing"

var cases = map[string]int{
	"abce":           0,
	"abcd":           0,
	"abcd abce abcd": 0,
	"a":              0,
	"abcf":           0,
}

func TestStateReadonly(t *testing.T) {
	for tc, exp := range cases {
		got, err := Parse("", []byte(tc), Memoize(false))

		if err != nil {
			t.Errorf(err.Error())
		}
		if got != exp {
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}
	}
}

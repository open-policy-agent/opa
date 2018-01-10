package stateclone

import "testing"

var cases = map[string]int{
	"abce":           11,
	"abcd":           13,
	"abcd abce abcd": 17,
	"a":              10,
	"abcf":           15,
}

func TestState(t *testing.T) {
	for tc, exp := range cases {
		vals := make(values, 1)
		vals[0] = 10
		got, err := Parse("", []byte(tc), Memoize(false), InitState("vals", vals))

		if err != nil {
			t.Errorf(err.Error())
		}
		vals = got.(values)
		res := 0
		for _, v := range vals {
			res += v
		}
		if res != exp {
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}
	}
}

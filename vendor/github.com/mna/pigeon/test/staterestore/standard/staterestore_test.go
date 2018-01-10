package staterestore

import "testing"

var cases = []struct {
	rule  string
	input string
	want  int
}{
	{"TestExpr", "f#\n", 1},
	{"TestExpr", "f\n", 1},
	{"TestAnd", "\n", 0},
	{"TestNot", "\n", 1},
}

func TestStateRestore(t *testing.T) {
	for _, c := range cases {
		got, err := Parse("", []byte(c.input), Entrypoint(c.rule))
		if err != nil {
			t.Errorf("%s:%q: %v", c.rule, c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:%q: want %v, got %v", c.rule, c.input, c.want, got)
		}
	}
}

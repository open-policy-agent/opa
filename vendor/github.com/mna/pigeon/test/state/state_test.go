package state

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
		got, err := Parse("", []byte(tc), Memoize(false), InitState("countCs", 10))

		if err != nil {
			t.Errorf(err.Error())
		}
		if got != exp {
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}
	}
}

func TestStateCloneRestore(t *testing.T) {
	var p parser
	p.cur.state = make(storeDict)
	p.restoreState(p.cloneState())
	c := &p.cur

	backup := p.cloneState()

	c.state["foo"] = true

	p.restoreState(backup)
	if len(p.cur.state) > 0 {
		t.Fatalf("leaking state! %#v", p.cur.state)
	}
}

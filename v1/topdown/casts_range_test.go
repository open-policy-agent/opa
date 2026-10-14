package topdown

import "testing"

func TestToNumberOutOfRange(t *testing.T) {
	for _, s := range []string{"1e-1000001", "1e1000001", "1e-99999999999"} {
		runTopDownTestCase(t, map[string]any{}, s, []string{`p := x if { x := to_number("` + s + `") }`}, ``)
	}
	runTopDownTestCase(t, map[string]any{}, "in range", []string{`p := x if { x := to_number("1.5e300") }`}, `1.5e300`)
}

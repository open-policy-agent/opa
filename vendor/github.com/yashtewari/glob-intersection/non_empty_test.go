package gintersect

import (
	"testing"
)

func TestNonEmptyIntersections(t *testing.T) {
	tests := map[string][]string{
		"abcd":        {"abcd", "....", "[a-d]*"},
		"pqrs":        {".qrs", "p.rs", "pq.s", "pqr."},
		".*":          {"asdklfj", "jasdfh", "asdhfajfh", "asdflkasdfjl"},
		"d*":          {"[abcd][abcd]", "d[a-z]+", ".....", "[d]*"},
		"[a-p]+":      {"[p-z]+", "apapapaapapapap", ".*", "abcdefgh*"},
		"abcd[a-c]z+": {"abcd[b-d][yz]*", "abcdazzzz", "abcdbzzz", "abcdcz"},
		".*\\\\":      {".*", "asdfasdf\\\\"}, // Escaped \ character.
		".a.a":        {"b.b.", "c.c.", "d.d.", "e.e."},
		".*.*.*.*.*.*.*.*.*.*.*.*.*.*.*": {".*.*.*.*.*.*.*.*.*.*.*"},
		"foo.*bar":                       {"foobar", "fooalkdsjfbar"},
	}

	for lhs, rhss := range tests {
		for _, rhs := range rhss {
			ne, err := NonEmpty(lhs, rhs)
			if err != nil {
				t.Error(err)
			}

			if !ne {
				t.Errorf("lhs: %s, rhs: %s should be non-empty", lhs, rhs)
			}
		}
	}
}

func TestEmptyIntersections(t *testing.T) {
	tests := map[string][]string{
		"abcd":      {"lsdfhda", "abcdla", "asdlfk", "ksdfj"},
		"[a-d]+":    {"xyz", "p+", "[e-f]+"},
		"[0-9]*":    {"[a-z]", ".\\*"},
		"mamama.*":  {"dadada.*", "nanana.*"},
		".*mamama":  {".*dadada", ".*nanana"},
		".xyz.":     {"paaap", ".*pqr.*"},
		"ab+":       {"a", "b", "abc"},
		".*.*.*.*f": {".*.*.*.*g"},
		".*":        {""},
	}

	for lhs, rhss := range tests {
		for _, rhs := range rhss {
			ne, err := NonEmpty(lhs, rhs)
			if err != nil {
				t.Error(err)
			}

			if ne {
				t.Errorf("lhs: %s, rhs: %s should be non-empty", lhs, rhs)
			}
		}
	}
}

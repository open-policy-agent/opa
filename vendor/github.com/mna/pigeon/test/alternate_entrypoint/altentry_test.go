package altentry

import (
	"strings"
	"testing"
)

func TestValidEntrypoints(t *testing.T) {
	cases := []struct {
		in         string
		entrypoint string
	}{
		{"aacc", ""},
		{"bbbcc", "Entry2"},
		{"cc", "Entry3"},
		{"cc", "C"},
	}

	for _, c := range cases {
		v, err := Parse("", []byte(c.in), Entrypoint(c.entrypoint))
		if err != nil {
			t.Errorf("%s:%s: got error %s", c.entrypoint, c.in, err)
		}

		got := string(v.([]byte))
		if got != c.in {
			t.Errorf("%s:%s: got %s", c.entrypoint, c.in, got)
		}
	}
}

func TestInvalidEntrypoints(t *testing.T) {
	cases := []struct {
		in         string
		entrypoint string
		errMsg     string
	}{
		{"bbbcc", "Z", errInvalidEntrypoint.Error()},
		{"bbbcc", "", "no match found"},
		{"bbbcc", "C", "no match found"},
		{"aacc", "Entry2", "no match found"},
		{"aacc", "C", "no match found"},
		{"cc", "", "no match found"},
		{"cc", "Entry2", "no match found"},
		// rules A and B are optimized away and not specified as alternate entrypoints
		{"aa", "A", errInvalidEntrypoint.Error()},
		{"bb", "B", errInvalidEntrypoint.Error()},
	}

	for _, c := range cases {
		_, err := Parse("", []byte(c.in), Entrypoint(c.entrypoint))
		if err == nil {
			t.Errorf("%s:%s: want error, got none", c.entrypoint, c.in)
		}
		if !strings.Contains(err.Error(), c.errMsg) {
			t.Errorf("%s:%s: want %s, got %s", c.entrypoint, c.in, c.errMsg, err)
		}
	}
}

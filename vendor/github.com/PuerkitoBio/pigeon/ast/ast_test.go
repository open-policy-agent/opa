package ast

import (
	"strings"
	"testing"
	"unicode/utf8"
)

var charClasses = []string{
	"[]",
	"[]i",
	"[^]",
	"[^]i",
	"[a]",
	"[ab]i",
	"[^abc]i",
	`[\a]`,
	`[\b\nt]`,
	`[\b\nt\pL]`,
	`[\p{Greek}\tz\\\pN]`,
	`[-]`,
	`[--]`,
	`[---]`,
	`[a-z]`,
	`[a-zB0-9]`,
	`[A-Z]i`,
	`[a-]`,
	`[----]`,
	`[\x00-\x05]`,
}

var expChars = []string{
	"",
	"",
	"",
	"",
	"a",
	"ab",
	"abc",
	"\a",
	"\b\nt",
	"\b\nt",
	"\tz\\",
	"-",
	"--",
	"",
	"",
	"B",
	"",
	"a-",
	"-",
	"",
}

var expUnicodeClasses = [][]string{
	9:  {"L"},
	10: {"Greek", "N"},
	19: nil,
}

var expRanges = []string{
	13: "--",
	14: "az",
	15: "az09",
	16: "AZ",
	18: "--",
	19: "\x00\x05",
}

func TestCharClassParse(t *testing.T) {
	for i, c := range charClasses {
		m := NewCharClassMatcher(Pos{}, c)

		ic := strings.HasSuffix(c, "i")
		if m.IgnoreCase != ic {
			t.Errorf("%q: want ignore case: %t, got %t", c, ic, m.IgnoreCase)
		}
		iv := c[1] == '^'
		if m.Inverted != iv {
			t.Errorf("%q: want inverted: %t, got %t", c, iv, m.Inverted)
		}

		if n := utf8.RuneCountInString(expChars[i]); len(m.Chars) != n {
			t.Errorf("%q: want %d chars, got %d", c, n, len(m.Chars))
		} else if string(m.Chars) != expChars[i] {
			t.Errorf("%q: want %q, got %q", c, expChars[i], string(m.Chars))
		}

		if n := utf8.RuneCountInString(expRanges[i]); len(m.Ranges) != n {
			t.Errorf("%q: want %d chars, got %d", c, n, len(m.Ranges))
		} else if string(m.Ranges) != expRanges[i] {
			t.Errorf("%q: want %q, got %q", c, expRanges[i], string(m.Ranges))
		}

		if n := len(expUnicodeClasses[i]); len(m.UnicodeClasses) != n {
			t.Errorf("%q: want %d Unicode classes, got %d", c, n, len(m.UnicodeClasses))
		} else if n > 0 {
			want := expUnicodeClasses[i]
			got := m.UnicodeClasses
			for j, wantClass := range want {
				if wantClass != got[j] {
					t.Errorf("%q: range table %d: want %v, got %v", c, j, wantClass, got[j])
				}
			}
		}
	}
}

package bootstrap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mna/pigeon/ast"
)

var scanValidCases = []string{
	"",
	"a",
	"ab",
	"abc",
	"_",
	"_0",
	"abc_012",
	`=`,
	`<-`,
	"\u2190",
	"\u27f5",
	"' '",
	"'*'",
	"'a'",
	"'a'i",
	"'a'b",
	`'\n'`,
	`'\t'`,
	`'\''`,
	`'\\'`,
	`'\xab'`,
	`'\x1F'`,
	`'\u1234'`,
	`'\U000B1234'`,
	`""`,
	`"a"`,
	`"a"i`,
	`"a"b`,
	`"a\b"`,
	`"a\b \n 1"`,
	`"\xAbc\u1234d\U000011FF"`,
	"``",
	"`a`",
	"`a`i",
	"`a`b",
	"`a \\n `", // `a \n `
	"`a \n `",  // `a <newline> `
	"`a \r\n `",
	"[]",
	"[[]",
	"[[\\]]",
	"[a]",
	"[a]i",
	"[a]b",
	"[ab]",
	"[a-b0-9]",
	"[\\a]",
	"[\\a\\pL_]",
	"[\\a\\p{Greek}]",
	"{}",
	"{a}",
	"{a}i",
	"{\nif something {\n\tdoSomething()\n}\n}",
	"// a",
	"// a\nb",
	"/a",
	"/\n",
	"/**/",
	"/*a*/",
	"/*a\nb*/",
	":",
	";",
	"(",
	")",
	".",
	"&",
	"!",
	"?",
	"+",
	"*",
	"\n",
	"pockage = a",
	`Rule <-
	E / ( 'a'? "bcd"i )+ / [efg-j]* { println() } // comment
	/ &'\xff' /* and
some
comment
*/`,
}

var scanExpTokens = [][]string{
	{"1:0 (0): eof \"\""},
	{"1:1 (0): ident \"a\"", "1:1 (0): eof \"\""},
	{"1:1 (0): ident \"ab\"", "1:2 (1): eof \"\""},
	{"1:1 (0): ident \"abc\"", "1:3 (2): eof \"\""},
	{"1:1 (0): ident \"_\"", "1:1 (0): eof \"\""},
	{"1:1 (0): ident \"_0\"", "1:2 (1): eof \"\""},
	{"1:1 (0): ident \"abc_012\"", "1:7 (6): eof \"\""},
	{"1:1 (0): ruledef \"=\"", "1:1 (0): eof \"\""},
	{"1:1 (0): ruledef \"<-\"", "1:2 (1): eof \"\""},
	{"1:1 (0): ruledef \"\u2190\"", "1:1 (0): eof \"\""},
	{"1:1 (0): ruledef \"\u27f5\"", "1:1 (0): eof \"\""},
	{"1:1 (0): char \"' '\"", "1:3 (2): eof \"\""},
	{"1:1 (0): char \"'*'\"", "1:3 (2): eof \"\""},
	{"1:1 (0): char \"'a'\"", "1:3 (2): eof \"\""},
	{"1:1 (0): char \"'a'i\"", "1:4 (3): eof \"\""},
	{"1:1 (0): char \"'a'\"", "1:4 (3): ident \"b\"", "1:4 (3): eof \"\""},
	{`1:1 (0): char "'\\n'"`, `1:4 (3): eof ""`},
	{`1:1 (0): char "'\\t'"`, `1:4 (3): eof ""`},
	{`1:1 (0): char "'\\''"`, `1:4 (3): eof ""`},
	{`1:1 (0): char "'\\\\'"`, `1:4 (3): eof ""`},
	{`1:1 (0): char "'\\xab'"`, `1:6 (5): eof ""`},
	{`1:1 (0): char "'\\x1F'"`, `1:6 (5): eof ""`},
	{`1:1 (0): char "'\\u1234'"`, `1:8 (7): eof ""`},
	{`1:1 (0): char "'\\U000B1234'"`, `1:12 (11): eof ""`},
	{`1:1 (0): str "\"\""`, `1:2 (1): eof ""`},
	{`1:1 (0): str "\"a\""`, `1:3 (2): eof ""`},
	{`1:1 (0): str "\"a\"i"`, `1:4 (3): eof ""`},
	{`1:1 (0): str "\"a\""`, `1:4 (3): ident "b"`, `1:4 (3): eof ""`},
	{`1:1 (0): str "\"a\\b\""`, `1:5 (4): eof ""`},
	{`1:1 (0): str "\"a\\b \\n 1\""`, `1:10 (9): eof ""`},
	{`1:1 (0): str "\"\\xAbc\\u1234d\\U000011FF\""`, `1:24 (23): eof ""`},
	{"1:1 (0): rstr \"``\"", `1:2 (1): eof ""`},
	{"1:1 (0): rstr \"`a`\"", `1:3 (2): eof ""`},
	{"1:1 (0): rstr \"`a`i\"", `1:4 (3): eof ""`},
	{"1:1 (0): rstr \"`a`\"", "1:4 (3): ident \"b\"", `1:4 (3): eof ""`},
	{"1:1 (0): rstr \"`a \\\\n `\"", `1:7 (6): eof ""`},
	{"1:1 (0): rstr \"`a \\n `\"", `2:2 (5): eof ""`},
	{"1:1 (0): rstr \"`a \\n `\"", `2:2 (6): eof ""`},
	{"1:1 (0): class \"[]\"", `1:2 (1): eof ""`},
	{"1:1 (0): class \"[[]\"", `1:3 (2): eof ""`},
	{"1:1 (0): class \"[[\\\\]]\"", `1:5 (4): eof ""`},
	{"1:1 (0): class \"[a]\"", `1:3 (2): eof ""`},
	{"1:1 (0): class \"[a]i\"", `1:4 (3): eof ""`},
	{"1:1 (0): class \"[a]\"", `1:4 (3): ident "b"`, `1:4 (3): eof ""`},
	{"1:1 (0): class \"[ab]\"", `1:4 (3): eof ""`},
	{"1:1 (0): class \"[a-b0-9]\"", `1:8 (7): eof ""`},
	{"1:1 (0): class \"[\\\\a]\"", `1:4 (3): eof ""`},
	{"1:1 (0): class \"[\\\\a\\\\pL_]\"", `1:8 (7): eof ""`},
	{"1:1 (0): class \"[\\\\a\\\\p{Greek}]\"", `1:13 (12): eof ""`},
	{"1:1 (0): code \"{}\"", `1:2 (1): eof ""`},
	{"1:1 (0): code \"{a}\"", `1:3 (2): eof ""`},
	{"1:1 (0): code \"{a}\"", "1:4 (3): ident \"i\"", `1:4 (3): eof ""`},
	{"1:1 (0): code \"{\\nif something {\\n\\tdoSomething()\\n}\\n}\"", `5:1 (34): eof ""`},
	{"1:1 (0): lcomment \"// a\"", `1:4 (3): eof ""`},
	{"1:1 (0): lcomment \"// a\"", `2:0 (4): eol "\n"`, `2:1 (5): ident "b"`, `2:1 (5): eof ""`},
	{"1:1 (0): slash \"/\"", `1:2 (1): ident "a"`, `1:2 (1): eof ""`},
	{"1:1 (0): slash \"/\"", `2:0 (1): eol "\n"`, `2:0 (1): eof ""`},
	{"1:1 (0): mlcomment \"/**/\"", `1:4 (3): eof ""`},
	{"1:1 (0): mlcomment \"/*a*/\"", `1:5 (4): eof ""`},
	{"1:1 (0): mlcomment \"/*a\\nb*/\"", `2:3 (6): eof ""`},
	{"1:1 (0): colon \":\"", `1:1 (0): eof ""`},
	{"1:1 (0): semicolon \";\"", `1:1 (0): eof ""`},
	{"1:1 (0): lparen \"(\"", `1:1 (0): eof ""`},
	{"1:1 (0): rparen \")\"", `1:1 (0): eof ""`},
	{"1:1 (0): dot \".\"", `1:1 (0): eof ""`},
	{"1:1 (0): ampersand \"&\"", `1:1 (0): eof ""`},
	{"1:1 (0): exclamation \"!\"", `1:1 (0): eof ""`},
	{"1:1 (0): question \"?\"", `1:1 (0): eof ""`},
	{"1:1 (0): plus \"+\"", `1:1 (0): eof ""`},
	{"1:1 (0): star \"*\"", `1:1 (0): eof ""`},
	{"2:0 (0): eol \"\\n\"", `2:0 (0): eof ""`},
	{"1:1 (0): ident \"pockage\"", `1:9 (8): ruledef "="`, `1:11 (10): ident "a"`, `1:11 (10): eof ""`},
	{
		`1:1 (0): ident "Rule"`,
		`1:6 (5): ruledef "<-"`,
		`2:0 (7): eol "\n"`,
		`2:2 (9): ident "E"`,
		`2:4 (11): slash "/"`,
		`2:6 (13): lparen "("`,
		`2:8 (15): char "'a'"`,
		`2:11 (18): question "?"`,
		`2:13 (20): str "\"bcd\"i"`,
		`2:20 (27): rparen ")"`,
		`2:21 (28): plus "+"`,
		`2:23 (30): slash "/"`,
		`2:25 (32): class "[efg-j]"`,
		`2:32 (39): star "*"`,
		`2:34 (41): code "{ println() }"`,
		`2:48 (55): lcomment "// comment"`,
		`3:0 (65): eol "\n"`,
		`3:2 (67): slash "/"`,
		`3:4 (69): ampersand "&"`,
		`3:5 (70): char "'\\xff'"`,
		`3:12 (77): mlcomment "/* and\nsome\ncomment\n*/"`,
		`6:2 (98): eof ""`,
	},
}

type errsink struct {
	errs []error
	pos  []ast.Pos
}

func (e *errsink) add(p ast.Pos, err error) {
	e.errs = append(e.errs, err)
	e.pos = append(e.pos, p)
}

func (e *errsink) reset() {
	e.errs = e.errs[:0]
	e.pos = e.pos[:0]
}

func (e *errsink) StringAt(i int) string {
	if i < 0 || i >= len(e.errs) {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.pos[i], e.errs[i])
}

func TestScanValid(t *testing.T) {
	old := tokenStringLen
	tokenStringLen = 100
	defer func() { tokenStringLen = old }()

	var s Scanner
	var errh errsink
	for i, c := range scanValidCases {
		errh.reset()
		s.Init("", strings.NewReader(c), errh.add)

		j := 0
		for {
			tok, ok := s.Scan()
			if j < len(scanExpTokens[i]) {
				got := tok.String()
				want := scanExpTokens[i][j]
				if got != want {
					t.Errorf("%d: token %d: want %q, got %q", i, j, want, got)
				}
			} else {
				t.Errorf("%d: want %d tokens, got #%d", i, len(scanExpTokens[i]), j+1)
			}
			if !ok {
				if j < len(scanExpTokens[i])-1 {
					t.Errorf("%d: wand %d tokens, got only %d", i, len(scanExpTokens[i]), j+1)
				}
				break
			}
			j++
		}
		if len(errh.errs) != 0 {
			t.Errorf("%d: want no error, got %d", i, len(errh.errs))
			t.Log(errh.errs)
		}
	}
}

var scanInvalidCases = []string{
	"|",
	"<",
	"'",
	"''",
	"'ab'",
	`'\xff\U00001234'`,
	`'\pA'`,
	`'\z'`,
	"'\\\n",
	`'\xg'`,
	`'\129'`,
	`'\12`,
	`'\xa`,
	`'\u123z'`,
	`'\u12`,
	`'\UFFFFffff'`,
	`'\uD800'`,
	`'\ue000'`,
	`'\ud901'`,
	`'\"'`,
	"\"\n",
	"\"",
	"\"\\'\"",
	"`",
	"[",
	"[\\\"",
	`[\[]`,
	`[\p]`,
	`[\p{]`,
	`[\p{`,
	`[\p{}]`,
	`{code{}`,
	`/*a*`,
	`/*a`,
	`func`,
}

var scanExpErrs = [][]string{
	{"1:1 (0): invalid character U+007C '|'"},
	{"1:1 (0): rule definition not terminated"},
	{"1:1 (0): rune literal not terminated"},
	{"1:2 (1): rune literal is not a single rune"},
	{"1:4 (3): rune literal is not a single rune"},
	{"1:16 (15): rune literal is not a single rune"},
	{"1:3 (2): unknown escape sequence",
		"1:5 (4): rune literal is not a single rune"},
	{"1:3 (2): unknown escape sequence"},
	{"2:0 (2): escape sequence not terminated",
		"2:0 (2): rune literal not terminated"},
	{"1:4 (3): illegal character U+0067 'g' in escape sequence"},
	{"1:5 (4): illegal character U+0039 '9' in escape sequence"},
	{"1:4 (3): escape sequence not terminated",
		"1:4 (3): rune literal not terminated"},
	{"1:4 (3): escape sequence not terminated",
		"1:4 (3): rune literal not terminated"},
	{"1:7 (6): illegal character U+007A 'z' in escape sequence"},
	{"1:5 (4): escape sequence not terminated",
		"1:5 (4): rune literal not terminated"},
	{"1:11 (10): escape sequence is invalid Unicode code point"},
	{"1:7 (6): escape sequence is invalid Unicode code point"},
	{"1:7 (6): escape sequence is invalid Unicode code point"},
	{"1:7 (6): escape sequence is invalid Unicode code point"},
	{"1:3 (2): unknown escape sequence"},
	{"2:0 (1): string literal not terminated"},
	{"1:1 (0): string literal not terminated"},
	{"1:3 (2): unknown escape sequence"},
	{"1:1 (0): raw string literal not terminated"},
	{"1:1 (0): character class not terminated"},
	{"1:3 (2): unknown escape sequence",
		"1:3 (2): character class not terminated"},
	{"1:3 (2): unknown escape sequence"},
	{"1:4 (3): character class not terminated"},
	{"1:5 (4): escape sequence not terminated",
		"1:5 (4): character class not terminated"},
	{"1:4 (3): escape sequence not terminated",
		"1:4 (3): character class not terminated"},
	{"1:5 (4): empty Unicode character class escape sequence"},
	{"1:7 (6): code block not terminated"},
	{"1:4 (3): comment not terminated"},
	{"1:3 (2): comment not terminated"},
	{"1:1 (0): illegal identifier \"func\""},
}

func TestScanInvalid(t *testing.T) {
	var s Scanner
	var errh errsink
	for i, c := range scanInvalidCases {
		errh.reset()
		s.Init("", strings.NewReader(c), errh.add)
		for {
			if _, ok := s.Scan(); !ok {
				break
			}
		}
		if len(errh.errs) != len(scanExpErrs[i]) {
			t.Errorf("%d: want %d errors, got %d", i, len(scanExpErrs[i]), len(errh.errs))
			continue
		}
		for j := range errh.errs {
			want := scanExpErrs[i][j]
			got := errh.StringAt(j)
			if want != got {
				t.Errorf("%d: error %d: want %q, got %q", i, j, want, got)
			}
		}
	}
}

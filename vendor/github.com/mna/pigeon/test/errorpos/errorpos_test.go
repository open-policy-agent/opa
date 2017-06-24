package errorpos

import "testing"

var cases = map[string]string{
	"case01 zero":           ``,
	" case01 inc dec zero ": ``,
	"":                `1:1 (0): no match found, expected: "case01", "case02", "case03", "case04", "case05", "case06", "case07", "case08", "case09", "case10", "case11" or [ \t\n\r]`,
	"kase01":          `1:1 (0): no match found, expected: "case01", "case02", "case03", "case04", "case05", "case06", "case07", "case08", "case09", "case10", "case11" or [ \t\n\r]`,
	"case01 zero ink": `1:13 (12): no match found, expected: "dec", "inc", "zero", [ \t\n\r] or EOF`,
	"case02 xyz":      ``,
	"case02 ":         `1:8 (7): no match found, expected: [^abc]`,
	"case02xyz":       `1:7 (6): no match found, expected: [ ]`,
	"case02 abc":      `1:8 (7): no match found, expected: [^abc]`,
	"case02 xya":      `1:10 (9): no match found, expected: [^abc] or EOF`,
	"case03 0":        ``,
	"case03 x0":       ``,
	"case03 y":        `1:8 (7): no match found, expected: "x" or [0-9]`,
	"case03 xy":       `1:9 (8): no match found, expected: [0-9]`,
	"case03 10":       `1:9 (8): no match found, expected: EOF`,
	"case04 0x0x":     ``,
	"case04 x":        `1:8 (7): no match found, expected: [\x30-\x39]`,
	"case04 00":       `1:9 (8): no match found, expected: [^\x30-\x39]`,
	"case04 0xx":      `1:10 (9): no match found, expected: [\pN]`,
	"case04 0x00":     `1:11 (10): no match found, expected: [^\pN]`,
	"case05 yes":      ``,
	"case05 not":      `1:8 (7): no match found, expected: !"not"`,
	"case06 x":        ``,
	"case06 0":        `1:8 (7): no match found, expected: ![0-9]`,
	"case07 0":        ``,
	"case07 a":        `1:8 (7): no match found, expected: ![a-c]i`,
	"case08 a":        ``,
	"case08 b":        `1:8 (7): no match found, expected: "a"i`,
	"case09 0":        ``,
	"case09 a":        `1:8 (7): no match found, expected: [0-9]`,
	"case10 9":        ``,
	"case10 x":        `1:8 (7): no match found, expected: "0", [012], [3-9] or [\pN]`,
	"case11 a":        ``,
	"case11 b":        `1:8 (7): no match found, expected: "a"`,
}

func TestErrorPos(t *testing.T) {
	for tc, exp := range cases {
		_, err := Parse("", []byte(tc))
		var got string
		if err != nil {
			got = err.Error()
		}
		if got != exp {
			_, _ = Parse("", []byte(tc), Debug(true))
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}
	}
}

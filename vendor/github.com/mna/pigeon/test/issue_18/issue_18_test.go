package issue18

import "testing"

var cases = map[string]string{
	`123455`: ``,
	`

    1

    2

    3

    `: ``,
	`

    1

    2

    x
    `: `7:5 (20): no match found, expected: [ \t\r\n], [0-9] or EOF`,
}

func TestErrorReporting(t *testing.T) {
	for tc, exp := range cases {
		_, err := Parse("", []byte(tc))
		var got string
		if err != nil {
			got = err.Error()
		}

		if got != exp {
			t.Errorf("%q: want %v, got %v", tc, exp, got)
		}

	}
}

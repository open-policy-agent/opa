package main

import "testing"

var longishExpr = `
18 + 3 - 27012 * ( (1234 - 43) / 7 ) + -4 * 8129
`

var validCases = map[string]int{
	"0":   0,
	"1":   1,
	"-1":  -1,
	"10":  10,
	"-10": -10,

	"(0)":   0,
	"(1)":   1,
	"(-1)":  -1,
	"(10)":  10,
	"(-10)": -10,

	"1+1":   2,
	"1-1":   0,
	"1*1":   1,
	"1/1":   1,
	"1 + 1": 2,
	"1 - 1": 0,
	"1 * 1": 1,
	"1 / 1": 1,

	"1+0":   1,
	"1-0":   1,
	"1*0":   0,
	"1 + 0": 1,
	"1 - 0": 1,
	"1 * 0": 0,

	"1\n+\t2\r\n +\n3\n": 6,
	"(2) * 3":            6,

	" 1 + 2 - 3 * 4 / 5 ":       1,
	" 1 + (2 - 3) * 4 / 5 ":     1,
	" (1 + 2 - 3) * 4 / 5 ":     0,
	" 1 + 2 - (3 * 4) / 5 ":     1,
	" 18 + 3 - 27 * (-18 / -3)": -141,
	longishExpr:                 -4624535,
}

func TestValidCases(t *testing.T) {
	for tc, exp := range validCases {
		got, err := Parse("", []byte(tc))
		if err != nil {
			t.Errorf("%q: want no error, got %v", tc, err)
			continue
		}
		goti, ok := got.(int)
		if !ok {
			t.Errorf("%q: want type %T, got %T", tc, exp, got)
			continue
		}
		if exp != goti {
			t.Errorf("%q: want %d, got %d", tc, exp, goti)
		}
	}
}

var invalidCases = map[string]string{
	"":        "1:1 (0): no match found",
	"(":       "1:1 (0): no match found",
	")":       "1:1 (0): no match found",
	"()":      "1:1 (0): no match found",
	"+":       "1:1 (0): no match found",
	"-":       "1:1 (0): no match found",
	"*":       "1:1 (0): no match found",
	"/":       "1:1 (0): no match found",
	"+1":      "1:1 (0): no match found",
	"*1":      "1:1 (0): no match found",
	"/1":      "1:1 (0): no match found",
	"1/0":     "1:4 (3): rule Term: runtime error: integer divide by zero",
	"1+":      "1:1 (0): no match found",
	"1-":      "1:1 (0): no match found",
	"1*":      "1:1 (0): no match found",
	"1/":      "1:1 (0): no match found",
	"1 (+ 2)": "1:1 (0): no match found",
	"1 (2)":   "1:1 (0): no match found",
	"\xfe":    "1:1 (0): invalid encoding",
}

func TestInvalidCases(t *testing.T) {
	for tc, exp := range invalidCases {
		got, err := Parse("", []byte(tc))
		if err == nil {
			t.Errorf("%q: want error, got none (%v)", tc, got)
			continue
		}
		el, ok := err.(errList)
		if !ok {
			t.Errorf("%q: want error type %T, got %T", tc, &errList{}, err)
			continue
		}
		for _, e := range el {
			if _, ok := e.(*parserError); !ok {
				t.Errorf("%q: want all individual errors to be %T, got %T (%[3]v)", tc, &parserError{}, e)
			}
		}
		if exp != err.Error() {
			t.Errorf("%q: want \n%s\n, got \n%s\n", tc, exp, err)
		}
	}
}

func TestPanicNoRecover(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			// all good
			return
		}
		t.Fatal("want panic, got none")
	}()

	// should panic
	Parse("", []byte("1 / 0"), Recover(false))
}

func TestMemoization(t *testing.T) {
	in := " 2 + 35 * ( 18 - -4 / ( 5 + 1) ) * 456 + -1"
	want := 287281

	p := newParser("", []byte(in), Memoize(false))
	got, err := p.parse(g)
	if err != nil {
		t.Fatal(err)
	}
	goti := got.(int)
	if goti != want {
		t.Errorf("want %d, got %d", want, goti)
	}
	if p.exprCnt != 415 {
		t.Errorf("with Memoize=false, want %d expressions evaluated, got %d", 415, p.exprCnt)
	}

	p = newParser("", []byte(in), Memoize(true))
	got, err = p.parse(g)
	if err != nil {
		t.Fatal(err)
	}
	goti = got.(int)
	if goti != want {
		t.Errorf("want %d, got %d", want, goti)
	}
	if p.exprCnt != 389 {
		t.Errorf("with Memoize=true, want %d expressions evaluated, got %d", 389, p.exprCnt)
	}
}

func BenchmarkPigeonCalculatorNoMemo(b *testing.B) {
	d := []byte(longishExpr)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", d, Memoize(false)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPigeonCalculatorMemo(b *testing.B) {
	d := []byte(longishExpr)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := Parse("", d, Memoize(true)); err != nil {
			b.Fatal(err)
		}
	}
}

// Command json parses JSON as defined by [1].
//
// BUGS: the escaped forward solidus (`\/`) is not currently handled.
//
// [1]: http://www.ecma-international.org/publications/files/ECMA-ST/ECMA-404.pdf
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func main() {
	in := os.Stdin
	nm := "stdin"
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		in = f
		nm = os.Args[1]
	}

	got, err := ParseReader(nm, in)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(got)
}

func toIfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

var g = &grammar{
	rules: []*rule{
		{
			name: "JSON",
			pos:  position{line: 37, col: 1, offset: 702},
			expr: &actionExpr{
				pos: position{line: 37, col: 8, offset: 711},
				run: (*parser).callonJSON1,
				expr: &seqExpr{
					pos: position{line: 37, col: 8, offset: 711},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 37, col: 8, offset: 711},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 37, col: 10, offset: 713},
							label: "vals",
							expr: &oneOrMoreExpr{
								pos: position{line: 37, col: 15, offset: 718},
								expr: &ruleRefExpr{
									pos:  position{line: 37, col: 15, offset: 718},
									name: "Value",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 37, col: 22, offset: 725},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Value",
			pos:  position{line: 49, col: 1, offset: 916},
			expr: &actionExpr{
				pos: position{line: 49, col: 9, offset: 926},
				run: (*parser).callonValue1,
				expr: &seqExpr{
					pos: position{line: 49, col: 9, offset: 926},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 49, col: 9, offset: 926},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 49, col: 15, offset: 932},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 49, col: 15, offset: 932},
										name: "Object",
									},
									&ruleRefExpr{
										pos:  position{line: 49, col: 24, offset: 941},
										name: "Array",
									},
									&ruleRefExpr{
										pos:  position{line: 49, col: 32, offset: 949},
										name: "Number",
									},
									&ruleRefExpr{
										pos:  position{line: 49, col: 41, offset: 958},
										name: "String",
									},
									&ruleRefExpr{
										pos:  position{line: 49, col: 50, offset: 967},
										name: "Bool",
									},
									&ruleRefExpr{
										pos:  position{line: 49, col: 57, offset: 974},
										name: "Null",
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 49, col: 64, offset: 981},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 53, col: 1, offset: 1008},
			expr: &actionExpr{
				pos: position{line: 53, col: 10, offset: 1019},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 53, col: 10, offset: 1019},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 53, col: 10, offset: 1019},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 53, col: 14, offset: 1023},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 53, col: 16, offset: 1025},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 53, col: 21, offset: 1030},
								expr: &seqExpr{
									pos: position{line: 53, col: 23, offset: 1032},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 53, col: 23, offset: 1032},
											name: "String",
										},
										&ruleRefExpr{
											pos:  position{line: 53, col: 30, offset: 1039},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 53, col: 32, offset: 1041},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 53, col: 36, offset: 1045},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 53, col: 38, offset: 1047},
											name: "Value",
										},
										&zeroOrMoreExpr{
											pos: position{line: 53, col: 44, offset: 1053},
											expr: &seqExpr{
												pos: position{line: 53, col: 46, offset: 1055},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 53, col: 46, offset: 1055},
														val:        ",",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 53, col: 50, offset: 1059},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 53, col: 52, offset: 1061},
														name: "String",
													},
													&ruleRefExpr{
														pos:  position{line: 53, col: 59, offset: 1068},
														name: "_",
													},
													&litMatcher{
														pos:        position{line: 53, col: 61, offset: 1070},
														val:        ":",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 53, col: 65, offset: 1074},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 53, col: 67, offset: 1076},
														name: "Value",
													},
												},
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 53, col: 79, offset: 1088},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 68, col: 1, offset: 1430},
			expr: &actionExpr{
				pos: position{line: 68, col: 9, offset: 1440},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 68, col: 9, offset: 1440},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 68, col: 9, offset: 1440},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 68, col: 13, offset: 1444},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 68, col: 15, offset: 1446},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 68, col: 20, offset: 1451},
								expr: &seqExpr{
									pos: position{line: 68, col: 22, offset: 1453},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 68, col: 22, offset: 1453},
											name: "Value",
										},
										&zeroOrMoreExpr{
											pos: position{line: 68, col: 28, offset: 1459},
											expr: &seqExpr{
												pos: position{line: 68, col: 30, offset: 1461},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 68, col: 30, offset: 1461},
														val:        ",",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 68, col: 34, offset: 1465},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 68, col: 36, offset: 1467},
														name: "Value",
													},
												},
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 68, col: 48, offset: 1479},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Number",
			pos:  position{line: 82, col: 1, offset: 1785},
			expr: &actionExpr{
				pos: position{line: 82, col: 10, offset: 1796},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 82, col: 10, offset: 1796},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 82, col: 10, offset: 1796},
							expr: &litMatcher{
								pos:        position{line: 82, col: 10, offset: 1796},
								val:        "-",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 82, col: 15, offset: 1801},
							name: "Integer",
						},
						&zeroOrOneExpr{
							pos: position{line: 82, col: 23, offset: 1809},
							expr: &seqExpr{
								pos: position{line: 82, col: 25, offset: 1811},
								exprs: []interface{}{
									&litMatcher{
										pos:        position{line: 82, col: 25, offset: 1811},
										val:        ".",
										ignoreCase: false,
									},
									&oneOrMoreExpr{
										pos: position{line: 82, col: 29, offset: 1815},
										expr: &ruleRefExpr{
											pos:  position{line: 82, col: 29, offset: 1815},
											name: "DecimalDigit",
										},
									},
								},
							},
						},
						&zeroOrOneExpr{
							pos: position{line: 82, col: 46, offset: 1832},
							expr: &ruleRefExpr{
								pos:  position{line: 82, col: 46, offset: 1832},
								name: "Exponent",
							},
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 88, col: 1, offset: 1987},
			expr: &choiceExpr{
				pos: position{line: 88, col: 11, offset: 1999},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 88, col: 11, offset: 1999},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 88, col: 17, offset: 2005},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 88, col: 17, offset: 2005},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 88, col: 37, offset: 2025},
								expr: &ruleRefExpr{
									pos:  position{line: 88, col: 37, offset: 2025},
									name: "DecimalDigit",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 90, col: 1, offset: 2040},
			expr: &seqExpr{
				pos: position{line: 90, col: 12, offset: 2053},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 90, col: 12, offset: 2053},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 90, col: 17, offset: 2058},
						expr: &charClassMatcher{
							pos:        position{line: 90, col: 17, offset: 2058},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 90, col: 23, offset: 2064},
						expr: &ruleRefExpr{
							pos:  position{line: 90, col: 23, offset: 2064},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "String",
			pos:  position{line: 92, col: 1, offset: 2079},
			expr: &actionExpr{
				pos: position{line: 92, col: 10, offset: 2090},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 92, col: 10, offset: 2090},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 92, col: 10, offset: 2090},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 92, col: 14, offset: 2094},
							expr: &choiceExpr{
								pos: position{line: 92, col: 16, offset: 2096},
								alternatives: []interface{}{
									&seqExpr{
										pos: position{line: 92, col: 16, offset: 2096},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 92, col: 16, offset: 2096},
												expr: &ruleRefExpr{
													pos:  position{line: 92, col: 17, offset: 2097},
													name: "EscapedChar",
												},
											},
											&anyMatcher{
												line: 92, col: 29, offset: 2109,
											},
										},
									},
									&seqExpr{
										pos: position{line: 92, col: 33, offset: 2113},
										exprs: []interface{}{
											&litMatcher{
												pos:        position{line: 92, col: 33, offset: 2113},
												val:        "\\",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 92, col: 38, offset: 2118},
												name: "EscapeSequence",
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 92, col: 56, offset: 2136},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 98, col: 1, offset: 2308},
			expr: &charClassMatcher{
				pos:        position{line: 98, col: 15, offset: 2324},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 100, col: 1, offset: 2340},
			expr: &choiceExpr{
				pos: position{line: 100, col: 18, offset: 2359},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 100, col: 18, offset: 2359},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 100, col: 37, offset: 2378},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 102, col: 1, offset: 2393},
			expr: &charClassMatcher{
				pos:        position{line: 102, col: 20, offset: 2414},
				val:        "[\"\\\\/bfnrt]",
				chars:      []rune{'"', '\\', '/', 'b', 'f', 'n', 'r', 't'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 104, col: 1, offset: 2427},
			expr: &seqExpr{
				pos: position{line: 104, col: 17, offset: 2445},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 104, col: 17, offset: 2445},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 104, col: 21, offset: 2449},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 104, col: 30, offset: 2458},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 104, col: 39, offset: 2467},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 104, col: 48, offset: 2476},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 106, col: 1, offset: 2486},
			expr: &charClassMatcher{
				pos:        position{line: 106, col: 16, offset: 2503},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 108, col: 1, offset: 2510},
			expr: &charClassMatcher{
				pos:        position{line: 108, col: 23, offset: 2534},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 110, col: 1, offset: 2541},
			expr: &charClassMatcher{
				pos:        position{line: 110, col: 12, offset: 2554},
				val:        "[0-9a-f]i",
				ranges:     []rune{'0', '9', 'a', 'f'},
				ignoreCase: true,
				inverted:   false,
			},
		},
		{
			name: "Bool",
			pos:  position{line: 112, col: 1, offset: 2565},
			expr: &choiceExpr{
				pos: position{line: 112, col: 8, offset: 2574},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 112, col: 8, offset: 2574},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 112, col: 8, offset: 2574},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 112, col: 38, offset: 2604},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 112, col: 38, offset: 2604},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 114, col: 1, offset: 2635},
			expr: &actionExpr{
				pos: position{line: 114, col: 8, offset: 2644},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 114, col: 8, offset: 2644},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name:        "_",
			displayName: "\"whitespace\"",
			pos:         position{line: 116, col: 1, offset: 2672},
			expr: &zeroOrMoreExpr{
				pos: position{line: 116, col: 18, offset: 2691},
				expr: &charClassMatcher{
					pos:        position{line: 116, col: 18, offset: 2691},
					val:        "[ \\t\\r\\n]",
					chars:      []rune{' ', '\t', '\r', '\n'},
					ignoreCase: false,
					inverted:   false,
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 118, col: 1, offset: 2703},
			expr: &notExpr{
				pos: position{line: 118, col: 7, offset: 2711},
				expr: &anyMatcher{
					line: 118, col: 8, offset: 2712,
				},
			},
		},
	},
}

func (c *current) onJSON1(vals interface{}) (interface{}, error) {
	valsSl := toIfaceSlice(vals)
	switch len(valsSl) {
	case 0:
		return nil, nil
	case 1:
		return valsSl[0], nil
	default:
		return valsSl, nil
	}
}

func (p *parser) callonJSON1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onJSON1(stack["vals"])
}

func (c *current) onValue1(val interface{}) (interface{}, error) {
	return val, nil
}

func (p *parser) callonValue1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onValue1(stack["val"])
}

func (c *current) onObject1(vals interface{}) (interface{}, error) {
	res := make(map[string]interface{})
	valsSl := toIfaceSlice(vals)
	if len(valsSl) == 0 {
		return res, nil
	}
	res[valsSl[0].(string)] = valsSl[4]
	restSl := toIfaceSlice(valsSl[5])
	for _, v := range restSl {
		vSl := toIfaceSlice(v)
		res[vSl[2].(string)] = vSl[6]
	}
	return res, nil
}

func (p *parser) callonObject1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObject1(stack["vals"])
}

func (c *current) onArray1(vals interface{}) (interface{}, error) {
	valsSl := toIfaceSlice(vals)
	if len(valsSl) == 0 {
		return []interface{}{}, nil
	}
	res := []interface{}{valsSl[0]}
	restSl := toIfaceSlice(valsSl[1])
	for _, v := range restSl {
		vSl := toIfaceSlice(v)
		res = append(res, vSl[2])
	}
	return res, nil
}

func (p *parser) callonArray1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArray1(stack["vals"])
}

func (c *current) onNumber1() (interface{}, error) {
	// JSON numbers have the same syntax as Go's, and are parseable using
	// strconv.
	return strconv.ParseFloat(string(c.text), 64)
}

func (p *parser) callonNumber1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNumber1()
}

func (c *current) onString1() (interface{}, error) {
	// TODO : the forward slash (solidus) is not a valid escape in Go, it will
	// fail if there's one in the string
	return strconv.Unquote(string(c.text))
}

func (p *parser) callonString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onString1()
}

func (c *current) onBool2() (interface{}, error) {
	return true, nil
}

func (p *parser) callonBool2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBool2()
}

func (c *current) onBool4() (interface{}, error) {
	return false, nil
}

func (p *parser) callonBool4() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBool4()
}

func (c *current) onNull1() (interface{}, error) {
	return nil, nil
}

func (p *parser) callonNull1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNull1()
}

var (
	// errNoRule is returned when the grammar to parse has no rule.
	errNoRule = errors.New("grammar has no rule")

	// errInvalidEncoding is returned when the source is not properly
	// utf8-encoded.
	errInvalidEncoding = errors.New("invalid encoding")

	// errNoMatch is returned if no match could be found.
	errNoMatch = errors.New("no match found")
)

// Option is a function that can set an option on the parser. It returns
// the previous setting as an Option.
type Option func(*parser) Option

// Debug creates an Option to set the debug flag to b. When set to true,
// debugging information is printed to stdout while parsing.
//
// The default is false.
func Debug(b bool) Option {
	return func(p *parser) Option {
		old := p.debug
		p.debug = b
		return Debug(old)
	}
}

// Memoize creates an Option to set the memoize flag to b. When set to true,
// the parser will cache all results so each expression is evaluated only
// once. This guarantees linear parsing time even for pathological cases,
// at the expense of more memory and slower times for typical cases.
//
// The default is false.
func Memoize(b bool) Option {
	return func(p *parser) Option {
		old := p.memoize
		p.memoize = b
		return Memoize(old)
	}
}

// Recover creates an Option to set the recover flag to b. When set to
// true, this causes the parser to recover from panics and convert it
// to an error. Setting it to false can be useful while debugging to
// access the full stack trace.
//
// The default is true.
func Recover(b bool) Option {
	return func(p *parser) Option {
		old := p.recover
		p.recover = b
		return Recover(old)
	}
}

// ParseFile parses the file identified by filename.
func ParseFile(filename string, opts ...Option) (interface{}, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseReader(filename, f, opts...)
}

// ParseReader parses the data from r using filename as information in the
// error messages.
func ParseReader(filename string, r io.Reader, opts ...Option) (interface{}, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return Parse(filename, b, opts...)
}

// Parse parses the data from b using filename as information in the
// error messages.
func Parse(filename string, b []byte, opts ...Option) (interface{}, error) {
	return newParser(filename, b, opts...).parse(g)
}

// position records a position in the text.
type position struct {
	line, col, offset int
}

func (p position) String() string {
	return fmt.Sprintf("%d:%d [%d]", p.line, p.col, p.offset)
}

// savepoint stores all state required to go back to this point in the
// parser.
type savepoint struct {
	position
	rn rune
	w  int
}

type current struct {
	pos  position // start position of the match
	text []byte   // raw text of the match
}

// the AST types...

type grammar struct {
	pos   position
	rules []*rule
}

type rule struct {
	pos         position
	name        string
	displayName string
	expr        interface{}
}

type choiceExpr struct {
	pos          position
	alternatives []interface{}
}

type actionExpr struct {
	pos  position
	expr interface{}
	run  func(*parser) (interface{}, error)
}

type seqExpr struct {
	pos   position
	exprs []interface{}
}

type labeledExpr struct {
	pos   position
	label string
	expr  interface{}
}

type expr struct {
	pos  position
	expr interface{}
}

type andExpr expr
type notExpr expr
type zeroOrOneExpr expr
type zeroOrMoreExpr expr
type oneOrMoreExpr expr

type ruleRefExpr struct {
	pos  position
	name string
}

type andCodeExpr struct {
	pos position
	run func(*parser) (bool, error)
}

type notCodeExpr struct {
	pos position
	run func(*parser) (bool, error)
}

type litMatcher struct {
	pos        position
	val        string
	ignoreCase bool
}

type charClassMatcher struct {
	pos        position
	val        string
	chars      []rune
	ranges     []rune
	classes    []*unicode.RangeTable
	ignoreCase bool
	inverted   bool
}

type anyMatcher position

// errList cumulates the errors found by the parser.
type errList []error

func (e *errList) add(err error) {
	*e = append(*e, err)
}

func (e errList) err() error {
	if len(e) == 0 {
		return nil
	}
	e.dedupe()
	return e
}

func (e *errList) dedupe() {
	var cleaned []error
	set := make(map[string]bool)
	for _, err := range *e {
		if msg := err.Error(); !set[msg] {
			set[msg] = true
			cleaned = append(cleaned, err)
		}
	}
	*e = cleaned
}

func (e errList) Error() string {
	switch len(e) {
	case 0:
		return ""
	case 1:
		return e[0].Error()
	default:
		var buf bytes.Buffer

		for i, err := range e {
			if i > 0 {
				buf.WriteRune('\n')
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
}

// parserError wraps an error with a prefix indicating the rule in which
// the error occurred. The original error is stored in the Inner field.
type parserError struct {
	Inner  error
	pos    position
	prefix string
}

// Error returns the error message.
func (p *parserError) Error() string {
	return p.prefix + ": " + p.Inner.Error()
}

// newParser creates a parser with the specified input source and options.
func newParser(filename string, b []byte, opts ...Option) *parser {
	p := &parser{
		filename: filename,
		errs:     new(errList),
		data:     b,
		pt:       savepoint{position: position{line: 1}},
		recover:  true,
	}
	p.setOptions(opts)
	return p
}

// setOptions applies the options to the parser.
func (p *parser) setOptions(opts []Option) {
	for _, opt := range opts {
		opt(p)
	}
}

type resultTuple struct {
	v   interface{}
	b   bool
	end savepoint
}

type parser struct {
	filename string
	pt       savepoint
	cur      current

	data []byte
	errs *errList

	recover bool
	debug   bool
	depth   int

	memoize bool
	// memoization table for the packrat algorithm:
	// map[offset in source] map[expression or rule] {value, match}
	memo map[int]map[interface{}]resultTuple

	// rules table, maps the rule identifier to the rule node
	rules map[string]*rule
	// variables stack, map of label to value
	vstack []map[string]interface{}
	// rule stack, allows identification of the current rule in errors
	rstack []*rule

	// stats
	exprCnt int
}

// push a variable set on the vstack.
func (p *parser) pushV() {
	if cap(p.vstack) == len(p.vstack) {
		// create new empty slot in the stack
		p.vstack = append(p.vstack, nil)
	} else {
		// slice to 1 more
		p.vstack = p.vstack[:len(p.vstack)+1]
	}

	// get the last args set
	m := p.vstack[len(p.vstack)-1]
	if m != nil && len(m) == 0 {
		// empty map, all good
		return
	}

	m = make(map[string]interface{})
	p.vstack[len(p.vstack)-1] = m
}

// pop a variable set from the vstack.
func (p *parser) popV() {
	// if the map is not empty, clear it
	m := p.vstack[len(p.vstack)-1]
	if len(m) > 0 {
		// GC that map
		p.vstack[len(p.vstack)-1] = nil
	}
	p.vstack = p.vstack[:len(p.vstack)-1]
}

func (p *parser) print(prefix, s string) string {
	if !p.debug {
		return s
	}

	fmt.Printf("%s %d:%d:%d: %s [%#U]\n",
		prefix, p.pt.line, p.pt.col, p.pt.offset, s, p.pt.rn)
	return s
}

func (p *parser) in(s string) string {
	p.depth++
	return p.print(strings.Repeat(" ", p.depth)+">", s)
}

func (p *parser) out(s string) string {
	p.depth--
	return p.print(strings.Repeat(" ", p.depth)+"<", s)
}

func (p *parser) addErr(err error) {
	p.addErrAt(err, p.pt.position)
}

func (p *parser) addErrAt(err error, pos position) {
	var buf bytes.Buffer
	if p.filename != "" {
		buf.WriteString(p.filename)
	}
	if buf.Len() > 0 {
		buf.WriteString(":")
	}
	buf.WriteString(fmt.Sprintf("%d:%d (%d)", pos.line, pos.col, pos.offset))
	if len(p.rstack) > 0 {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		rule := p.rstack[len(p.rstack)-1]
		if rule.displayName != "" {
			buf.WriteString("rule " + rule.displayName)
		} else {
			buf.WriteString("rule " + rule.name)
		}
	}
	pe := &parserError{Inner: err, pos: pos, prefix: buf.String()}
	p.errs.add(pe)
}

// read advances the parser to the next rune.
func (p *parser) read() {
	p.pt.offset += p.pt.w
	rn, n := utf8.DecodeRune(p.data[p.pt.offset:])
	p.pt.rn = rn
	p.pt.w = n
	p.pt.col++
	if rn == '\n' {
		p.pt.line++
		p.pt.col = 0
	}

	if rn == utf8.RuneError {
		if n > 0 {
			p.addErr(errInvalidEncoding)
		}
	}
}

// restore parser position to the savepoint pt.
func (p *parser) restore(pt savepoint) {
	if p.debug {
		defer p.out(p.in("restore"))
	}
	if pt.offset == p.pt.offset {
		return
	}
	p.pt = pt
}

// get the slice of bytes from the savepoint start to the current position.
func (p *parser) sliceFrom(start savepoint) []byte {
	return p.data[start.position.offset:p.pt.position.offset]
}

func (p *parser) getMemoized(node interface{}) (resultTuple, bool) {
	if len(p.memo) == 0 {
		return resultTuple{}, false
	}
	m := p.memo[p.pt.offset]
	if len(m) == 0 {
		return resultTuple{}, false
	}
	res, ok := m[node]
	return res, ok
}

func (p *parser) setMemoized(pt savepoint, node interface{}, tuple resultTuple) {
	if p.memo == nil {
		p.memo = make(map[int]map[interface{}]resultTuple)
	}
	m := p.memo[pt.offset]
	if m == nil {
		m = make(map[interface{}]resultTuple)
		p.memo[pt.offset] = m
	}
	m[node] = tuple
}

func (p *parser) buildRulesTable(g *grammar) {
	p.rules = make(map[string]*rule, len(g.rules))
	for _, r := range g.rules {
		p.rules[r.name] = r
	}
}

func (p *parser) parse(g *grammar) (val interface{}, err error) {
	if len(g.rules) == 0 {
		p.addErr(errNoRule)
		return nil, p.errs.err()
	}

	// TODO : not super critical but this could be generated
	p.buildRulesTable(g)

	if p.recover {
		// panic can be used in action code to stop parsing immediately
		// and return the panic as an error.
		defer func() {
			if e := recover(); e != nil {
				if p.debug {
					defer p.out(p.in("panic handler"))
				}
				val = nil
				switch e := e.(type) {
				case error:
					p.addErr(e)
				default:
					p.addErr(fmt.Errorf("%v", e))
				}
				err = p.errs.err()
			}
		}()
	}

	// start rule is rule [0]
	p.read() // advance to first rune
	val, ok := p.parseRule(g.rules[0])
	if !ok {
		if len(*p.errs) == 0 {
			// make sure this doesn't go out silently
			p.addErr(errNoMatch)
		}
		return nil, p.errs.err()
	}
	return val, p.errs.err()
}

func (p *parser) parseRule(rule *rule) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseRule " + rule.name))
	}

	if p.memoize {
		res, ok := p.getMemoized(rule)
		if ok {
			p.restore(res.end)
			return res.v, res.b
		}
	}

	start := p.pt
	p.rstack = append(p.rstack, rule)
	p.pushV()
	val, ok := p.parseExpr(rule.expr)
	p.popV()
	p.rstack = p.rstack[:len(p.rstack)-1]
	if ok && p.debug {
		p.print(strings.Repeat(" ", p.depth)+"MATCH", string(p.sliceFrom(start)))
	}

	if p.memoize {
		p.setMemoized(start, rule, resultTuple{val, ok, p.pt})
	}
	return val, ok
}

func (p *parser) parseExpr(expr interface{}) (interface{}, bool) {
	var pt savepoint
	var ok bool

	if p.memoize {
		res, ok := p.getMemoized(expr)
		if ok {
			p.restore(res.end)
			return res.v, res.b
		}
		pt = p.pt
	}

	p.exprCnt++
	var val interface{}
	switch expr := expr.(type) {
	case *actionExpr:
		val, ok = p.parseActionExpr(expr)
	case *andCodeExpr:
		val, ok = p.parseAndCodeExpr(expr)
	case *andExpr:
		val, ok = p.parseAndExpr(expr)
	case *anyMatcher:
		val, ok = p.parseAnyMatcher(expr)
	case *charClassMatcher:
		val, ok = p.parseCharClassMatcher(expr)
	case *choiceExpr:
		val, ok = p.parseChoiceExpr(expr)
	case *labeledExpr:
		val, ok = p.parseLabeledExpr(expr)
	case *litMatcher:
		val, ok = p.parseLitMatcher(expr)
	case *notCodeExpr:
		val, ok = p.parseNotCodeExpr(expr)
	case *notExpr:
		val, ok = p.parseNotExpr(expr)
	case *oneOrMoreExpr:
		val, ok = p.parseOneOrMoreExpr(expr)
	case *ruleRefExpr:
		val, ok = p.parseRuleRefExpr(expr)
	case *seqExpr:
		val, ok = p.parseSeqExpr(expr)
	case *zeroOrMoreExpr:
		val, ok = p.parseZeroOrMoreExpr(expr)
	case *zeroOrOneExpr:
		val, ok = p.parseZeroOrOneExpr(expr)
	default:
		panic(fmt.Sprintf("unknown expression type %T", expr))
	}
	if p.memoize {
		p.setMemoized(pt, expr, resultTuple{val, ok, p.pt})
	}
	return val, ok
}

func (p *parser) parseActionExpr(act *actionExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseActionExpr"))
	}

	start := p.pt
	val, ok := p.parseExpr(act.expr)
	if ok {
		p.cur.pos = start.position
		p.cur.text = p.sliceFrom(start)
		actVal, err := act.run(p)
		if err != nil {
			p.addErrAt(err, start.position)
		}
		val = actVal
	}
	if ok && p.debug {
		p.print(strings.Repeat(" ", p.depth)+"MATCH", string(p.sliceFrom(start)))
	}
	return val, ok
}

func (p *parser) parseAndCodeExpr(and *andCodeExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAndCodeExpr"))
	}

	ok, err := and.run(p)
	if err != nil {
		p.addErr(err)
	}
	return nil, ok
}

func (p *parser) parseAndExpr(and *andExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAndExpr"))
	}

	pt := p.pt
	p.pushV()
	_, ok := p.parseExpr(and.expr)
	p.popV()
	p.restore(pt)
	return nil, ok
}

func (p *parser) parseAnyMatcher(any *anyMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAnyMatcher"))
	}

	if p.pt.rn != utf8.RuneError {
		start := p.pt
		p.read()
		return p.sliceFrom(start), true
	}
	return nil, false
}

func (p *parser) parseCharClassMatcher(chr *charClassMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseCharClassMatcher"))
	}

	cur := p.pt.rn
	// can't match EOF
	if cur == utf8.RuneError {
		return nil, false
	}
	start := p.pt
	if chr.ignoreCase {
		cur = unicode.ToLower(cur)
	}

	// try to match in the list of available chars
	for _, rn := range chr.chars {
		if rn == cur {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of ranges
	for i := 0; i < len(chr.ranges); i += 2 {
		if cur >= chr.ranges[i] && cur <= chr.ranges[i+1] {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of Unicode classes
	for _, cl := range chr.classes {
		if unicode.Is(cl, cur) {
			if chr.inverted {
				return nil, false
			}
			p.read()
			return p.sliceFrom(start), true
		}
	}

	if chr.inverted {
		p.read()
		return p.sliceFrom(start), true
	}
	return nil, false
}

func (p *parser) parseChoiceExpr(ch *choiceExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseChoiceExpr"))
	}

	for _, alt := range ch.alternatives {
		p.pushV()
		val, ok := p.parseExpr(alt)
		p.popV()
		if ok {
			return val, ok
		}
	}
	return nil, false
}

func (p *parser) parseLabeledExpr(lab *labeledExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseLabeledExpr"))
	}

	p.pushV()
	val, ok := p.parseExpr(lab.expr)
	p.popV()
	if ok && lab.label != "" {
		m := p.vstack[len(p.vstack)-1]
		m[lab.label] = val
	}
	return val, ok
}

func (p *parser) parseLitMatcher(lit *litMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseLitMatcher"))
	}

	start := p.pt
	for _, want := range lit.val {
		cur := p.pt.rn
		if lit.ignoreCase {
			cur = unicode.ToLower(cur)
		}
		if cur != want {
			p.restore(start)
			return nil, false
		}
		p.read()
	}
	return p.sliceFrom(start), true
}

func (p *parser) parseNotCodeExpr(not *notCodeExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseNotCodeExpr"))
	}

	ok, err := not.run(p)
	if err != nil {
		p.addErr(err)
	}
	return nil, !ok
}

func (p *parser) parseNotExpr(not *notExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseNotExpr"))
	}

	pt := p.pt
	p.pushV()
	_, ok := p.parseExpr(not.expr)
	p.popV()
	p.restore(pt)
	return nil, !ok
}

func (p *parser) parseOneOrMoreExpr(expr *oneOrMoreExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseOneOrMoreExpr"))
	}

	var vals []interface{}

	for {
		p.pushV()
		val, ok := p.parseExpr(expr.expr)
		p.popV()
		if !ok {
			if len(vals) == 0 {
				// did not match once, no match
				return nil, false
			}
			return vals, true
		}
		vals = append(vals, val)
	}
}

func (p *parser) parseRuleRefExpr(ref *ruleRefExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseRuleRefExpr " + ref.name))
	}

	if ref.name == "" {
		panic(fmt.Sprintf("%s: invalid rule: missing name", ref.pos))
	}

	rule := p.rules[ref.name]
	if rule == nil {
		p.addErr(fmt.Errorf("undefined rule: %s", ref.name))
		return nil, false
	}
	return p.parseRule(rule)
}

func (p *parser) parseSeqExpr(seq *seqExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseSeqExpr"))
	}

	var vals []interface{}

	pt := p.pt
	for _, expr := range seq.exprs {
		val, ok := p.parseExpr(expr)
		if !ok {
			p.restore(pt)
			return nil, false
		}
		vals = append(vals, val)
	}
	return vals, true
}

func (p *parser) parseZeroOrMoreExpr(expr *zeroOrMoreExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseZeroOrMoreExpr"))
	}

	var vals []interface{}

	for {
		p.pushV()
		val, ok := p.parseExpr(expr.expr)
		p.popV()
		if !ok {
			return vals, true
		}
		vals = append(vals, val)
	}
}

func (p *parser) parseZeroOrOneExpr(expr *zeroOrOneExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseZeroOrOneExpr"))
	}

	p.pushV()
	val, _ := p.parseExpr(expr.expr)
	p.popV()
	// whether it matched or not, consider it a match
	return val, true
}

func rangeTable(class string) *unicode.RangeTable {
	if rt, ok := unicode.Categories[class]; ok {
		return rt
	}
	if rt, ok := unicode.Properties[class]; ok {
		return rt
	}
	if rt, ok := unicode.Scripts[class]; ok {
		return rt
	}

	// cannot happen
	panic(fmt.Sprintf("invalid Unicode class: %s", class))
}

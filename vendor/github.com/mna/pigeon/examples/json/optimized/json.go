// Package json parses JSON as defined by [1].
//
// BUGS: the escaped forward solidus (`\/`) is not currently handled.
//
// [1]: http://www.ecma-international.org/publications/files/ECMA-ST/ECMA-404.pdf
package json

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

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
			pos:  position{line: 17, col: 1, offset: 347},
			expr: &actionExpr{
				pos: position{line: 17, col: 8, offset: 356},
				run: (*parser).callonJSON1,
				expr: &seqExpr{
					pos: position{line: 17, col: 8, offset: 356},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 17, col: 8, offset: 356},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 17, col: 10, offset: 358},
							label: "vals",
							expr: &oneOrMoreExpr{
								pos: position{line: 17, col: 15, offset: 363},
								expr: &ruleRefExpr{
									pos:  position{line: 17, col: 15, offset: 363},
									name: "Value",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 17, col: 22, offset: 370},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Value",
			pos:  position{line: 29, col: 1, offset: 561},
			expr: &actionExpr{
				pos: position{line: 29, col: 9, offset: 571},
				run: (*parser).callonValue1,
				expr: &seqExpr{
					pos: position{line: 29, col: 9, offset: 571},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 29, col: 9, offset: 571},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 29, col: 15, offset: 577},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 29, col: 15, offset: 577},
										name: "Object",
									},
									&ruleRefExpr{
										pos:  position{line: 29, col: 24, offset: 586},
										name: "Array",
									},
									&ruleRefExpr{
										pos:  position{line: 29, col: 32, offset: 594},
										name: "Number",
									},
									&ruleRefExpr{
										pos:  position{line: 29, col: 41, offset: 603},
										name: "String",
									},
									&ruleRefExpr{
										pos:  position{line: 29, col: 50, offset: 612},
										name: "Bool",
									},
									&ruleRefExpr{
										pos:  position{line: 29, col: 57, offset: 619},
										name: "Null",
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 29, col: 64, offset: 626},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 33, col: 1, offset: 653},
			expr: &actionExpr{
				pos: position{line: 33, col: 10, offset: 664},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 33, col: 10, offset: 664},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 33, col: 10, offset: 664},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 33, col: 14, offset: 668},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 33, col: 16, offset: 670},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 33, col: 21, offset: 675},
								expr: &seqExpr{
									pos: position{line: 33, col: 23, offset: 677},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 33, col: 23, offset: 677},
											name: "String",
										},
										&ruleRefExpr{
											pos:  position{line: 33, col: 30, offset: 684},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 33, col: 32, offset: 686},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 33, col: 36, offset: 690},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 33, col: 38, offset: 692},
											name: "Value",
										},
										&zeroOrMoreExpr{
											pos: position{line: 33, col: 44, offset: 698},
											expr: &seqExpr{
												pos: position{line: 33, col: 46, offset: 700},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 33, col: 46, offset: 700},
														val:        ",",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 33, col: 50, offset: 704},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 33, col: 52, offset: 706},
														name: "String",
													},
													&ruleRefExpr{
														pos:  position{line: 33, col: 59, offset: 713},
														name: "_",
													},
													&litMatcher{
														pos:        position{line: 33, col: 61, offset: 715},
														val:        ":",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 33, col: 65, offset: 719},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 33, col: 67, offset: 721},
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
							pos:        position{line: 33, col: 79, offset: 733},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 48, col: 1, offset: 1075},
			expr: &actionExpr{
				pos: position{line: 48, col: 9, offset: 1085},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 48, col: 9, offset: 1085},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 48, col: 9, offset: 1085},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 48, col: 13, offset: 1089},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 48, col: 15, offset: 1091},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 48, col: 20, offset: 1096},
								expr: &seqExpr{
									pos: position{line: 48, col: 22, offset: 1098},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 48, col: 22, offset: 1098},
											name: "Value",
										},
										&zeroOrMoreExpr{
											pos: position{line: 48, col: 28, offset: 1104},
											expr: &seqExpr{
												pos: position{line: 48, col: 30, offset: 1106},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 48, col: 30, offset: 1106},
														val:        ",",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 48, col: 34, offset: 1110},
														name: "_",
													},
													&ruleRefExpr{
														pos:  position{line: 48, col: 36, offset: 1112},
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
							pos:        position{line: 48, col: 48, offset: 1124},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Number",
			pos:  position{line: 62, col: 1, offset: 1430},
			expr: &actionExpr{
				pos: position{line: 62, col: 10, offset: 1441},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 62, col: 10, offset: 1441},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 62, col: 10, offset: 1441},
							expr: &litMatcher{
								pos:        position{line: 62, col: 10, offset: 1441},
								val:        "-",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 62, col: 15, offset: 1446},
							name: "Integer",
						},
						&zeroOrOneExpr{
							pos: position{line: 62, col: 23, offset: 1454},
							expr: &seqExpr{
								pos: position{line: 62, col: 25, offset: 1456},
								exprs: []interface{}{
									&litMatcher{
										pos:        position{line: 62, col: 25, offset: 1456},
										val:        ".",
										ignoreCase: false,
									},
									&oneOrMoreExpr{
										pos: position{line: 62, col: 29, offset: 1460},
										expr: &ruleRefExpr{
											pos:  position{line: 62, col: 29, offset: 1460},
											name: "DecimalDigit",
										},
									},
								},
							},
						},
						&zeroOrOneExpr{
							pos: position{line: 62, col: 46, offset: 1477},
							expr: &ruleRefExpr{
								pos:  position{line: 62, col: 46, offset: 1477},
								name: "Exponent",
							},
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 68, col: 1, offset: 1632},
			expr: &choiceExpr{
				pos: position{line: 68, col: 11, offset: 1644},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 68, col: 11, offset: 1644},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 68, col: 17, offset: 1650},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 68, col: 17, offset: 1650},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 68, col: 37, offset: 1670},
								expr: &ruleRefExpr{
									pos:  position{line: 68, col: 37, offset: 1670},
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
			pos:  position{line: 70, col: 1, offset: 1685},
			expr: &seqExpr{
				pos: position{line: 70, col: 12, offset: 1698},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 70, col: 12, offset: 1698},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 70, col: 17, offset: 1703},
						expr: &charClassMatcher{
							pos:             position{line: 70, col: 17, offset: 1703},
							val:             "[+-]",
							chars:           []rune{'+', '-'},
							basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
							ignoreCase:      false,
							inverted:        false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 70, col: 23, offset: 1709},
						expr: &ruleRefExpr{
							pos:  position{line: 70, col: 23, offset: 1709},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "String",
			pos:  position{line: 72, col: 1, offset: 1724},
			expr: &actionExpr{
				pos: position{line: 72, col: 10, offset: 1735},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 72, col: 10, offset: 1735},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 72, col: 10, offset: 1735},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 72, col: 14, offset: 1739},
							expr: &choiceExpr{
								pos: position{line: 72, col: 16, offset: 1741},
								alternatives: []interface{}{
									&seqExpr{
										pos: position{line: 72, col: 16, offset: 1741},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 72, col: 16, offset: 1741},
												expr: &ruleRefExpr{
													pos:  position{line: 72, col: 17, offset: 1742},
													name: "EscapedChar",
												},
											},
											&anyMatcher{
												line: 72, col: 29, offset: 1754,
											},
										},
									},
									&seqExpr{
										pos: position{line: 72, col: 33, offset: 1758},
										exprs: []interface{}{
											&litMatcher{
												pos:        position{line: 72, col: 33, offset: 1758},
												val:        "\\",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 72, col: 38, offset: 1763},
												name: "EscapeSequence",
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 72, col: 56, offset: 1781},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 78, col: 1, offset: 1953},
			expr: &charClassMatcher{
				pos:             position{line: 78, col: 15, offset: 1969},
				val:             "[\\x00-\\x1f\"\\\\]",
				chars:           []rune{'"', '\\'},
				ranges:          []rune{'\x00', '\x1f'},
				basicLatinChars: [128]bool{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, true, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
				ignoreCase:      false,
				inverted:        false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 80, col: 1, offset: 1985},
			expr: &choiceExpr{
				pos: position{line: 80, col: 18, offset: 2004},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 80, col: 18, offset: 2004},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 80, col: 37, offset: 2023},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 82, col: 1, offset: 2038},
			expr: &charClassMatcher{
				pos:             position{line: 82, col: 20, offset: 2059},
				val:             "[\"\\\\/bfnrt]",
				chars:           []rune{'"', '\\', '/', 'b', 'f', 'n', 'r', 't'},
				basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, true, false, false, false, true, false, false, false, false, false, false, false, true, false, false, false, true, false, true, false, false, false, false, false, false, false, false, false, false, false},
				ignoreCase:      false,
				inverted:        false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 84, col: 1, offset: 2072},
			expr: &seqExpr{
				pos: position{line: 84, col: 17, offset: 2090},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 84, col: 17, offset: 2090},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 84, col: 21, offset: 2094},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 84, col: 30, offset: 2103},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 84, col: 39, offset: 2112},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 84, col: 48, offset: 2121},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 86, col: 1, offset: 2131},
			expr: &charClassMatcher{
				pos:             position{line: 86, col: 16, offset: 2148},
				val:             "[0-9]",
				ranges:          []rune{'0', '9'},
				basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
				ignoreCase:      false,
				inverted:        false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 88, col: 1, offset: 2155},
			expr: &charClassMatcher{
				pos:             position{line: 88, col: 23, offset: 2179},
				val:             "[1-9]",
				ranges:          []rune{'1', '9'},
				basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
				ignoreCase:      false,
				inverted:        false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 90, col: 1, offset: 2186},
			expr: &charClassMatcher{
				pos:             position{line: 90, col: 12, offset: 2199},
				val:             "[0-9a-f]i",
				ranges:          []rune{'0', '9', 'a', 'f'},
				basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, true, true, true, true, false, false, false, false, false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
				ignoreCase:      true,
				inverted:        false,
			},
		},
		{
			name: "Bool",
			pos:  position{line: 92, col: 1, offset: 2210},
			expr: &choiceExpr{
				pos: position{line: 92, col: 8, offset: 2219},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 92, col: 8, offset: 2219},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 92, col: 8, offset: 2219},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 92, col: 38, offset: 2249},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 92, col: 38, offset: 2249},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 94, col: 1, offset: 2280},
			expr: &actionExpr{
				pos: position{line: 94, col: 8, offset: 2289},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 94, col: 8, offset: 2289},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name:        "_",
			displayName: "\"whitespace\"",
			pos:         position{line: 96, col: 1, offset: 2317},
			expr: &zeroOrMoreExpr{
				pos: position{line: 96, col: 18, offset: 2336},
				expr: &charClassMatcher{
					pos:             position{line: 96, col: 18, offset: 2336},
					val:             "[ \\t\\r\\n]",
					chars:           []rune{' ', '\t', '\r', '\n'},
					basicLatinChars: [128]bool{false, false, false, false, false, false, false, false, false, true, true, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, true, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
					ignoreCase:      false,
					inverted:        false,
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 98, col: 1, offset: 2348},
			expr: &notExpr{
				pos: position{line: 98, col: 7, offset: 2356},
				expr: &anyMatcher{
					line: 98, col: 8, offset: 2357,
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

	// errInvalidEntrypoint is returned when the specified entrypoint rule
	// does not exit.
	errInvalidEntrypoint = errors.New("invalid entrypoint")

	// errInvalidEncoding is returned when the source is not properly
	// utf8-encoded.
	errInvalidEncoding = errors.New("invalid encoding")

	// errMaxExprCnt is used to signal that the maximum number of
	// expressions have been parsed.
	errMaxExprCnt = errors.New("max number of expresssions parsed")
)

// Option is a function that can set an option on the parser. It returns
// the previous setting as an Option.
type Option func(*parser) Option

// MaxExpressions creates an Option to stop parsing after the provided
// number of expressions have been parsed, if the value is 0 then the parser will
// parse for as many steps as needed (possibly an infinite number).
//
// The default for maxExprCnt is 0.
func MaxExpressions(maxExprCnt uint64) Option {
	return func(p *parser) Option {
		oldMaxExprCnt := p.maxExprCnt
		p.maxExprCnt = maxExprCnt
		return MaxExpressions(oldMaxExprCnt)
	}
}

// Entrypoint creates an Option to set the rule name to use as entrypoint.
// The rule name must have been specified in the -alternate-entrypoints
// if generating the parser with the -optimize-grammar flag, otherwise
// it may have been optimized out. Passing an empty string sets the
// entrypoint to the first rule in the grammar.
//
// The default is to start parsing at the first rule in the grammar.
func Entrypoint(ruleName string) Option {
	return func(p *parser) Option {
		oldEntrypoint := p.entrypoint
		p.entrypoint = ruleName
		if ruleName == "" {
			p.entrypoint = g.rules[0].name
		}
		return Entrypoint(oldEntrypoint)
	}
}

// AllowInvalidUTF8 creates an Option to allow invalid UTF-8 bytes.
// Every invalid UTF-8 byte is treated as a utf8.RuneError (U+FFFD)
// by character class matchers and is matched by the any matcher.
// The returned matched value, c.text and c.offset are NOT affected.
//
// The default is false.
func AllowInvalidUTF8(b bool) Option {
	return func(p *parser) Option {
		old := p.allowInvalidUTF8
		p.allowInvalidUTF8 = b
		return AllowInvalidUTF8(old)
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

// GlobalStore creates an Option to set a key to a certain value in
// the globalStore.
func GlobalStore(key string, value interface{}) Option {
	return func(p *parser) Option {
		old := p.cur.globalStore[key]
		p.cur.globalStore[key] = value
		return GlobalStore(key, old)
	}
}

// ParseFile parses the file identified by filename.
func ParseFile(filename string, opts ...Option) (i interface{}, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = closeErr
		}
	}()
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

	// globalStore is a general store for the user to store arbitrary key-value
	// pairs that they need to manage and that they do not want tied to the
	// backtracking of the parser. This is only modified by the user and never
	// rolled back by the parser. It is always up to the user to keep this in a
	// consistent state.
	globalStore storeDict
}

type storeDict map[string]interface{}

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

type recoveryExpr struct {
	pos          position
	expr         interface{}
	recoverExpr  interface{}
	failureLabel []string
}

type seqExpr struct {
	pos   position
	exprs []interface{}
}

type throwExpr struct {
	pos   position
	label string
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
	pos             position
	val             string
	basicLatinChars [128]bool
	chars           []rune
	ranges          []rune
	classes         []*unicode.RangeTable
	ignoreCase      bool
	inverted        bool
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
	Inner    error
	pos      position
	prefix   string
	expected []string
}

// Error returns the error message.
func (p *parserError) Error() string {
	return p.prefix + ": " + p.Inner.Error()
}

// newParser creates a parser with the specified input source and options.
func newParser(filename string, b []byte, opts ...Option) *parser {
	stats := Stats{
		ChoiceAltCnt: make(map[string]map[string]int),
	}

	p := &parser{
		filename: filename,
		errs:     new(errList),
		data:     b,
		pt:       savepoint{position: position{line: 1}},
		recover:  true,
		cur: current{
			globalStore: make(storeDict),
		},
		maxFailPos:      position{col: 1, line: 1},
		maxFailExpected: make([]string, 0, 20),
		Stats:           &stats,
		// start rule is rule [0] unless an alternate entrypoint is specified
		entrypoint: g.rules[0].name,
	}
	p.setOptions(opts)

	if p.maxExprCnt == 0 {
		p.maxExprCnt = math.MaxUint64
	}

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

const choiceNoMatch = -1

// Stats stores some statistics, gathered during parsing
type Stats struct {
	// ExprCnt counts the number of expressions processed during parsing
	// This value is compared to the maximum number of expressions allowed
	// (set by the MaxExpressions option).
	ExprCnt uint64

	// ChoiceAltCnt is used to count for each ordered choice expression,
	// which alternative is used how may times.
	// These numbers allow to optimize the order of the ordered choice expression
	// to increase the performance of the parser
	//
	// The outer key of ChoiceAltCnt is composed of the name of the rule as well
	// as the line and the column of the ordered choice.
	// The inner key of ChoiceAltCnt is the number (one-based) of the matching alternative.
	// For each alternative the number of matches are counted. If an ordered choice does not
	// match, a special counter is incremented. The name of this counter is set with
	// the parser option Statistics.
	// For an alternative to be included in ChoiceAltCnt, it has to match at least once.
	ChoiceAltCnt map[string]map[string]int
}

type parser struct {
	filename string
	pt       savepoint
	cur      current

	data []byte
	errs *errList

	depth   int
	recover bool

	// rules table, maps the rule identifier to the rule node
	rules map[string]*rule
	// variables stack, map of label to value
	vstack []map[string]interface{}
	// rule stack, allows identification of the current rule in errors
	rstack []*rule

	// parse fail
	maxFailPos            position
	maxFailExpected       []string
	maxFailInvertExpected bool

	// max number of expressions to be parsed
	maxExprCnt uint64
	// entrypoint for the parser
	entrypoint string

	allowInvalidUTF8 bool

	*Stats

	choiceNoMatch string
	// recovery expression stack, keeps track of the currently available recovery expression, these are traversed in reverse
	recoveryStack []map[string]interface{}

	// emptyState contains an empty storeDict, which is used to optimize cloneState if global "state" store is not used.
	emptyState storeDict
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

// push a recovery expression with its labels to the recoveryStack
func (p *parser) pushRecovery(labels []string, expr interface{}) {
	if cap(p.recoveryStack) == len(p.recoveryStack) {
		// create new empty slot in the stack
		p.recoveryStack = append(p.recoveryStack, nil)
	} else {
		// slice to 1 more
		p.recoveryStack = p.recoveryStack[:len(p.recoveryStack)+1]
	}

	m := make(map[string]interface{}, len(labels))
	for _, fl := range labels {
		m[fl] = expr
	}
	p.recoveryStack[len(p.recoveryStack)-1] = m
}

// pop a recovery expression from the recoveryStack
func (p *parser) popRecovery() {
	// GC that map
	p.recoveryStack[len(p.recoveryStack)-1] = nil

	p.recoveryStack = p.recoveryStack[:len(p.recoveryStack)-1]
}

func (p *parser) addErr(err error) {
	p.addErrAt(err, p.pt.position, []string{})
}

func (p *parser) addErrAt(err error, pos position, expected []string) {
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
	pe := &parserError{Inner: err, pos: pos, prefix: buf.String(), expected: expected}
	p.errs.add(pe)
}

func (p *parser) failAt(fail bool, pos position, want string) {
	// process fail if parsing fails and not inverted or parsing succeeds and invert is set
	if fail == p.maxFailInvertExpected {
		if pos.offset < p.maxFailPos.offset {
			return
		}

		if pos.offset > p.maxFailPos.offset {
			p.maxFailPos = pos
			p.maxFailExpected = p.maxFailExpected[:0]
		}

		if p.maxFailInvertExpected {
			want = "!" + want
		}
		p.maxFailExpected = append(p.maxFailExpected, want)
	}
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

	if rn == utf8.RuneError && n == 1 { // see utf8.DecodeRune
		if !p.allowInvalidUTF8 {
			p.addErr(errInvalidEncoding)
		}
	}
}

// restore parser position to the savepoint pt.
func (p *parser) restore(pt savepoint) {
	if pt.offset == p.pt.offset {
		return
	}
	p.pt = pt
}

// get the slice of bytes from the savepoint start to the current position.
func (p *parser) sliceFrom(start savepoint) []byte {
	return p.data[start.position.offset:p.pt.position.offset]
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

	startRule, ok := p.rules[p.entrypoint]
	if !ok {
		p.addErr(errInvalidEntrypoint)
		return nil, p.errs.err()
	}

	p.read() // advance to first rune
	val, ok = p.parseRule(startRule)
	if !ok {
		if len(*p.errs) == 0 {
			// If parsing fails, but no errors have been recorded, the expected values
			// for the farthest parser position are returned as error.
			maxFailExpectedMap := make(map[string]struct{}, len(p.maxFailExpected))
			for _, v := range p.maxFailExpected {
				maxFailExpectedMap[v] = struct{}{}
			}
			expected := make([]string, 0, len(maxFailExpectedMap))
			eof := false
			if _, ok := maxFailExpectedMap["!."]; ok {
				delete(maxFailExpectedMap, "!.")
				eof = true
			}
			for k := range maxFailExpectedMap {
				expected = append(expected, k)
			}
			sort.Strings(expected)
			if eof {
				expected = append(expected, "EOF")
			}
			p.addErrAt(errors.New("no match found, expected: "+listJoin(expected, ", ", "or")), p.maxFailPos, expected)
		}

		return nil, p.errs.err()
	}
	return val, p.errs.err()
}

func listJoin(list []string, sep string, lastSep string) string {
	switch len(list) {
	case 0:
		return ""
	case 1:
		return list[0]
	default:
		return fmt.Sprintf("%s %s %s", strings.Join(list[:len(list)-1], sep), lastSep, list[len(list)-1])
	}
}

func (p *parser) parseRule(rule *rule) (interface{}, bool) {
	p.rstack = append(p.rstack, rule)
	p.pushV()
	val, ok := p.parseExpr(rule.expr)
	p.popV()
	p.rstack = p.rstack[:len(p.rstack)-1]
	return val, ok
}

func (p *parser) parseExpr(expr interface{}) (interface{}, bool) {

	p.ExprCnt++
	if p.ExprCnt > p.maxExprCnt {
		panic(errMaxExprCnt)
	}

	var val interface{}
	var ok bool
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
	case *recoveryExpr:
		val, ok = p.parseRecoveryExpr(expr)
	case *ruleRefExpr:
		val, ok = p.parseRuleRefExpr(expr)
	case *seqExpr:
		val, ok = p.parseSeqExpr(expr)
	case *throwExpr:
		val, ok = p.parseThrowExpr(expr)
	case *zeroOrMoreExpr:
		val, ok = p.parseZeroOrMoreExpr(expr)
	case *zeroOrOneExpr:
		val, ok = p.parseZeroOrOneExpr(expr)
	default:
		panic(fmt.Sprintf("unknown expression type %T", expr))
	}
	return val, ok
}

func (p *parser) parseActionExpr(act *actionExpr) (interface{}, bool) {
	start := p.pt
	val, ok := p.parseExpr(act.expr)
	if ok {
		p.cur.pos = start.position
		p.cur.text = p.sliceFrom(start)
		actVal, err := act.run(p)
		if err != nil {
			p.addErrAt(err, start.position, []string{})
		}

		val = actVal
	}
	return val, ok
}

func (p *parser) parseAndCodeExpr(and *andCodeExpr) (interface{}, bool) {

	ok, err := and.run(p)
	if err != nil {
		p.addErr(err)
	}

	return nil, ok
}

func (p *parser) parseAndExpr(and *andExpr) (interface{}, bool) {
	pt := p.pt
	p.pushV()
	_, ok := p.parseExpr(and.expr)
	p.popV()
	p.restore(pt)

	return nil, ok
}

func (p *parser) parseAnyMatcher(any *anyMatcher) (interface{}, bool) {
	if p.pt.rn == utf8.RuneError && p.pt.w == 0 {
		// EOF - see utf8.DecodeRune
		p.failAt(false, p.pt.position, ".")
		return nil, false
	}
	start := p.pt
	p.read()
	p.failAt(true, start.position, ".")
	return p.sliceFrom(start), true
}

func (p *parser) parseCharClassMatcher(chr *charClassMatcher) (interface{}, bool) {
	cur := p.pt.rn
	start := p.pt

	if cur < 128 {
		if chr.basicLatinChars[cur] != chr.inverted {
			p.read()
			p.failAt(true, start.position, chr.val)
			return p.sliceFrom(start), true
		}
		p.failAt(false, start.position, chr.val)
		return nil, false
	}

	// can't match EOF
	if cur == utf8.RuneError && p.pt.w == 0 { // see utf8.DecodeRune
		p.failAt(false, start.position, chr.val)
		return nil, false
	}

	if chr.ignoreCase {
		cur = unicode.ToLower(cur)
	}

	// try to match in the list of available chars
	for _, rn := range chr.chars {
		if rn == cur {
			if chr.inverted {
				p.failAt(false, start.position, chr.val)
				return nil, false
			}
			p.read()
			p.failAt(true, start.position, chr.val)
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of ranges
	for i := 0; i < len(chr.ranges); i += 2 {
		if cur >= chr.ranges[i] && cur <= chr.ranges[i+1] {
			if chr.inverted {
				p.failAt(false, start.position, chr.val)
				return nil, false
			}
			p.read()
			p.failAt(true, start.position, chr.val)
			return p.sliceFrom(start), true
		}
	}

	// try to match in the list of Unicode classes
	for _, cl := range chr.classes {
		if unicode.Is(cl, cur) {
			if chr.inverted {
				p.failAt(false, start.position, chr.val)
				return nil, false
			}
			p.read()
			p.failAt(true, start.position, chr.val)
			return p.sliceFrom(start), true
		}
	}

	if chr.inverted {
		p.read()
		p.failAt(true, start.position, chr.val)
		return p.sliceFrom(start), true
	}
	p.failAt(false, start.position, chr.val)
	return nil, false
}

func (p *parser) parseChoiceExpr(ch *choiceExpr) (interface{}, bool) {
	for altI, alt := range ch.alternatives {
		// dummy assignment to prevent compile error if optimized
		_ = altI

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
	ignoreCase := ""
	if lit.ignoreCase {
		ignoreCase = "i"
	}
	val := fmt.Sprintf("%q%s", lit.val, ignoreCase)
	start := p.pt
	for _, want := range lit.val {
		cur := p.pt.rn
		if lit.ignoreCase {
			cur = unicode.ToLower(cur)
		}
		if cur != want {
			p.failAt(false, start.position, val)
			p.restore(start)
			return nil, false
		}
		p.read()
	}
	p.failAt(true, start.position, val)
	return p.sliceFrom(start), true
}

func (p *parser) parseNotCodeExpr(not *notCodeExpr) (interface{}, bool) {
	ok, err := not.run(p)
	if err != nil {
		p.addErr(err)
	}

	return nil, !ok
}

func (p *parser) parseNotExpr(not *notExpr) (interface{}, bool) {
	pt := p.pt
	p.pushV()
	p.maxFailInvertExpected = !p.maxFailInvertExpected
	_, ok := p.parseExpr(not.expr)
	p.maxFailInvertExpected = !p.maxFailInvertExpected
	p.popV()
	p.restore(pt)

	return nil, !ok
}

func (p *parser) parseOneOrMoreExpr(expr *oneOrMoreExpr) (interface{}, bool) {
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

func (p *parser) parseRecoveryExpr(recover *recoveryExpr) (interface{}, bool) {

	p.pushRecovery(recover.failureLabel, recover.recoverExpr)
	val, ok := p.parseExpr(recover.expr)
	p.popRecovery()

	return val, ok
}

func (p *parser) parseRuleRefExpr(ref *ruleRefExpr) (interface{}, bool) {
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
	vals := make([]interface{}, 0, len(seq.exprs))

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

func (p *parser) parseThrowExpr(expr *throwExpr) (interface{}, bool) {

	for i := len(p.recoveryStack) - 1; i >= 0; i-- {
		if recoverExpr, ok := p.recoveryStack[i][expr.label]; ok {
			if val, ok := p.parseExpr(recoverExpr); ok {
				return val, ok
			}
		}
	}

	return nil, false
}

func (p *parser) parseZeroOrMoreExpr(expr *zeroOrMoreExpr) (interface{}, bool) {
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
	p.pushV()
	val, _ := p.parseExpr(expr.expr)
	p.popV()
	// whether it matched or not, consider it a match
	return val, true
}

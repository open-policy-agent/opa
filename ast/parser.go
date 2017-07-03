package ast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// commentsKey is the global map key for the comments slice.
	commentsKey = "comments"

	// filenameKey is the global map key for the filename.
	filenameKey = "filename"
)

type program struct {
	buf      []interface{}
	comments interface{}
}

type ruleExt struct {
	loc  *Location
	term *Term
	body Body
}

// currentLocation converts the parser context to a Location object.
func currentLocation(c *current) *Location {
	return NewLocation(c.text, c.globalStore[filenameKey].(string), c.pos.line, c.pos.col)
}

func ifaceSliceToByteSlice(i interface{}) []byte {
	var buf bytes.Buffer
	for _, x := range i.([]interface{}) {
		buf.Write(x.([]byte))
	}
	return buf.Bytes()
}

func ifacesToBody(i interface{}, a ...interface{}) Body {
	var buf Body
	buf = append(buf, i.(*Expr))
	for _, s := range a {
		expr := s.([]interface{})[3].(*Expr)
		buf = append(buf, expr)
	}
	return buf
}

func makeObject(head interface{}, tail interface{}, loc *Location) (*Term, error) {
	obj := ObjectTerm()
	obj.Location = loc

	// Empty object.
	if head == nil {
		return obj, nil
	}

	// Object definition above describes the "head" structure. We only care about the "Key" and "Term" elements.
	headSlice := head.([]interface{})
	obj.Value = append(obj.Value.(Object), Item(headSlice[0].(*Term), headSlice[len(headSlice)-1].(*Term)))

	// Non-empty object, remaining key/value pairs.
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		// Object definition above describes the "tail" structure. We only care about the "Key" and "Term" elements.
		obj.Value = append(obj.Value.(Object), Item(s[3].(*Term), s[len(s)-1].(*Term)))
	}

	return obj, nil
}

func makeArray(head interface{}, tail interface{}, loc *Location) (*Term, error) {
	arr := ArrayTerm()
	arr.Location = loc

	// Empty array.
	if head == nil {
		return arr, nil
	}

	// Non-empty array, first element.
	arr.Value = append(arr.Value.(Array), head.(*Term))

	// Non-empty array, remaining elements.
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		// Array definition above describes the "tail" structure. We only care about the "Term" elements.
		arr.Value = append(arr.Value.(Array), s[len(s)-1].(*Term))
	}

	return arr, nil
}

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 96, col: 1, offset: 2438},
			expr: &actionExpr{
				pos: position{line: 96, col: 12, offset: 2449},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 96, col: 12, offset: 2449},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 96, col: 12, offset: 2449},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 96, col: 14, offset: 2451},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 96, col: 19, offset: 2456},
								expr: &seqExpr{
									pos: position{line: 96, col: 20, offset: 2457},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 96, col: 20, offset: 2457},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 96, col: 25, offset: 2462},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 96, col: 30, offset: 2467},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 96, col: 35, offset: 2472},
												expr: &seqExpr{
													pos: position{line: 96, col: 36, offset: 2473},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 96, col: 37, offset: 2474},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 96, col: 37, offset: 2474},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 96, col: 42, offset: 2479},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 96, col: 54, offset: 2491},
															name: "Stmt",
														},
													},
												},
											},
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 96, col: 63, offset: 2500},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 96, col: 65, offset: 2502},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 114, col: 1, offset: 2876},
			expr: &actionExpr{
				pos: position{line: 114, col: 9, offset: 2884},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 114, col: 9, offset: 2884},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 114, col: 14, offset: 2889},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 114, col: 14, offset: 2889},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 24, offset: 2899},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 33, offset: 2908},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 41, offset: 2916},
								name: "UserFunc",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 52, offset: 2927},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 59, offset: 2934},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 69, offset: 2944},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 123, col: 1, offset: 3308},
			expr: &actionExpr{
				pos: position{line: 123, col: 15, offset: 3322},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 123, col: 15, offset: 3322,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 127, col: 1, offset: 3395},
			expr: &actionExpr{
				pos: position{line: 127, col: 12, offset: 3406},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 127, col: 12, offset: 3406},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 127, col: 12, offset: 3406},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 127, col: 22, offset: 3416},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 127, col: 25, offset: 3419},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 127, col: 30, offset: 3424},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 127, col: 30, offset: 3424},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 127, col: 36, offset: 3430},
										name: "Var",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Import",
			pos:  position{line: 161, col: 1, offset: 4746},
			expr: &actionExpr{
				pos: position{line: 161, col: 11, offset: 4756},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 161, col: 11, offset: 4756},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 161, col: 11, offset: 4756},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 161, col: 20, offset: 4765},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 161, col: 23, offset: 4768},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 161, col: 29, offset: 4774},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 161, col: 29, offset: 4774},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 161, col: 35, offset: 4780},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 161, col: 40, offset: 4785},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 161, col: 46, offset: 4791},
								expr: &seqExpr{
									pos: position{line: 161, col: 47, offset: 4792},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 161, col: 47, offset: 4792},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 161, col: 50, offset: 4795},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 161, col: 55, offset: 4800},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 161, col: 58, offset: 4803},
											name: "Var",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Rules",
			pos:  position{line: 177, col: 1, offset: 5253},
			expr: &choiceExpr{
				pos: position{line: 177, col: 10, offset: 5262},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 177, col: 10, offset: 5262},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 177, col: 25, offset: 5277},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 179, col: 1, offset: 5290},
			expr: &actionExpr{
				pos: position{line: 179, col: 17, offset: 5306},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 179, col: 17, offset: 5306},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 179, col: 17, offset: 5306},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 179, col: 27, offset: 5316},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 179, col: 30, offset: 5319},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 179, col: 35, offset: 5324},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 179, col: 39, offset: 5328},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 179, col: 41, offset: 5330},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 179, col: 45, offset: 5334},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 179, col: 47, offset: 5336},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 179, col: 53, offset: 5342},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 222, col: 1, offset: 6270},
			expr: &actionExpr{
				pos: position{line: 222, col: 16, offset: 6285},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 222, col: 16, offset: 6285},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 222, col: 16, offset: 6285},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 222, col: 21, offset: 6290},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 222, col: 30, offset: 6299},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 222, col: 32, offset: 6301},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 222, col: 35, offset: 6304},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 222, col: 35, offset: 6304},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 222, col: 61, offset: 6330},
										expr: &seqExpr{
											pos: position{line: 222, col: 63, offset: 6332},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 222, col: 63, offset: 6332},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 222, col: 65, offset: 6334},
													name: "RuleExt",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "UserFunc",
			pos:  position{line: 277, col: 1, offset: 7630},
			expr: &actionExpr{
				pos: position{line: 277, col: 13, offset: 7642},
				run: (*parser).callonUserFunc1,
				expr: &seqExpr{
					pos: position{line: 277, col: 13, offset: 7642},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 277, col: 13, offset: 7642},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 277, col: 18, offset: 7647},
								name: "FuncHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 277, col: 27, offset: 7656},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 277, col: 29, offset: 7658},
							label: "b",
							expr: &ruleRefExpr{
								pos:  position{line: 277, col: 31, offset: 7660},
								name: "NonEmptyBraceEnclosedBody",
							},
						},
					},
				},
			},
		},
		{
			name: "FuncHead",
			pos:  position{line: 292, col: 1, offset: 7879},
			expr: &actionExpr{
				pos: position{line: 292, col: 13, offset: 7891},
				run: (*parser).callonFuncHead1,
				expr: &seqExpr{
					pos: position{line: 292, col: 13, offset: 7891},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 292, col: 13, offset: 7891},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 292, col: 18, offset: 7896},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 292, col: 22, offset: 7900},
							label: "args",
							expr: &ruleRefExpr{
								pos:  position{line: 292, col: 27, offset: 7905},
								name: "FuncArgs",
							},
						},
						&labeledExpr{
							pos:   position{line: 292, col: 36, offset: 7914},
							label: "output",
							expr: &zeroOrOneExpr{
								pos: position{line: 292, col: 43, offset: 7921},
								expr: &seqExpr{
									pos: position{line: 292, col: 45, offset: 7923},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 292, col: 45, offset: 7923},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 292, col: 47, offset: 7925},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 292, col: 51, offset: 7929},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 292, col: 53, offset: 7931},
											name: "Term",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "FuncArgs",
			pos:  position{line: 308, col: 1, offset: 8235},
			expr: &actionExpr{
				pos: position{line: 308, col: 13, offset: 8247},
				run: (*parser).callonFuncArgs1,
				expr: &seqExpr{
					pos: position{line: 308, col: 13, offset: 8247},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 308, col: 13, offset: 8247},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 308, col: 15, offset: 8249},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 308, col: 20, offset: 8254},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 308, col: 23, offset: 8257},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 308, col: 28, offset: 8262},
								expr: &ruleRefExpr{
									pos:  position{line: 308, col: 28, offset: 8262},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 308, col: 37, offset: 8271},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 308, col: 42, offset: 8276},
								expr: &seqExpr{
									pos: position{line: 308, col: 43, offset: 8277},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 308, col: 43, offset: 8277},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 308, col: 45, offset: 8279},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 308, col: 49, offset: 8283},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 308, col: 51, offset: 8285},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 308, col: 61, offset: 8295},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 308, col: 63, offset: 8297},
							val:        ")",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 308, col: 67, offset: 8301},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "RuleHead",
			pos:  position{line: 329, col: 1, offset: 8721},
			expr: &actionExpr{
				pos: position{line: 329, col: 13, offset: 8733},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 329, col: 13, offset: 8733},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 329, col: 13, offset: 8733},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 329, col: 18, offset: 8738},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 329, col: 22, offset: 8742},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 329, col: 26, offset: 8746},
								expr: &seqExpr{
									pos: position{line: 329, col: 28, offset: 8748},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 329, col: 28, offset: 8748},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 329, col: 30, offset: 8750},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 34, offset: 8754},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 36, offset: 8756},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 41, offset: 8761},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 329, col: 43, offset: 8763},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 47, offset: 8767},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 329, col: 52, offset: 8772},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 329, col: 58, offset: 8778},
								expr: &seqExpr{
									pos: position{line: 329, col: 60, offset: 8780},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 329, col: 60, offset: 8780},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 329, col: 62, offset: 8782},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 66, offset: 8786},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 329, col: 68, offset: 8788},
											name: "Term",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Else",
			pos:  position{line: 364, col: 1, offset: 9776},
			expr: &actionExpr{
				pos: position{line: 364, col: 9, offset: 9784},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 364, col: 9, offset: 9784},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 364, col: 9, offset: 9784},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 364, col: 16, offset: 9791},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 364, col: 20, offset: 9795},
								expr: &seqExpr{
									pos: position{line: 364, col: 22, offset: 9797},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 364, col: 22, offset: 9797},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 364, col: 24, offset: 9799},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 364, col: 28, offset: 9803},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 364, col: 30, offset: 9805},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 364, col: 38, offset: 9813},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 364, col: 42, offset: 9817},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 364, col: 42, offset: 9817},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 364, col: 44, offset: 9819},
										name: "NonEmptyBraceEnclosedBody",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "RuleDup",
			pos:  position{line: 379, col: 1, offset: 10171},
			expr: &actionExpr{
				pos: position{line: 379, col: 12, offset: 10182},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 379, col: 12, offset: 10182},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 379, col: 14, offset: 10184},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 383, col: 1, offset: 10280},
			expr: &choiceExpr{
				pos: position{line: 383, col: 12, offset: 10291},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 383, col: 12, offset: 10291},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 383, col: 19, offset: 10298},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 385, col: 1, offset: 10307},
			expr: &choiceExpr{
				pos: position{line: 385, col: 9, offset: 10315},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 385, col: 9, offset: 10315},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 385, col: 29, offset: 10335},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 387, col: 1, offset: 10354},
			expr: &actionExpr{
				pos: position{line: 387, col: 30, offset: 10383},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 387, col: 30, offset: 10383},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 387, col: 30, offset: 10383},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 387, col: 34, offset: 10387},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 387, col: 36, offset: 10389},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 387, col: 40, offset: 10393},
								expr: &ruleRefExpr{
									pos:  position{line: 387, col: 40, offset: 10393},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 387, col: 56, offset: 10409},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 387, col: 58, offset: 10411},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 394, col: 1, offset: 10506},
			expr: &actionExpr{
				pos: position{line: 394, col: 22, offset: 10527},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 394, col: 22, offset: 10527},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 394, col: 22, offset: 10527},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 394, col: 26, offset: 10531},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 394, col: 28, offset: 10533},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 394, col: 32, offset: 10537},
								expr: &ruleRefExpr{
									pos:  position{line: 394, col: 32, offset: 10537},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 394, col: 48, offset: 10553},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 394, col: 50, offset: 10555},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 408, col: 1, offset: 10907},
			expr: &actionExpr{
				pos: position{line: 408, col: 19, offset: 10925},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 408, col: 19, offset: 10925},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 408, col: 19, offset: 10925},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 408, col: 24, offset: 10930},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 408, col: 32, offset: 10938},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 408, col: 37, offset: 10943},
								expr: &seqExpr{
									pos: position{line: 408, col: 38, offset: 10944},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 408, col: 38, offset: 10944},
											expr: &charClassMatcher{
												pos:        position{line: 408, col: 38, offset: 10944},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 408, col: 46, offset: 10952},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 408, col: 47, offset: 10953},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 408, col: 47, offset: 10953},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 408, col: 51, offset: 10957},
															expr: &ruleRefExpr{
																pos:  position{line: 408, col: 51, offset: 10957},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 408, col: 64, offset: 10970},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 408, col: 64, offset: 10970},
															expr: &ruleRefExpr{
																pos:  position{line: 408, col: 64, offset: 10970},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 408, col: 73, offset: 10979},
															val:        "[\\r\\n]",
															chars:      []rune{'\r', '\n'},
															ignoreCase: false,
															inverted:   false,
														},
													},
												},
											},
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 82, offset: 10988},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 84, offset: 10990},
											name: "Literal",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "NonWhitespaceBody",
			pos:  position{line: 414, col: 1, offset: 11179},
			expr: &actionExpr{
				pos: position{line: 414, col: 22, offset: 11200},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 414, col: 22, offset: 11200},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 414, col: 22, offset: 11200},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 414, col: 27, offset: 11205},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 414, col: 35, offset: 11213},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 414, col: 40, offset: 11218},
								expr: &seqExpr{
									pos: position{line: 414, col: 42, offset: 11220},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 414, col: 42, offset: 11220},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 414, col: 44, offset: 11222},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 414, col: 48, offset: 11226},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 414, col: 51, offset: 11229},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 414, col: 51, offset: 11229},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 414, col: 61, offset: 11239},
													name: "ParseError",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Literal",
			pos:  position{line: 418, col: 1, offset: 11318},
			expr: &actionExpr{
				pos: position{line: 418, col: 12, offset: 11329},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 418, col: 12, offset: 11329},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 418, col: 12, offset: 11329},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 418, col: 16, offset: 11333},
								expr: &seqExpr{
									pos: position{line: 418, col: 18, offset: 11335},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 418, col: 18, offset: 11335},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 418, col: 24, offset: 11341},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 418, col: 30, offset: 11347},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 418, col: 34, offset: 11351},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 418, col: 39, offset: 11356},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 418, col: 44, offset: 11361},
								expr: &seqExpr{
									pos: position{line: 418, col: 46, offset: 11363},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 418, col: 46, offset: 11363},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 418, col: 49, offset: 11366},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 418, col: 54, offset: 11371},
											expr: &seqExpr{
												pos: position{line: 418, col: 55, offset: 11372},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 418, col: 55, offset: 11372},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 418, col: 58, offset: 11375},
														name: "With",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "With",
			pos:  position{line: 441, col: 1, offset: 11947},
			expr: &actionExpr{
				pos: position{line: 441, col: 9, offset: 11955},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 441, col: 9, offset: 11955},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 441, col: 9, offset: 11955},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 16, offset: 11962},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 441, col: 19, offset: 11965},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 441, col: 26, offset: 11972},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 31, offset: 11977},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 441, col: 34, offset: 11980},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 39, offset: 11985},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 441, col: 42, offset: 11988},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 441, col: 48, offset: 11994},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 452, col: 1, offset: 12243},
			expr: &choiceExpr{
				pos: position{line: 452, col: 9, offset: 12251},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 452, col: 10, offset: 12252},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 452, col: 10, offset: 12252},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 452, col: 27, offset: 12269},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 452, col: 52, offset: 12294},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 452, col: 64, offset: 12306},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 452, col: 77, offset: 12319},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 454, col: 1, offset: 12325},
			expr: &actionExpr{
				pos: position{line: 454, col: 19, offset: 12343},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 454, col: 19, offset: 12343},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 454, col: 19, offset: 12343},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 454, col: 26, offset: 12350},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 454, col: 31, offset: 12355},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 454, col: 33, offset: 12357},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 454, col: 37, offset: 12361},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 454, col: 39, offset: 12363},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 454, col: 44, offset: 12368},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 454, col: 49, offset: 12373},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 454, col: 51, offset: 12375},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 454, col: 54, offset: 12378},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 454, col: 67, offset: 12391},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 454, col: 69, offset: 12393},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 454, col: 75, offset: 12399},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 458, col: 1, offset: 12490},
			expr: &actionExpr{
				pos: position{line: 458, col: 26, offset: 12515},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 458, col: 26, offset: 12515},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 458, col: 26, offset: 12515},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 458, col: 31, offset: 12520},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 458, col: 36, offset: 12525},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 458, col: 38, offset: 12527},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 458, col: 41, offset: 12530},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 458, col: 54, offset: 12543},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 458, col: 56, offset: 12545},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 458, col: 62, offset: 12551},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 458, col: 67, offset: 12556},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 458, col: 69, offset: 12558},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 458, col: 73, offset: 12562},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 458, col: 75, offset: 12564},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 458, col: 82, offset: 12571},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 462, col: 1, offset: 12662},
			expr: &actionExpr{
				pos: position{line: 462, col: 17, offset: 12678},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 462, col: 17, offset: 12678},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 462, col: 22, offset: 12683},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 462, col: 22, offset: 12683},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 28, offset: 12689},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 34, offset: 12695},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 40, offset: 12701},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 46, offset: 12707},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 52, offset: 12713},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 462, col: 58, offset: 12719},
								val:        "-",
								ignoreCase: false,
							},
						},
					},
				},
			},
		},
		{
			name: "InfixExpr",
			pos:  position{line: 474, col: 1, offset: 12966},
			expr: &actionExpr{
				pos: position{line: 474, col: 14, offset: 12979},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 474, col: 14, offset: 12979},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 474, col: 14, offset: 12979},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 474, col: 19, offset: 12984},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 474, col: 24, offset: 12989},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 474, col: 26, offset: 12991},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 474, col: 29, offset: 12994},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 474, col: 37, offset: 13002},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 474, col: 39, offset: 13004},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 474, col: 45, offset: 13010},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 478, col: 1, offset: 13085},
			expr: &actionExpr{
				pos: position{line: 478, col: 12, offset: 13096},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 478, col: 12, offset: 13096},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 478, col: 17, offset: 13101},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 478, col: 17, offset: 13101},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 478, col: 23, offset: 13107},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 478, col: 30, offset: 13114},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 478, col: 37, offset: 13121},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 478, col: 44, offset: 13128},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 478, col: 50, offset: 13134},
								val:        ">",
								ignoreCase: false,
							},
						},
					},
				},
			},
		},
		{
			name: "PrefixExpr",
			pos:  position{line: 490, col: 1, offset: 13381},
			expr: &choiceExpr{
				pos: position{line: 490, col: 15, offset: 13395},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 490, col: 15, offset: 13395},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 490, col: 26, offset: 13406},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 492, col: 1, offset: 13415},
			expr: &actionExpr{
				pos: position{line: 492, col: 12, offset: 13426},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 492, col: 12, offset: 13426},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 492, col: 12, offset: 13426},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 492, col: 17, offset: 13431},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 492, col: 29, offset: 13443},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 492, col: 33, offset: 13447},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 492, col: 35, offset: 13449},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 492, col: 40, offset: 13454},
								expr: &ruleRefExpr{
									pos:  position{line: 492, col: 40, offset: 13454},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 492, col: 46, offset: 13460},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 492, col: 51, offset: 13465},
								expr: &seqExpr{
									pos: position{line: 492, col: 53, offset: 13467},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 492, col: 53, offset: 13467},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 492, col: 55, offset: 13469},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 492, col: 59, offset: 13473},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 492, col: 61, offset: 13475},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 492, col: 69, offset: 13483},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 492, col: 72, offset: 13486},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 508, col: 1, offset: 13890},
			expr: &actionExpr{
				pos: position{line: 508, col: 16, offset: 13905},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 508, col: 16, offset: 13905},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 508, col: 16, offset: 13905},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 508, col: 21, offset: 13910},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 508, col: 25, offset: 13914},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 508, col: 30, offset: 13919},
								expr: &seqExpr{
									pos: position{line: 508, col: 32, offset: 13921},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 508, col: 32, offset: 13921},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 508, col: 36, offset: 13925},
											name: "Var",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 522, col: 1, offset: 14330},
			expr: &actionExpr{
				pos: position{line: 522, col: 9, offset: 14338},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 522, col: 9, offset: 14338},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 522, col: 15, offset: 14344},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 522, col: 15, offset: 14344},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 522, col: 31, offset: 14360},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 522, col: 43, offset: 14372},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 522, col: 52, offset: 14381},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 522, col: 58, offset: 14387},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 526, col: 1, offset: 14418},
			expr: &ruleRefExpr{
				pos:  position{line: 526, col: 18, offset: 14435},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 528, col: 1, offset: 14455},
			expr: &actionExpr{
				pos: position{line: 528, col: 23, offset: 14477},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 528, col: 23, offset: 14477},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 528, col: 23, offset: 14477},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 528, col: 27, offset: 14481},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 528, col: 29, offset: 14483},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 528, col: 34, offset: 14488},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 528, col: 39, offset: 14493},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 528, col: 41, offset: 14495},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 528, col: 45, offset: 14499},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 528, col: 47, offset: 14501},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 528, col: 52, offset: 14506},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 528, col: 67, offset: 14521},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 528, col: 69, offset: 14523},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 534, col: 1, offset: 14648},
			expr: &choiceExpr{
				pos: position{line: 534, col: 14, offset: 14661},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 534, col: 14, offset: 14661},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 534, col: 23, offset: 14670},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 534, col: 31, offset: 14678},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 536, col: 1, offset: 14683},
			expr: &choiceExpr{
				pos: position{line: 536, col: 11, offset: 14693},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 536, col: 11, offset: 14693},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 536, col: 20, offset: 14702},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 536, col: 29, offset: 14711},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 536, col: 36, offset: 14718},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 538, col: 1, offset: 14724},
			expr: &choiceExpr{
				pos: position{line: 538, col: 8, offset: 14731},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 538, col: 8, offset: 14731},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 538, col: 17, offset: 14740},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 538, col: 23, offset: 14746},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 540, col: 1, offset: 14751},
			expr: &actionExpr{
				pos: position{line: 540, col: 11, offset: 14761},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 540, col: 11, offset: 14761},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 540, col: 11, offset: 14761},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 540, col: 15, offset: 14765},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 540, col: 17, offset: 14767},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 540, col: 22, offset: 14772},
								expr: &seqExpr{
									pos: position{line: 540, col: 23, offset: 14773},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 540, col: 23, offset: 14773},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 27, offset: 14777},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 540, col: 29, offset: 14779},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 33, offset: 14783},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 35, offset: 14785},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 540, col: 42, offset: 14792},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 540, col: 47, offset: 14797},
								expr: &seqExpr{
									pos: position{line: 540, col: 49, offset: 14799},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 540, col: 49, offset: 14799},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 540, col: 51, offset: 14801},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 55, offset: 14805},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 57, offset: 14807},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 61, offset: 14811},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 540, col: 63, offset: 14813},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 67, offset: 14817},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 540, col: 69, offset: 14819},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 540, col: 77, offset: 14827},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 540, col: 79, offset: 14829},
							expr: &litMatcher{
								pos:        position{line: 540, col: 79, offset: 14829},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 540, col: 84, offset: 14834},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 540, col: 86, offset: 14836},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 544, col: 1, offset: 14899},
			expr: &actionExpr{
				pos: position{line: 544, col: 10, offset: 14908},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 544, col: 10, offset: 14908},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 544, col: 10, offset: 14908},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 544, col: 14, offset: 14912},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 544, col: 17, offset: 14915},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 544, col: 22, offset: 14920},
								expr: &ruleRefExpr{
									pos:  position{line: 544, col: 22, offset: 14920},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 544, col: 28, offset: 14926},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 544, col: 33, offset: 14931},
								expr: &seqExpr{
									pos: position{line: 544, col: 34, offset: 14932},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 544, col: 34, offset: 14932},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 544, col: 36, offset: 14934},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 544, col: 40, offset: 14938},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 544, col: 42, offset: 14940},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 544, col: 49, offset: 14947},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 544, col: 51, offset: 14949},
							expr: &litMatcher{
								pos:        position{line: 544, col: 51, offset: 14949},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 544, col: 56, offset: 14954},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 544, col: 59, offset: 14957},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgTerm",
			pos:  position{line: 553, col: 1, offset: 15352},
			expr: &actionExpr{
				pos: position{line: 553, col: 12, offset: 15363},
				run: (*parser).callonArgTerm1,
				expr: &labeledExpr{
					pos:   position{line: 553, col: 12, offset: 15363},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 553, col: 17, offset: 15368},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 553, col: 17, offset: 15368},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 553, col: 26, offset: 15377},
								name: "Var",
							},
							&ruleRefExpr{
								pos:  position{line: 553, col: 32, offset: 15383},
								name: "ArgObject",
							},
							&ruleRefExpr{
								pos:  position{line: 553, col: 44, offset: 15395},
								name: "ArgArray",
							},
						},
					},
				},
			},
		},
		{
			name: "ArgObject",
			pos:  position{line: 557, col: 1, offset: 15430},
			expr: &actionExpr{
				pos: position{line: 557, col: 14, offset: 15443},
				run: (*parser).callonArgObject1,
				expr: &seqExpr{
					pos: position{line: 557, col: 14, offset: 15443},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 557, col: 14, offset: 15443},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 557, col: 18, offset: 15447},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 557, col: 20, offset: 15449},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 557, col: 25, offset: 15454},
								expr: &seqExpr{
									pos: position{line: 557, col: 26, offset: 15455},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 557, col: 26, offset: 15455},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 33, offset: 15462},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 557, col: 35, offset: 15464},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 39, offset: 15468},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 41, offset: 15470},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 557, col: 51, offset: 15480},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 557, col: 56, offset: 15485},
								expr: &seqExpr{
									pos: position{line: 557, col: 58, offset: 15487},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 557, col: 58, offset: 15487},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 557, col: 60, offset: 15489},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 64, offset: 15493},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 66, offset: 15495},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 73, offset: 15502},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 557, col: 75, offset: 15504},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 79, offset: 15508},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 557, col: 81, offset: 15510},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 557, col: 92, offset: 15521},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 557, col: 94, offset: 15523},
							expr: &litMatcher{
								pos:        position{line: 557, col: 94, offset: 15523},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 557, col: 99, offset: 15528},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 557, col: 101, offset: 15530},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgKey",
			pos:  position{line: 561, col: 1, offset: 15593},
			expr: &ruleRefExpr{
				pos:  position{line: 561, col: 11, offset: 15603},
				name: "Scalar",
			},
		},
		{
			name: "ArgArray",
			pos:  position{line: 563, col: 1, offset: 15611},
			expr: &actionExpr{
				pos: position{line: 563, col: 13, offset: 15623},
				run: (*parser).callonArgArray1,
				expr: &seqExpr{
					pos: position{line: 563, col: 13, offset: 15623},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 563, col: 13, offset: 15623},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 563, col: 17, offset: 15627},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 563, col: 20, offset: 15630},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 563, col: 25, offset: 15635},
								expr: &ruleRefExpr{
									pos:  position{line: 563, col: 25, offset: 15635},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 563, col: 34, offset: 15644},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 563, col: 39, offset: 15649},
								expr: &seqExpr{
									pos: position{line: 563, col: 40, offset: 15650},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 563, col: 40, offset: 15650},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 563, col: 42, offset: 15652},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 563, col: 46, offset: 15656},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 563, col: 48, offset: 15658},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 563, col: 58, offset: 15668},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 563, col: 60, offset: 15670},
							expr: &litMatcher{
								pos:        position{line: 563, col: 60, offset: 15670},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 563, col: 65, offset: 15675},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 563, col: 68, offset: 15678},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 567, col: 1, offset: 15740},
			expr: &choiceExpr{
				pos: position{line: 567, col: 8, offset: 15747},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 567, col: 8, offset: 15747},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 567, col: 19, offset: 15758},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 569, col: 1, offset: 15771},
			expr: &actionExpr{
				pos: position{line: 569, col: 13, offset: 15783},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 569, col: 13, offset: 15783},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 569, col: 13, offset: 15783},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 569, col: 20, offset: 15790},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 569, col: 22, offset: 15792},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 575, col: 1, offset: 15880},
			expr: &actionExpr{
				pos: position{line: 575, col: 16, offset: 15895},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 575, col: 16, offset: 15895},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 575, col: 16, offset: 15895},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 575, col: 20, offset: 15899},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 575, col: 22, offset: 15901},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 575, col: 27, offset: 15906},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 575, col: 32, offset: 15911},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 575, col: 37, offset: 15916},
								expr: &seqExpr{
									pos: position{line: 575, col: 38, offset: 15917},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 575, col: 38, offset: 15917},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 575, col: 40, offset: 15919},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 575, col: 44, offset: 15923},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 575, col: 46, offset: 15925},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 575, col: 53, offset: 15932},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 575, col: 55, offset: 15934},
							expr: &litMatcher{
								pos:        position{line: 575, col: 55, offset: 15934},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 575, col: 60, offset: 15939},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 575, col: 62, offset: 15941},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 592, col: 1, offset: 16346},
			expr: &actionExpr{
				pos: position{line: 592, col: 8, offset: 16353},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 592, col: 8, offset: 16353},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 592, col: 8, offset: 16353},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 592, col: 13, offset: 16358},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 592, col: 17, offset: 16362},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 592, col: 22, offset: 16367},
								expr: &choiceExpr{
									pos: position{line: 592, col: 24, offset: 16369},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 592, col: 24, offset: 16369},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 592, col: 33, offset: 16378},
											name: "RefBracket",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "RefDot",
			pos:  position{line: 605, col: 1, offset: 16617},
			expr: &actionExpr{
				pos: position{line: 605, col: 11, offset: 16627},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 605, col: 11, offset: 16627},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 605, col: 11, offset: 16627},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 605, col: 15, offset: 16631},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 605, col: 19, offset: 16635},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 612, col: 1, offset: 16854},
			expr: &actionExpr{
				pos: position{line: 612, col: 15, offset: 16868},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 612, col: 15, offset: 16868},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 612, col: 15, offset: 16868},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 612, col: 19, offset: 16872},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 612, col: 24, offset: 16877},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 612, col: 24, offset: 16877},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 612, col: 30, offset: 16883},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 612, col: 39, offset: 16892},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 612, col: 44, offset: 16897},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 616, col: 1, offset: 16926},
			expr: &actionExpr{
				pos: position{line: 616, col: 8, offset: 16933},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 616, col: 8, offset: 16933},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 616, col: 12, offset: 16937},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 621, col: 1, offset: 17059},
			expr: &seqExpr{
				pos: position{line: 621, col: 15, offset: 17073},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 621, col: 15, offset: 17073},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 621, col: 19, offset: 17077},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 621, col: 32, offset: 17090},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 625, col: 1, offset: 17155},
			expr: &actionExpr{
				pos: position{line: 625, col: 17, offset: 17171},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 625, col: 17, offset: 17171},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 625, col: 17, offset: 17171},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 625, col: 29, offset: 17183},
							expr: &choiceExpr{
								pos: position{line: 625, col: 30, offset: 17184},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 625, col: 30, offset: 17184},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 625, col: 44, offset: 17198},
										name: "DecimalDigit",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Number",
			pos:  position{line: 632, col: 1, offset: 17341},
			expr: &actionExpr{
				pos: position{line: 632, col: 11, offset: 17351},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 632, col: 11, offset: 17351},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 632, col: 11, offset: 17351},
							expr: &litMatcher{
								pos:        position{line: 632, col: 11, offset: 17351},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 632, col: 18, offset: 17358},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 632, col: 18, offset: 17358},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 632, col: 26, offset: 17366},
									name: "Integer",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Float",
			pos:  position{line: 645, col: 1, offset: 17757},
			expr: &choiceExpr{
				pos: position{line: 645, col: 10, offset: 17766},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 645, col: 10, offset: 17766},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 645, col: 26, offset: 17782},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 647, col: 1, offset: 17794},
			expr: &seqExpr{
				pos: position{line: 647, col: 18, offset: 17811},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 647, col: 20, offset: 17813},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 647, col: 20, offset: 17813},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 647, col: 33, offset: 17826},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 647, col: 43, offset: 17836},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 649, col: 1, offset: 17846},
			expr: &seqExpr{
				pos: position{line: 649, col: 15, offset: 17860},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 649, col: 15, offset: 17860},
						expr: &ruleRefExpr{
							pos:  position{line: 649, col: 15, offset: 17860},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 649, col: 24, offset: 17869},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 651, col: 1, offset: 17879},
			expr: &seqExpr{
				pos: position{line: 651, col: 13, offset: 17891},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 651, col: 13, offset: 17891},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 651, col: 17, offset: 17895},
						expr: &ruleRefExpr{
							pos:  position{line: 651, col: 17, offset: 17895},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 653, col: 1, offset: 17910},
			expr: &seqExpr{
				pos: position{line: 653, col: 13, offset: 17922},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 653, col: 13, offset: 17922},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 653, col: 18, offset: 17927},
						expr: &charClassMatcher{
							pos:        position{line: 653, col: 18, offset: 17927},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 653, col: 24, offset: 17933},
						expr: &ruleRefExpr{
							pos:  position{line: 653, col: 24, offset: 17933},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 655, col: 1, offset: 17948},
			expr: &choiceExpr{
				pos: position{line: 655, col: 12, offset: 17959},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 655, col: 12, offset: 17959},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 655, col: 20, offset: 17967},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 655, col: 20, offset: 17967},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 655, col: 40, offset: 17987},
								expr: &ruleRefExpr{
									pos:  position{line: 655, col: 40, offset: 17987},
									name: "DecimalDigit",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "String",
			pos:  position{line: 657, col: 1, offset: 18004},
			expr: &actionExpr{
				pos: position{line: 657, col: 11, offset: 18014},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 657, col: 11, offset: 18014},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 657, col: 11, offset: 18014},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 657, col: 15, offset: 18018},
							expr: &ruleRefExpr{
								pos:  position{line: 657, col: 15, offset: 18018},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 657, col: 21, offset: 18024},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 665, col: 1, offset: 18179},
			expr: &choiceExpr{
				pos: position{line: 665, col: 9, offset: 18187},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 665, col: 9, offset: 18187},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 665, col: 9, offset: 18187},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 669, col: 5, offset: 18287},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 669, col: 5, offset: 18287},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 675, col: 1, offset: 18388},
			expr: &actionExpr{
				pos: position{line: 675, col: 9, offset: 18396},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 675, col: 9, offset: 18396},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 681, col: 1, offset: 18491},
			expr: &charClassMatcher{
				pos:        position{line: 681, col: 16, offset: 18506},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 683, col: 1, offset: 18517},
			expr: &choiceExpr{
				pos: position{line: 683, col: 9, offset: 18525},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 683, col: 11, offset: 18527},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 683, col: 11, offset: 18527},
								expr: &ruleRefExpr{
									pos:  position{line: 683, col: 12, offset: 18528},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 683, col: 24, offset: 18540,
							},
						},
					},
					&seqExpr{
						pos: position{line: 683, col: 32, offset: 18548},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 683, col: 32, offset: 18548},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 683, col: 37, offset: 18553},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 685, col: 1, offset: 18571},
			expr: &charClassMatcher{
				pos:        position{line: 685, col: 16, offset: 18586},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 687, col: 1, offset: 18602},
			expr: &choiceExpr{
				pos: position{line: 687, col: 19, offset: 18620},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 687, col: 19, offset: 18620},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 687, col: 38, offset: 18639},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 689, col: 1, offset: 18654},
			expr: &charClassMatcher{
				pos:        position{line: 689, col: 21, offset: 18674},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 691, col: 1, offset: 18696},
			expr: &seqExpr{
				pos: position{line: 691, col: 18, offset: 18713},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 691, col: 18, offset: 18713},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 691, col: 22, offset: 18717},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 691, col: 31, offset: 18726},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 691, col: 40, offset: 18735},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 691, col: 49, offset: 18744},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 693, col: 1, offset: 18754},
			expr: &charClassMatcher{
				pos:        position{line: 693, col: 17, offset: 18770},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 695, col: 1, offset: 18777},
			expr: &charClassMatcher{
				pos:        position{line: 695, col: 24, offset: 18800},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 697, col: 1, offset: 18807},
			expr: &charClassMatcher{
				pos:        position{line: 697, col: 13, offset: 18819},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 699, col: 1, offset: 18832},
			expr: &oneOrMoreExpr{
				pos: position{line: 699, col: 20, offset: 18851},
				expr: &charClassMatcher{
					pos:        position{line: 699, col: 20, offset: 18851},
					val:        "[ \\t\\r\\n]",
					chars:      []rune{' ', '\t', '\r', '\n'},
					ignoreCase: false,
					inverted:   false,
				},
			},
		},
		{
			name:        "_",
			displayName: "\"whitespace\"",
			pos:         position{line: 701, col: 1, offset: 18863},
			expr: &zeroOrMoreExpr{
				pos: position{line: 701, col: 19, offset: 18881},
				expr: &choiceExpr{
					pos: position{line: 701, col: 21, offset: 18883},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 701, col: 21, offset: 18883},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 701, col: 33, offset: 18895},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 703, col: 1, offset: 18907},
			expr: &actionExpr{
				pos: position{line: 703, col: 12, offset: 18918},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 703, col: 12, offset: 18918},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 703, col: 12, offset: 18918},
							expr: &charClassMatcher{
								pos:        position{line: 703, col: 12, offset: 18918},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 703, col: 19, offset: 18925},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 703, col: 23, offset: 18929},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 703, col: 28, offset: 18934},
								expr: &charClassMatcher{
									pos:        position{line: 703, col: 28, offset: 18934},
									val:        "[^\\r\\n]",
									chars:      []rune{'\r', '\n'},
									ignoreCase: false,
									inverted:   true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 714, col: 1, offset: 19210},
			expr: &notExpr{
				pos: position{line: 714, col: 8, offset: 19217},
				expr: &anyMatcher{
					line: 714, col: 9, offset: 19218,
				},
			},
		},
	},
}

func (c *current) onProgram1(vals interface{}) (interface{}, error) {
	var buf []interface{}

	if vals == nil {
		return buf, nil
	}

	ifaceSlice := vals.([]interface{})
	head := ifaceSlice[0]
	buf = append(buf, head)
	for _, tail := range ifaceSlice[1].([]interface{}) {
		stmt := tail.([]interface{})[1]
		buf = append(buf, stmt)
	}

	return program{buf, c.globalStore[commentsKey]}, nil
}

func (p *parser) callonProgram1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onProgram1(stack["vals"])
}

func (c *current) onStmt1(val interface{}) (interface{}, error) {
	return val, nil
}

func (p *parser) callonStmt1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onStmt1(stack["val"])
}

func (c *current) onParseError1() (interface{}, error) {
	panic(fmt.Sprintf("no match found, unexpected '%s'", c.text))
}

func (p *parser) callonParseError1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onParseError1()
}

func (c *current) onPackage1(val interface{}) (interface{}, error) {
	// All packages are implicitly declared under the default root document.
	term := val.(*Term)
	path := Ref{DefaultRootDocument.Copy().SetLocation(term.Location)}
	switch v := term.Value.(type) {
	case Ref:
		// Convert head of package Ref to String because it will be prefixed
		// with the root document variable.
		head := StringTerm(string(v[0].Value.(Var))).SetLocation(v[0].Location)
		tail := v[1:]
		if !tail.IsGround() {
			return nil, fmt.Errorf("package name cannot contain variables: %v", v)
		}

		// We do not allow non-string values in package names.
		// Because documents are typically represented as JSON, non-string keys are
		// not allowed for now.
		// TODO(tsandall): consider special syntax for namespacing under arrays.
		for _, p := range tail {
			_, ok := p.Value.(String)
			if !ok {
				return nil, fmt.Errorf("package name cannot contain non-string values: %v", v)
			}
		}
		path = append(path, head)
		path = append(path, tail...)
	case Var:
		s := StringTerm(string(v)).SetLocation(term.Location)
		path = append(path, s)
	}
	pkg := &Package{Location: currentLocation(c), Path: path}
	return pkg, nil
}

func (p *parser) callonPackage1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPackage1(stack["val"])
}

func (c *current) onImport1(path, alias interface{}) (interface{}, error) {
	imp := &Import{}
	imp.Location = currentLocation(c)
	imp.Path = path.(*Term)
	if err := IsValidImportPath(imp.Path.Value); err != nil {
		return nil, err
	}
	if alias == nil {
		return imp, nil
	}
	aliasSlice := alias.([]interface{})
	// Import definition above describes the "alias" slice. We only care about the "Var" element.
	imp.Alias = aliasSlice[3].(*Term).Value.(Var)
	return imp, nil
}

func (p *parser) callonImport1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onImport1(stack["path"], stack["alias"])
}

func (c *current) onDefaultRules1(name, value interface{}) (interface{}, error) {

	term := value.(*Term)
	var err error

	vis := NewGenericVisitor(func(x interface{}) bool {
		if err != nil {
			return true
		}
		switch x.(type) {
		case *ArrayComprehension: // skip closures
			return true
		case Ref, Var:
			err = fmt.Errorf("default rule value cannot contain %v", TypeName(x))
			return true
		}
		return false
	})

	Walk(vis, term)

	if err != nil {
		return nil, err
	}

	loc := currentLocation(c)
	body := NewBody(NewExpr(BooleanTerm(true).SetLocation(loc)))

	rule := &Rule{
		Location: loc,
		Default:  true,
		Head: &Head{
			Location: currentLocation(c),
			Name:     name.(*Term).Value.(Var),
			Value:    value.(*Term),
		},
		Body: body,
	}
	rule.Body[0].Location = currentLocation(c)

	return []*Rule{rule}, nil
}

func (p *parser) callonDefaultRules1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onDefaultRules1(stack["name"], stack["value"])
}

func (c *current) onNormalRules1(head, b interface{}) (interface{}, error) {

	if head == nil {
		return nil, nil
	}

	sl := b.([]interface{})

	rules := []*Rule{
		&Rule{
			Location: currentLocation(c),
			Head:     head.(*Head),
			Body:     sl[0].(Body),
		},
	}

	var ordered bool
	prev := rules[0]

	for i, elem := range sl[1].([]interface{}) {

		next := elem.([]interface{})
		re := next[1].(ruleExt)

		if re.term == nil {
			if ordered {
				return nil, fmt.Errorf("expected 'else' keyword")
			}
			rules = append(rules, &Rule{
				Location: re.loc,
				Head:     prev.Head.Copy(),
				Body:     re.body,
			})
		} else {
			if (rules[0].Head.DocKind() != CompleteDoc) || (i != 0 && !ordered) {
				return nil, fmt.Errorf("unexpected 'else' keyword")
			}
			ordered = true
			curr := &Rule{
				Location: re.loc,
				Head: &Head{
					Name:     prev.Head.Name,
					Value:    re.term,
					Location: re.term.Location,
				},
				Body: re.body,
			}
			prev.Else = curr
			prev = curr
		}
	}

	return rules, nil
}

func (p *parser) callonNormalRules1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNormalRules1(stack["head"], stack["b"])
}

func (c *current) onUserFunc1(head, b interface{}) (interface{}, error) {

	if head == nil {
		return nil, nil
	}

	f := &Func{
		Location: currentLocation(c),
		Head:     head.(*FuncHead),
		Body:     b.(Body),
	}

	return f, nil
}

func (p *parser) callonUserFunc1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onUserFunc1(stack["head"], stack["b"])
}

func (c *current) onFuncHead1(name, args, output interface{}) (interface{}, error) {

	head := &FuncHead{}

	head.Location = currentLocation(c)
	head.Name = name.(*Term).Value.(Var)
	head.Args = args.(Args)

	if output != nil {
		valueSlice := output.([]interface{})
		head.Output = valueSlice[len(valueSlice)-1].(*Term)
	}

	return head, nil
}

func (p *parser) callonFuncHead1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onFuncHead1(stack["name"], stack["args"], stack["output"])
}

func (c *current) onFuncArgs1(head, tail interface{}) (interface{}, error) {
	args := Args{}
	if head == nil {
		return args, nil
	}

	first := head.(*Term)
	first.Location = currentLocation(c)
	args = append(args, first)

	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		arg := s[len(s)-1].(*Term)
		arg.Location = currentLocation(c)
		args = append(args, arg)
	}

	return args, nil
}

func (p *parser) callonFuncArgs1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onFuncArgs1(stack["head"], stack["tail"])
}

func (c *current) onRuleHead1(name, key, value interface{}) (interface{}, error) {

	head := &Head{}

	head.Location = currentLocation(c)
	head.Name = name.(*Term).Value.(Var)

	if key != nil {
		keySlice := key.([]interface{})
		// Head definition above describes the "key" slice. We care about the "Term" element.
		head.Key = keySlice[3].(*Term)
	}

	if value != nil {
		valueSlice := value.([]interface{})
		// Head definition above describes the "value" slice. We care about the "Term" element.
		head.Value = valueSlice[len(valueSlice)-1].(*Term)
	}

	if key == nil && value == nil {
		head.Value = BooleanTerm(true)
		head.Value.Location = head.Location
	}

	if key != nil && value != nil {
		switch head.Key.Value.(type) {
		case Var, String, Ref: // nop
		default:
			return nil, fmt.Errorf("object key must be one of %v, %v, %v not %v", StringTypeName, VarTypeName, RefTypeName, TypeName(head.Key.Value))
		}
	}

	return head, nil
}

func (p *parser) callonRuleHead1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRuleHead1(stack["name"], stack["key"], stack["value"])
}

func (c *current) onElse1(val, b interface{}) (interface{}, error) {
	bs := b.([]interface{})
	body := bs[1].(Body)

	if val == nil {
		term := BooleanTerm(true)
		term.Location = currentLocation(c)
		return ruleExt{term.Location, term, body}, nil
	}

	vs := val.([]interface{})
	t := vs[3].(*Term)
	return ruleExt{currentLocation(c), t, body}, nil
}

func (p *parser) callonElse1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onElse1(stack["val"], stack["b"])
}

func (c *current) onRuleDup1(b interface{}) (interface{}, error) {
	return ruleExt{loc: currentLocation(c), body: b.(Body)}, nil
}

func (p *parser) callonRuleDup1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRuleDup1(stack["b"])
}

func (c *current) onNonEmptyBraceEnclosedBody1(val interface{}) (interface{}, error) {
	if val == nil {
		panic("body must be non-empty")
	}
	return val, nil
}

func (p *parser) callonNonEmptyBraceEnclosedBody1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNonEmptyBraceEnclosedBody1(stack["val"])
}

func (c *current) onBraceEnclosedBody1(val interface{}) (interface{}, error) {

	if val == nil {
		loc := currentLocation(c)
		body := NewBody(NewExpr(ObjectTerm().SetLocation(loc)))
		body[0].Location = loc
		return body, nil
	}

	return val, nil
}

func (p *parser) callonBraceEnclosedBody1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBraceEnclosedBody1(stack["val"])
}

func (c *current) onWhitespaceBody1(head, tail interface{}) (interface{}, error) {
	return ifacesToBody(head, tail.([]interface{})...), nil
}

func (p *parser) callonWhitespaceBody1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onWhitespaceBody1(stack["head"], stack["tail"])
}

func (c *current) onNonWhitespaceBody1(head, tail interface{}) (interface{}, error) {
	return ifacesToBody(head, tail.([]interface{})...), nil
}

func (p *parser) callonNonWhitespaceBody1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNonWhitespaceBody1(stack["head"], stack["tail"])
}

func (c *current) onLiteral1(neg, val, with interface{}) (interface{}, error) {
	expr := &Expr{}
	expr.Location = currentLocation(c)
	expr.Negated = neg != nil
	expr.Terms = val

	if with != nil {
		sl := with.([]interface{})
		if head, ok := sl[1].(*With); ok {
			expr.With = []*With{head}
			if sl, ok := sl[2].([]interface{}); ok {
				for i := range sl {
					if w, ok := sl[i].([]interface{})[1].(*With); ok {
						expr.With = append(expr.With, w)
					}
				}
			}
		}
	}

	return expr, nil
}

func (p *parser) callonLiteral1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLiteral1(stack["neg"], stack["val"], stack["with"])
}

func (c *current) onWith1(target, value interface{}) (interface{}, error) {
	with := &With{}
	with.Location = currentLocation(c)
	with.Target = target.(*Term)
	if err := IsValidImportPath(with.Target.Value); err != nil {
		return nil, err
	}
	with.Value = value.(*Term)
	return with, nil
}

func (p *parser) callonWith1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onWith1(stack["target"], stack["value"])
}

func (c *current) onInfixArithExpr1(output, left, op, right interface{}) (interface{}, error) {
	return []*Term{op.(*Term), left.(*Term), right.(*Term), output.(*Term)}, nil
}

func (p *parser) callonInfixArithExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixArithExpr1(stack["output"], stack["left"], stack["op"], stack["right"])
}

func (c *current) onInfixArithExprReverse1(left, op, right, output interface{}) (interface{}, error) {
	return []*Term{op.(*Term), left.(*Term), right.(*Term), output.(*Term)}, nil
}

func (p *parser) callonInfixArithExprReverse1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixArithExprReverse1(stack["left"], stack["op"], stack["right"], stack["output"])
}

func (c *current) onArithInfixOp1(val interface{}) (interface{}, error) {
	op := string(c.text)
	for _, b := range Builtins {
		if string(b.Infix) == op {
			op = string(b.Name)
		}
	}
	operator := StringTerm(op)
	operator.Location = currentLocation(c)
	return operator, nil
}

func (p *parser) callonArithInfixOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArithInfixOp1(stack["val"])
}

func (c *current) onInfixExpr1(left, op, right interface{}) (interface{}, error) {
	return []*Term{op.(*Term), left.(*Term), right.(*Term)}, nil
}

func (p *parser) callonInfixExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixExpr1(stack["left"], stack["op"], stack["right"])
}

func (c *current) onInfixOp1(val interface{}) (interface{}, error) {
	op := string(c.text)
	for _, b := range Builtins {
		if string(b.Infix) == op {
			op = string(b.Name)
		}
	}
	operator := StringTerm(op)
	operator.Location = currentLocation(c)
	return operator, nil
}

func (p *parser) callonInfixOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixOp1(stack["val"])
}

func (c *current) onBuiltin1(name, head, tail interface{}) (interface{}, error) {
	buf := []*Term{name.(*Term)}
	if head == nil {
		return buf, nil
	}
	buf = append(buf, head.(*Term))

	// PrefixExpr above describes the "tail" structure. We only care about the "Term" elements.
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		buf = append(buf, s[len(s)-1].(*Term))
	}
	return buf, nil
}

func (p *parser) callonBuiltin1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBuiltin1(stack["name"], stack["head"], stack["tail"])
}

func (c *current) onBuiltinName1(head, tail interface{}) (interface{}, error) {
	tailSlice := tail.([]interface{})
	buf := make([]string, 1+len(tailSlice))
	buf[0] = string(head.(*Term).Value.(Var))
	for i := range tailSlice {
		elem := tailSlice[i]
		part := elem.([]interface{})[1].(*Term).Value.(Var)
		buf[i+1] = string(part)
	}
	name := StringTerm(strings.Join(buf, "."))
	name.Location = currentLocation(c)
	return name, nil
}

func (p *parser) callonBuiltinName1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBuiltinName1(stack["head"], stack["tail"])
}

func (c *current) onTerm1(val interface{}) (interface{}, error) {
	return val, nil
}

func (p *parser) callonTerm1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onTerm1(stack["val"])
}

func (c *current) onArrayComprehension1(term, body interface{}) (interface{}, error) {
	ac := ArrayComprehensionTerm(term.(*Term), body.(Body))
	ac.Location = currentLocation(c)
	return ac, nil
}

func (p *parser) callonArrayComprehension1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArrayComprehension1(stack["term"], stack["body"])
}

func (c *current) onObject1(head, tail interface{}) (interface{}, error) {
	return makeObject(head, tail, currentLocation(c))
}

func (p *parser) callonObject1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObject1(stack["head"], stack["tail"])
}

func (c *current) onArray1(head, tail interface{}) (interface{}, error) {
	return makeArray(head, tail, currentLocation(c))
}

func (p *parser) callonArray1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArray1(stack["head"], stack["tail"])
}

func (c *current) onArgTerm1(val interface{}) (interface{}, error) {
	return val, nil
}

func (p *parser) callonArgTerm1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArgTerm1(stack["val"])
}

func (c *current) onArgObject1(head, tail interface{}) (interface{}, error) {
	return makeObject(head, tail, currentLocation(c))
}

func (p *parser) callonArgObject1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArgObject1(stack["head"], stack["tail"])
}

func (c *current) onArgArray1(head, tail interface{}) (interface{}, error) {
	return makeArray(head, tail, currentLocation(c))
}

func (p *parser) callonArgArray1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArgArray1(stack["head"], stack["tail"])
}

func (c *current) onSetEmpty1() (interface{}, error) {
	set := SetTerm()
	set.Location = currentLocation(c)
	return set, nil
}

func (p *parser) callonSetEmpty1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSetEmpty1()
}

func (c *current) onSetNonEmpty1(head, tail interface{}) (interface{}, error) {
	set := SetTerm()
	set.Location = currentLocation(c)

	val := set.Value.(*Set)
	val.Add(head.(*Term))

	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		// SetNonEmpty definition above describes the "tail" structure. We only care about the "Term" elements.
		val.Add(s[len(s)-1].(*Term))
	}

	return set, nil
}

func (p *parser) callonSetNonEmpty1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSetNonEmpty1(stack["head"], stack["tail"])
}

func (c *current) onRef1(head, tail interface{}) (interface{}, error) {

	ref := RefTerm(head.(*Term))
	ref.Location = currentLocation(c)

	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		ref.Value = append(ref.Value.(Ref), v.(*Term))
	}

	return ref, nil
}

func (p *parser) callonRef1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRef1(stack["head"], stack["tail"])
}

func (c *current) onRefDot1(val interface{}) (interface{}, error) {
	// Convert the Var into a string because 'foo.bar.baz' is equivalent to 'foo["bar"]["baz"]'.
	str := StringTerm(string(val.(*Term).Value.(Var)))
	str.Location = currentLocation(c)
	return str, nil
}

func (p *parser) callonRefDot1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRefDot1(stack["val"])
}

func (c *current) onRefBracket1(val interface{}) (interface{}, error) {
	return val, nil
}

func (p *parser) callonRefBracket1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRefBracket1(stack["val"])
}

func (c *current) onVar1(val interface{}) (interface{}, error) {
	return val.([]interface{})[0], nil
}

func (p *parser) callonVar1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onVar1(stack["val"])
}

func (c *current) onVarChecked4(val interface{}) (bool, error) {
	return IsKeyword(string(val.(*Term).Value.(Var))), nil
}

func (p *parser) callonVarChecked4() (bool, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onVarChecked4(stack["val"])
}

func (c *current) onVarUnchecked1() (interface{}, error) {
	str := string(c.text)
	variable := VarTerm(str)
	variable.Location = currentLocation(c)
	return variable, nil
}

func (p *parser) callonVarUnchecked1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onVarUnchecked1()
}

func (c *current) onNumber1() (interface{}, error) {
	f, ok := new(big.Float).SetString(string(c.text))
	if !ok {
		// This indicates the grammar is out-of-sync with what the string
		// representation of floating point numbers. This should not be
		// possible.
		panic("illegal value")
	}
	num := NumberTerm(json.Number(f.String()))
	num.Location = currentLocation(c)
	return num, nil
}

func (p *parser) callonNumber1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNumber1()
}

func (c *current) onString1() (interface{}, error) {
	var v string
	err := json.Unmarshal([]byte(c.text), &v)
	str := StringTerm(v)
	str.Location = currentLocation(c)
	return str, err
}

func (p *parser) callonString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onString1()
}

func (c *current) onBool2() (interface{}, error) {
	bol := BooleanTerm(true)
	bol.Location = currentLocation(c)
	return bol, nil
}

func (p *parser) callonBool2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBool2()
}

func (c *current) onBool4() (interface{}, error) {
	bol := BooleanTerm(false)
	bol.Location = currentLocation(c)
	return bol, nil
}

func (p *parser) callonBool4() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBool4()
}

func (c *current) onNull1() (interface{}, error) {
	null := NullTerm()
	null.Location = currentLocation(c)
	return null, nil
}

func (p *parser) callonNull1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNull1()
}

func (c *current) onComment1(text interface{}) (interface{}, error) {
	comment := NewComment(ifaceSliceToByteSlice(text))
	comment.Location = currentLocation(c)

	comments := c.globalStore[commentsKey].([]*Comment)
	comments = append(comments, comment)
	c.globalStore[commentsKey] = comments

	return comment, nil
}

func (p *parser) callonComment1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onComment1(stack["text"])
}

var (
	// errNoRule is returned when the grammar to parse has no rule.
	errNoRule = errors.New("grammar has no rule")

	// errInvalidEncoding is returned when the source is not properly
	// utf8-encoded.
	errInvalidEncoding = errors.New("invalid encoding")
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
		err = f.Close()
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

	// the globalStore allows the parser to store arbitrary values
	globalStore map[string]interface{}
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
	p := &parser{
		filename: filename,
		errs:     new(errList),
		data:     b,
		pt:       savepoint{position: position{line: 1}},
		recover:  true,
		cur: current{
			globalStore: make(map[string]interface{}),
		},
		maxFailPos:      position{col: 1, line: 1},
		maxFailExpected: make(map[string]struct{}),
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

	depth   int
	recover bool
	debug   bool

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

	// parse fail
	maxFailPos            position
	maxFailExpected       map[string]struct{}
	maxFailInvertExpected bool
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
			p.maxFailExpected = make(map[string]struct{})
		}

		if p.maxFailInvertExpected {
			want = "!" + want
		}
		p.maxFailExpected[want] = struct{}{}
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

	if rn == utf8.RuneError {
		if n == 1 {
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
			// If parsing fails, but no errors have been recorded, the expected values
			// for the farthest parser position are returned as error.
			expected := make([]string, 0, len(p.maxFailExpected))
			eof := false
			if _, ok := p.maxFailExpected["!."]; ok {
				delete(p.maxFailExpected, "!.")
				eof = true
			}
			for k := range p.maxFailExpected {
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
			p.addErrAt(err, start.position, []string{})
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
		p.failAt(true, start.position, ".")
		return p.sliceFrom(start), true
	}
	p.failAt(false, p.pt.position, ".")
	return nil, false
}

func (p *parser) parseCharClassMatcher(chr *charClassMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseCharClassMatcher"))
	}

	cur := p.pt.rn
	start := p.pt
	// can't match EOF
	if cur == utf8.RuneError {
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
	p.maxFailInvertExpected = !p.maxFailInvertExpected
	_, ok := p.parseExpr(not.expr)
	p.maxFailInvertExpected = !p.maxFailInvertExpected
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

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
			pos:  position{line: 291, col: 1, offset: 7841},
			expr: &actionExpr{
				pos: position{line: 291, col: 13, offset: 7853},
				run: (*parser).callonFuncHead1,
				expr: &seqExpr{
					pos: position{line: 291, col: 13, offset: 7853},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 291, col: 13, offset: 7853},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 291, col: 18, offset: 7858},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 291, col: 22, offset: 7862},
							label: "args",
							expr: &ruleRefExpr{
								pos:  position{line: 291, col: 27, offset: 7867},
								name: "FuncArgs",
							},
						},
						&labeledExpr{
							pos:   position{line: 291, col: 36, offset: 7876},
							label: "output",
							expr: &zeroOrOneExpr{
								pos: position{line: 291, col: 43, offset: 7883},
								expr: &seqExpr{
									pos: position{line: 291, col: 45, offset: 7885},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 291, col: 45, offset: 7885},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 291, col: 47, offset: 7887},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 291, col: 51, offset: 7891},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 291, col: 53, offset: 7893},
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
			pos:  position{line: 307, col: 1, offset: 8197},
			expr: &actionExpr{
				pos: position{line: 307, col: 13, offset: 8209},
				run: (*parser).callonFuncArgs1,
				expr: &seqExpr{
					pos: position{line: 307, col: 13, offset: 8209},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 307, col: 13, offset: 8209},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 307, col: 15, offset: 8211},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 307, col: 20, offset: 8216},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 307, col: 23, offset: 8219},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 307, col: 28, offset: 8224},
								expr: &ruleRefExpr{
									pos:  position{line: 307, col: 28, offset: 8224},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 307, col: 37, offset: 8233},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 307, col: 42, offset: 8238},
								expr: &seqExpr{
									pos: position{line: 307, col: 43, offset: 8239},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 307, col: 43, offset: 8239},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 307, col: 45, offset: 8241},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 307, col: 49, offset: 8245},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 307, col: 51, offset: 8247},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 307, col: 61, offset: 8257},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 307, col: 63, offset: 8259},
							val:        ")",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 307, col: 67, offset: 8263},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "RuleHead",
			pos:  position{line: 328, col: 1, offset: 8683},
			expr: &actionExpr{
				pos: position{line: 328, col: 13, offset: 8695},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 328, col: 13, offset: 8695},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 328, col: 13, offset: 8695},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 328, col: 18, offset: 8700},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 328, col: 22, offset: 8704},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 328, col: 26, offset: 8708},
								expr: &seqExpr{
									pos: position{line: 328, col: 28, offset: 8710},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 328, col: 28, offset: 8710},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 328, col: 30, offset: 8712},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 34, offset: 8716},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 36, offset: 8718},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 41, offset: 8723},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 328, col: 43, offset: 8725},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 47, offset: 8729},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 328, col: 52, offset: 8734},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 328, col: 58, offset: 8740},
								expr: &seqExpr{
									pos: position{line: 328, col: 60, offset: 8742},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 328, col: 60, offset: 8742},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 328, col: 62, offset: 8744},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 66, offset: 8748},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 328, col: 68, offset: 8750},
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
			pos:  position{line: 363, col: 1, offset: 9738},
			expr: &actionExpr{
				pos: position{line: 363, col: 9, offset: 9746},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 363, col: 9, offset: 9746},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 363, col: 9, offset: 9746},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 363, col: 16, offset: 9753},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 363, col: 20, offset: 9757},
								expr: &seqExpr{
									pos: position{line: 363, col: 22, offset: 9759},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 363, col: 22, offset: 9759},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 363, col: 24, offset: 9761},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 363, col: 28, offset: 9765},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 363, col: 30, offset: 9767},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 363, col: 38, offset: 9775},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 363, col: 42, offset: 9779},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 363, col: 42, offset: 9779},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 363, col: 44, offset: 9781},
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
			pos:  position{line: 378, col: 1, offset: 10133},
			expr: &actionExpr{
				pos: position{line: 378, col: 12, offset: 10144},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 378, col: 12, offset: 10144},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 378, col: 14, offset: 10146},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 382, col: 1, offset: 10242},
			expr: &choiceExpr{
				pos: position{line: 382, col: 12, offset: 10253},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 382, col: 12, offset: 10253},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 382, col: 19, offset: 10260},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 384, col: 1, offset: 10269},
			expr: &choiceExpr{
				pos: position{line: 384, col: 9, offset: 10277},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 384, col: 9, offset: 10277},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 384, col: 29, offset: 10297},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 386, col: 1, offset: 10316},
			expr: &actionExpr{
				pos: position{line: 386, col: 30, offset: 10345},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 386, col: 30, offset: 10345},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 386, col: 30, offset: 10345},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 386, col: 34, offset: 10349},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 386, col: 36, offset: 10351},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 386, col: 40, offset: 10355},
								expr: &ruleRefExpr{
									pos:  position{line: 386, col: 40, offset: 10355},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 386, col: 56, offset: 10371},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 386, col: 58, offset: 10373},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 393, col: 1, offset: 10468},
			expr: &actionExpr{
				pos: position{line: 393, col: 22, offset: 10489},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 393, col: 22, offset: 10489},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 393, col: 22, offset: 10489},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 393, col: 26, offset: 10493},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 393, col: 28, offset: 10495},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 393, col: 32, offset: 10499},
								expr: &ruleRefExpr{
									pos:  position{line: 393, col: 32, offset: 10499},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 393, col: 48, offset: 10515},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 393, col: 50, offset: 10517},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 407, col: 1, offset: 10869},
			expr: &actionExpr{
				pos: position{line: 407, col: 19, offset: 10887},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 407, col: 19, offset: 10887},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 407, col: 19, offset: 10887},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 407, col: 24, offset: 10892},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 407, col: 32, offset: 10900},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 407, col: 37, offset: 10905},
								expr: &seqExpr{
									pos: position{line: 407, col: 38, offset: 10906},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 407, col: 38, offset: 10906},
											expr: &charClassMatcher{
												pos:        position{line: 407, col: 38, offset: 10906},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 407, col: 46, offset: 10914},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 407, col: 47, offset: 10915},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 407, col: 47, offset: 10915},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 407, col: 51, offset: 10919},
															expr: &ruleRefExpr{
																pos:  position{line: 407, col: 51, offset: 10919},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 407, col: 64, offset: 10932},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 407, col: 64, offset: 10932},
															expr: &ruleRefExpr{
																pos:  position{line: 407, col: 64, offset: 10932},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 407, col: 73, offset: 10941},
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
											pos:  position{line: 407, col: 82, offset: 10950},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 407, col: 84, offset: 10952},
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
			pos:  position{line: 413, col: 1, offset: 11141},
			expr: &actionExpr{
				pos: position{line: 413, col: 22, offset: 11162},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 413, col: 22, offset: 11162},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 413, col: 22, offset: 11162},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 413, col: 27, offset: 11167},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 413, col: 35, offset: 11175},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 413, col: 40, offset: 11180},
								expr: &seqExpr{
									pos: position{line: 413, col: 42, offset: 11182},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 413, col: 42, offset: 11182},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 413, col: 44, offset: 11184},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 413, col: 48, offset: 11188},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 413, col: 51, offset: 11191},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 413, col: 51, offset: 11191},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 413, col: 61, offset: 11201},
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
			pos:  position{line: 417, col: 1, offset: 11280},
			expr: &actionExpr{
				pos: position{line: 417, col: 12, offset: 11291},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 417, col: 12, offset: 11291},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 417, col: 12, offset: 11291},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 417, col: 16, offset: 11295},
								expr: &seqExpr{
									pos: position{line: 417, col: 18, offset: 11297},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 417, col: 18, offset: 11297},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 417, col: 24, offset: 11303},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 417, col: 30, offset: 11309},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 417, col: 34, offset: 11313},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 417, col: 39, offset: 11318},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 417, col: 44, offset: 11323},
								expr: &seqExpr{
									pos: position{line: 417, col: 46, offset: 11325},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 417, col: 46, offset: 11325},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 417, col: 49, offset: 11328},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 417, col: 54, offset: 11333},
											expr: &seqExpr{
												pos: position{line: 417, col: 55, offset: 11334},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 417, col: 55, offset: 11334},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 417, col: 58, offset: 11337},
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
			pos:  position{line: 440, col: 1, offset: 11909},
			expr: &actionExpr{
				pos: position{line: 440, col: 9, offset: 11917},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 440, col: 9, offset: 11917},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 440, col: 9, offset: 11917},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 16, offset: 11924},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 440, col: 19, offset: 11927},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 440, col: 26, offset: 11934},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 31, offset: 11939},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 440, col: 34, offset: 11942},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 39, offset: 11947},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 440, col: 42, offset: 11950},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 440, col: 48, offset: 11956},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 451, col: 1, offset: 12205},
			expr: &choiceExpr{
				pos: position{line: 451, col: 9, offset: 12213},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 451, col: 10, offset: 12214},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 451, col: 10, offset: 12214},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 451, col: 27, offset: 12231},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 451, col: 52, offset: 12256},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 451, col: 64, offset: 12268},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 451, col: 77, offset: 12281},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 453, col: 1, offset: 12287},
			expr: &actionExpr{
				pos: position{line: 453, col: 19, offset: 12305},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 453, col: 19, offset: 12305},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 453, col: 19, offset: 12305},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 26, offset: 12312},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 31, offset: 12317},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 453, col: 33, offset: 12319},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 37, offset: 12323},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 39, offset: 12325},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 44, offset: 12330},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 49, offset: 12335},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 51, offset: 12337},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 54, offset: 12340},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 67, offset: 12353},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 69, offset: 12355},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 75, offset: 12361},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 457, col: 1, offset: 12452},
			expr: &actionExpr{
				pos: position{line: 457, col: 26, offset: 12477},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 457, col: 26, offset: 12477},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 457, col: 26, offset: 12477},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 457, col: 31, offset: 12482},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 457, col: 36, offset: 12487},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 457, col: 38, offset: 12489},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 457, col: 41, offset: 12492},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 457, col: 54, offset: 12505},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 457, col: 56, offset: 12507},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 457, col: 62, offset: 12513},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 457, col: 67, offset: 12518},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 457, col: 69, offset: 12520},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 457, col: 73, offset: 12524},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 457, col: 75, offset: 12526},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 457, col: 82, offset: 12533},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 461, col: 1, offset: 12624},
			expr: &actionExpr{
				pos: position{line: 461, col: 17, offset: 12640},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 461, col: 17, offset: 12640},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 461, col: 22, offset: 12645},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 461, col: 22, offset: 12645},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 28, offset: 12651},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 34, offset: 12657},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 40, offset: 12663},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 46, offset: 12669},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 52, offset: 12675},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 461, col: 58, offset: 12681},
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
			pos:  position{line: 473, col: 1, offset: 12928},
			expr: &actionExpr{
				pos: position{line: 473, col: 14, offset: 12941},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 473, col: 14, offset: 12941},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 473, col: 14, offset: 12941},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 473, col: 19, offset: 12946},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 473, col: 24, offset: 12951},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 473, col: 26, offset: 12953},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 473, col: 29, offset: 12956},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 473, col: 37, offset: 12964},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 473, col: 39, offset: 12966},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 473, col: 45, offset: 12972},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 477, col: 1, offset: 13047},
			expr: &actionExpr{
				pos: position{line: 477, col: 12, offset: 13058},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 477, col: 12, offset: 13058},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 477, col: 17, offset: 13063},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 477, col: 17, offset: 13063},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 477, col: 23, offset: 13069},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 477, col: 30, offset: 13076},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 477, col: 37, offset: 13083},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 477, col: 44, offset: 13090},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 477, col: 50, offset: 13096},
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
			pos:  position{line: 489, col: 1, offset: 13343},
			expr: &choiceExpr{
				pos: position{line: 489, col: 15, offset: 13357},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 489, col: 15, offset: 13357},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 489, col: 26, offset: 13368},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 491, col: 1, offset: 13377},
			expr: &actionExpr{
				pos: position{line: 491, col: 12, offset: 13388},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 491, col: 12, offset: 13388},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 491, col: 12, offset: 13388},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 491, col: 17, offset: 13393},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 491, col: 29, offset: 13405},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 491, col: 33, offset: 13409},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 491, col: 35, offset: 13411},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 491, col: 40, offset: 13416},
								expr: &ruleRefExpr{
									pos:  position{line: 491, col: 40, offset: 13416},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 491, col: 46, offset: 13422},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 491, col: 51, offset: 13427},
								expr: &seqExpr{
									pos: position{line: 491, col: 53, offset: 13429},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 491, col: 53, offset: 13429},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 491, col: 55, offset: 13431},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 491, col: 59, offset: 13435},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 491, col: 61, offset: 13437},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 491, col: 69, offset: 13445},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 491, col: 72, offset: 13448},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 507, col: 1, offset: 13852},
			expr: &actionExpr{
				pos: position{line: 507, col: 16, offset: 13867},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 507, col: 16, offset: 13867},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 507, col: 16, offset: 13867},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 507, col: 21, offset: 13872},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 507, col: 25, offset: 13876},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 507, col: 30, offset: 13881},
								expr: &seqExpr{
									pos: position{line: 507, col: 32, offset: 13883},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 507, col: 32, offset: 13883},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 507, col: 36, offset: 13887},
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
			pos:  position{line: 521, col: 1, offset: 14292},
			expr: &actionExpr{
				pos: position{line: 521, col: 9, offset: 14300},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 521, col: 9, offset: 14300},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 521, col: 15, offset: 14306},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 521, col: 15, offset: 14306},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 521, col: 31, offset: 14322},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 521, col: 43, offset: 14334},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 521, col: 52, offset: 14343},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 521, col: 58, offset: 14349},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 525, col: 1, offset: 14380},
			expr: &ruleRefExpr{
				pos:  position{line: 525, col: 18, offset: 14397},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 527, col: 1, offset: 14417},
			expr: &actionExpr{
				pos: position{line: 527, col: 23, offset: 14439},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 527, col: 23, offset: 14439},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 527, col: 23, offset: 14439},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 527, col: 27, offset: 14443},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 527, col: 29, offset: 14445},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 527, col: 34, offset: 14450},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 527, col: 39, offset: 14455},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 527, col: 41, offset: 14457},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 527, col: 45, offset: 14461},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 527, col: 47, offset: 14463},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 527, col: 52, offset: 14468},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 527, col: 67, offset: 14483},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 527, col: 69, offset: 14485},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 533, col: 1, offset: 14610},
			expr: &choiceExpr{
				pos: position{line: 533, col: 14, offset: 14623},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 533, col: 14, offset: 14623},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 533, col: 23, offset: 14632},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 533, col: 31, offset: 14640},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 535, col: 1, offset: 14645},
			expr: &choiceExpr{
				pos: position{line: 535, col: 11, offset: 14655},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 535, col: 11, offset: 14655},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 535, col: 20, offset: 14664},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 535, col: 29, offset: 14673},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 535, col: 36, offset: 14680},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 537, col: 1, offset: 14686},
			expr: &choiceExpr{
				pos: position{line: 537, col: 8, offset: 14693},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 537, col: 8, offset: 14693},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 537, col: 17, offset: 14702},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 537, col: 23, offset: 14708},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 539, col: 1, offset: 14713},
			expr: &actionExpr{
				pos: position{line: 539, col: 11, offset: 14723},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 539, col: 11, offset: 14723},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 539, col: 11, offset: 14723},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 539, col: 15, offset: 14727},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 539, col: 17, offset: 14729},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 539, col: 22, offset: 14734},
								expr: &seqExpr{
									pos: position{line: 539, col: 23, offset: 14735},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 539, col: 23, offset: 14735},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 27, offset: 14739},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 539, col: 29, offset: 14741},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 33, offset: 14745},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 35, offset: 14747},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 539, col: 42, offset: 14754},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 539, col: 47, offset: 14759},
								expr: &seqExpr{
									pos: position{line: 539, col: 49, offset: 14761},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 539, col: 49, offset: 14761},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 539, col: 51, offset: 14763},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 55, offset: 14767},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 57, offset: 14769},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 61, offset: 14773},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 539, col: 63, offset: 14775},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 67, offset: 14779},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 539, col: 69, offset: 14781},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 539, col: 77, offset: 14789},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 539, col: 79, offset: 14791},
							expr: &litMatcher{
								pos:        position{line: 539, col: 79, offset: 14791},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 539, col: 84, offset: 14796},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 539, col: 86, offset: 14798},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 543, col: 1, offset: 14861},
			expr: &actionExpr{
				pos: position{line: 543, col: 10, offset: 14870},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 543, col: 10, offset: 14870},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 543, col: 10, offset: 14870},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 14, offset: 14874},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 543, col: 17, offset: 14877},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 543, col: 22, offset: 14882},
								expr: &ruleRefExpr{
									pos:  position{line: 543, col: 22, offset: 14882},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 543, col: 28, offset: 14888},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 543, col: 33, offset: 14893},
								expr: &seqExpr{
									pos: position{line: 543, col: 34, offset: 14894},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 543, col: 34, offset: 14894},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 543, col: 36, offset: 14896},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 40, offset: 14900},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 42, offset: 14902},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 49, offset: 14909},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 543, col: 51, offset: 14911},
							expr: &litMatcher{
								pos:        position{line: 543, col: 51, offset: 14911},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 56, offset: 14916},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 543, col: 59, offset: 14919},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgTerm",
			pos:  position{line: 552, col: 1, offset: 15314},
			expr: &actionExpr{
				pos: position{line: 552, col: 12, offset: 15325},
				run: (*parser).callonArgTerm1,
				expr: &labeledExpr{
					pos:   position{line: 552, col: 12, offset: 15325},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 552, col: 17, offset: 15330},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 552, col: 17, offset: 15330},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 552, col: 26, offset: 15339},
								name: "Var",
							},
							&ruleRefExpr{
								pos:  position{line: 552, col: 32, offset: 15345},
								name: "ArgObject",
							},
							&ruleRefExpr{
								pos:  position{line: 552, col: 44, offset: 15357},
								name: "ArgArray",
							},
						},
					},
				},
			},
		},
		{
			name: "ArgObject",
			pos:  position{line: 556, col: 1, offset: 15392},
			expr: &actionExpr{
				pos: position{line: 556, col: 14, offset: 15405},
				run: (*parser).callonArgObject1,
				expr: &seqExpr{
					pos: position{line: 556, col: 14, offset: 15405},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 556, col: 14, offset: 15405},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 556, col: 18, offset: 15409},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 556, col: 20, offset: 15411},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 556, col: 25, offset: 15416},
								expr: &seqExpr{
									pos: position{line: 556, col: 26, offset: 15417},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 556, col: 26, offset: 15417},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 33, offset: 15424},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 556, col: 35, offset: 15426},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 39, offset: 15430},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 41, offset: 15432},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 556, col: 51, offset: 15442},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 556, col: 56, offset: 15447},
								expr: &seqExpr{
									pos: position{line: 556, col: 58, offset: 15449},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 556, col: 58, offset: 15449},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 556, col: 60, offset: 15451},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 64, offset: 15455},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 66, offset: 15457},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 73, offset: 15464},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 556, col: 75, offset: 15466},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 79, offset: 15470},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 556, col: 81, offset: 15472},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 556, col: 92, offset: 15483},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 556, col: 94, offset: 15485},
							expr: &litMatcher{
								pos:        position{line: 556, col: 94, offset: 15485},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 556, col: 99, offset: 15490},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 556, col: 101, offset: 15492},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgKey",
			pos:  position{line: 560, col: 1, offset: 15555},
			expr: &ruleRefExpr{
				pos:  position{line: 560, col: 11, offset: 15565},
				name: "Scalar",
			},
		},
		{
			name: "ArgArray",
			pos:  position{line: 562, col: 1, offset: 15573},
			expr: &actionExpr{
				pos: position{line: 562, col: 13, offset: 15585},
				run: (*parser).callonArgArray1,
				expr: &seqExpr{
					pos: position{line: 562, col: 13, offset: 15585},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 562, col: 13, offset: 15585},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 562, col: 17, offset: 15589},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 562, col: 20, offset: 15592},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 562, col: 25, offset: 15597},
								expr: &ruleRefExpr{
									pos:  position{line: 562, col: 25, offset: 15597},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 562, col: 34, offset: 15606},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 562, col: 39, offset: 15611},
								expr: &seqExpr{
									pos: position{line: 562, col: 40, offset: 15612},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 562, col: 40, offset: 15612},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 562, col: 42, offset: 15614},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 562, col: 46, offset: 15618},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 562, col: 48, offset: 15620},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 562, col: 58, offset: 15630},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 562, col: 60, offset: 15632},
							expr: &litMatcher{
								pos:        position{line: 562, col: 60, offset: 15632},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 562, col: 65, offset: 15637},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 562, col: 68, offset: 15640},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 566, col: 1, offset: 15702},
			expr: &choiceExpr{
				pos: position{line: 566, col: 8, offset: 15709},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 566, col: 8, offset: 15709},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 566, col: 19, offset: 15720},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 568, col: 1, offset: 15733},
			expr: &actionExpr{
				pos: position{line: 568, col: 13, offset: 15745},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 568, col: 13, offset: 15745},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 568, col: 13, offset: 15745},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 568, col: 20, offset: 15752},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 568, col: 22, offset: 15754},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 574, col: 1, offset: 15842},
			expr: &actionExpr{
				pos: position{line: 574, col: 16, offset: 15857},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 574, col: 16, offset: 15857},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 574, col: 16, offset: 15857},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 574, col: 20, offset: 15861},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 574, col: 22, offset: 15863},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 574, col: 27, offset: 15868},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 574, col: 32, offset: 15873},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 574, col: 37, offset: 15878},
								expr: &seqExpr{
									pos: position{line: 574, col: 38, offset: 15879},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 574, col: 38, offset: 15879},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 574, col: 40, offset: 15881},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 574, col: 44, offset: 15885},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 574, col: 46, offset: 15887},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 574, col: 53, offset: 15894},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 574, col: 55, offset: 15896},
							expr: &litMatcher{
								pos:        position{line: 574, col: 55, offset: 15896},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 574, col: 60, offset: 15901},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 574, col: 62, offset: 15903},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 591, col: 1, offset: 16308},
			expr: &actionExpr{
				pos: position{line: 591, col: 8, offset: 16315},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 591, col: 8, offset: 16315},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 591, col: 8, offset: 16315},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 591, col: 13, offset: 16320},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 591, col: 17, offset: 16324},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 591, col: 22, offset: 16329},
								expr: &choiceExpr{
									pos: position{line: 591, col: 24, offset: 16331},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 591, col: 24, offset: 16331},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 591, col: 33, offset: 16340},
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
			pos:  position{line: 604, col: 1, offset: 16579},
			expr: &actionExpr{
				pos: position{line: 604, col: 11, offset: 16589},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 604, col: 11, offset: 16589},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 604, col: 11, offset: 16589},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 604, col: 15, offset: 16593},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 604, col: 19, offset: 16597},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 611, col: 1, offset: 16816},
			expr: &actionExpr{
				pos: position{line: 611, col: 15, offset: 16830},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 611, col: 15, offset: 16830},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 611, col: 15, offset: 16830},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 611, col: 19, offset: 16834},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 611, col: 24, offset: 16839},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 611, col: 24, offset: 16839},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 611, col: 30, offset: 16845},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 611, col: 39, offset: 16854},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 611, col: 44, offset: 16859},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 615, col: 1, offset: 16888},
			expr: &actionExpr{
				pos: position{line: 615, col: 8, offset: 16895},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 615, col: 8, offset: 16895},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 615, col: 12, offset: 16899},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 620, col: 1, offset: 17021},
			expr: &seqExpr{
				pos: position{line: 620, col: 15, offset: 17035},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 620, col: 15, offset: 17035},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 620, col: 19, offset: 17039},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 620, col: 32, offset: 17052},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 624, col: 1, offset: 17117},
			expr: &actionExpr{
				pos: position{line: 624, col: 17, offset: 17133},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 624, col: 17, offset: 17133},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 624, col: 17, offset: 17133},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 624, col: 29, offset: 17145},
							expr: &choiceExpr{
								pos: position{line: 624, col: 30, offset: 17146},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 624, col: 30, offset: 17146},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 624, col: 44, offset: 17160},
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
			pos:  position{line: 631, col: 1, offset: 17303},
			expr: &actionExpr{
				pos: position{line: 631, col: 11, offset: 17313},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 631, col: 11, offset: 17313},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 631, col: 11, offset: 17313},
							expr: &litMatcher{
								pos:        position{line: 631, col: 11, offset: 17313},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 631, col: 18, offset: 17320},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 631, col: 18, offset: 17320},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 631, col: 26, offset: 17328},
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
			pos:  position{line: 644, col: 1, offset: 17719},
			expr: &choiceExpr{
				pos: position{line: 644, col: 10, offset: 17728},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 644, col: 10, offset: 17728},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 644, col: 26, offset: 17744},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 646, col: 1, offset: 17756},
			expr: &seqExpr{
				pos: position{line: 646, col: 18, offset: 17773},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 646, col: 20, offset: 17775},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 646, col: 20, offset: 17775},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 646, col: 33, offset: 17788},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 646, col: 43, offset: 17798},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 648, col: 1, offset: 17808},
			expr: &seqExpr{
				pos: position{line: 648, col: 15, offset: 17822},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 648, col: 15, offset: 17822},
						expr: &ruleRefExpr{
							pos:  position{line: 648, col: 15, offset: 17822},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 648, col: 24, offset: 17831},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 650, col: 1, offset: 17841},
			expr: &seqExpr{
				pos: position{line: 650, col: 13, offset: 17853},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 650, col: 13, offset: 17853},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 650, col: 17, offset: 17857},
						expr: &ruleRefExpr{
							pos:  position{line: 650, col: 17, offset: 17857},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 652, col: 1, offset: 17872},
			expr: &seqExpr{
				pos: position{line: 652, col: 13, offset: 17884},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 652, col: 13, offset: 17884},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 652, col: 18, offset: 17889},
						expr: &charClassMatcher{
							pos:        position{line: 652, col: 18, offset: 17889},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 652, col: 24, offset: 17895},
						expr: &ruleRefExpr{
							pos:  position{line: 652, col: 24, offset: 17895},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 654, col: 1, offset: 17910},
			expr: &choiceExpr{
				pos: position{line: 654, col: 12, offset: 17921},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 654, col: 12, offset: 17921},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 654, col: 20, offset: 17929},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 654, col: 20, offset: 17929},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 654, col: 40, offset: 17949},
								expr: &ruleRefExpr{
									pos:  position{line: 654, col: 40, offset: 17949},
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
			pos:  position{line: 656, col: 1, offset: 17966},
			expr: &actionExpr{
				pos: position{line: 656, col: 11, offset: 17976},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 656, col: 11, offset: 17976},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 656, col: 11, offset: 17976},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 656, col: 15, offset: 17980},
							expr: &ruleRefExpr{
								pos:  position{line: 656, col: 15, offset: 17980},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 656, col: 21, offset: 17986},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 664, col: 1, offset: 18141},
			expr: &choiceExpr{
				pos: position{line: 664, col: 9, offset: 18149},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 664, col: 9, offset: 18149},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 664, col: 9, offset: 18149},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 668, col: 5, offset: 18249},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 668, col: 5, offset: 18249},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 674, col: 1, offset: 18350},
			expr: &actionExpr{
				pos: position{line: 674, col: 9, offset: 18358},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 674, col: 9, offset: 18358},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 680, col: 1, offset: 18453},
			expr: &charClassMatcher{
				pos:        position{line: 680, col: 16, offset: 18468},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 682, col: 1, offset: 18479},
			expr: &choiceExpr{
				pos: position{line: 682, col: 9, offset: 18487},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 682, col: 11, offset: 18489},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 682, col: 11, offset: 18489},
								expr: &ruleRefExpr{
									pos:  position{line: 682, col: 12, offset: 18490},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 682, col: 24, offset: 18502,
							},
						},
					},
					&seqExpr{
						pos: position{line: 682, col: 32, offset: 18510},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 682, col: 32, offset: 18510},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 682, col: 37, offset: 18515},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 684, col: 1, offset: 18533},
			expr: &charClassMatcher{
				pos:        position{line: 684, col: 16, offset: 18548},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 686, col: 1, offset: 18564},
			expr: &choiceExpr{
				pos: position{line: 686, col: 19, offset: 18582},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 686, col: 19, offset: 18582},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 686, col: 38, offset: 18601},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 688, col: 1, offset: 18616},
			expr: &charClassMatcher{
				pos:        position{line: 688, col: 21, offset: 18636},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 690, col: 1, offset: 18658},
			expr: &seqExpr{
				pos: position{line: 690, col: 18, offset: 18675},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 690, col: 18, offset: 18675},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 690, col: 22, offset: 18679},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 690, col: 31, offset: 18688},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 690, col: 40, offset: 18697},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 690, col: 49, offset: 18706},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 692, col: 1, offset: 18716},
			expr: &charClassMatcher{
				pos:        position{line: 692, col: 17, offset: 18732},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 694, col: 1, offset: 18739},
			expr: &charClassMatcher{
				pos:        position{line: 694, col: 24, offset: 18762},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 696, col: 1, offset: 18769},
			expr: &charClassMatcher{
				pos:        position{line: 696, col: 13, offset: 18781},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 698, col: 1, offset: 18794},
			expr: &oneOrMoreExpr{
				pos: position{line: 698, col: 20, offset: 18813},
				expr: &charClassMatcher{
					pos:        position{line: 698, col: 20, offset: 18813},
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
			pos:         position{line: 700, col: 1, offset: 18825},
			expr: &zeroOrMoreExpr{
				pos: position{line: 700, col: 19, offset: 18843},
				expr: &choiceExpr{
					pos: position{line: 700, col: 21, offset: 18845},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 700, col: 21, offset: 18845},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 700, col: 33, offset: 18857},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 702, col: 1, offset: 18869},
			expr: &actionExpr{
				pos: position{line: 702, col: 12, offset: 18880},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 702, col: 12, offset: 18880},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 702, col: 12, offset: 18880},
							expr: &charClassMatcher{
								pos:        position{line: 702, col: 12, offset: 18880},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 702, col: 19, offset: 18887},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 702, col: 23, offset: 18891},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 702, col: 28, offset: 18896},
								expr: &charClassMatcher{
									pos:        position{line: 702, col: 28, offset: 18896},
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
			pos:  position{line: 713, col: 1, offset: 19172},
			expr: &notExpr{
				pos: position{line: 713, col: 8, offset: 19179},
				expr: &anyMatcher{
					line: 713, col: 9, offset: 19180,
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
		Head: head.(*FuncHead),
		Body: b.(Body),
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

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
														&ruleRefExpr{
															pos:  position{line: 96, col: 36, offset: 2473},
															name: "ws",
														},
														&ruleRefExpr{
															pos:  position{line: 96, col: 39, offset: 2476},
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
							pos:  position{line: 96, col: 48, offset: 2485},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 96, col: 50, offset: 2487},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 114, col: 1, offset: 2861},
			expr: &actionExpr{
				pos: position{line: 114, col: 9, offset: 2869},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 114, col: 9, offset: 2869},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 114, col: 14, offset: 2874},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 114, col: 14, offset: 2874},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 24, offset: 2884},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 33, offset: 2893},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 41, offset: 2901},
								name: "UserFunc",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 52, offset: 2912},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 114, col: 59, offset: 2919},
								name: "Comment",
							},
						},
					},
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 118, col: 1, offset: 2953},
			expr: &actionExpr{
				pos: position{line: 118, col: 12, offset: 2964},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 118, col: 12, offset: 2964},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 118, col: 12, offset: 2964},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 118, col: 22, offset: 2974},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 118, col: 25, offset: 2977},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 118, col: 30, offset: 2982},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 118, col: 30, offset: 2982},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 118, col: 36, offset: 2988},
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
			pos:  position{line: 152, col: 1, offset: 4304},
			expr: &actionExpr{
				pos: position{line: 152, col: 11, offset: 4314},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 152, col: 11, offset: 4314},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 152, col: 11, offset: 4314},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 152, col: 20, offset: 4323},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 152, col: 23, offset: 4326},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 152, col: 29, offset: 4332},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 152, col: 29, offset: 4332},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 152, col: 35, offset: 4338},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 152, col: 40, offset: 4343},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 152, col: 46, offset: 4349},
								expr: &seqExpr{
									pos: position{line: 152, col: 47, offset: 4350},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 152, col: 47, offset: 4350},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 152, col: 50, offset: 4353},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 152, col: 55, offset: 4358},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 152, col: 58, offset: 4361},
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
			pos:  position{line: 168, col: 1, offset: 4811},
			expr: &choiceExpr{
				pos: position{line: 168, col: 10, offset: 4820},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 168, col: 10, offset: 4820},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 168, col: 25, offset: 4835},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 170, col: 1, offset: 4848},
			expr: &actionExpr{
				pos: position{line: 170, col: 17, offset: 4864},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 170, col: 17, offset: 4864},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 170, col: 17, offset: 4864},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 170, col: 27, offset: 4874},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 170, col: 30, offset: 4877},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 170, col: 35, offset: 4882},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 170, col: 39, offset: 4886},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 170, col: 41, offset: 4888},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 170, col: 45, offset: 4892},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 170, col: 47, offset: 4894},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 170, col: 53, offset: 4900},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 213, col: 1, offset: 5869},
			expr: &actionExpr{
				pos: position{line: 213, col: 16, offset: 5884},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 213, col: 16, offset: 5884},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 213, col: 16, offset: 5884},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 213, col: 21, offset: 5889},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 213, col: 30, offset: 5898},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 213, col: 32, offset: 5900},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 213, col: 35, offset: 5903},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 213, col: 35, offset: 5903},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 213, col: 61, offset: 5929},
										expr: &seqExpr{
											pos: position{line: 213, col: 63, offset: 5931},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 213, col: 63, offset: 5931},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 213, col: 65, offset: 5933},
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
			pos:  position{line: 268, col: 1, offset: 7229},
			expr: &actionExpr{
				pos: position{line: 268, col: 13, offset: 7241},
				run: (*parser).callonUserFunc1,
				expr: &seqExpr{
					pos: position{line: 268, col: 13, offset: 7241},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 268, col: 13, offset: 7241},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 268, col: 18, offset: 7246},
								name: "FuncHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 268, col: 27, offset: 7255},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 268, col: 29, offset: 7257},
							label: "b",
							expr: &ruleRefExpr{
								pos:  position{line: 268, col: 31, offset: 7259},
								name: "NonEmptyBraceEnclosedBody",
							},
						},
					},
				},
			},
		},
		{
			name: "FuncHead",
			pos:  position{line: 283, col: 1, offset: 7478},
			expr: &actionExpr{
				pos: position{line: 283, col: 13, offset: 7490},
				run: (*parser).callonFuncHead1,
				expr: &seqExpr{
					pos: position{line: 283, col: 13, offset: 7490},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 283, col: 13, offset: 7490},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 283, col: 18, offset: 7495},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 283, col: 22, offset: 7499},
							label: "args",
							expr: &ruleRefExpr{
								pos:  position{line: 283, col: 27, offset: 7504},
								name: "FuncArgs",
							},
						},
						&labeledExpr{
							pos:   position{line: 283, col: 36, offset: 7513},
							label: "output",
							expr: &zeroOrOneExpr{
								pos: position{line: 283, col: 43, offset: 7520},
								expr: &seqExpr{
									pos: position{line: 283, col: 45, offset: 7522},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 283, col: 45, offset: 7522},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 283, col: 47, offset: 7524},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 283, col: 51, offset: 7528},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 283, col: 53, offset: 7530},
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
			pos:  position{line: 299, col: 1, offset: 7834},
			expr: &actionExpr{
				pos: position{line: 299, col: 13, offset: 7846},
				run: (*parser).callonFuncArgs1,
				expr: &seqExpr{
					pos: position{line: 299, col: 13, offset: 7846},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 299, col: 13, offset: 7846},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 299, col: 15, offset: 7848},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 299, col: 20, offset: 7853},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 299, col: 23, offset: 7856},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 299, col: 28, offset: 7861},
								expr: &ruleRefExpr{
									pos:  position{line: 299, col: 28, offset: 7861},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 299, col: 37, offset: 7870},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 299, col: 42, offset: 7875},
								expr: &seqExpr{
									pos: position{line: 299, col: 43, offset: 7876},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 299, col: 43, offset: 7876},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 299, col: 45, offset: 7878},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 299, col: 49, offset: 7882},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 299, col: 51, offset: 7884},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 299, col: 61, offset: 7894},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 299, col: 63, offset: 7896},
							val:        ")",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 299, col: 67, offset: 7900},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "RuleHead",
			pos:  position{line: 320, col: 1, offset: 8320},
			expr: &actionExpr{
				pos: position{line: 320, col: 13, offset: 8332},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 320, col: 13, offset: 8332},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 320, col: 13, offset: 8332},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 320, col: 18, offset: 8337},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 320, col: 22, offset: 8341},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 320, col: 26, offset: 8345},
								expr: &seqExpr{
									pos: position{line: 320, col: 28, offset: 8347},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 320, col: 28, offset: 8347},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 320, col: 30, offset: 8349},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 34, offset: 8353},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 36, offset: 8355},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 41, offset: 8360},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 320, col: 43, offset: 8362},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 47, offset: 8366},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 320, col: 52, offset: 8371},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 320, col: 58, offset: 8377},
								expr: &seqExpr{
									pos: position{line: 320, col: 60, offset: 8379},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 320, col: 60, offset: 8379},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 320, col: 62, offset: 8381},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 66, offset: 8385},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 320, col: 68, offset: 8387},
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
			pos:  position{line: 355, col: 1, offset: 9375},
			expr: &actionExpr{
				pos: position{line: 355, col: 9, offset: 9383},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 355, col: 9, offset: 9383},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 355, col: 9, offset: 9383},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 355, col: 16, offset: 9390},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 355, col: 20, offset: 9394},
								expr: &seqExpr{
									pos: position{line: 355, col: 22, offset: 9396},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 355, col: 22, offset: 9396},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 355, col: 24, offset: 9398},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 355, col: 28, offset: 9402},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 355, col: 30, offset: 9404},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 355, col: 38, offset: 9412},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 355, col: 42, offset: 9416},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 355, col: 42, offset: 9416},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 355, col: 44, offset: 9418},
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
			pos:  position{line: 370, col: 1, offset: 9770},
			expr: &actionExpr{
				pos: position{line: 370, col: 12, offset: 9781},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 370, col: 12, offset: 9781},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 370, col: 14, offset: 9783},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 374, col: 1, offset: 9879},
			expr: &choiceExpr{
				pos: position{line: 374, col: 12, offset: 9890},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 374, col: 12, offset: 9890},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 374, col: 19, offset: 9897},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 376, col: 1, offset: 9906},
			expr: &choiceExpr{
				pos: position{line: 376, col: 9, offset: 9914},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 376, col: 9, offset: 9914},
						name: "NonWhitespaceBody",
					},
					&ruleRefExpr{
						pos:  position{line: 376, col: 29, offset: 9934},
						name: "BraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 378, col: 1, offset: 9953},
			expr: &actionExpr{
				pos: position{line: 378, col: 30, offset: 9982},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 378, col: 30, offset: 9982},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 378, col: 30, offset: 9982},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 378, col: 34, offset: 9986},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 378, col: 36, offset: 9988},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 378, col: 40, offset: 9992},
								expr: &ruleRefExpr{
									pos:  position{line: 378, col: 40, offset: 9992},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 378, col: 56, offset: 10008},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 378, col: 58, offset: 10010},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 385, col: 1, offset: 10105},
			expr: &actionExpr{
				pos: position{line: 385, col: 22, offset: 10126},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 385, col: 22, offset: 10126},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 385, col: 22, offset: 10126},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 385, col: 26, offset: 10130},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 385, col: 28, offset: 10132},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 385, col: 32, offset: 10136},
								expr: &ruleRefExpr{
									pos:  position{line: 385, col: 32, offset: 10136},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 385, col: 48, offset: 10152},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 385, col: 50, offset: 10154},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 399, col: 1, offset: 10506},
			expr: &actionExpr{
				pos: position{line: 399, col: 19, offset: 10524},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 399, col: 19, offset: 10524},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 399, col: 19, offset: 10524},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 399, col: 24, offset: 10529},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 399, col: 32, offset: 10537},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 399, col: 37, offset: 10542},
								expr: &seqExpr{
									pos: position{line: 399, col: 38, offset: 10543},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 399, col: 38, offset: 10543},
											expr: &charClassMatcher{
												pos:        position{line: 399, col: 38, offset: 10543},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 399, col: 46, offset: 10551},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 399, col: 47, offset: 10552},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 399, col: 47, offset: 10552},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 399, col: 51, offset: 10556},
															expr: &ruleRefExpr{
																pos:  position{line: 399, col: 51, offset: 10556},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 399, col: 64, offset: 10569},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 399, col: 64, offset: 10569},
															expr: &ruleRefExpr{
																pos:  position{line: 399, col: 64, offset: 10569},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 399, col: 73, offset: 10578},
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
											pos:  position{line: 399, col: 82, offset: 10587},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 399, col: 84, offset: 10589},
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
			pos:  position{line: 405, col: 1, offset: 10778},
			expr: &actionExpr{
				pos: position{line: 405, col: 22, offset: 10799},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 405, col: 22, offset: 10799},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 405, col: 22, offset: 10799},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 405, col: 27, offset: 10804},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 405, col: 35, offset: 10812},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 405, col: 40, offset: 10817},
								expr: &seqExpr{
									pos: position{line: 405, col: 42, offset: 10819},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 405, col: 42, offset: 10819},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 405, col: 44, offset: 10821},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 405, col: 48, offset: 10825},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 405, col: 50, offset: 10827},
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
			name: "Literal",
			pos:  position{line: 409, col: 1, offset: 10902},
			expr: &actionExpr{
				pos: position{line: 409, col: 12, offset: 10913},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 409, col: 12, offset: 10913},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 409, col: 12, offset: 10913},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 409, col: 16, offset: 10917},
								expr: &seqExpr{
									pos: position{line: 409, col: 18, offset: 10919},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 409, col: 18, offset: 10919},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 24, offset: 10925},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 409, col: 30, offset: 10931},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 409, col: 34, offset: 10935},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 409, col: 39, offset: 10940},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 409, col: 44, offset: 10945},
								expr: &seqExpr{
									pos: position{line: 409, col: 46, offset: 10947},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 409, col: 46, offset: 10947},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 49, offset: 10950},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 409, col: 54, offset: 10955},
											expr: &seqExpr{
												pos: position{line: 409, col: 55, offset: 10956},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 409, col: 55, offset: 10956},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 409, col: 58, offset: 10959},
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
			pos:  position{line: 432, col: 1, offset: 11531},
			expr: &actionExpr{
				pos: position{line: 432, col: 9, offset: 11539},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 432, col: 9, offset: 11539},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 432, col: 9, offset: 11539},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 16, offset: 11546},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 432, col: 19, offset: 11549},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 432, col: 26, offset: 11556},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 31, offset: 11561},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 432, col: 34, offset: 11564},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 39, offset: 11569},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 432, col: 42, offset: 11572},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 432, col: 48, offset: 11578},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 443, col: 1, offset: 11827},
			expr: &choiceExpr{
				pos: position{line: 443, col: 9, offset: 11835},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 443, col: 10, offset: 11836},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 443, col: 10, offset: 11836},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 443, col: 27, offset: 11853},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 443, col: 52, offset: 11878},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 443, col: 64, offset: 11890},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 443, col: 77, offset: 11903},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 445, col: 1, offset: 11909},
			expr: &actionExpr{
				pos: position{line: 445, col: 19, offset: 11927},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 445, col: 19, offset: 11927},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 445, col: 19, offset: 11927},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 26, offset: 11934},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 31, offset: 11939},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 445, col: 33, offset: 11941},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 37, offset: 11945},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 445, col: 39, offset: 11947},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 44, offset: 11952},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 49, offset: 11957},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 445, col: 51, offset: 11959},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 54, offset: 11962},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 67, offset: 11975},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 445, col: 69, offset: 11977},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 75, offset: 11983},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 449, col: 1, offset: 12074},
			expr: &actionExpr{
				pos: position{line: 449, col: 26, offset: 12099},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 449, col: 26, offset: 12099},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 449, col: 26, offset: 12099},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 31, offset: 12104},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 36, offset: 12109},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 38, offset: 12111},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 41, offset: 12114},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 54, offset: 12127},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 56, offset: 12129},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 62, offset: 12135},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 67, offset: 12140},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 449, col: 69, offset: 12142},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 73, offset: 12146},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 75, offset: 12148},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 82, offset: 12155},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 453, col: 1, offset: 12246},
			expr: &actionExpr{
				pos: position{line: 453, col: 17, offset: 12262},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 453, col: 17, offset: 12262},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 453, col: 22, offset: 12267},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 453, col: 22, offset: 12267},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 28, offset: 12273},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 34, offset: 12279},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 40, offset: 12285},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 46, offset: 12291},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 52, offset: 12297},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 453, col: 58, offset: 12303},
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
			pos:  position{line: 465, col: 1, offset: 12550},
			expr: &actionExpr{
				pos: position{line: 465, col: 14, offset: 12563},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 465, col: 14, offset: 12563},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 465, col: 14, offset: 12563},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 465, col: 19, offset: 12568},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 24, offset: 12573},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 465, col: 26, offset: 12575},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 465, col: 29, offset: 12578},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 37, offset: 12586},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 465, col: 39, offset: 12588},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 465, col: 45, offset: 12594},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 469, col: 1, offset: 12669},
			expr: &actionExpr{
				pos: position{line: 469, col: 12, offset: 12680},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 469, col: 12, offset: 12680},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 469, col: 17, offset: 12685},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 469, col: 17, offset: 12685},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 469, col: 23, offset: 12691},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 469, col: 30, offset: 12698},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 469, col: 37, offset: 12705},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 469, col: 44, offset: 12712},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 469, col: 50, offset: 12718},
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
			pos:  position{line: 481, col: 1, offset: 12965},
			expr: &choiceExpr{
				pos: position{line: 481, col: 15, offset: 12979},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 481, col: 15, offset: 12979},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 481, col: 26, offset: 12990},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 483, col: 1, offset: 12999},
			expr: &actionExpr{
				pos: position{line: 483, col: 12, offset: 13010},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 483, col: 12, offset: 13010},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 483, col: 12, offset: 13010},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 483, col: 17, offset: 13015},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 483, col: 29, offset: 13027},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 483, col: 33, offset: 13031},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 483, col: 35, offset: 13033},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 483, col: 40, offset: 13038},
								expr: &ruleRefExpr{
									pos:  position{line: 483, col: 40, offset: 13038},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 483, col: 46, offset: 13044},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 483, col: 51, offset: 13049},
								expr: &seqExpr{
									pos: position{line: 483, col: 53, offset: 13051},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 483, col: 53, offset: 13051},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 483, col: 55, offset: 13053},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 483, col: 59, offset: 13057},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 483, col: 61, offset: 13059},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 483, col: 69, offset: 13067},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 483, col: 72, offset: 13070},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 499, col: 1, offset: 13474},
			expr: &actionExpr{
				pos: position{line: 499, col: 16, offset: 13489},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 499, col: 16, offset: 13489},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 499, col: 16, offset: 13489},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 499, col: 21, offset: 13494},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 499, col: 25, offset: 13498},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 499, col: 30, offset: 13503},
								expr: &seqExpr{
									pos: position{line: 499, col: 32, offset: 13505},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 499, col: 32, offset: 13505},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 499, col: 36, offset: 13509},
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
			pos:  position{line: 513, col: 1, offset: 13914},
			expr: &actionExpr{
				pos: position{line: 513, col: 9, offset: 13922},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 513, col: 9, offset: 13922},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 513, col: 15, offset: 13928},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 513, col: 15, offset: 13928},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 513, col: 31, offset: 13944},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 513, col: 43, offset: 13956},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 513, col: 52, offset: 13965},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 513, col: 58, offset: 13971},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 517, col: 1, offset: 14002},
			expr: &choiceExpr{
				pos: position{line: 517, col: 18, offset: 14019},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 517, col: 18, offset: 14019},
						name: "ArrayComprehension",
					},
					&ruleRefExpr{
						pos:  position{line: 517, col: 39, offset: 14040},
						name: "ObjectComprehension",
					},
					&ruleRefExpr{
						pos:  position{line: 517, col: 61, offset: 14062},
						name: "SetComprehension",
					},
				},
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 519, col: 1, offset: 14080},
			expr: &actionExpr{
				pos: position{line: 519, col: 23, offset: 14102},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 519, col: 23, offset: 14102},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 519, col: 23, offset: 14102},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 519, col: 27, offset: 14106},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 519, col: 29, offset: 14108},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 519, col: 34, offset: 14113},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 519, col: 39, offset: 14118},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 519, col: 41, offset: 14120},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 519, col: 45, offset: 14124},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 519, col: 47, offset: 14126},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 519, col: 52, offset: 14131},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 519, col: 67, offset: 14146},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 519, col: 69, offset: 14148},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ObjectComprehension",
			pos:  position{line: 525, col: 1, offset: 14273},
			expr: &actionExpr{
				pos: position{line: 525, col: 24, offset: 14296},
				run: (*parser).callonObjectComprehension1,
				expr: &seqExpr{
					pos: position{line: 525, col: 24, offset: 14296},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 525, col: 24, offset: 14296},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 28, offset: 14300},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 525, col: 30, offset: 14302},
							label: "key",
							expr: &ruleRefExpr{
								pos:  position{line: 525, col: 34, offset: 14306},
								name: "Key",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 38, offset: 14310},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 525, col: 40, offset: 14312},
							val:        ":",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 44, offset: 14316},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 525, col: 46, offset: 14318},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 525, col: 52, offset: 14324},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 58, offset: 14330},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 525, col: 60, offset: 14332},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 64, offset: 14336},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 525, col: 66, offset: 14338},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 525, col: 71, offset: 14343},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 525, col: 86, offset: 14358},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 525, col: 88, offset: 14360},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetComprehension",
			pos:  position{line: 531, col: 1, offset: 14500},
			expr: &actionExpr{
				pos: position{line: 531, col: 21, offset: 14520},
				run: (*parser).callonSetComprehension1,
				expr: &seqExpr{
					pos: position{line: 531, col: 21, offset: 14520},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 531, col: 21, offset: 14520},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 531, col: 25, offset: 14524},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 531, col: 27, offset: 14526},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 531, col: 32, offset: 14531},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 531, col: 37, offset: 14536},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 531, col: 39, offset: 14538},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 531, col: 43, offset: 14542},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 531, col: 45, offset: 14544},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 531, col: 50, offset: 14549},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 531, col: 65, offset: 14564},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 531, col: 67, offset: 14566},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 537, col: 1, offset: 14689},
			expr: &choiceExpr{
				pos: position{line: 537, col: 14, offset: 14702},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 537, col: 14, offset: 14702},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 537, col: 23, offset: 14711},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 537, col: 31, offset: 14719},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 539, col: 1, offset: 14724},
			expr: &choiceExpr{
				pos: position{line: 539, col: 11, offset: 14734},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 539, col: 11, offset: 14734},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 539, col: 20, offset: 14743},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 539, col: 29, offset: 14752},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 539, col: 36, offset: 14759},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 541, col: 1, offset: 14765},
			expr: &choiceExpr{
				pos: position{line: 541, col: 8, offset: 14772},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 541, col: 8, offset: 14772},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 541, col: 17, offset: 14781},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 541, col: 23, offset: 14787},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 543, col: 1, offset: 14792},
			expr: &actionExpr{
				pos: position{line: 543, col: 11, offset: 14802},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 543, col: 11, offset: 14802},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 543, col: 11, offset: 14802},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 15, offset: 14806},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 543, col: 17, offset: 14808},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 543, col: 22, offset: 14813},
								expr: &seqExpr{
									pos: position{line: 543, col: 23, offset: 14814},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 543, col: 23, offset: 14814},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 27, offset: 14818},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 543, col: 29, offset: 14820},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 33, offset: 14824},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 35, offset: 14826},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 543, col: 42, offset: 14833},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 543, col: 47, offset: 14838},
								expr: &seqExpr{
									pos: position{line: 543, col: 49, offset: 14840},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 543, col: 49, offset: 14840},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 543, col: 51, offset: 14842},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 55, offset: 14846},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 57, offset: 14848},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 61, offset: 14852},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 543, col: 63, offset: 14854},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 67, offset: 14858},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 543, col: 69, offset: 14860},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 77, offset: 14868},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 543, col: 79, offset: 14870},
							expr: &litMatcher{
								pos:        position{line: 543, col: 79, offset: 14870},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 543, col: 84, offset: 14875},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 543, col: 86, offset: 14877},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 547, col: 1, offset: 14940},
			expr: &actionExpr{
				pos: position{line: 547, col: 10, offset: 14949},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 547, col: 10, offset: 14949},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 547, col: 10, offset: 14949},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 547, col: 14, offset: 14953},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 547, col: 17, offset: 14956},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 547, col: 22, offset: 14961},
								expr: &ruleRefExpr{
									pos:  position{line: 547, col: 22, offset: 14961},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 547, col: 28, offset: 14967},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 547, col: 33, offset: 14972},
								expr: &seqExpr{
									pos: position{line: 547, col: 34, offset: 14973},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 547, col: 34, offset: 14973},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 547, col: 36, offset: 14975},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 547, col: 40, offset: 14979},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 547, col: 42, offset: 14981},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 547, col: 49, offset: 14988},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 547, col: 51, offset: 14990},
							expr: &litMatcher{
								pos:        position{line: 547, col: 51, offset: 14990},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 547, col: 56, offset: 14995},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 547, col: 59, offset: 14998},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgTerm",
			pos:  position{line: 556, col: 1, offset: 15393},
			expr: &actionExpr{
				pos: position{line: 556, col: 12, offset: 15404},
				run: (*parser).callonArgTerm1,
				expr: &labeledExpr{
					pos:   position{line: 556, col: 12, offset: 15404},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 556, col: 17, offset: 15409},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 556, col: 17, offset: 15409},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 556, col: 26, offset: 15418},
								name: "Var",
							},
							&ruleRefExpr{
								pos:  position{line: 556, col: 32, offset: 15424},
								name: "ArgObject",
							},
							&ruleRefExpr{
								pos:  position{line: 556, col: 44, offset: 15436},
								name: "ArgArray",
							},
						},
					},
				},
			},
		},
		{
			name: "ArgObject",
			pos:  position{line: 560, col: 1, offset: 15471},
			expr: &actionExpr{
				pos: position{line: 560, col: 14, offset: 15484},
				run: (*parser).callonArgObject1,
				expr: &seqExpr{
					pos: position{line: 560, col: 14, offset: 15484},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 560, col: 14, offset: 15484},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 560, col: 18, offset: 15488},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 560, col: 20, offset: 15490},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 560, col: 25, offset: 15495},
								expr: &seqExpr{
									pos: position{line: 560, col: 26, offset: 15496},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 560, col: 26, offset: 15496},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 33, offset: 15503},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 560, col: 35, offset: 15505},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 39, offset: 15509},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 41, offset: 15511},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 560, col: 51, offset: 15521},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 560, col: 56, offset: 15526},
								expr: &seqExpr{
									pos: position{line: 560, col: 58, offset: 15528},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 560, col: 58, offset: 15528},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 560, col: 60, offset: 15530},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 64, offset: 15534},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 66, offset: 15536},
											name: "ArgKey",
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 73, offset: 15543},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 560, col: 75, offset: 15545},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 79, offset: 15549},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 560, col: 81, offset: 15551},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 560, col: 92, offset: 15562},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 560, col: 94, offset: 15564},
							expr: &litMatcher{
								pos:        position{line: 560, col: 94, offset: 15564},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 560, col: 99, offset: 15569},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 560, col: 101, offset: 15571},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ArgKey",
			pos:  position{line: 564, col: 1, offset: 15634},
			expr: &ruleRefExpr{
				pos:  position{line: 564, col: 11, offset: 15644},
				name: "Scalar",
			},
		},
		{
			name: "ArgArray",
			pos:  position{line: 566, col: 1, offset: 15652},
			expr: &actionExpr{
				pos: position{line: 566, col: 13, offset: 15664},
				run: (*parser).callonArgArray1,
				expr: &seqExpr{
					pos: position{line: 566, col: 13, offset: 15664},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 566, col: 13, offset: 15664},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 566, col: 17, offset: 15668},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 566, col: 20, offset: 15671},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 566, col: 25, offset: 15676},
								expr: &ruleRefExpr{
									pos:  position{line: 566, col: 25, offset: 15676},
									name: "ArgTerm",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 566, col: 34, offset: 15685},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 566, col: 39, offset: 15690},
								expr: &seqExpr{
									pos: position{line: 566, col: 40, offset: 15691},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 566, col: 40, offset: 15691},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 566, col: 42, offset: 15693},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 566, col: 46, offset: 15697},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 566, col: 48, offset: 15699},
											name: "ArgTerm",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 566, col: 58, offset: 15709},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 566, col: 60, offset: 15711},
							expr: &litMatcher{
								pos:        position{line: 566, col: 60, offset: 15711},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 566, col: 65, offset: 15716},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 566, col: 68, offset: 15719},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 570, col: 1, offset: 15781},
			expr: &choiceExpr{
				pos: position{line: 570, col: 8, offset: 15788},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 570, col: 8, offset: 15788},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 570, col: 19, offset: 15799},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 572, col: 1, offset: 15812},
			expr: &actionExpr{
				pos: position{line: 572, col: 13, offset: 15824},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 572, col: 13, offset: 15824},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 572, col: 13, offset: 15824},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 572, col: 20, offset: 15831},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 572, col: 22, offset: 15833},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 578, col: 1, offset: 15921},
			expr: &actionExpr{
				pos: position{line: 578, col: 16, offset: 15936},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 578, col: 16, offset: 15936},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 578, col: 16, offset: 15936},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 578, col: 20, offset: 15940},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 578, col: 22, offset: 15942},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 578, col: 27, offset: 15947},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 578, col: 32, offset: 15952},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 578, col: 37, offset: 15957},
								expr: &seqExpr{
									pos: position{line: 578, col: 38, offset: 15958},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 578, col: 38, offset: 15958},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 578, col: 40, offset: 15960},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 578, col: 44, offset: 15964},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 578, col: 46, offset: 15966},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 578, col: 53, offset: 15973},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 578, col: 55, offset: 15975},
							expr: &litMatcher{
								pos:        position{line: 578, col: 55, offset: 15975},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 578, col: 60, offset: 15980},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 578, col: 62, offset: 15982},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 595, col: 1, offset: 16387},
			expr: &actionExpr{
				pos: position{line: 595, col: 8, offset: 16394},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 595, col: 8, offset: 16394},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 595, col: 8, offset: 16394},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 595, col: 13, offset: 16399},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 595, col: 17, offset: 16403},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 595, col: 22, offset: 16408},
								expr: &choiceExpr{
									pos: position{line: 595, col: 24, offset: 16410},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 595, col: 24, offset: 16410},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 595, col: 33, offset: 16419},
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
			pos:  position{line: 608, col: 1, offset: 16658},
			expr: &actionExpr{
				pos: position{line: 608, col: 11, offset: 16668},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 608, col: 11, offset: 16668},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 608, col: 11, offset: 16668},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 608, col: 15, offset: 16672},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 608, col: 19, offset: 16676},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 615, col: 1, offset: 16895},
			expr: &actionExpr{
				pos: position{line: 615, col: 15, offset: 16909},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 615, col: 15, offset: 16909},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 615, col: 15, offset: 16909},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 615, col: 19, offset: 16913},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 615, col: 24, offset: 16918},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 615, col: 24, offset: 16918},
										name: "Composite",
									},
									&ruleRefExpr{
										pos:  position{line: 615, col: 36, offset: 16930},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 615, col: 42, offset: 16936},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 615, col: 51, offset: 16945},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 615, col: 56, offset: 16950},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 619, col: 1, offset: 16979},
			expr: &actionExpr{
				pos: position{line: 619, col: 8, offset: 16986},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 619, col: 8, offset: 16986},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 619, col: 12, offset: 16990},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 624, col: 1, offset: 17112},
			expr: &seqExpr{
				pos: position{line: 624, col: 15, offset: 17126},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 624, col: 15, offset: 17126},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 624, col: 19, offset: 17130},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 624, col: 32, offset: 17143},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 628, col: 1, offset: 17208},
			expr: &actionExpr{
				pos: position{line: 628, col: 17, offset: 17224},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 628, col: 17, offset: 17224},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 628, col: 17, offset: 17224},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 628, col: 29, offset: 17236},
							expr: &choiceExpr{
								pos: position{line: 628, col: 30, offset: 17237},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 628, col: 30, offset: 17237},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 628, col: 44, offset: 17251},
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
			pos:  position{line: 635, col: 1, offset: 17394},
			expr: &actionExpr{
				pos: position{line: 635, col: 11, offset: 17404},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 635, col: 11, offset: 17404},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 635, col: 11, offset: 17404},
							expr: &litMatcher{
								pos:        position{line: 635, col: 11, offset: 17404},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 635, col: 18, offset: 17411},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 635, col: 18, offset: 17411},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 635, col: 26, offset: 17419},
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
			pos:  position{line: 648, col: 1, offset: 17810},
			expr: &choiceExpr{
				pos: position{line: 648, col: 10, offset: 17819},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 648, col: 10, offset: 17819},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 648, col: 26, offset: 17835},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 650, col: 1, offset: 17847},
			expr: &seqExpr{
				pos: position{line: 650, col: 18, offset: 17864},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 650, col: 20, offset: 17866},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 650, col: 20, offset: 17866},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 650, col: 33, offset: 17879},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 650, col: 43, offset: 17889},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 652, col: 1, offset: 17899},
			expr: &seqExpr{
				pos: position{line: 652, col: 15, offset: 17913},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 652, col: 15, offset: 17913},
						expr: &ruleRefExpr{
							pos:  position{line: 652, col: 15, offset: 17913},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 652, col: 24, offset: 17922},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 654, col: 1, offset: 17932},
			expr: &seqExpr{
				pos: position{line: 654, col: 13, offset: 17944},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 654, col: 13, offset: 17944},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 654, col: 17, offset: 17948},
						expr: &ruleRefExpr{
							pos:  position{line: 654, col: 17, offset: 17948},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 656, col: 1, offset: 17963},
			expr: &seqExpr{
				pos: position{line: 656, col: 13, offset: 17975},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 656, col: 13, offset: 17975},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 656, col: 18, offset: 17980},
						expr: &charClassMatcher{
							pos:        position{line: 656, col: 18, offset: 17980},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 656, col: 24, offset: 17986},
						expr: &ruleRefExpr{
							pos:  position{line: 656, col: 24, offset: 17986},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 658, col: 1, offset: 18001},
			expr: &choiceExpr{
				pos: position{line: 658, col: 12, offset: 18012},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 658, col: 12, offset: 18012},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 658, col: 20, offset: 18020},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 658, col: 20, offset: 18020},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 658, col: 40, offset: 18040},
								expr: &ruleRefExpr{
									pos:  position{line: 658, col: 40, offset: 18040},
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
			pos:  position{line: 660, col: 1, offset: 18057},
			expr: &choiceExpr{
				pos: position{line: 660, col: 11, offset: 18067},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 660, col: 11, offset: 18067},
						name: "QuotedString",
					},
					&ruleRefExpr{
						pos:  position{line: 660, col: 26, offset: 18082},
						name: "RawString",
					},
				},
			},
		},
		{
			name: "QuotedString",
			pos:  position{line: 662, col: 1, offset: 18093},
			expr: &actionExpr{
				pos: position{line: 662, col: 17, offset: 18109},
				run: (*parser).callonQuotedString1,
				expr: &seqExpr{
					pos: position{line: 662, col: 17, offset: 18109},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 662, col: 17, offset: 18109},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 662, col: 21, offset: 18113},
							expr: &ruleRefExpr{
								pos:  position{line: 662, col: 21, offset: 18113},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 662, col: 27, offset: 18119},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "RawString",
			pos:  position{line: 670, col: 1, offset: 18274},
			expr: &actionExpr{
				pos: position{line: 670, col: 14, offset: 18287},
				run: (*parser).callonRawString1,
				expr: &seqExpr{
					pos: position{line: 670, col: 14, offset: 18287},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 670, col: 14, offset: 18287},
							val:        "`",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 670, col: 18, offset: 18291},
							expr: &charClassMatcher{
								pos:        position{line: 670, col: 18, offset: 18291},
								val:        "[^`]",
								chars:      []rune{'`'},
								ignoreCase: false,
								inverted:   true,
							},
						},
						&litMatcher{
							pos:        position{line: 670, col: 24, offset: 18297},
							val:        "`",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 679, col: 1, offset: 18464},
			expr: &choiceExpr{
				pos: position{line: 679, col: 9, offset: 18472},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 679, col: 9, offset: 18472},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 679, col: 9, offset: 18472},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 683, col: 5, offset: 18572},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 683, col: 5, offset: 18572},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 689, col: 1, offset: 18673},
			expr: &actionExpr{
				pos: position{line: 689, col: 9, offset: 18681},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 689, col: 9, offset: 18681},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 695, col: 1, offset: 18776},
			expr: &charClassMatcher{
				pos:        position{line: 695, col: 16, offset: 18791},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 697, col: 1, offset: 18802},
			expr: &choiceExpr{
				pos: position{line: 697, col: 9, offset: 18810},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 697, col: 11, offset: 18812},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 697, col: 11, offset: 18812},
								expr: &ruleRefExpr{
									pos:  position{line: 697, col: 12, offset: 18813},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 697, col: 24, offset: 18825,
							},
						},
					},
					&seqExpr{
						pos: position{line: 697, col: 32, offset: 18833},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 697, col: 32, offset: 18833},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 697, col: 37, offset: 18838},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 699, col: 1, offset: 18856},
			expr: &charClassMatcher{
				pos:        position{line: 699, col: 16, offset: 18871},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 701, col: 1, offset: 18887},
			expr: &choiceExpr{
				pos: position{line: 701, col: 19, offset: 18905},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 701, col: 19, offset: 18905},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 701, col: 38, offset: 18924},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 703, col: 1, offset: 18939},
			expr: &charClassMatcher{
				pos:        position{line: 703, col: 21, offset: 18959},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 705, col: 1, offset: 18981},
			expr: &seqExpr{
				pos: position{line: 705, col: 18, offset: 18998},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 705, col: 18, offset: 18998},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 705, col: 22, offset: 19002},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 705, col: 31, offset: 19011},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 705, col: 40, offset: 19020},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 705, col: 49, offset: 19029},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 707, col: 1, offset: 19039},
			expr: &charClassMatcher{
				pos:        position{line: 707, col: 17, offset: 19055},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 709, col: 1, offset: 19062},
			expr: &charClassMatcher{
				pos:        position{line: 709, col: 24, offset: 19085},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 711, col: 1, offset: 19092},
			expr: &charClassMatcher{
				pos:        position{line: 711, col: 13, offset: 19104},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 713, col: 1, offset: 19117},
			expr: &oneOrMoreExpr{
				pos: position{line: 713, col: 20, offset: 19136},
				expr: &charClassMatcher{
					pos:        position{line: 713, col: 20, offset: 19136},
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
			pos:         position{line: 715, col: 1, offset: 19148},
			expr: &zeroOrMoreExpr{
				pos: position{line: 715, col: 19, offset: 19166},
				expr: &choiceExpr{
					pos: position{line: 715, col: 21, offset: 19168},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 715, col: 21, offset: 19168},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 715, col: 33, offset: 19180},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 717, col: 1, offset: 19192},
			expr: &actionExpr{
				pos: position{line: 717, col: 12, offset: 19203},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 717, col: 12, offset: 19203},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 717, col: 12, offset: 19203},
							expr: &charClassMatcher{
								pos:        position{line: 717, col: 12, offset: 19203},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 717, col: 19, offset: 19210},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 717, col: 23, offset: 19214},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 717, col: 28, offset: 19219},
								expr: &charClassMatcher{
									pos:        position{line: 717, col: 28, offset: 19219},
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
			pos:  position{line: 728, col: 1, offset: 19495},
			expr: &notExpr{
				pos: position{line: 728, col: 8, offset: 19502},
				expr: &anyMatcher{
					line: 728, col: 9, offset: 19503,
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
		case *ArrayComprehension, *ObjectComprehension, *SetComprehension: // skip closures
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

func (c *current) onObjectComprehension1(key, value, body interface{}) (interface{}, error) {
	oc := ObjectComprehensionTerm(key.(*Term), value.(*Term), body.(Body))
	oc.Location = currentLocation(c)
	return oc, nil
}

func (p *parser) callonObjectComprehension1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObjectComprehension1(stack["key"], stack["value"], stack["body"])
}

func (c *current) onSetComprehension1(term, body interface{}) (interface{}, error) {
	sc := SetComprehensionTerm(term.(*Term), body.(Body))
	sc.Location = currentLocation(c)
	return sc, nil
}

func (p *parser) callonSetComprehension1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSetComprehension1(stack["term"], stack["body"])
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

func (c *current) onQuotedString1() (interface{}, error) {
	var v string
	err := json.Unmarshal([]byte(c.text), &v)
	str := StringTerm(v)
	str.Location = currentLocation(c)
	return str, err
}

func (p *parser) callonQuotedString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onQuotedString1()
}

func (c *current) onRawString1() (interface{}, error) {
	s := string(c.text)
	s = s[1 : len(s)-1] // Trim surrounding quotes.

	str := StringTerm(s)
	str.Location = currentLocation(c)
	return str, nil
}

func (p *parser) callonRawString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRawString1()
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
		maxFailExpected: make([]string, 0, 20),
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
	maxFailExpected       []string
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

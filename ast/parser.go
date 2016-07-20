package ast

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//
// BUGS: the escaped forward solidus (`\/`) is not currently handled for strings.
//

// currentLocation converts the parser context to a Location object.
func currentLocation(c *current) *Location {
	// TODO(tsandall): is it possible to access the filename from inside the parser?
	return NewLocation(c.text, "", c.pos.line, c.pos.col)
}

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 16, col: 1, offset: 367},
			expr: &actionExpr{
				pos: position{line: 16, col: 12, offset: 378},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 16, col: 12, offset: 378},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 16, col: 12, offset: 378},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 16, col: 14, offset: 380},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 16, col: 19, offset: 385},
								expr: &seqExpr{
									pos: position{line: 16, col: 20, offset: 386},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 16, col: 20, offset: 386},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 16, col: 25, offset: 391},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 16, col: 30, offset: 396},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 16, col: 35, offset: 401},
												expr: &seqExpr{
													pos: position{line: 16, col: 36, offset: 402},
													exprs: []interface{}{
														&ruleRefExpr{
															pos:  position{line: 16, col: 36, offset: 402},
															name: "ws",
														},
														&ruleRefExpr{
															pos:  position{line: 16, col: 39, offset: 405},
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
							pos:  position{line: 16, col: 48, offset: 414},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 16, col: 50, offset: 416},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 34, col: 1, offset: 753},
			expr: &actionExpr{
				pos: position{line: 34, col: 9, offset: 761},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 34, col: 9, offset: 761},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 34, col: 14, offset: 766},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 34, col: 14, offset: 766},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 24, offset: 776},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 33, offset: 785},
								name: "Rule",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 40, offset: 792},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 47, offset: 799},
								name: "Comment",
							},
						},
					},
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 38, col: 1, offset: 833},
			expr: &actionExpr{
				pos: position{line: 38, col: 12, offset: 844},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 38, col: 12, offset: 844},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 38, col: 12, offset: 844},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 38, col: 22, offset: 854},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 38, col: 25, offset: 857},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 38, col: 30, offset: 862},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 38, col: 30, offset: 862},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 38, col: 36, offset: 868},
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
			pos:  position{line: 74, col: 1, offset: 2249},
			expr: &actionExpr{
				pos: position{line: 74, col: 11, offset: 2259},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 74, col: 11, offset: 2259},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 74, col: 11, offset: 2259},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 74, col: 20, offset: 2268},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 74, col: 23, offset: 2271},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 74, col: 29, offset: 2277},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 74, col: 29, offset: 2277},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 74, col: 35, offset: 2283},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 74, col: 40, offset: 2288},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 74, col: 46, offset: 2294},
								expr: &seqExpr{
									pos: position{line: 74, col: 47, offset: 2295},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 74, col: 47, offset: 2295},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 74, col: 50, offset: 2298},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 74, col: 55, offset: 2303},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 74, col: 58, offset: 2306},
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
			name: "Rule",
			pos:  position{line: 95, col: 1, offset: 2924},
			expr: &actionExpr{
				pos: position{line: 95, col: 9, offset: 2932},
				run: (*parser).callonRule1,
				expr: &seqExpr{
					pos: position{line: 95, col: 9, offset: 2932},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 95, col: 9, offset: 2932},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 95, col: 14, offset: 2937},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 95, col: 18, offset: 2941},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 95, col: 22, offset: 2945},
								expr: &seqExpr{
									pos: position{line: 95, col: 24, offset: 2947},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 95, col: 24, offset: 2947},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 95, col: 26, offset: 2949},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 30, offset: 2953},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 32, offset: 2955},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 37, offset: 2960},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 95, col: 39, offset: 2962},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 43, offset: 2966},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 95, col: 48, offset: 2971},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 95, col: 54, offset: 2977},
								expr: &seqExpr{
									pos: position{line: 95, col: 56, offset: 2979},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 95, col: 56, offset: 2979},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 95, col: 58, offset: 2981},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 62, offset: 2985},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 95, col: 64, offset: 2987},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 95, col: 72, offset: 2995},
							label: "body",
							expr: &seqExpr{
								pos: position{line: 95, col: 79, offset: 3002},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 95, col: 79, offset: 3002},
										name: "_",
									},
									&litMatcher{
										pos:        position{line: 95, col: 81, offset: 3004},
										val:        ":-",
										ignoreCase: false,
									},
									&ruleRefExpr{
										pos:  position{line: 95, col: 86, offset: 3009},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 95, col: 88, offset: 3011},
										name: "Body",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 171, col: 1, offset: 5137},
			expr: &actionExpr{
				pos: position{line: 171, col: 9, offset: 5145},
				run: (*parser).callonBody1,
				expr: &seqExpr{
					pos: position{line: 171, col: 9, offset: 5145},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 171, col: 9, offset: 5145},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 171, col: 14, offset: 5150},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 171, col: 19, offset: 5155},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 171, col: 24, offset: 5160},
								expr: &seqExpr{
									pos: position{line: 171, col: 26, offset: 5162},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 171, col: 26, offset: 5162},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 171, col: 28, offset: 5164},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 171, col: 32, offset: 5168},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 171, col: 34, offset: 5170},
											name: "Expr",
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
			name: "Expr",
			pos:  position{line: 181, col: 1, offset: 5383},
			expr: &actionExpr{
				pos: position{line: 181, col: 9, offset: 5391},
				run: (*parser).callonExpr1,
				expr: &seqExpr{
					pos: position{line: 181, col: 9, offset: 5391},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 181, col: 9, offset: 5391},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 181, col: 13, offset: 5395},
								expr: &seqExpr{
									pos: position{line: 181, col: 15, offset: 5397},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 181, col: 15, offset: 5397},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 181, col: 21, offset: 5403},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 181, col: 27, offset: 5409},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 181, col: 32, offset: 5414},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 181, col: 32, offset: 5414},
										name: "InfixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 181, col: 44, offset: 5426},
										name: "PrefixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 181, col: 57, offset: 5439},
										name: "Term",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "InfixExpr",
			pos:  position{line: 189, col: 1, offset: 5581},
			expr: &actionExpr{
				pos: position{line: 189, col: 14, offset: 5594},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 189, col: 14, offset: 5594},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 189, col: 14, offset: 5594},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 189, col: 19, offset: 5599},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 189, col: 24, offset: 5604},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 189, col: 26, offset: 5606},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 189, col: 29, offset: 5609},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 189, col: 37, offset: 5617},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 189, col: 39, offset: 5619},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 189, col: 45, offset: 5625},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 193, col: 1, offset: 5700},
			expr: &actionExpr{
				pos: position{line: 193, col: 12, offset: 5711},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 193, col: 12, offset: 5711},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 193, col: 17, offset: 5716},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 193, col: 17, offset: 5716},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 193, col: 23, offset: 5722},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 193, col: 30, offset: 5729},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 193, col: 37, offset: 5736},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 193, col: 44, offset: 5743},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 193, col: 50, offset: 5749},
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
			pos:  position{line: 199, col: 1, offset: 5864},
			expr: &actionExpr{
				pos: position{line: 199, col: 15, offset: 5878},
				run: (*parser).callonPrefixExpr1,
				expr: &seqExpr{
					pos: position{line: 199, col: 15, offset: 5878},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 199, col: 15, offset: 5878},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 199, col: 18, offset: 5881},
								name: "Var",
							},
						},
						&litMatcher{
							pos:        position{line: 199, col: 22, offset: 5885},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 199, col: 26, offset: 5889},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 199, col: 28, offset: 5891},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 199, col: 33, offset: 5896},
								expr: &ruleRefExpr{
									pos:  position{line: 199, col: 33, offset: 5896},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 199, col: 39, offset: 5902},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 199, col: 44, offset: 5907},
								expr: &seqExpr{
									pos: position{line: 199, col: 46, offset: 5909},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 199, col: 46, offset: 5909},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 199, col: 48, offset: 5911},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 199, col: 52, offset: 5915},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 199, col: 54, offset: 5917},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 199, col: 62, offset: 5925},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 199, col: 65, offset: 5928},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 215, col: 1, offset: 6330},
			expr: &actionExpr{
				pos: position{line: 215, col: 9, offset: 6338},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 215, col: 9, offset: 6338},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 215, col: 15, offset: 6344},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 215, col: 15, offset: 6344},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 215, col: 31, offset: 6360},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 215, col: 43, offset: 6372},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 215, col: 52, offset: 6381},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 215, col: 58, offset: 6387},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 219, col: 1, offset: 6418},
			expr: &ruleRefExpr{
				pos:  position{line: 219, col: 18, offset: 6435},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 221, col: 1, offset: 6455},
			expr: &actionExpr{
				pos: position{line: 221, col: 23, offset: 6477},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 221, col: 23, offset: 6477},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 221, col: 23, offset: 6477},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 221, col: 27, offset: 6481},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 221, col: 29, offset: 6483},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 221, col: 34, offset: 6488},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 221, col: 39, offset: 6493},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 221, col: 41, offset: 6495},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 221, col: 45, offset: 6499},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 221, col: 47, offset: 6501},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 221, col: 52, offset: 6506},
								name: "Body",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 221, col: 57, offset: 6511},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 221, col: 59, offset: 6513},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 227, col: 1, offset: 6638},
			expr: &choiceExpr{
				pos: position{line: 227, col: 14, offset: 6651},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 227, col: 14, offset: 6651},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 227, col: 23, offset: 6660},
						name: "Array",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 229, col: 1, offset: 6667},
			expr: &choiceExpr{
				pos: position{line: 229, col: 11, offset: 6677},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 229, col: 11, offset: 6677},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 229, col: 20, offset: 6686},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 229, col: 29, offset: 6695},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 229, col: 36, offset: 6702},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 231, col: 1, offset: 6708},
			expr: &choiceExpr{
				pos: position{line: 231, col: 8, offset: 6715},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 231, col: 8, offset: 6715},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 231, col: 17, offset: 6724},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 231, col: 23, offset: 6730},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 233, col: 1, offset: 6735},
			expr: &actionExpr{
				pos: position{line: 233, col: 11, offset: 6745},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 233, col: 11, offset: 6745},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 233, col: 11, offset: 6745},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 15, offset: 6749},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 233, col: 17, offset: 6751},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 233, col: 22, offset: 6756},
								expr: &seqExpr{
									pos: position{line: 233, col: 23, offset: 6757},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 233, col: 23, offset: 6757},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 27, offset: 6761},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 233, col: 29, offset: 6763},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 33, offset: 6767},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 35, offset: 6769},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 233, col: 42, offset: 6776},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 233, col: 47, offset: 6781},
								expr: &seqExpr{
									pos: position{line: 233, col: 49, offset: 6783},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 233, col: 49, offset: 6783},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 233, col: 51, offset: 6785},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 55, offset: 6789},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 57, offset: 6791},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 61, offset: 6795},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 233, col: 63, offset: 6797},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 67, offset: 6801},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 69, offset: 6803},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 77, offset: 6811},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 233, col: 79, offset: 6813},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 257, col: 1, offset: 7592},
			expr: &actionExpr{
				pos: position{line: 257, col: 10, offset: 7601},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 257, col: 10, offset: 7601},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 257, col: 10, offset: 7601},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 257, col: 14, offset: 7605},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 257, col: 17, offset: 7608},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 257, col: 22, offset: 7613},
								expr: &ruleRefExpr{
									pos:  position{line: 257, col: 22, offset: 7613},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 257, col: 28, offset: 7619},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 257, col: 33, offset: 7624},
								expr: &seqExpr{
									pos: position{line: 257, col: 34, offset: 7625},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 257, col: 34, offset: 7625},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 257, col: 36, offset: 7627},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 257, col: 40, offset: 7631},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 257, col: 42, offset: 7633},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 257, col: 49, offset: 7640},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 257, col: 51, offset: 7642},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 281, col: 1, offset: 8215},
			expr: &actionExpr{
				pos: position{line: 281, col: 8, offset: 8222},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 281, col: 8, offset: 8222},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 281, col: 8, offset: 8222},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 281, col: 13, offset: 8227},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 281, col: 17, offset: 8231},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 281, col: 22, offset: 8236},
								expr: &choiceExpr{
									pos: position{line: 281, col: 24, offset: 8238},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 281, col: 24, offset: 8238},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 281, col: 33, offset: 8247},
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
			pos:  position{line: 294, col: 1, offset: 8486},
			expr: &actionExpr{
				pos: position{line: 294, col: 11, offset: 8496},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 294, col: 11, offset: 8496},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 294, col: 11, offset: 8496},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 294, col: 15, offset: 8500},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 294, col: 19, offset: 8504},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 301, col: 1, offset: 8723},
			expr: &actionExpr{
				pos: position{line: 301, col: 15, offset: 8737},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 301, col: 15, offset: 8737},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 301, col: 15, offset: 8737},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 301, col: 19, offset: 8741},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 301, col: 24, offset: 8746},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 301, col: 24, offset: 8746},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 301, col: 30, offset: 8752},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 301, col: 39, offset: 8761},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 301, col: 44, offset: 8766},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 305, col: 1, offset: 8795},
			expr: &actionExpr{
				pos: position{line: 305, col: 8, offset: 8802},
				run: (*parser).callonVar1,
				expr: &seqExpr{
					pos: position{line: 305, col: 8, offset: 8802},
					exprs: []interface{}{
						&notExpr{
							pos: position{line: 305, col: 8, offset: 8802},
							expr: &ruleRefExpr{
								pos:  position{line: 305, col: 9, offset: 8803},
								name: "Reserved",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 305, col: 18, offset: 8812},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 305, col: 30, offset: 8824},
							expr: &choiceExpr{
								pos: position{line: 305, col: 31, offset: 8825},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 305, col: 31, offset: 8825},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 305, col: 45, offset: 8839},
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
			pos:  position{line: 312, col: 1, offset: 8982},
			expr: &actionExpr{
				pos: position{line: 312, col: 11, offset: 8992},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 312, col: 11, offset: 8992},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 312, col: 11, offset: 8992},
							expr: &litMatcher{
								pos:        position{line: 312, col: 11, offset: 8992},
								val:        "-",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 312, col: 16, offset: 8997},
							name: "Integer",
						},
						&zeroOrOneExpr{
							pos: position{line: 312, col: 24, offset: 9005},
							expr: &seqExpr{
								pos: position{line: 312, col: 26, offset: 9007},
								exprs: []interface{}{
									&litMatcher{
										pos:        position{line: 312, col: 26, offset: 9007},
										val:        ".",
										ignoreCase: false,
									},
									&oneOrMoreExpr{
										pos: position{line: 312, col: 30, offset: 9011},
										expr: &ruleRefExpr{
											pos:  position{line: 312, col: 30, offset: 9011},
											name: "DecimalDigit",
										},
									},
								},
							},
						},
						&zeroOrOneExpr{
							pos: position{line: 312, col: 47, offset: 9028},
							expr: &ruleRefExpr{
								pos:  position{line: 312, col: 47, offset: 9028},
								name: "Exponent",
							},
						},
					},
				},
			},
		},
		{
			name: "String",
			pos:  position{line: 321, col: 1, offset: 9269},
			expr: &actionExpr{
				pos: position{line: 321, col: 11, offset: 9279},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 321, col: 11, offset: 9279},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 321, col: 11, offset: 9279},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 321, col: 15, offset: 9283},
							expr: &choiceExpr{
								pos: position{line: 321, col: 17, offset: 9285},
								alternatives: []interface{}{
									&seqExpr{
										pos: position{line: 321, col: 17, offset: 9285},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 321, col: 17, offset: 9285},
												expr: &ruleRefExpr{
													pos:  position{line: 321, col: 18, offset: 9286},
													name: "EscapedChar",
												},
											},
											&anyMatcher{
												line: 321, col: 30, offset: 9298,
											},
										},
									},
									&seqExpr{
										pos: position{line: 321, col: 34, offset: 9302},
										exprs: []interface{}{
											&litMatcher{
												pos:        position{line: 321, col: 34, offset: 9302},
												val:        "\\",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 321, col: 39, offset: 9307},
												name: "EscapeSequence",
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 321, col: 57, offset: 9325},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 330, col: 1, offset: 9583},
			expr: &choiceExpr{
				pos: position{line: 330, col: 9, offset: 9591},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 330, col: 9, offset: 9591},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 330, col: 9, offset: 9591},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 334, col: 5, offset: 9691},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 334, col: 5, offset: 9691},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 340, col: 1, offset: 9792},
			expr: &actionExpr{
				pos: position{line: 340, col: 9, offset: 9800},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 340, col: 9, offset: 9800},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "Reserved",
			pos:  position{line: 346, col: 1, offset: 9895},
			expr: &choiceExpr{
				pos: position{line: 346, col: 14, offset: 9908},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 346, col: 14, offset: 9908},
						val:        "not",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 346, col: 22, offset: 9916},
						val:        "package",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 346, col: 34, offset: 9928},
						val:        "import",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 346, col: 45, offset: 9939},
						val:        "null",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 346, col: 54, offset: 9948},
						val:        "true",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 346, col: 63, offset: 9957},
						val:        "false",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 348, col: 1, offset: 9967},
			expr: &choiceExpr{
				pos: position{line: 348, col: 12, offset: 9978},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 348, col: 12, offset: 9978},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 348, col: 18, offset: 9984},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 348, col: 18, offset: 9984},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 348, col: 38, offset: 10004},
								expr: &ruleRefExpr{
									pos:  position{line: 348, col: 38, offset: 10004},
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
			pos:  position{line: 350, col: 1, offset: 10019},
			expr: &seqExpr{
				pos: position{line: 350, col: 13, offset: 10031},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 350, col: 13, offset: 10031},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 350, col: 18, offset: 10036},
						expr: &charClassMatcher{
							pos:        position{line: 350, col: 18, offset: 10036},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 350, col: 24, offset: 10042},
						expr: &ruleRefExpr{
							pos:  position{line: 350, col: 24, offset: 10042},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 352, col: 1, offset: 10057},
			expr: &charClassMatcher{
				pos:        position{line: 352, col: 16, offset: 10072},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 354, col: 1, offset: 10083},
			expr: &charClassMatcher{
				pos:        position{line: 354, col: 16, offset: 10098},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 356, col: 1, offset: 10114},
			expr: &choiceExpr{
				pos: position{line: 356, col: 19, offset: 10132},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 356, col: 19, offset: 10132},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 356, col: 38, offset: 10151},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 358, col: 1, offset: 10166},
			expr: &charClassMatcher{
				pos:        position{line: 358, col: 21, offset: 10186},
				val:        "[\"\\\\/bfnrt]",
				chars:      []rune{'"', '\\', '/', 'b', 'f', 'n', 'r', 't'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 360, col: 1, offset: 10199},
			expr: &seqExpr{
				pos: position{line: 360, col: 18, offset: 10216},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 360, col: 18, offset: 10216},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 360, col: 22, offset: 10220},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 360, col: 31, offset: 10229},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 360, col: 40, offset: 10238},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 360, col: 49, offset: 10247},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 362, col: 1, offset: 10257},
			expr: &charClassMatcher{
				pos:        position{line: 362, col: 17, offset: 10273},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 364, col: 1, offset: 10280},
			expr: &charClassMatcher{
				pos:        position{line: 364, col: 24, offset: 10303},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 366, col: 1, offset: 10310},
			expr: &charClassMatcher{
				pos:        position{line: 366, col: 13, offset: 10322},
				val:        "[0-9a-f]",
				ranges:     []rune{'0', '9', 'a', 'f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 368, col: 1, offset: 10332},
			expr: &oneOrMoreExpr{
				pos: position{line: 368, col: 20, offset: 10351},
				expr: &charClassMatcher{
					pos:        position{line: 368, col: 20, offset: 10351},
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
			pos:         position{line: 370, col: 1, offset: 10363},
			expr: &zeroOrMoreExpr{
				pos: position{line: 370, col: 19, offset: 10381},
				expr: &choiceExpr{
					pos: position{line: 370, col: 21, offset: 10383},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 370, col: 21, offset: 10383},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 370, col: 33, offset: 10395},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 372, col: 1, offset: 10407},
			expr: &seqExpr{
				pos: position{line: 372, col: 12, offset: 10418},
				exprs: []interface{}{
					&zeroOrMoreExpr{
						pos: position{line: 372, col: 12, offset: 10418},
						expr: &charClassMatcher{
							pos:        position{line: 372, col: 12, offset: 10418},
							val:        "[ \\t]",
							chars:      []rune{' ', '\t'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&litMatcher{
						pos:        position{line: 372, col: 19, offset: 10425},
						val:        "#",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 372, col: 23, offset: 10429},
						expr: &charClassMatcher{
							pos:        position{line: 372, col: 23, offset: 10429},
							val:        "[^\\r\\n]",
							chars:      []rune{'\r', '\n'},
							ignoreCase: false,
							inverted:   true,
						},
					},
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 374, col: 1, offset: 10439},
			expr: &notExpr{
				pos: position{line: 374, col: 8, offset: 10446},
				expr: &anyMatcher{
					line: 374, col: 9, offset: 10447,
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

	return buf, nil
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
	path := RefTerm(DefaultRootDocument)
	switch v := val.(*Term).Value.(type) {
	case Ref:
		// Convert head of package Ref to String because it will be prefixed
		// with the root document variable.
		head := v[0]
		head = StringTerm(string(head.Value.(Var)))
		head.Location = v[0].Location
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
		path.Value = append(path.Value.(Ref), head)
		path.Value = append(path.Value.(Ref), tail...)
	case Var:
		s := StringTerm(string(v))
		s.Location = val.(*Term).Location
		path.Value = append(path.Value.(Ref), s)
	}
	pkg := &Package{Location: currentLocation(c), Path: path.Value.(Ref)}
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
	switch p := imp.Path.Value.(type) {
	case Ref:
		for _, x := range p[1:] {
			if _, ok := x.Value.(String); !ok {
				return nil, fmt.Errorf("import path cannot contain non-string values: %v", x)
			}
		}
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

func (c *current) onRule1(name, key, value, body interface{}) (interface{}, error) {

	rule := &Rule{}
	rule.Location = currentLocation(c)
	rule.Name = name.(*Term).Value.(Var)

	if key != nil {
		keySlice := key.([]interface{})
		// Rule definition above describes the "key" slice. We care about the "Term" element.
		rule.Key = keySlice[3].(*Term)

		var ref Ref
		WalkRefs(rule.Key, func(x Ref) bool {
			ref = x
			return true
		})

		if ref != nil {
			return nil, fmt.Errorf("head cannot contain references (%v appears in key)", ref)
		}

		var closure interface{}
		WalkClosures(rule.Key, func(x interface{}) bool {
			closure = x
			return true
		})

		if closure != nil {
			return nil, fmt.Errorf("head cannot contain closures (%v appears in key)", closure)
		}
	}

	if value != nil {
		valueSlice := value.([]interface{})
		// Rule definition above describes the "value" slice. We care about the "Term" element.
		rule.Value = valueSlice[len(valueSlice)-1].(*Term)

		var ref Ref
		WalkRefs(rule.Value, func(x Ref) bool {
			ref = x
			return true
		})

		if ref != nil {
			return nil, fmt.Errorf("head cannot contain references (%v appears in value)", ref)
		}

		var closure interface{}
		WalkClosures(rule.Value, func(x interface{}) bool {
			closure = x
			return true
		})

		if closure != nil {
			return nil, fmt.Errorf("head cannot contain closures (%v appears in value)", closure)
		}
	}

	if key == nil && value == nil {
		rule.Value = BooleanTerm(true)
	}

	if key != nil && value != nil {
		switch rule.Key.Value.(type) {
		case Var, String: // nop
		default:
			return nil, fmt.Errorf("head of object rule must have string or var key (%s is not allowed)", rule.Key)
		}
	}

	// Rule definition above describes the "body" slice. We only care about the "Body" element.
	rule.Body = body.([]interface{})[3].(Body)

	return rule, nil
}

func (p *parser) callonRule1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRule1(stack["name"], stack["key"], stack["value"], stack["body"])
}

func (c *current) onBody1(head, tail interface{}) (interface{}, error) {
	var buf Body
	buf = append(buf, head.(*Expr))
	for _, s := range tail.([]interface{}) {
		expr := s.([]interface{})[3].(*Expr)
		buf = append(buf, expr)
	}
	return buf, nil
}

func (p *parser) callonBody1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBody1(stack["head"], stack["tail"])
}

func (c *current) onExpr1(neg, val interface{}) (interface{}, error) {
	expr := &Expr{}
	expr.Location = currentLocation(c)
	expr.Negated = neg != nil
	expr.Terms = val
	return expr, nil
}

func (p *parser) callonExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onExpr1(stack["neg"], stack["val"])
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
	operator := VarTerm(string(c.text))
	operator.Location = currentLocation(c)
	return operator, nil
}

func (p *parser) callonInfixOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixOp1(stack["val"])
}

func (c *current) onPrefixExpr1(op, head, tail interface{}) (interface{}, error) {
	buf := []*Term{op.(*Term)}
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

func (p *parser) callonPrefixExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPrefixExpr1(stack["op"], stack["head"], stack["tail"])
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
	obj := ObjectTerm()
	obj.Location = currentLocation(c)

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

func (p *parser) callonObject1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObject1(stack["head"], stack["tail"])
}

func (c *current) onArray1(head, tail interface{}) (interface{}, error) {

	arr := ArrayTerm()
	arr.Location = currentLocation(c)

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

func (p *parser) callonArray1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArray1(stack["head"], stack["tail"])
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

func (c *current) onVar1() (interface{}, error) {
	str := string(c.text)
	variable := VarTerm(str)
	variable.Location = currentLocation(c)
	return variable, nil
}

func (p *parser) callonVar1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onVar1()
}

func (c *current) onNumber1() (interface{}, error) {
	// JSON numbers have the same syntax as Go's, and are parseable using
	// strconv.
	v, err := strconv.ParseFloat(string(c.text), 64)
	num := NumberTerm(v)
	num.Location = currentLocation(c)
	return num, err
}

func (p *parser) callonNumber1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNumber1()
}

func (c *current) onString1() (interface{}, error) {
	// TODO : the forward slash (solidus) is not a valid escape in Go, it will
	// fail if there's one in the string
	v, err := strconv.Unquote(string(c.text))
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

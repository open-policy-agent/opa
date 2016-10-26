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
														&choiceExpr{
															pos: position{line: 16, col: 37, offset: 403},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 16, col: 37, offset: 403},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 16, col: 42, offset: 408},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 16, col: 54, offset: 420},
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
							pos:  position{line: 16, col: 63, offset: 429},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 16, col: 65, offset: 431},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 34, col: 1, offset: 768},
			expr: &actionExpr{
				pos: position{line: 34, col: 9, offset: 776},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 34, col: 9, offset: 776},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 34, col: 14, offset: 781},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 34, col: 14, offset: 781},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 24, offset: 791},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 33, offset: 800},
								name: "Rule",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 40, offset: 807},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 47, offset: 814},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 34, col: 57, offset: 824},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 43, col: 1, offset: 1188},
			expr: &actionExpr{
				pos: position{line: 43, col: 15, offset: 1202},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 43, col: 15, offset: 1202,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 47, col: 1, offset: 1275},
			expr: &actionExpr{
				pos: position{line: 47, col: 12, offset: 1286},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 47, col: 12, offset: 1286},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 47, col: 12, offset: 1286},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 47, col: 22, offset: 1296},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 47, col: 25, offset: 1299},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 47, col: 30, offset: 1304},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 47, col: 30, offset: 1304},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 47, col: 36, offset: 1310},
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
			pos:  position{line: 83, col: 1, offset: 2691},
			expr: &actionExpr{
				pos: position{line: 83, col: 11, offset: 2701},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 83, col: 11, offset: 2701},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 83, col: 11, offset: 2701},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 83, col: 20, offset: 2710},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 83, col: 23, offset: 2713},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 83, col: 29, offset: 2719},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 83, col: 29, offset: 2719},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 83, col: 35, offset: 2725},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 83, col: 40, offset: 2730},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 83, col: 46, offset: 2736},
								expr: &seqExpr{
									pos: position{line: 83, col: 47, offset: 2737},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 83, col: 47, offset: 2737},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 83, col: 50, offset: 2740},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 83, col: 55, offset: 2745},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 83, col: 58, offset: 2748},
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
			pos:  position{line: 104, col: 1, offset: 3366},
			expr: &actionExpr{
				pos: position{line: 104, col: 9, offset: 3374},
				run: (*parser).callonRule1,
				expr: &seqExpr{
					pos: position{line: 104, col: 9, offset: 3374},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 104, col: 9, offset: 3374},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 104, col: 14, offset: 3379},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 104, col: 18, offset: 3383},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 104, col: 22, offset: 3387},
								expr: &seqExpr{
									pos: position{line: 104, col: 24, offset: 3389},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 104, col: 24, offset: 3389},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 104, col: 26, offset: 3391},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 30, offset: 3395},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 32, offset: 3397},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 37, offset: 3402},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 104, col: 39, offset: 3404},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 43, offset: 3408},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 104, col: 48, offset: 3413},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 104, col: 54, offset: 3419},
								expr: &seqExpr{
									pos: position{line: 104, col: 56, offset: 3421},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 104, col: 56, offset: 3421},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 104, col: 58, offset: 3423},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 62, offset: 3427},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 104, col: 64, offset: 3429},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 104, col: 72, offset: 3437},
							label: "body",
							expr: &seqExpr{
								pos: position{line: 104, col: 79, offset: 3444},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 104, col: 79, offset: 3444},
										name: "_",
									},
									&litMatcher{
										pos:        position{line: 104, col: 81, offset: 3446},
										val:        ":-",
										ignoreCase: false,
									},
									&ruleRefExpr{
										pos:  position{line: 104, col: 86, offset: 3451},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 104, col: 88, offset: 3453},
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
			pos:  position{line: 180, col: 1, offset: 5579},
			expr: &actionExpr{
				pos: position{line: 180, col: 9, offset: 5587},
				run: (*parser).callonBody1,
				expr: &seqExpr{
					pos: position{line: 180, col: 9, offset: 5587},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 180, col: 9, offset: 5587},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 180, col: 14, offset: 5592},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 180, col: 19, offset: 5597},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 180, col: 24, offset: 5602},
								expr: &seqExpr{
									pos: position{line: 180, col: 26, offset: 5604},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 180, col: 26, offset: 5604},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 180, col: 28, offset: 5606},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 180, col: 32, offset: 5610},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 180, col: 35, offset: 5613},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 180, col: 35, offset: 5613},
													name: "Expr",
												},
												&ruleRefExpr{
													pos:  position{line: 180, col: 42, offset: 5620},
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
			name: "Expr",
			pos:  position{line: 190, col: 1, offset: 5840},
			expr: &actionExpr{
				pos: position{line: 190, col: 9, offset: 5848},
				run: (*parser).callonExpr1,
				expr: &seqExpr{
					pos: position{line: 190, col: 9, offset: 5848},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 190, col: 9, offset: 5848},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 190, col: 13, offset: 5852},
								expr: &seqExpr{
									pos: position{line: 190, col: 15, offset: 5854},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 190, col: 15, offset: 5854},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 190, col: 21, offset: 5860},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 190, col: 27, offset: 5866},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 190, col: 32, offset: 5871},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 190, col: 32, offset: 5871},
										name: "InfixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 190, col: 44, offset: 5883},
										name: "PrefixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 190, col: 57, offset: 5896},
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
			pos:  position{line: 198, col: 1, offset: 6038},
			expr: &actionExpr{
				pos: position{line: 198, col: 14, offset: 6051},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 198, col: 14, offset: 6051},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 198, col: 14, offset: 6051},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 198, col: 19, offset: 6056},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 198, col: 24, offset: 6061},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 198, col: 26, offset: 6063},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 198, col: 29, offset: 6066},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 198, col: 37, offset: 6074},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 198, col: 39, offset: 6076},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 198, col: 45, offset: 6082},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 202, col: 1, offset: 6157},
			expr: &actionExpr{
				pos: position{line: 202, col: 12, offset: 6168},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 202, col: 12, offset: 6168},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 202, col: 17, offset: 6173},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 202, col: 17, offset: 6173},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 202, col: 23, offset: 6179},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 202, col: 30, offset: 6186},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 202, col: 37, offset: 6193},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 202, col: 44, offset: 6200},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 202, col: 50, offset: 6206},
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
			pos:  position{line: 214, col: 1, offset: 6450},
			expr: &actionExpr{
				pos: position{line: 214, col: 15, offset: 6464},
				run: (*parser).callonPrefixExpr1,
				expr: &seqExpr{
					pos: position{line: 214, col: 15, offset: 6464},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 214, col: 15, offset: 6464},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 214, col: 18, offset: 6467},
								name: "Var",
							},
						},
						&litMatcher{
							pos:        position{line: 214, col: 22, offset: 6471},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 214, col: 26, offset: 6475},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 214, col: 28, offset: 6477},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 214, col: 33, offset: 6482},
								expr: &ruleRefExpr{
									pos:  position{line: 214, col: 33, offset: 6482},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 214, col: 39, offset: 6488},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 214, col: 44, offset: 6493},
								expr: &seqExpr{
									pos: position{line: 214, col: 46, offset: 6495},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 214, col: 46, offset: 6495},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 214, col: 48, offset: 6497},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 52, offset: 6501},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 54, offset: 6503},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 214, col: 62, offset: 6511},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 214, col: 65, offset: 6514},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 230, col: 1, offset: 6916},
			expr: &actionExpr{
				pos: position{line: 230, col: 9, offset: 6924},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 230, col: 9, offset: 6924},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 230, col: 15, offset: 6930},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 230, col: 15, offset: 6930},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 230, col: 31, offset: 6946},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 230, col: 43, offset: 6958},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 230, col: 52, offset: 6967},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 230, col: 58, offset: 6973},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 234, col: 1, offset: 7004},
			expr: &ruleRefExpr{
				pos:  position{line: 234, col: 18, offset: 7021},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 236, col: 1, offset: 7041},
			expr: &actionExpr{
				pos: position{line: 236, col: 23, offset: 7063},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 236, col: 23, offset: 7063},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 236, col: 23, offset: 7063},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 236, col: 27, offset: 7067},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 236, col: 29, offset: 7069},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 236, col: 34, offset: 7074},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 236, col: 39, offset: 7079},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 236, col: 41, offset: 7081},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 236, col: 45, offset: 7085},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 236, col: 47, offset: 7087},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 236, col: 52, offset: 7092},
								name: "Body",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 236, col: 57, offset: 7097},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 236, col: 59, offset: 7099},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 242, col: 1, offset: 7224},
			expr: &choiceExpr{
				pos: position{line: 242, col: 14, offset: 7237},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 242, col: 14, offset: 7237},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 242, col: 23, offset: 7246},
						name: "Array",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 244, col: 1, offset: 7253},
			expr: &choiceExpr{
				pos: position{line: 244, col: 11, offset: 7263},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 244, col: 11, offset: 7263},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 244, col: 20, offset: 7272},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 244, col: 29, offset: 7281},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 244, col: 36, offset: 7288},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 246, col: 1, offset: 7294},
			expr: &choiceExpr{
				pos: position{line: 246, col: 8, offset: 7301},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 246, col: 8, offset: 7301},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 246, col: 17, offset: 7310},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 246, col: 23, offset: 7316},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 248, col: 1, offset: 7321},
			expr: &actionExpr{
				pos: position{line: 248, col: 11, offset: 7331},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 248, col: 11, offset: 7331},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 248, col: 11, offset: 7331},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 248, col: 15, offset: 7335},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 248, col: 17, offset: 7337},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 248, col: 22, offset: 7342},
								expr: &seqExpr{
									pos: position{line: 248, col: 23, offset: 7343},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 248, col: 23, offset: 7343},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 27, offset: 7347},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 248, col: 29, offset: 7349},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 33, offset: 7353},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 35, offset: 7355},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 248, col: 42, offset: 7362},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 248, col: 47, offset: 7367},
								expr: &seqExpr{
									pos: position{line: 248, col: 49, offset: 7369},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 248, col: 49, offset: 7369},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 248, col: 51, offset: 7371},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 55, offset: 7375},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 57, offset: 7377},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 61, offset: 7381},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 248, col: 63, offset: 7383},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 67, offset: 7387},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 69, offset: 7389},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 248, col: 77, offset: 7397},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 248, col: 79, offset: 7399},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 272, col: 1, offset: 8178},
			expr: &actionExpr{
				pos: position{line: 272, col: 10, offset: 8187},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 272, col: 10, offset: 8187},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 272, col: 10, offset: 8187},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 272, col: 14, offset: 8191},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 272, col: 17, offset: 8194},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 272, col: 22, offset: 8199},
								expr: &ruleRefExpr{
									pos:  position{line: 272, col: 22, offset: 8199},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 272, col: 28, offset: 8205},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 272, col: 33, offset: 8210},
								expr: &seqExpr{
									pos: position{line: 272, col: 34, offset: 8211},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 272, col: 34, offset: 8211},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 272, col: 36, offset: 8213},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 272, col: 40, offset: 8217},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 272, col: 42, offset: 8219},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 272, col: 49, offset: 8226},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 272, col: 51, offset: 8228},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 296, col: 1, offset: 8801},
			expr: &actionExpr{
				pos: position{line: 296, col: 8, offset: 8808},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 296, col: 8, offset: 8808},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 296, col: 8, offset: 8808},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 296, col: 13, offset: 8813},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 296, col: 17, offset: 8817},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 296, col: 22, offset: 8822},
								expr: &choiceExpr{
									pos: position{line: 296, col: 24, offset: 8824},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 296, col: 24, offset: 8824},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 296, col: 33, offset: 8833},
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
			pos:  position{line: 309, col: 1, offset: 9072},
			expr: &actionExpr{
				pos: position{line: 309, col: 11, offset: 9082},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 309, col: 11, offset: 9082},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 309, col: 11, offset: 9082},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 309, col: 15, offset: 9086},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 309, col: 19, offset: 9090},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 316, col: 1, offset: 9309},
			expr: &actionExpr{
				pos: position{line: 316, col: 15, offset: 9323},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 316, col: 15, offset: 9323},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 316, col: 15, offset: 9323},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 316, col: 19, offset: 9327},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 316, col: 24, offset: 9332},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 316, col: 24, offset: 9332},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 316, col: 30, offset: 9338},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 316, col: 39, offset: 9347},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 316, col: 44, offset: 9352},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 320, col: 1, offset: 9381},
			expr: &actionExpr{
				pos: position{line: 320, col: 8, offset: 9388},
				run: (*parser).callonVar1,
				expr: &seqExpr{
					pos: position{line: 320, col: 8, offset: 9388},
					exprs: []interface{}{
						&notExpr{
							pos: position{line: 320, col: 8, offset: 9388},
							expr: &ruleRefExpr{
								pos:  position{line: 320, col: 9, offset: 9389},
								name: "Reserved",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 320, col: 18, offset: 9398},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 320, col: 30, offset: 9410},
							expr: &choiceExpr{
								pos: position{line: 320, col: 31, offset: 9411},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 320, col: 31, offset: 9411},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 320, col: 45, offset: 9425},
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
			pos:  position{line: 327, col: 1, offset: 9568},
			expr: &actionExpr{
				pos: position{line: 327, col: 11, offset: 9578},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 327, col: 11, offset: 9578},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 327, col: 11, offset: 9578},
							expr: &litMatcher{
								pos:        position{line: 327, col: 11, offset: 9578},
								val:        "-",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 16, offset: 9583},
							name: "Integer",
						},
						&zeroOrOneExpr{
							pos: position{line: 327, col: 24, offset: 9591},
							expr: &seqExpr{
								pos: position{line: 327, col: 26, offset: 9593},
								exprs: []interface{}{
									&litMatcher{
										pos:        position{line: 327, col: 26, offset: 9593},
										val:        ".",
										ignoreCase: false,
									},
									&oneOrMoreExpr{
										pos: position{line: 327, col: 30, offset: 9597},
										expr: &ruleRefExpr{
											pos:  position{line: 327, col: 30, offset: 9597},
											name: "DecimalDigit",
										},
									},
								},
							},
						},
						&zeroOrOneExpr{
							pos: position{line: 327, col: 47, offset: 9614},
							expr: &ruleRefExpr{
								pos:  position{line: 327, col: 47, offset: 9614},
								name: "Exponent",
							},
						},
					},
				},
			},
		},
		{
			name: "String",
			pos:  position{line: 336, col: 1, offset: 9855},
			expr: &actionExpr{
				pos: position{line: 336, col: 11, offset: 9865},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 336, col: 11, offset: 9865},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 336, col: 11, offset: 9865},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 336, col: 15, offset: 9869},
							expr: &choiceExpr{
								pos: position{line: 336, col: 17, offset: 9871},
								alternatives: []interface{}{
									&seqExpr{
										pos: position{line: 336, col: 17, offset: 9871},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 336, col: 17, offset: 9871},
												expr: &ruleRefExpr{
													pos:  position{line: 336, col: 18, offset: 9872},
													name: "EscapedChar",
												},
											},
											&anyMatcher{
												line: 336, col: 30, offset: 9884,
											},
										},
									},
									&seqExpr{
										pos: position{line: 336, col: 34, offset: 9888},
										exprs: []interface{}{
											&litMatcher{
												pos:        position{line: 336, col: 34, offset: 9888},
												val:        "\\",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 336, col: 39, offset: 9893},
												name: "EscapeSequence",
											},
										},
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 336, col: 57, offset: 9911},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 345, col: 1, offset: 10169},
			expr: &choiceExpr{
				pos: position{line: 345, col: 9, offset: 10177},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 345, col: 9, offset: 10177},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 345, col: 9, offset: 10177},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 349, col: 5, offset: 10277},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 349, col: 5, offset: 10277},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 355, col: 1, offset: 10378},
			expr: &actionExpr{
				pos: position{line: 355, col: 9, offset: 10386},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 355, col: 9, offset: 10386},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "Reserved",
			pos:  position{line: 361, col: 1, offset: 10481},
			expr: &choiceExpr{
				pos: position{line: 361, col: 14, offset: 10494},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 361, col: 14, offset: 10494},
						val:        "not",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 361, col: 22, offset: 10502},
						val:        "package",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 361, col: 34, offset: 10514},
						val:        "import",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 361, col: 45, offset: 10525},
						val:        "null",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 361, col: 54, offset: 10534},
						val:        "true",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 361, col: 63, offset: 10543},
						val:        "false",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 363, col: 1, offset: 10553},
			expr: &choiceExpr{
				pos: position{line: 363, col: 12, offset: 10564},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 363, col: 12, offset: 10564},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 363, col: 18, offset: 10570},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 363, col: 18, offset: 10570},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 363, col: 38, offset: 10590},
								expr: &ruleRefExpr{
									pos:  position{line: 363, col: 38, offset: 10590},
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
			pos:  position{line: 365, col: 1, offset: 10605},
			expr: &seqExpr{
				pos: position{line: 365, col: 13, offset: 10617},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 365, col: 13, offset: 10617},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 365, col: 18, offset: 10622},
						expr: &charClassMatcher{
							pos:        position{line: 365, col: 18, offset: 10622},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 365, col: 24, offset: 10628},
						expr: &ruleRefExpr{
							pos:  position{line: 365, col: 24, offset: 10628},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 367, col: 1, offset: 10643},
			expr: &charClassMatcher{
				pos:        position{line: 367, col: 16, offset: 10658},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 369, col: 1, offset: 10669},
			expr: &charClassMatcher{
				pos:        position{line: 369, col: 16, offset: 10684},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 371, col: 1, offset: 10700},
			expr: &choiceExpr{
				pos: position{line: 371, col: 19, offset: 10718},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 371, col: 19, offset: 10718},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 371, col: 38, offset: 10737},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 373, col: 1, offset: 10752},
			expr: &charClassMatcher{
				pos:        position{line: 373, col: 21, offset: 10772},
				val:        "[\"\\\\/bfnrt]",
				chars:      []rune{'"', '\\', '/', 'b', 'f', 'n', 'r', 't'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 375, col: 1, offset: 10785},
			expr: &seqExpr{
				pos: position{line: 375, col: 18, offset: 10802},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 375, col: 18, offset: 10802},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 375, col: 22, offset: 10806},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 375, col: 31, offset: 10815},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 375, col: 40, offset: 10824},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 375, col: 49, offset: 10833},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 377, col: 1, offset: 10843},
			expr: &charClassMatcher{
				pos:        position{line: 377, col: 17, offset: 10859},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 379, col: 1, offset: 10866},
			expr: &charClassMatcher{
				pos:        position{line: 379, col: 24, offset: 10889},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 381, col: 1, offset: 10896},
			expr: &charClassMatcher{
				pos:        position{line: 381, col: 13, offset: 10908},
				val:        "[0-9a-f]",
				ranges:     []rune{'0', '9', 'a', 'f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 383, col: 1, offset: 10918},
			expr: &oneOrMoreExpr{
				pos: position{line: 383, col: 20, offset: 10937},
				expr: &charClassMatcher{
					pos:        position{line: 383, col: 20, offset: 10937},
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
			pos:         position{line: 385, col: 1, offset: 10949},
			expr: &zeroOrMoreExpr{
				pos: position{line: 385, col: 19, offset: 10967},
				expr: &choiceExpr{
					pos: position{line: 385, col: 21, offset: 10969},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 385, col: 21, offset: 10969},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 385, col: 33, offset: 10981},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 387, col: 1, offset: 10993},
			expr: &seqExpr{
				pos: position{line: 387, col: 12, offset: 11004},
				exprs: []interface{}{
					&zeroOrMoreExpr{
						pos: position{line: 387, col: 12, offset: 11004},
						expr: &charClassMatcher{
							pos:        position{line: 387, col: 12, offset: 11004},
							val:        "[ \\t]",
							chars:      []rune{' ', '\t'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&litMatcher{
						pos:        position{line: 387, col: 19, offset: 11011},
						val:        "#",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 387, col: 23, offset: 11015},
						expr: &charClassMatcher{
							pos:        position{line: 387, col: 23, offset: 11015},
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
			pos:  position{line: 389, col: 1, offset: 11025},
			expr: &notExpr{
				pos: position{line: 389, col: 8, offset: 11032},
				expr: &anyMatcher{
					line: 389, col: 9, offset: 11033,
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
	op := string(c.text)
	for _, b := range Builtins {
		if string(b.Infix) == op {
			op = string(b.Name)
		}
	}
	operator := VarTerm(op)
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

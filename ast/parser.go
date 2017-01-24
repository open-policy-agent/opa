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
	"strings"
	"unicode"
	"unicode/utf8"
)

// currentLocation converts the parser context to a Location object.
func currentLocation(c *current) *Location {
	return NewLocation(c.text, "", c.pos.line, c.pos.col)
}

func ifaceSliceToByteSlice(i interface{}) []byte {
	var buf bytes.Buffer
	for _, x := range i.([]interface{}) {
		buf.Write(x.([]byte))
	}
	return buf.Bytes()
}

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 19, col: 1, offset: 373},
			expr: &actionExpr{
				pos: position{line: 19, col: 12, offset: 384},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 19, col: 12, offset: 384},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 19, col: 12, offset: 384},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 19, col: 14, offset: 386},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 19, col: 19, offset: 391},
								expr: &seqExpr{
									pos: position{line: 19, col: 20, offset: 392},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 19, col: 20, offset: 392},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 19, col: 25, offset: 397},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 19, col: 30, offset: 402},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 19, col: 35, offset: 407},
												expr: &seqExpr{
													pos: position{line: 19, col: 36, offset: 408},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 19, col: 37, offset: 409},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 19, col: 37, offset: 409},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 19, col: 42, offset: 414},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 19, col: 54, offset: 426},
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
							pos:  position{line: 19, col: 63, offset: 435},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 19, col: 65, offset: 437},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 37, col: 1, offset: 774},
			expr: &actionExpr{
				pos: position{line: 37, col: 9, offset: 782},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 37, col: 9, offset: 782},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 37, col: 14, offset: 787},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 37, col: 14, offset: 787},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 37, col: 24, offset: 797},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 37, col: 33, offset: 806},
								name: "Rule",
							},
							&ruleRefExpr{
								pos:  position{line: 37, col: 40, offset: 813},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 37, col: 47, offset: 820},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 37, col: 57, offset: 830},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 46, col: 1, offset: 1194},
			expr: &actionExpr{
				pos: position{line: 46, col: 15, offset: 1208},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 46, col: 15, offset: 1208,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 50, col: 1, offset: 1281},
			expr: &actionExpr{
				pos: position{line: 50, col: 12, offset: 1292},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 50, col: 12, offset: 1292},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 50, col: 12, offset: 1292},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 50, col: 22, offset: 1302},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 50, col: 25, offset: 1305},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 50, col: 30, offset: 1310},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 50, col: 30, offset: 1310},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 50, col: 36, offset: 1316},
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
			pos:  position{line: 86, col: 1, offset: 2697},
			expr: &actionExpr{
				pos: position{line: 86, col: 11, offset: 2707},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 86, col: 11, offset: 2707},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 86, col: 11, offset: 2707},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 86, col: 20, offset: 2716},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 86, col: 23, offset: 2719},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 86, col: 29, offset: 2725},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 86, col: 29, offset: 2725},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 86, col: 35, offset: 2731},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 86, col: 40, offset: 2736},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 86, col: 46, offset: 2742},
								expr: &seqExpr{
									pos: position{line: 86, col: 47, offset: 2743},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 86, col: 47, offset: 2743},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 86, col: 50, offset: 2746},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 86, col: 55, offset: 2751},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 86, col: 58, offset: 2754},
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
			pos:  position{line: 102, col: 1, offset: 3204},
			expr: &actionExpr{
				pos: position{line: 102, col: 9, offset: 3212},
				run: (*parser).callonRule1,
				expr: &seqExpr{
					pos: position{line: 102, col: 9, offset: 3212},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 102, col: 9, offset: 3212},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 102, col: 14, offset: 3217},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 102, col: 18, offset: 3221},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 102, col: 22, offset: 3225},
								expr: &seqExpr{
									pos: position{line: 102, col: 24, offset: 3227},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 102, col: 24, offset: 3227},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 102, col: 26, offset: 3229},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 30, offset: 3233},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 32, offset: 3235},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 37, offset: 3240},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 102, col: 39, offset: 3242},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 43, offset: 3246},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 102, col: 48, offset: 3251},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 102, col: 54, offset: 3257},
								expr: &seqExpr{
									pos: position{line: 102, col: 56, offset: 3259},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 102, col: 56, offset: 3259},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 102, col: 58, offset: 3261},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 62, offset: 3265},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 102, col: 64, offset: 3267},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 102, col: 72, offset: 3275},
							label: "body",
							expr: &seqExpr{
								pos: position{line: 102, col: 79, offset: 3282},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 102, col: 79, offset: 3282},
										name: "_",
									},
									&litMatcher{
										pos:        position{line: 102, col: 81, offset: 3284},
										val:        ":-",
										ignoreCase: false,
									},
									&ruleRefExpr{
										pos:  position{line: 102, col: 86, offset: 3289},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 102, col: 88, offset: 3291},
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
			pos:  position{line: 158, col: 1, offset: 4922},
			expr: &actionExpr{
				pos: position{line: 158, col: 9, offset: 4930},
				run: (*parser).callonBody1,
				expr: &seqExpr{
					pos: position{line: 158, col: 9, offset: 4930},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 158, col: 9, offset: 4930},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 158, col: 14, offset: 4935},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 158, col: 19, offset: 4940},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 158, col: 24, offset: 4945},
								expr: &seqExpr{
									pos: position{line: 158, col: 26, offset: 4947},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 158, col: 26, offset: 4947},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 158, col: 28, offset: 4949},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 158, col: 32, offset: 4953},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 158, col: 35, offset: 4956},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 158, col: 35, offset: 4956},
													name: "Expr",
												},
												&ruleRefExpr{
													pos:  position{line: 158, col: 42, offset: 4963},
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
			pos:  position{line: 168, col: 1, offset: 5183},
			expr: &actionExpr{
				pos: position{line: 168, col: 9, offset: 5191},
				run: (*parser).callonExpr1,
				expr: &seqExpr{
					pos: position{line: 168, col: 9, offset: 5191},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 168, col: 9, offset: 5191},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 168, col: 13, offset: 5195},
								expr: &seqExpr{
									pos: position{line: 168, col: 15, offset: 5197},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 168, col: 15, offset: 5197},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 168, col: 21, offset: 5203},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 168, col: 27, offset: 5209},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 168, col: 32, offset: 5214},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 168, col: 32, offset: 5214},
										name: "InfixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 168, col: 44, offset: 5226},
										name: "PrefixExpr",
									},
									&ruleRefExpr{
										pos:  position{line: 168, col: 57, offset: 5239},
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
			pos:  position{line: 176, col: 1, offset: 5381},
			expr: &actionExpr{
				pos: position{line: 176, col: 14, offset: 5394},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 176, col: 14, offset: 5394},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 176, col: 14, offset: 5394},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 176, col: 19, offset: 5399},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 176, col: 24, offset: 5404},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 176, col: 26, offset: 5406},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 176, col: 29, offset: 5409},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 176, col: 37, offset: 5417},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 176, col: 39, offset: 5419},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 176, col: 45, offset: 5425},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 180, col: 1, offset: 5500},
			expr: &actionExpr{
				pos: position{line: 180, col: 12, offset: 5511},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 180, col: 12, offset: 5511},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 180, col: 17, offset: 5516},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 180, col: 17, offset: 5516},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 180, col: 23, offset: 5522},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 180, col: 30, offset: 5529},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 180, col: 37, offset: 5536},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 180, col: 44, offset: 5543},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 180, col: 50, offset: 5549},
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
			pos:  position{line: 192, col: 1, offset: 5793},
			expr: &choiceExpr{
				pos: position{line: 192, col: 15, offset: 5807},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 192, col: 15, offset: 5807},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 192, col: 26, offset: 5818},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 194, col: 1, offset: 5827},
			expr: &actionExpr{
				pos: position{line: 194, col: 12, offset: 5838},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 194, col: 12, offset: 5838},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 194, col: 12, offset: 5838},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 194, col: 15, offset: 5841},
								name: "Var",
							},
						},
						&litMatcher{
							pos:        position{line: 194, col: 19, offset: 5845},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 194, col: 23, offset: 5849},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 194, col: 25, offset: 5851},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 194, col: 30, offset: 5856},
								expr: &ruleRefExpr{
									pos:  position{line: 194, col: 30, offset: 5856},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 194, col: 36, offset: 5862},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 194, col: 41, offset: 5867},
								expr: &seqExpr{
									pos: position{line: 194, col: 43, offset: 5869},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 194, col: 43, offset: 5869},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 194, col: 45, offset: 5871},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 194, col: 49, offset: 5875},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 194, col: 51, offset: 5877},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 194, col: 59, offset: 5885},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 194, col: 62, offset: 5888},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 210, col: 1, offset: 6290},
			expr: &actionExpr{
				pos: position{line: 210, col: 9, offset: 6298},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 210, col: 9, offset: 6298},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 210, col: 15, offset: 6304},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 210, col: 15, offset: 6304},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 210, col: 31, offset: 6320},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 210, col: 43, offset: 6332},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 210, col: 52, offset: 6341},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 210, col: 58, offset: 6347},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 214, col: 1, offset: 6378},
			expr: &ruleRefExpr{
				pos:  position{line: 214, col: 18, offset: 6395},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 216, col: 1, offset: 6415},
			expr: &actionExpr{
				pos: position{line: 216, col: 23, offset: 6437},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 216, col: 23, offset: 6437},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 216, col: 23, offset: 6437},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 27, offset: 6441},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 216, col: 29, offset: 6443},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 216, col: 34, offset: 6448},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 39, offset: 6453},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 216, col: 41, offset: 6455},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 45, offset: 6459},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 216, col: 47, offset: 6461},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 216, col: 52, offset: 6466},
								name: "Body",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 57, offset: 6471},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 216, col: 59, offset: 6473},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 222, col: 1, offset: 6598},
			expr: &choiceExpr{
				pos: position{line: 222, col: 14, offset: 6611},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 222, col: 14, offset: 6611},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 222, col: 23, offset: 6620},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 222, col: 31, offset: 6628},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 224, col: 1, offset: 6633},
			expr: &choiceExpr{
				pos: position{line: 224, col: 11, offset: 6643},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 224, col: 11, offset: 6643},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 224, col: 20, offset: 6652},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 224, col: 29, offset: 6661},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 224, col: 36, offset: 6668},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 226, col: 1, offset: 6674},
			expr: &choiceExpr{
				pos: position{line: 226, col: 8, offset: 6681},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 226, col: 8, offset: 6681},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 226, col: 17, offset: 6690},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 226, col: 23, offset: 6696},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 228, col: 1, offset: 6701},
			expr: &actionExpr{
				pos: position{line: 228, col: 11, offset: 6711},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 228, col: 11, offset: 6711},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 228, col: 11, offset: 6711},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 228, col: 15, offset: 6715},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 228, col: 17, offset: 6717},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 228, col: 22, offset: 6722},
								expr: &seqExpr{
									pos: position{line: 228, col: 23, offset: 6723},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 228, col: 23, offset: 6723},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 27, offset: 6727},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 228, col: 29, offset: 6729},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 33, offset: 6733},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 35, offset: 6735},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 228, col: 42, offset: 6742},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 228, col: 47, offset: 6747},
								expr: &seqExpr{
									pos: position{line: 228, col: 49, offset: 6749},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 228, col: 49, offset: 6749},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 228, col: 51, offset: 6751},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 55, offset: 6755},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 57, offset: 6757},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 61, offset: 6761},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 228, col: 63, offset: 6763},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 67, offset: 6767},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 69, offset: 6769},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 228, col: 77, offset: 6777},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 228, col: 79, offset: 6779},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 252, col: 1, offset: 7558},
			expr: &actionExpr{
				pos: position{line: 252, col: 10, offset: 7567},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 252, col: 10, offset: 7567},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 252, col: 10, offset: 7567},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 252, col: 14, offset: 7571},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 252, col: 17, offset: 7574},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 252, col: 22, offset: 7579},
								expr: &ruleRefExpr{
									pos:  position{line: 252, col: 22, offset: 7579},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 252, col: 28, offset: 7585},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 252, col: 33, offset: 7590},
								expr: &seqExpr{
									pos: position{line: 252, col: 34, offset: 7591},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 252, col: 34, offset: 7591},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 252, col: 36, offset: 7593},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 40, offset: 7597},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 42, offset: 7599},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 252, col: 49, offset: 7606},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 252, col: 51, offset: 7608},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 276, col: 1, offset: 8181},
			expr: &choiceExpr{
				pos: position{line: 276, col: 8, offset: 8188},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 276, col: 8, offset: 8188},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 276, col: 19, offset: 8199},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 278, col: 1, offset: 8212},
			expr: &actionExpr{
				pos: position{line: 278, col: 13, offset: 8224},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 278, col: 13, offset: 8224},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 278, col: 13, offset: 8224},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 278, col: 20, offset: 8231},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 278, col: 22, offset: 8233},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 284, col: 1, offset: 8321},
			expr: &actionExpr{
				pos: position{line: 284, col: 16, offset: 8336},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 284, col: 16, offset: 8336},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 284, col: 16, offset: 8336},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 284, col: 20, offset: 8340},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 284, col: 22, offset: 8342},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 284, col: 27, offset: 8347},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 284, col: 32, offset: 8352},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 284, col: 37, offset: 8357},
								expr: &seqExpr{
									pos: position{line: 284, col: 38, offset: 8358},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 284, col: 38, offset: 8358},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 284, col: 40, offset: 8360},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 284, col: 44, offset: 8364},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 284, col: 46, offset: 8366},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 284, col: 53, offset: 8373},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 284, col: 55, offset: 8375},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 301, col: 1, offset: 8780},
			expr: &actionExpr{
				pos: position{line: 301, col: 8, offset: 8787},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 301, col: 8, offset: 8787},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 301, col: 8, offset: 8787},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 301, col: 13, offset: 8792},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 301, col: 17, offset: 8796},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 301, col: 22, offset: 8801},
								expr: &choiceExpr{
									pos: position{line: 301, col: 24, offset: 8803},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 301, col: 24, offset: 8803},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 33, offset: 8812},
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
			pos:  position{line: 314, col: 1, offset: 9051},
			expr: &actionExpr{
				pos: position{line: 314, col: 11, offset: 9061},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 314, col: 11, offset: 9061},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 314, col: 11, offset: 9061},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 314, col: 15, offset: 9065},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 314, col: 19, offset: 9069},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 321, col: 1, offset: 9288},
			expr: &actionExpr{
				pos: position{line: 321, col: 15, offset: 9302},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 321, col: 15, offset: 9302},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 321, col: 15, offset: 9302},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 321, col: 19, offset: 9306},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 321, col: 24, offset: 9311},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 321, col: 24, offset: 9311},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 321, col: 30, offset: 9317},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 321, col: 39, offset: 9326},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 321, col: 44, offset: 9331},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 325, col: 1, offset: 9360},
			expr: &actionExpr{
				pos: position{line: 325, col: 8, offset: 9367},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 325, col: 8, offset: 9367},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 325, col: 12, offset: 9371},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 330, col: 1, offset: 9493},
			expr: &seqExpr{
				pos: position{line: 330, col: 15, offset: 9507},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 330, col: 15, offset: 9507},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 330, col: 19, offset: 9511},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 330, col: 32, offset: 9524},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 334, col: 1, offset: 9589},
			expr: &actionExpr{
				pos: position{line: 334, col: 17, offset: 9605},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 334, col: 17, offset: 9605},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 334, col: 17, offset: 9605},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 334, col: 29, offset: 9617},
							expr: &choiceExpr{
								pos: position{line: 334, col: 30, offset: 9618},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 334, col: 30, offset: 9618},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 334, col: 44, offset: 9632},
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
			pos:  position{line: 341, col: 1, offset: 9775},
			expr: &actionExpr{
				pos: position{line: 341, col: 11, offset: 9785},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 341, col: 11, offset: 9785},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 341, col: 11, offset: 9785},
							expr: &litMatcher{
								pos:        position{line: 341, col: 11, offset: 9785},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 341, col: 18, offset: 9792},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 341, col: 18, offset: 9792},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 341, col: 26, offset: 9800},
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
			pos:  position{line: 354, col: 1, offset: 10191},
			expr: &choiceExpr{
				pos: position{line: 354, col: 10, offset: 10200},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 354, col: 10, offset: 10200},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 354, col: 26, offset: 10216},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 356, col: 1, offset: 10228},
			expr: &seqExpr{
				pos: position{line: 356, col: 18, offset: 10245},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 356, col: 20, offset: 10247},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 356, col: 20, offset: 10247},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 356, col: 33, offset: 10260},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 356, col: 43, offset: 10270},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 358, col: 1, offset: 10280},
			expr: &seqExpr{
				pos: position{line: 358, col: 15, offset: 10294},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 358, col: 15, offset: 10294},
						expr: &ruleRefExpr{
							pos:  position{line: 358, col: 15, offset: 10294},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 358, col: 24, offset: 10303},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 360, col: 1, offset: 10313},
			expr: &seqExpr{
				pos: position{line: 360, col: 13, offset: 10325},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 360, col: 13, offset: 10325},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 360, col: 17, offset: 10329},
						expr: &ruleRefExpr{
							pos:  position{line: 360, col: 17, offset: 10329},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 362, col: 1, offset: 10344},
			expr: &seqExpr{
				pos: position{line: 362, col: 13, offset: 10356},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 362, col: 13, offset: 10356},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 362, col: 18, offset: 10361},
						expr: &charClassMatcher{
							pos:        position{line: 362, col: 18, offset: 10361},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 362, col: 24, offset: 10367},
						expr: &ruleRefExpr{
							pos:  position{line: 362, col: 24, offset: 10367},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 364, col: 1, offset: 10382},
			expr: &choiceExpr{
				pos: position{line: 364, col: 12, offset: 10393},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 364, col: 12, offset: 10393},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 364, col: 20, offset: 10401},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 364, col: 20, offset: 10401},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 364, col: 40, offset: 10421},
								expr: &ruleRefExpr{
									pos:  position{line: 364, col: 40, offset: 10421},
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
			pos:  position{line: 366, col: 1, offset: 10438},
			expr: &actionExpr{
				pos: position{line: 366, col: 11, offset: 10448},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 366, col: 11, offset: 10448},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 366, col: 11, offset: 10448},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 366, col: 15, offset: 10452},
							expr: &ruleRefExpr{
								pos:  position{line: 366, col: 15, offset: 10452},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 366, col: 21, offset: 10458},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 374, col: 1, offset: 10613},
			expr: &choiceExpr{
				pos: position{line: 374, col: 9, offset: 10621},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 374, col: 9, offset: 10621},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 374, col: 9, offset: 10621},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 378, col: 5, offset: 10721},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 378, col: 5, offset: 10721},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 384, col: 1, offset: 10822},
			expr: &actionExpr{
				pos: position{line: 384, col: 9, offset: 10830},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 384, col: 9, offset: 10830},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 390, col: 1, offset: 10925},
			expr: &charClassMatcher{
				pos:        position{line: 390, col: 16, offset: 10940},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 392, col: 1, offset: 10951},
			expr: &choiceExpr{
				pos: position{line: 392, col: 9, offset: 10959},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 392, col: 11, offset: 10961},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 392, col: 11, offset: 10961},
								expr: &ruleRefExpr{
									pos:  position{line: 392, col: 12, offset: 10962},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 392, col: 24, offset: 10974,
							},
						},
					},
					&seqExpr{
						pos: position{line: 392, col: 32, offset: 10982},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 392, col: 32, offset: 10982},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 392, col: 37, offset: 10987},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 394, col: 1, offset: 11005},
			expr: &charClassMatcher{
				pos:        position{line: 394, col: 16, offset: 11020},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 396, col: 1, offset: 11036},
			expr: &choiceExpr{
				pos: position{line: 396, col: 19, offset: 11054},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 396, col: 19, offset: 11054},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 396, col: 38, offset: 11073},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 398, col: 1, offset: 11088},
			expr: &charClassMatcher{
				pos:        position{line: 398, col: 21, offset: 11108},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 400, col: 1, offset: 11130},
			expr: &seqExpr{
				pos: position{line: 400, col: 18, offset: 11147},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 400, col: 18, offset: 11147},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 400, col: 22, offset: 11151},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 400, col: 31, offset: 11160},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 400, col: 40, offset: 11169},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 400, col: 49, offset: 11178},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 402, col: 1, offset: 11188},
			expr: &charClassMatcher{
				pos:        position{line: 402, col: 17, offset: 11204},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 404, col: 1, offset: 11211},
			expr: &charClassMatcher{
				pos:        position{line: 404, col: 24, offset: 11234},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 406, col: 1, offset: 11241},
			expr: &charClassMatcher{
				pos:        position{line: 406, col: 13, offset: 11253},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 408, col: 1, offset: 11266},
			expr: &oneOrMoreExpr{
				pos: position{line: 408, col: 20, offset: 11285},
				expr: &charClassMatcher{
					pos:        position{line: 408, col: 20, offset: 11285},
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
			pos:         position{line: 410, col: 1, offset: 11297},
			expr: &zeroOrMoreExpr{
				pos: position{line: 410, col: 19, offset: 11315},
				expr: &choiceExpr{
					pos: position{line: 410, col: 21, offset: 11317},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 410, col: 21, offset: 11317},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 410, col: 33, offset: 11329},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 412, col: 1, offset: 11341},
			expr: &actionExpr{
				pos: position{line: 412, col: 12, offset: 11352},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 412, col: 12, offset: 11352},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 412, col: 12, offset: 11352},
							expr: &charClassMatcher{
								pos:        position{line: 412, col: 12, offset: 11352},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 412, col: 19, offset: 11359},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 412, col: 23, offset: 11363},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 412, col: 28, offset: 11368},
								expr: &charClassMatcher{
									pos:        position{line: 412, col: 28, offset: 11368},
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
			pos:  position{line: 418, col: 1, offset: 11503},
			expr: &notExpr{
				pos: position{line: 418, col: 8, offset: 11510},
				expr: &anyMatcher{
					line: 418, col: 9, offset: 11511,
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

func (c *current) onRule1(name, key, value, body interface{}) (interface{}, error) {

	rule := &Rule{}
	rule.Location = currentLocation(c)
	rule.Name = name.(*Term).Value.(Var)

	if key != nil {
		keySlice := key.([]interface{})
		// Rule definition above describes the "key" slice. We care about the "Term" element.
		rule.Key = keySlice[3].(*Term)

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
		case Var, String, Ref: // nop
		default:
			return nil, fmt.Errorf("head of object rule must have string, var, or ref key (%s is not allowed)", rule.Key)
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

func (c *current) onBuiltin1(op, head, tail interface{}) (interface{}, error) {
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

func (p *parser) callonBuiltin1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onBuiltin1(stack["op"], stack["head"], stack["tail"])
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

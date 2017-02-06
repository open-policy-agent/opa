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
			expr: &choiceExpr{
				pos: position{line: 102, col: 9, offset: 3212},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 102, col: 9, offset: 3212},
						name: "DefaultRule",
					},
					&ruleRefExpr{
						pos:  position{line: 102, col: 23, offset: 3226},
						name: "NoDefaultRule",
					},
				},
			},
		},
		{
			name: "DefaultRule",
			pos:  position{line: 104, col: 1, offset: 3241},
			expr: &actionExpr{
				pos: position{line: 104, col: 16, offset: 3256},
				run: (*parser).callonDefaultRule1,
				expr: &seqExpr{
					pos: position{line: 104, col: 16, offset: 3256},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 104, col: 16, offset: 3256},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 104, col: 26, offset: 3266},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 104, col: 29, offset: 3269},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 104, col: 34, offset: 3274},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 104, col: 38, offset: 3278},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 104, col: 40, offset: 3280},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 104, col: 44, offset: 3284},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 104, col: 46, offset: 3286},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 104, col: 52, offset: 3292},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NoDefaultRule",
			pos:  position{line: 144, col: 1, offset: 4124},
			expr: &actionExpr{
				pos: position{line: 144, col: 18, offset: 4141},
				run: (*parser).callonNoDefaultRule1,
				expr: &seqExpr{
					pos: position{line: 144, col: 18, offset: 4141},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 144, col: 18, offset: 4141},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 144, col: 23, offset: 4146},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 144, col: 27, offset: 4150},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 144, col: 31, offset: 4154},
								expr: &seqExpr{
									pos: position{line: 144, col: 33, offset: 4156},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 144, col: 33, offset: 4156},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 144, col: 35, offset: 4158},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 39, offset: 4162},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 41, offset: 4164},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 46, offset: 4169},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 144, col: 48, offset: 4171},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 52, offset: 4175},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 144, col: 57, offset: 4180},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 144, col: 63, offset: 4186},
								expr: &seqExpr{
									pos: position{line: 144, col: 65, offset: 4188},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 144, col: 65, offset: 4188},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 144, col: 67, offset: 4190},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 71, offset: 4194},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 144, col: 73, offset: 4196},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 144, col: 81, offset: 4204},
							label: "body",
							expr: &seqExpr{
								pos: position{line: 144, col: 88, offset: 4211},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 144, col: 88, offset: 4211},
										name: "_",
									},
									&litMatcher{
										pos:        position{line: 144, col: 90, offset: 4213},
										val:        ":-",
										ignoreCase: false,
									},
									&ruleRefExpr{
										pos:  position{line: 144, col: 95, offset: 4218},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 144, col: 97, offset: 4220},
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
			pos:  position{line: 183, col: 1, offset: 5369},
			expr: &actionExpr{
				pos: position{line: 183, col: 9, offset: 5377},
				run: (*parser).callonBody1,
				expr: &seqExpr{
					pos: position{line: 183, col: 9, offset: 5377},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 183, col: 9, offset: 5377},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 183, col: 14, offset: 5382},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 183, col: 22, offset: 5390},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 183, col: 27, offset: 5395},
								expr: &seqExpr{
									pos: position{line: 183, col: 29, offset: 5397},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 183, col: 29, offset: 5397},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 183, col: 31, offset: 5399},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 183, col: 35, offset: 5403},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 183, col: 38, offset: 5406},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 183, col: 38, offset: 5406},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 183, col: 48, offset: 5416},
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
			pos:  position{line: 193, col: 1, offset: 5636},
			expr: &actionExpr{
				pos: position{line: 193, col: 12, offset: 5647},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 193, col: 12, offset: 5647},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 193, col: 12, offset: 5647},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 193, col: 16, offset: 5651},
								expr: &seqExpr{
									pos: position{line: 193, col: 18, offset: 5653},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 193, col: 18, offset: 5653},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 193, col: 24, offset: 5659},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 193, col: 30, offset: 5665},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 193, col: 34, offset: 5669},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 193, col: 39, offset: 5674},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 193, col: 44, offset: 5679},
								expr: &seqExpr{
									pos: position{line: 193, col: 46, offset: 5681},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 193, col: 46, offset: 5681},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 193, col: 49, offset: 5684},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 193, col: 54, offset: 5689},
											expr: &seqExpr{
												pos: position{line: 193, col: 55, offset: 5690},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 193, col: 55, offset: 5690},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 193, col: 58, offset: 5693},
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
			pos:  position{line: 216, col: 1, offset: 6265},
			expr: &actionExpr{
				pos: position{line: 216, col: 9, offset: 6273},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 216, col: 9, offset: 6273},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 216, col: 9, offset: 6273},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 16, offset: 6280},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 216, col: 19, offset: 6283},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 216, col: 26, offset: 6290},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 31, offset: 6295},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 216, col: 34, offset: 6298},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 216, col: 39, offset: 6303},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 216, col: 42, offset: 6306},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 216, col: 48, offset: 6312},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 227, col: 1, offset: 6561},
			expr: &choiceExpr{
				pos: position{line: 227, col: 9, offset: 6569},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 227, col: 10, offset: 6570},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 227, col: 10, offset: 6570},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 227, col: 27, offset: 6587},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 227, col: 52, offset: 6612},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 227, col: 64, offset: 6624},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 227, col: 77, offset: 6637},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 229, col: 1, offset: 6643},
			expr: &actionExpr{
				pos: position{line: 229, col: 19, offset: 6661},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 229, col: 19, offset: 6661},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 229, col: 19, offset: 6661},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 229, col: 26, offset: 6668},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 229, col: 31, offset: 6673},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 229, col: 33, offset: 6675},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 229, col: 37, offset: 6679},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 229, col: 39, offset: 6681},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 229, col: 44, offset: 6686},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 229, col: 49, offset: 6691},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 229, col: 51, offset: 6693},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 229, col: 54, offset: 6696},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 229, col: 67, offset: 6709},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 229, col: 69, offset: 6711},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 229, col: 75, offset: 6717},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 233, col: 1, offset: 6808},
			expr: &actionExpr{
				pos: position{line: 233, col: 26, offset: 6833},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 233, col: 26, offset: 6833},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 233, col: 26, offset: 6833},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 233, col: 31, offset: 6838},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 36, offset: 6843},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 233, col: 38, offset: 6845},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 233, col: 41, offset: 6848},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 54, offset: 6861},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 233, col: 56, offset: 6863},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 233, col: 62, offset: 6869},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 67, offset: 6874},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 233, col: 69, offset: 6876},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 233, col: 73, offset: 6880},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 233, col: 75, offset: 6882},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 233, col: 82, offset: 6889},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 237, col: 1, offset: 6980},
			expr: &actionExpr{
				pos: position{line: 237, col: 17, offset: 6996},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 237, col: 17, offset: 6996},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 237, col: 22, offset: 7001},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 237, col: 22, offset: 7001},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 28, offset: 7007},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 34, offset: 7013},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 40, offset: 7019},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 46, offset: 7025},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 52, offset: 7031},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 237, col: 58, offset: 7037},
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
			pos:  position{line: 249, col: 1, offset: 7281},
			expr: &actionExpr{
				pos: position{line: 249, col: 14, offset: 7294},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 249, col: 14, offset: 7294},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 249, col: 14, offset: 7294},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 249, col: 19, offset: 7299},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 249, col: 24, offset: 7304},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 249, col: 26, offset: 7306},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 249, col: 29, offset: 7309},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 249, col: 37, offset: 7317},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 249, col: 39, offset: 7319},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 249, col: 45, offset: 7325},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 253, col: 1, offset: 7400},
			expr: &actionExpr{
				pos: position{line: 253, col: 12, offset: 7411},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 253, col: 12, offset: 7411},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 253, col: 17, offset: 7416},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 253, col: 17, offset: 7416},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 253, col: 23, offset: 7422},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 253, col: 30, offset: 7429},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 253, col: 37, offset: 7436},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 253, col: 44, offset: 7443},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 253, col: 50, offset: 7449},
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
			pos:  position{line: 265, col: 1, offset: 7693},
			expr: &choiceExpr{
				pos: position{line: 265, col: 15, offset: 7707},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 265, col: 15, offset: 7707},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 265, col: 26, offset: 7718},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 267, col: 1, offset: 7727},
			expr: &actionExpr{
				pos: position{line: 267, col: 12, offset: 7738},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 267, col: 12, offset: 7738},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 267, col: 12, offset: 7738},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 267, col: 15, offset: 7741},
								name: "Var",
							},
						},
						&litMatcher{
							pos:        position{line: 267, col: 19, offset: 7745},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 267, col: 23, offset: 7749},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 267, col: 25, offset: 7751},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 267, col: 30, offset: 7756},
								expr: &ruleRefExpr{
									pos:  position{line: 267, col: 30, offset: 7756},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 267, col: 36, offset: 7762},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 267, col: 41, offset: 7767},
								expr: &seqExpr{
									pos: position{line: 267, col: 43, offset: 7769},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 267, col: 43, offset: 7769},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 267, col: 45, offset: 7771},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 267, col: 49, offset: 7775},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 267, col: 51, offset: 7777},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 267, col: 59, offset: 7785},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 267, col: 62, offset: 7788},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 283, col: 1, offset: 8190},
			expr: &actionExpr{
				pos: position{line: 283, col: 9, offset: 8198},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 283, col: 9, offset: 8198},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 283, col: 15, offset: 8204},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 283, col: 15, offset: 8204},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 31, offset: 8220},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 43, offset: 8232},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 52, offset: 8241},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 58, offset: 8247},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 287, col: 1, offset: 8278},
			expr: &ruleRefExpr{
				pos:  position{line: 287, col: 18, offset: 8295},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 289, col: 1, offset: 8315},
			expr: &actionExpr{
				pos: position{line: 289, col: 23, offset: 8337},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 289, col: 23, offset: 8337},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 289, col: 23, offset: 8337},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 289, col: 27, offset: 8341},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 289, col: 29, offset: 8343},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 289, col: 34, offset: 8348},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 289, col: 39, offset: 8353},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 289, col: 41, offset: 8355},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 289, col: 45, offset: 8359},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 289, col: 47, offset: 8361},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 289, col: 52, offset: 8366},
								name: "Body",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 289, col: 57, offset: 8371},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 289, col: 59, offset: 8373},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 295, col: 1, offset: 8498},
			expr: &choiceExpr{
				pos: position{line: 295, col: 14, offset: 8511},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 295, col: 14, offset: 8511},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 295, col: 23, offset: 8520},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 295, col: 31, offset: 8528},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 297, col: 1, offset: 8533},
			expr: &choiceExpr{
				pos: position{line: 297, col: 11, offset: 8543},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 297, col: 11, offset: 8543},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 297, col: 20, offset: 8552},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 297, col: 29, offset: 8561},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 297, col: 36, offset: 8568},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 299, col: 1, offset: 8574},
			expr: &choiceExpr{
				pos: position{line: 299, col: 8, offset: 8581},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 299, col: 8, offset: 8581},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 299, col: 17, offset: 8590},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 299, col: 23, offset: 8596},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 301, col: 1, offset: 8601},
			expr: &actionExpr{
				pos: position{line: 301, col: 11, offset: 8611},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 301, col: 11, offset: 8611},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 301, col: 11, offset: 8611},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 301, col: 15, offset: 8615},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 301, col: 17, offset: 8617},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 301, col: 22, offset: 8622},
								expr: &seqExpr{
									pos: position{line: 301, col: 23, offset: 8623},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 301, col: 23, offset: 8623},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 27, offset: 8627},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 301, col: 29, offset: 8629},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 33, offset: 8633},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 35, offset: 8635},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 301, col: 42, offset: 8642},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 301, col: 47, offset: 8647},
								expr: &seqExpr{
									pos: position{line: 301, col: 49, offset: 8649},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 301, col: 49, offset: 8649},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 301, col: 51, offset: 8651},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 55, offset: 8655},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 57, offset: 8657},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 61, offset: 8661},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 301, col: 63, offset: 8663},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 67, offset: 8667},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 301, col: 69, offset: 8669},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 301, col: 77, offset: 8677},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 301, col: 79, offset: 8679},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 325, col: 1, offset: 9458},
			expr: &actionExpr{
				pos: position{line: 325, col: 10, offset: 9467},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 325, col: 10, offset: 9467},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 325, col: 10, offset: 9467},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 325, col: 14, offset: 9471},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 325, col: 17, offset: 9474},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 325, col: 22, offset: 9479},
								expr: &ruleRefExpr{
									pos:  position{line: 325, col: 22, offset: 9479},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 325, col: 28, offset: 9485},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 325, col: 33, offset: 9490},
								expr: &seqExpr{
									pos: position{line: 325, col: 34, offset: 9491},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 325, col: 34, offset: 9491},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 325, col: 36, offset: 9493},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 325, col: 40, offset: 9497},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 325, col: 42, offset: 9499},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 325, col: 49, offset: 9506},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 325, col: 51, offset: 9508},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 349, col: 1, offset: 10081},
			expr: &choiceExpr{
				pos: position{line: 349, col: 8, offset: 10088},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 349, col: 8, offset: 10088},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 349, col: 19, offset: 10099},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 351, col: 1, offset: 10112},
			expr: &actionExpr{
				pos: position{line: 351, col: 13, offset: 10124},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 351, col: 13, offset: 10124},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 351, col: 13, offset: 10124},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 351, col: 20, offset: 10131},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 351, col: 22, offset: 10133},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 357, col: 1, offset: 10221},
			expr: &actionExpr{
				pos: position{line: 357, col: 16, offset: 10236},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 357, col: 16, offset: 10236},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 357, col: 16, offset: 10236},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 357, col: 20, offset: 10240},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 357, col: 22, offset: 10242},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 357, col: 27, offset: 10247},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 357, col: 32, offset: 10252},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 357, col: 37, offset: 10257},
								expr: &seqExpr{
									pos: position{line: 357, col: 38, offset: 10258},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 357, col: 38, offset: 10258},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 357, col: 40, offset: 10260},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 357, col: 44, offset: 10264},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 357, col: 46, offset: 10266},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 357, col: 53, offset: 10273},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 357, col: 55, offset: 10275},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 374, col: 1, offset: 10680},
			expr: &actionExpr{
				pos: position{line: 374, col: 8, offset: 10687},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 374, col: 8, offset: 10687},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 374, col: 8, offset: 10687},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 374, col: 13, offset: 10692},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 374, col: 17, offset: 10696},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 374, col: 22, offset: 10701},
								expr: &choiceExpr{
									pos: position{line: 374, col: 24, offset: 10703},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 374, col: 24, offset: 10703},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 374, col: 33, offset: 10712},
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
			pos:  position{line: 387, col: 1, offset: 10951},
			expr: &actionExpr{
				pos: position{line: 387, col: 11, offset: 10961},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 387, col: 11, offset: 10961},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 387, col: 11, offset: 10961},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 387, col: 15, offset: 10965},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 387, col: 19, offset: 10969},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 394, col: 1, offset: 11188},
			expr: &actionExpr{
				pos: position{line: 394, col: 15, offset: 11202},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 394, col: 15, offset: 11202},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 394, col: 15, offset: 11202},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 394, col: 19, offset: 11206},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 394, col: 24, offset: 11211},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 394, col: 24, offset: 11211},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 394, col: 30, offset: 11217},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 394, col: 39, offset: 11226},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 394, col: 44, offset: 11231},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 398, col: 1, offset: 11260},
			expr: &actionExpr{
				pos: position{line: 398, col: 8, offset: 11267},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 398, col: 8, offset: 11267},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 398, col: 12, offset: 11271},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 403, col: 1, offset: 11393},
			expr: &seqExpr{
				pos: position{line: 403, col: 15, offset: 11407},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 403, col: 15, offset: 11407},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 403, col: 19, offset: 11411},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 403, col: 32, offset: 11424},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 407, col: 1, offset: 11489},
			expr: &actionExpr{
				pos: position{line: 407, col: 17, offset: 11505},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 407, col: 17, offset: 11505},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 407, col: 17, offset: 11505},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 407, col: 29, offset: 11517},
							expr: &choiceExpr{
								pos: position{line: 407, col: 30, offset: 11518},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 407, col: 30, offset: 11518},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 407, col: 44, offset: 11532},
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
			pos:  position{line: 414, col: 1, offset: 11675},
			expr: &actionExpr{
				pos: position{line: 414, col: 11, offset: 11685},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 414, col: 11, offset: 11685},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 414, col: 11, offset: 11685},
							expr: &litMatcher{
								pos:        position{line: 414, col: 11, offset: 11685},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 414, col: 18, offset: 11692},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 414, col: 18, offset: 11692},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 414, col: 26, offset: 11700},
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
			pos:  position{line: 427, col: 1, offset: 12091},
			expr: &choiceExpr{
				pos: position{line: 427, col: 10, offset: 12100},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 427, col: 10, offset: 12100},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 427, col: 26, offset: 12116},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 429, col: 1, offset: 12128},
			expr: &seqExpr{
				pos: position{line: 429, col: 18, offset: 12145},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 429, col: 20, offset: 12147},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 429, col: 20, offset: 12147},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 429, col: 33, offset: 12160},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 429, col: 43, offset: 12170},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 431, col: 1, offset: 12180},
			expr: &seqExpr{
				pos: position{line: 431, col: 15, offset: 12194},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 431, col: 15, offset: 12194},
						expr: &ruleRefExpr{
							pos:  position{line: 431, col: 15, offset: 12194},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 431, col: 24, offset: 12203},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 433, col: 1, offset: 12213},
			expr: &seqExpr{
				pos: position{line: 433, col: 13, offset: 12225},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 433, col: 13, offset: 12225},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 433, col: 17, offset: 12229},
						expr: &ruleRefExpr{
							pos:  position{line: 433, col: 17, offset: 12229},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 435, col: 1, offset: 12244},
			expr: &seqExpr{
				pos: position{line: 435, col: 13, offset: 12256},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 435, col: 13, offset: 12256},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 435, col: 18, offset: 12261},
						expr: &charClassMatcher{
							pos:        position{line: 435, col: 18, offset: 12261},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 435, col: 24, offset: 12267},
						expr: &ruleRefExpr{
							pos:  position{line: 435, col: 24, offset: 12267},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 437, col: 1, offset: 12282},
			expr: &choiceExpr{
				pos: position{line: 437, col: 12, offset: 12293},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 437, col: 12, offset: 12293},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 437, col: 20, offset: 12301},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 437, col: 20, offset: 12301},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 437, col: 40, offset: 12321},
								expr: &ruleRefExpr{
									pos:  position{line: 437, col: 40, offset: 12321},
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
			pos:  position{line: 439, col: 1, offset: 12338},
			expr: &actionExpr{
				pos: position{line: 439, col: 11, offset: 12348},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 439, col: 11, offset: 12348},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 439, col: 11, offset: 12348},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 439, col: 15, offset: 12352},
							expr: &ruleRefExpr{
								pos:  position{line: 439, col: 15, offset: 12352},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 439, col: 21, offset: 12358},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 447, col: 1, offset: 12513},
			expr: &choiceExpr{
				pos: position{line: 447, col: 9, offset: 12521},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 447, col: 9, offset: 12521},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 447, col: 9, offset: 12521},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 451, col: 5, offset: 12621},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 451, col: 5, offset: 12621},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 457, col: 1, offset: 12722},
			expr: &actionExpr{
				pos: position{line: 457, col: 9, offset: 12730},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 457, col: 9, offset: 12730},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 463, col: 1, offset: 12825},
			expr: &charClassMatcher{
				pos:        position{line: 463, col: 16, offset: 12840},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 465, col: 1, offset: 12851},
			expr: &choiceExpr{
				pos: position{line: 465, col: 9, offset: 12859},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 465, col: 11, offset: 12861},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 465, col: 11, offset: 12861},
								expr: &ruleRefExpr{
									pos:  position{line: 465, col: 12, offset: 12862},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 465, col: 24, offset: 12874,
							},
						},
					},
					&seqExpr{
						pos: position{line: 465, col: 32, offset: 12882},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 465, col: 32, offset: 12882},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 465, col: 37, offset: 12887},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 467, col: 1, offset: 12905},
			expr: &charClassMatcher{
				pos:        position{line: 467, col: 16, offset: 12920},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 469, col: 1, offset: 12936},
			expr: &choiceExpr{
				pos: position{line: 469, col: 19, offset: 12954},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 469, col: 19, offset: 12954},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 469, col: 38, offset: 12973},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 471, col: 1, offset: 12988},
			expr: &charClassMatcher{
				pos:        position{line: 471, col: 21, offset: 13008},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 473, col: 1, offset: 13030},
			expr: &seqExpr{
				pos: position{line: 473, col: 18, offset: 13047},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 473, col: 18, offset: 13047},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 473, col: 22, offset: 13051},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 473, col: 31, offset: 13060},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 473, col: 40, offset: 13069},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 473, col: 49, offset: 13078},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 475, col: 1, offset: 13088},
			expr: &charClassMatcher{
				pos:        position{line: 475, col: 17, offset: 13104},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 477, col: 1, offset: 13111},
			expr: &charClassMatcher{
				pos:        position{line: 477, col: 24, offset: 13134},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 479, col: 1, offset: 13141},
			expr: &charClassMatcher{
				pos:        position{line: 479, col: 13, offset: 13153},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 481, col: 1, offset: 13166},
			expr: &oneOrMoreExpr{
				pos: position{line: 481, col: 20, offset: 13185},
				expr: &charClassMatcher{
					pos:        position{line: 481, col: 20, offset: 13185},
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
			pos:         position{line: 483, col: 1, offset: 13197},
			expr: &zeroOrMoreExpr{
				pos: position{line: 483, col: 19, offset: 13215},
				expr: &choiceExpr{
					pos: position{line: 483, col: 21, offset: 13217},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 483, col: 21, offset: 13217},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 483, col: 33, offset: 13229},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 485, col: 1, offset: 13241},
			expr: &actionExpr{
				pos: position{line: 485, col: 12, offset: 13252},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 485, col: 12, offset: 13252},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 485, col: 12, offset: 13252},
							expr: &charClassMatcher{
								pos:        position{line: 485, col: 12, offset: 13252},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 485, col: 19, offset: 13259},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 485, col: 23, offset: 13263},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 485, col: 28, offset: 13268},
								expr: &charClassMatcher{
									pos:        position{line: 485, col: 28, offset: 13268},
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
			pos:  position{line: 491, col: 1, offset: 13403},
			expr: &notExpr{
				pos: position{line: 491, col: 8, offset: 13410},
				expr: &anyMatcher{
					line: 491, col: 9, offset: 13411,
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

func (c *current) onDefaultRule1(name, value interface{}) (interface{}, error) {

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

	rule := &Rule{
		Default: true,
		Head: &Head{
			Location: currentLocation(c),
			Name:     name.(*Term).Value.(Var),
			Value:    value.(*Term),
		},
		Body: NewBody(NewExpr(BooleanTerm(true))),
	}

	rule.Body[0].Location = currentLocation(c)

	return rule, nil
}

func (p *parser) callonDefaultRule1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onDefaultRule1(stack["name"], stack["value"])
}

func (c *current) onNoDefaultRule1(name, key, value, body interface{}) (interface{}, error) {

	rule := &Rule{
		Head: &Head{},
	}

	rule.Head.Location = currentLocation(c)
	rule.Head.Name = name.(*Term).Value.(Var)

	if key != nil {
		keySlice := key.([]interface{})
		// Rule definition above describes the "key" slice. We care about the "Term" element.
		rule.Head.Key = keySlice[3].(*Term)
	}

	if value != nil {
		valueSlice := value.([]interface{})
		// Rule definition above describes the "value" slice. We care about the "Term" element.
		rule.Head.Value = valueSlice[len(valueSlice)-1].(*Term)
	}

	if key == nil && value == nil {
		rule.Head.Value = BooleanTerm(true)
	}

	if key != nil && value != nil {
		switch rule.Head.Key.Value.(type) {
		case Var, String, Ref: // nop
		default:
			return nil, fmt.Errorf("object key must be one of %v, %v, %v not %v", StringTypeName, VarTypeName, RefTypeName, TypeName(rule.Head.Key.Value))
		}
	}

	// Rule definition above describes the "body" slice. We only care about the "Body" element.
	rule.Body = body.([]interface{})[3].(Body)

	return rule, nil
}

func (p *parser) callonNoDefaultRule1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNoDefaultRule1(stack["name"], stack["key"], stack["value"], stack["body"])
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
	operator := VarTerm(op)
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

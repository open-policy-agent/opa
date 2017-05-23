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

func ifacesToBody(i interface{}, a ...interface{}) Body {
	var buf Body
	buf = append(buf, i.(*Expr))
	for _, s := range a {
		expr := s.([]interface{})[3].(*Expr)
		buf = append(buf, expr)
	}
	return buf
}

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 29, col: 1, offset: 608},
			expr: &actionExpr{
				pos: position{line: 29, col: 12, offset: 619},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 29, col: 12, offset: 619},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 29, col: 12, offset: 619},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 29, col: 14, offset: 621},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 29, col: 19, offset: 626},
								expr: &seqExpr{
									pos: position{line: 29, col: 20, offset: 627},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 29, col: 20, offset: 627},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 29, col: 25, offset: 632},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 29, col: 30, offset: 637},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 29, col: 35, offset: 642},
												expr: &seqExpr{
													pos: position{line: 29, col: 36, offset: 643},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 29, col: 37, offset: 644},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 29, col: 37, offset: 644},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 29, col: 42, offset: 649},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 29, col: 54, offset: 661},
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
							pos:  position{line: 29, col: 63, offset: 670},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 29, col: 65, offset: 672},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 47, col: 1, offset: 1009},
			expr: &actionExpr{
				pos: position{line: 47, col: 9, offset: 1017},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 47, col: 9, offset: 1017},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 47, col: 14, offset: 1022},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 47, col: 14, offset: 1022},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 47, col: 24, offset: 1032},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 47, col: 33, offset: 1041},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 47, col: 41, offset: 1049},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 47, col: 48, offset: 1056},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 47, col: 58, offset: 1066},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 56, col: 1, offset: 1430},
			expr: &actionExpr{
				pos: position{line: 56, col: 15, offset: 1444},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 56, col: 15, offset: 1444,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 60, col: 1, offset: 1517},
			expr: &actionExpr{
				pos: position{line: 60, col: 12, offset: 1528},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 60, col: 12, offset: 1528},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 60, col: 12, offset: 1528},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 60, col: 22, offset: 1538},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 60, col: 25, offset: 1541},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 60, col: 30, offset: 1546},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 60, col: 30, offset: 1546},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 60, col: 36, offset: 1552},
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
			pos:  position{line: 96, col: 1, offset: 2933},
			expr: &actionExpr{
				pos: position{line: 96, col: 11, offset: 2943},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 96, col: 11, offset: 2943},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 96, col: 11, offset: 2943},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 96, col: 20, offset: 2952},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 96, col: 23, offset: 2955},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 96, col: 29, offset: 2961},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 96, col: 29, offset: 2961},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 96, col: 35, offset: 2967},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 96, col: 40, offset: 2972},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 96, col: 46, offset: 2978},
								expr: &seqExpr{
									pos: position{line: 96, col: 47, offset: 2979},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 96, col: 47, offset: 2979},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 96, col: 50, offset: 2982},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 96, col: 55, offset: 2987},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 96, col: 58, offset: 2990},
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
			pos:  position{line: 112, col: 1, offset: 3440},
			expr: &choiceExpr{
				pos: position{line: 112, col: 10, offset: 3449},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 112, col: 10, offset: 3449},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 112, col: 25, offset: 3464},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 114, col: 1, offset: 3477},
			expr: &actionExpr{
				pos: position{line: 114, col: 17, offset: 3493},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 114, col: 17, offset: 3493},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 114, col: 17, offset: 3493},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 114, col: 27, offset: 3503},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 114, col: 30, offset: 3506},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 114, col: 35, offset: 3511},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 114, col: 39, offset: 3515},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 114, col: 41, offset: 3517},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 114, col: 45, offset: 3521},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 114, col: 47, offset: 3523},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 114, col: 53, offset: 3529},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 157, col: 1, offset: 4430},
			expr: &actionExpr{
				pos: position{line: 157, col: 16, offset: 4445},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 157, col: 16, offset: 4445},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 157, col: 16, offset: 4445},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 157, col: 21, offset: 4450},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 157, col: 30, offset: 4459},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 157, col: 32, offset: 4461},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 157, col: 35, offset: 4464},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 157, col: 35, offset: 4464},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 157, col: 61, offset: 4490},
										expr: &seqExpr{
											pos: position{line: 157, col: 63, offset: 4492},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 157, col: 63, offset: 4492},
													name: "_",
												},
												&zeroOrOneExpr{
													pos: position{line: 157, col: 65, offset: 4494},
													expr: &ruleRefExpr{
														pos:  position{line: 157, col: 65, offset: 4494},
														name: "Else",
													},
												},
												&ruleRefExpr{
													pos:  position{line: 157, col: 71, offset: 4500},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 157, col: 73, offset: 4502},
													name: "NonEmptyBraceEnclosedBody",
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
			name: "RuleHead",
			pos:  position{line: 208, col: 1, offset: 5700},
			expr: &actionExpr{
				pos: position{line: 208, col: 13, offset: 5712},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 208, col: 13, offset: 5712},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 208, col: 13, offset: 5712},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 208, col: 18, offset: 5717},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 208, col: 22, offset: 5721},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 208, col: 26, offset: 5725},
								expr: &seqExpr{
									pos: position{line: 208, col: 28, offset: 5727},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 208, col: 28, offset: 5727},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 208, col: 30, offset: 5729},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 34, offset: 5733},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 36, offset: 5735},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 41, offset: 5740},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 208, col: 43, offset: 5742},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 47, offset: 5746},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 208, col: 52, offset: 5751},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 208, col: 58, offset: 5757},
								expr: &seqExpr{
									pos: position{line: 208, col: 60, offset: 5759},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 208, col: 60, offset: 5759},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 208, col: 62, offset: 5761},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 66, offset: 5765},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 208, col: 68, offset: 5767},
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
			pos:  position{line: 242, col: 1, offset: 6711},
			expr: &actionExpr{
				pos: position{line: 242, col: 9, offset: 6719},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 242, col: 9, offset: 6719},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 242, col: 9, offset: 6719},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 242, col: 16, offset: 6726},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 242, col: 20, offset: 6730},
								expr: &seqExpr{
									pos: position{line: 242, col: 22, offset: 6732},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 242, col: 22, offset: 6732},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 242, col: 24, offset: 6734},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 242, col: 28, offset: 6738},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 242, col: 30, offset: 6740},
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
			name: "Body",
			pos:  position{line: 254, col: 1, offset: 6943},
			expr: &choiceExpr{
				pos: position{line: 254, col: 9, offset: 6951},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 254, col: 9, offset: 6951},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 254, col: 29, offset: 6971},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 256, col: 1, offset: 6990},
			expr: &actionExpr{
				pos: position{line: 256, col: 30, offset: 7019},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 256, col: 30, offset: 7019},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 256, col: 30, offset: 7019},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 256, col: 34, offset: 7023},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 256, col: 36, offset: 7025},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 256, col: 40, offset: 7029},
								expr: &ruleRefExpr{
									pos:  position{line: 256, col: 40, offset: 7029},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 256, col: 56, offset: 7045},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 256, col: 58, offset: 7047},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 263, col: 1, offset: 7142},
			expr: &actionExpr{
				pos: position{line: 263, col: 22, offset: 7163},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 263, col: 22, offset: 7163},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 263, col: 22, offset: 7163},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 26, offset: 7167},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 263, col: 28, offset: 7169},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 263, col: 32, offset: 7173},
								expr: &ruleRefExpr{
									pos:  position{line: 263, col: 32, offset: 7173},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 48, offset: 7189},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 263, col: 50, offset: 7191},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 276, col: 1, offset: 7490},
			expr: &actionExpr{
				pos: position{line: 276, col: 19, offset: 7508},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 276, col: 19, offset: 7508},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 276, col: 19, offset: 7508},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 276, col: 24, offset: 7513},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 276, col: 32, offset: 7521},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 276, col: 37, offset: 7526},
								expr: &seqExpr{
									pos: position{line: 276, col: 38, offset: 7527},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 276, col: 38, offset: 7527},
											expr: &charClassMatcher{
												pos:        position{line: 276, col: 38, offset: 7527},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 276, col: 46, offset: 7535},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 276, col: 47, offset: 7536},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 276, col: 47, offset: 7536},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 276, col: 51, offset: 7540},
															expr: &ruleRefExpr{
																pos:  position{line: 276, col: 51, offset: 7540},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 276, col: 64, offset: 7553},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 276, col: 64, offset: 7553},
															expr: &ruleRefExpr{
																pos:  position{line: 276, col: 64, offset: 7553},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 276, col: 73, offset: 7562},
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
											pos:  position{line: 276, col: 82, offset: 7571},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 276, col: 84, offset: 7573},
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
			pos:  position{line: 282, col: 1, offset: 7762},
			expr: &actionExpr{
				pos: position{line: 282, col: 22, offset: 7783},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 282, col: 22, offset: 7783},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 282, col: 22, offset: 7783},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 282, col: 27, offset: 7788},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 282, col: 35, offset: 7796},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 282, col: 40, offset: 7801},
								expr: &seqExpr{
									pos: position{line: 282, col: 42, offset: 7803},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 282, col: 42, offset: 7803},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 282, col: 44, offset: 7805},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 282, col: 48, offset: 7809},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 282, col: 51, offset: 7812},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 282, col: 51, offset: 7812},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 282, col: 61, offset: 7822},
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
			pos:  position{line: 286, col: 1, offset: 7901},
			expr: &actionExpr{
				pos: position{line: 286, col: 12, offset: 7912},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 286, col: 12, offset: 7912},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 286, col: 12, offset: 7912},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 286, col: 16, offset: 7916},
								expr: &seqExpr{
									pos: position{line: 286, col: 18, offset: 7918},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 286, col: 18, offset: 7918},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 286, col: 24, offset: 7924},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 286, col: 30, offset: 7930},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 286, col: 34, offset: 7934},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 286, col: 39, offset: 7939},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 286, col: 44, offset: 7944},
								expr: &seqExpr{
									pos: position{line: 286, col: 46, offset: 7946},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 286, col: 46, offset: 7946},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 286, col: 49, offset: 7949},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 286, col: 54, offset: 7954},
											expr: &seqExpr{
												pos: position{line: 286, col: 55, offset: 7955},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 286, col: 55, offset: 7955},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 286, col: 58, offset: 7958},
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
			pos:  position{line: 309, col: 1, offset: 8530},
			expr: &actionExpr{
				pos: position{line: 309, col: 9, offset: 8538},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 309, col: 9, offset: 8538},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 309, col: 9, offset: 8538},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 309, col: 16, offset: 8545},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 309, col: 19, offset: 8548},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 309, col: 26, offset: 8555},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 309, col: 31, offset: 8560},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 309, col: 34, offset: 8563},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 309, col: 39, offset: 8568},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 309, col: 42, offset: 8571},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 309, col: 48, offset: 8577},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 320, col: 1, offset: 8826},
			expr: &choiceExpr{
				pos: position{line: 320, col: 9, offset: 8834},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 320, col: 10, offset: 8835},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 320, col: 10, offset: 8835},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 320, col: 27, offset: 8852},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 320, col: 52, offset: 8877},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 320, col: 64, offset: 8889},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 320, col: 77, offset: 8902},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 322, col: 1, offset: 8908},
			expr: &actionExpr{
				pos: position{line: 322, col: 19, offset: 8926},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 322, col: 19, offset: 8926},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 322, col: 19, offset: 8926},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 322, col: 26, offset: 8933},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 322, col: 31, offset: 8938},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 322, col: 33, offset: 8940},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 322, col: 37, offset: 8944},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 322, col: 39, offset: 8946},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 322, col: 44, offset: 8951},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 322, col: 49, offset: 8956},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 322, col: 51, offset: 8958},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 322, col: 54, offset: 8961},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 322, col: 67, offset: 8974},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 322, col: 69, offset: 8976},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 322, col: 75, offset: 8982},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 326, col: 1, offset: 9073},
			expr: &actionExpr{
				pos: position{line: 326, col: 26, offset: 9098},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 326, col: 26, offset: 9098},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 326, col: 26, offset: 9098},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 326, col: 31, offset: 9103},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 326, col: 36, offset: 9108},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 326, col: 38, offset: 9110},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 326, col: 41, offset: 9113},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 326, col: 54, offset: 9126},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 326, col: 56, offset: 9128},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 326, col: 62, offset: 9134},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 326, col: 67, offset: 9139},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 326, col: 69, offset: 9141},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 326, col: 73, offset: 9145},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 326, col: 75, offset: 9147},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 326, col: 82, offset: 9154},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 330, col: 1, offset: 9245},
			expr: &actionExpr{
				pos: position{line: 330, col: 17, offset: 9261},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 330, col: 17, offset: 9261},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 330, col: 22, offset: 9266},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 330, col: 22, offset: 9266},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 28, offset: 9272},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 34, offset: 9278},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 40, offset: 9284},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 46, offset: 9290},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 52, offset: 9296},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 330, col: 58, offset: 9302},
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
			pos:  position{line: 342, col: 1, offset: 9549},
			expr: &actionExpr{
				pos: position{line: 342, col: 14, offset: 9562},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 342, col: 14, offset: 9562},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 342, col: 14, offset: 9562},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 342, col: 19, offset: 9567},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 342, col: 24, offset: 9572},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 342, col: 26, offset: 9574},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 342, col: 29, offset: 9577},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 342, col: 37, offset: 9585},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 342, col: 39, offset: 9587},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 342, col: 45, offset: 9593},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 346, col: 1, offset: 9668},
			expr: &actionExpr{
				pos: position{line: 346, col: 12, offset: 9679},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 346, col: 12, offset: 9679},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 346, col: 17, offset: 9684},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 346, col: 17, offset: 9684},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 346, col: 23, offset: 9690},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 346, col: 30, offset: 9697},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 346, col: 37, offset: 9704},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 346, col: 44, offset: 9711},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 346, col: 50, offset: 9717},
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
			pos:  position{line: 358, col: 1, offset: 9964},
			expr: &choiceExpr{
				pos: position{line: 358, col: 15, offset: 9978},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 358, col: 15, offset: 9978},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 358, col: 26, offset: 9989},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 360, col: 1, offset: 9998},
			expr: &actionExpr{
				pos: position{line: 360, col: 12, offset: 10009},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 360, col: 12, offset: 10009},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 360, col: 12, offset: 10009},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 360, col: 17, offset: 10014},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 360, col: 29, offset: 10026},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 33, offset: 10030},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 360, col: 35, offset: 10032},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 360, col: 40, offset: 10037},
								expr: &ruleRefExpr{
									pos:  position{line: 360, col: 40, offset: 10037},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 360, col: 46, offset: 10043},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 360, col: 51, offset: 10048},
								expr: &seqExpr{
									pos: position{line: 360, col: 53, offset: 10050},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 360, col: 53, offset: 10050},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 360, col: 55, offset: 10052},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 360, col: 59, offset: 10056},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 360, col: 61, offset: 10058},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 69, offset: 10066},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 360, col: 72, offset: 10069},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 376, col: 1, offset: 10473},
			expr: &actionExpr{
				pos: position{line: 376, col: 16, offset: 10488},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 376, col: 16, offset: 10488},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 376, col: 16, offset: 10488},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 376, col: 21, offset: 10493},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 376, col: 25, offset: 10497},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 376, col: 30, offset: 10502},
								expr: &seqExpr{
									pos: position{line: 376, col: 32, offset: 10504},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 376, col: 32, offset: 10504},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 376, col: 36, offset: 10508},
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
			pos:  position{line: 390, col: 1, offset: 10913},
			expr: &actionExpr{
				pos: position{line: 390, col: 9, offset: 10921},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 390, col: 9, offset: 10921},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 390, col: 15, offset: 10927},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 390, col: 15, offset: 10927},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 390, col: 31, offset: 10943},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 390, col: 43, offset: 10955},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 390, col: 52, offset: 10964},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 390, col: 58, offset: 10970},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 394, col: 1, offset: 11001},
			expr: &ruleRefExpr{
				pos:  position{line: 394, col: 18, offset: 11018},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 396, col: 1, offset: 11038},
			expr: &actionExpr{
				pos: position{line: 396, col: 23, offset: 11060},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 396, col: 23, offset: 11060},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 396, col: 23, offset: 11060},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 27, offset: 11064},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 396, col: 29, offset: 11066},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 396, col: 34, offset: 11071},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 39, offset: 11076},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 396, col: 41, offset: 11078},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 45, offset: 11082},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 396, col: 47, offset: 11084},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 396, col: 52, offset: 11089},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 67, offset: 11104},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 396, col: 69, offset: 11106},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 402, col: 1, offset: 11231},
			expr: &choiceExpr{
				pos: position{line: 402, col: 14, offset: 11244},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 402, col: 14, offset: 11244},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 402, col: 23, offset: 11253},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 402, col: 31, offset: 11261},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 404, col: 1, offset: 11266},
			expr: &choiceExpr{
				pos: position{line: 404, col: 11, offset: 11276},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 404, col: 11, offset: 11276},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 404, col: 20, offset: 11285},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 404, col: 29, offset: 11294},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 404, col: 36, offset: 11301},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 406, col: 1, offset: 11307},
			expr: &choiceExpr{
				pos: position{line: 406, col: 8, offset: 11314},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 406, col: 8, offset: 11314},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 406, col: 17, offset: 11323},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 406, col: 23, offset: 11329},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 408, col: 1, offset: 11334},
			expr: &actionExpr{
				pos: position{line: 408, col: 11, offset: 11344},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 408, col: 11, offset: 11344},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 408, col: 11, offset: 11344},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 408, col: 15, offset: 11348},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 408, col: 17, offset: 11350},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 408, col: 22, offset: 11355},
								expr: &seqExpr{
									pos: position{line: 408, col: 23, offset: 11356},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 408, col: 23, offset: 11356},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 27, offset: 11360},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 408, col: 29, offset: 11362},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 33, offset: 11366},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 35, offset: 11368},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 408, col: 42, offset: 11375},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 408, col: 47, offset: 11380},
								expr: &seqExpr{
									pos: position{line: 408, col: 49, offset: 11382},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 408, col: 49, offset: 11382},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 408, col: 51, offset: 11384},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 55, offset: 11388},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 57, offset: 11390},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 61, offset: 11394},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 408, col: 63, offset: 11396},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 67, offset: 11400},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 408, col: 69, offset: 11402},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 408, col: 77, offset: 11410},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 408, col: 79, offset: 11412},
							expr: &litMatcher{
								pos:        position{line: 408, col: 79, offset: 11412},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 408, col: 84, offset: 11417},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 408, col: 86, offset: 11419},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 432, col: 1, offset: 12198},
			expr: &actionExpr{
				pos: position{line: 432, col: 10, offset: 12207},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 432, col: 10, offset: 12207},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 432, col: 10, offset: 12207},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 14, offset: 12211},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 432, col: 17, offset: 12214},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 432, col: 22, offset: 12219},
								expr: &ruleRefExpr{
									pos:  position{line: 432, col: 22, offset: 12219},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 432, col: 28, offset: 12225},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 432, col: 33, offset: 12230},
								expr: &seqExpr{
									pos: position{line: 432, col: 34, offset: 12231},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 432, col: 34, offset: 12231},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 432, col: 36, offset: 12233},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 432, col: 40, offset: 12237},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 432, col: 42, offset: 12239},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 49, offset: 12246},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 432, col: 51, offset: 12248},
							expr: &litMatcher{
								pos:        position{line: 432, col: 51, offset: 12248},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 432, col: 56, offset: 12253},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 432, col: 59, offset: 12256},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 456, col: 1, offset: 12829},
			expr: &choiceExpr{
				pos: position{line: 456, col: 8, offset: 12836},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 456, col: 8, offset: 12836},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 456, col: 19, offset: 12847},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 458, col: 1, offset: 12860},
			expr: &actionExpr{
				pos: position{line: 458, col: 13, offset: 12872},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 458, col: 13, offset: 12872},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 458, col: 13, offset: 12872},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 458, col: 20, offset: 12879},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 458, col: 22, offset: 12881},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 464, col: 1, offset: 12969},
			expr: &actionExpr{
				pos: position{line: 464, col: 16, offset: 12984},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 464, col: 16, offset: 12984},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 464, col: 16, offset: 12984},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 464, col: 20, offset: 12988},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 464, col: 22, offset: 12990},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 464, col: 27, offset: 12995},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 464, col: 32, offset: 13000},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 464, col: 37, offset: 13005},
								expr: &seqExpr{
									pos: position{line: 464, col: 38, offset: 13006},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 464, col: 38, offset: 13006},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 464, col: 40, offset: 13008},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 464, col: 44, offset: 13012},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 464, col: 46, offset: 13014},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 464, col: 53, offset: 13021},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 464, col: 55, offset: 13023},
							expr: &litMatcher{
								pos:        position{line: 464, col: 55, offset: 13023},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 464, col: 60, offset: 13028},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 464, col: 62, offset: 13030},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 481, col: 1, offset: 13435},
			expr: &actionExpr{
				pos: position{line: 481, col: 8, offset: 13442},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 481, col: 8, offset: 13442},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 481, col: 8, offset: 13442},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 481, col: 13, offset: 13447},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 481, col: 17, offset: 13451},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 481, col: 22, offset: 13456},
								expr: &choiceExpr{
									pos: position{line: 481, col: 24, offset: 13458},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 481, col: 24, offset: 13458},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 481, col: 33, offset: 13467},
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
			pos:  position{line: 494, col: 1, offset: 13706},
			expr: &actionExpr{
				pos: position{line: 494, col: 11, offset: 13716},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 494, col: 11, offset: 13716},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 494, col: 11, offset: 13716},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 494, col: 15, offset: 13720},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 494, col: 19, offset: 13724},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 501, col: 1, offset: 13943},
			expr: &actionExpr{
				pos: position{line: 501, col: 15, offset: 13957},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 501, col: 15, offset: 13957},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 501, col: 15, offset: 13957},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 501, col: 19, offset: 13961},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 501, col: 24, offset: 13966},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 501, col: 24, offset: 13966},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 501, col: 30, offset: 13972},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 501, col: 39, offset: 13981},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 501, col: 44, offset: 13986},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 505, col: 1, offset: 14015},
			expr: &actionExpr{
				pos: position{line: 505, col: 8, offset: 14022},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 505, col: 8, offset: 14022},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 505, col: 12, offset: 14026},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 510, col: 1, offset: 14148},
			expr: &seqExpr{
				pos: position{line: 510, col: 15, offset: 14162},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 510, col: 15, offset: 14162},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 510, col: 19, offset: 14166},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 510, col: 32, offset: 14179},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 514, col: 1, offset: 14244},
			expr: &actionExpr{
				pos: position{line: 514, col: 17, offset: 14260},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 514, col: 17, offset: 14260},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 514, col: 17, offset: 14260},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 514, col: 29, offset: 14272},
							expr: &choiceExpr{
								pos: position{line: 514, col: 30, offset: 14273},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 514, col: 30, offset: 14273},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 514, col: 44, offset: 14287},
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
			pos:  position{line: 521, col: 1, offset: 14430},
			expr: &actionExpr{
				pos: position{line: 521, col: 11, offset: 14440},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 521, col: 11, offset: 14440},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 521, col: 11, offset: 14440},
							expr: &litMatcher{
								pos:        position{line: 521, col: 11, offset: 14440},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 521, col: 18, offset: 14447},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 521, col: 18, offset: 14447},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 521, col: 26, offset: 14455},
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
			pos:  position{line: 534, col: 1, offset: 14846},
			expr: &choiceExpr{
				pos: position{line: 534, col: 10, offset: 14855},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 534, col: 10, offset: 14855},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 534, col: 26, offset: 14871},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 536, col: 1, offset: 14883},
			expr: &seqExpr{
				pos: position{line: 536, col: 18, offset: 14900},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 536, col: 20, offset: 14902},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 536, col: 20, offset: 14902},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 536, col: 33, offset: 14915},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 536, col: 43, offset: 14925},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 538, col: 1, offset: 14935},
			expr: &seqExpr{
				pos: position{line: 538, col: 15, offset: 14949},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 538, col: 15, offset: 14949},
						expr: &ruleRefExpr{
							pos:  position{line: 538, col: 15, offset: 14949},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 538, col: 24, offset: 14958},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 540, col: 1, offset: 14968},
			expr: &seqExpr{
				pos: position{line: 540, col: 13, offset: 14980},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 540, col: 13, offset: 14980},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 540, col: 17, offset: 14984},
						expr: &ruleRefExpr{
							pos:  position{line: 540, col: 17, offset: 14984},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 542, col: 1, offset: 14999},
			expr: &seqExpr{
				pos: position{line: 542, col: 13, offset: 15011},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 542, col: 13, offset: 15011},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 542, col: 18, offset: 15016},
						expr: &charClassMatcher{
							pos:        position{line: 542, col: 18, offset: 15016},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 542, col: 24, offset: 15022},
						expr: &ruleRefExpr{
							pos:  position{line: 542, col: 24, offset: 15022},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 544, col: 1, offset: 15037},
			expr: &choiceExpr{
				pos: position{line: 544, col: 12, offset: 15048},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 544, col: 12, offset: 15048},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 544, col: 20, offset: 15056},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 544, col: 20, offset: 15056},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 544, col: 40, offset: 15076},
								expr: &ruleRefExpr{
									pos:  position{line: 544, col: 40, offset: 15076},
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
			pos:  position{line: 546, col: 1, offset: 15093},
			expr: &actionExpr{
				pos: position{line: 546, col: 11, offset: 15103},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 546, col: 11, offset: 15103},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 546, col: 11, offset: 15103},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 546, col: 15, offset: 15107},
							expr: &ruleRefExpr{
								pos:  position{line: 546, col: 15, offset: 15107},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 546, col: 21, offset: 15113},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 554, col: 1, offset: 15268},
			expr: &choiceExpr{
				pos: position{line: 554, col: 9, offset: 15276},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 554, col: 9, offset: 15276},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 554, col: 9, offset: 15276},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 558, col: 5, offset: 15376},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 558, col: 5, offset: 15376},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 564, col: 1, offset: 15477},
			expr: &actionExpr{
				pos: position{line: 564, col: 9, offset: 15485},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 564, col: 9, offset: 15485},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 570, col: 1, offset: 15580},
			expr: &charClassMatcher{
				pos:        position{line: 570, col: 16, offset: 15595},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 572, col: 1, offset: 15606},
			expr: &choiceExpr{
				pos: position{line: 572, col: 9, offset: 15614},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 572, col: 11, offset: 15616},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 572, col: 11, offset: 15616},
								expr: &ruleRefExpr{
									pos:  position{line: 572, col: 12, offset: 15617},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 572, col: 24, offset: 15629,
							},
						},
					},
					&seqExpr{
						pos: position{line: 572, col: 32, offset: 15637},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 572, col: 32, offset: 15637},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 572, col: 37, offset: 15642},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 574, col: 1, offset: 15660},
			expr: &charClassMatcher{
				pos:        position{line: 574, col: 16, offset: 15675},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 576, col: 1, offset: 15691},
			expr: &choiceExpr{
				pos: position{line: 576, col: 19, offset: 15709},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 576, col: 19, offset: 15709},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 576, col: 38, offset: 15728},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 578, col: 1, offset: 15743},
			expr: &charClassMatcher{
				pos:        position{line: 578, col: 21, offset: 15763},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 580, col: 1, offset: 15785},
			expr: &seqExpr{
				pos: position{line: 580, col: 18, offset: 15802},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 580, col: 18, offset: 15802},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 580, col: 22, offset: 15806},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 580, col: 31, offset: 15815},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 580, col: 40, offset: 15824},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 580, col: 49, offset: 15833},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 582, col: 1, offset: 15843},
			expr: &charClassMatcher{
				pos:        position{line: 582, col: 17, offset: 15859},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 584, col: 1, offset: 15866},
			expr: &charClassMatcher{
				pos:        position{line: 584, col: 24, offset: 15889},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 586, col: 1, offset: 15896},
			expr: &charClassMatcher{
				pos:        position{line: 586, col: 13, offset: 15908},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 588, col: 1, offset: 15921},
			expr: &oneOrMoreExpr{
				pos: position{line: 588, col: 20, offset: 15940},
				expr: &charClassMatcher{
					pos:        position{line: 588, col: 20, offset: 15940},
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
			pos:         position{line: 590, col: 1, offset: 15952},
			expr: &zeroOrMoreExpr{
				pos: position{line: 590, col: 19, offset: 15970},
				expr: &choiceExpr{
					pos: position{line: 590, col: 21, offset: 15972},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 590, col: 21, offset: 15972},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 590, col: 33, offset: 15984},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 592, col: 1, offset: 15996},
			expr: &actionExpr{
				pos: position{line: 592, col: 12, offset: 16007},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 592, col: 12, offset: 16007},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 592, col: 12, offset: 16007},
							expr: &charClassMatcher{
								pos:        position{line: 592, col: 12, offset: 16007},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 592, col: 19, offset: 16014},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 592, col: 23, offset: 16018},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 592, col: 28, offset: 16023},
								expr: &charClassMatcher{
									pos:        position{line: 592, col: 28, offset: 16023},
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
			pos:  position{line: 598, col: 1, offset: 16158},
			expr: &notExpr{
				pos: position{line: 598, col: 8, offset: 16165},
				expr: &anyMatcher{
					line: 598, col: 9, offset: 16166,
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

	body := NewBody(NewExpr(BooleanTerm(true)))
	body[0].Location = currentLocation(c)

	rule := &Rule{
		Default: true,
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
			Head: head.(*Head),
			Body: sl[0].(Body),
		},
	}

	var ordered bool
	prev := rules[0]

	for i, elem := range sl[1].([]interface{}) {

		next := elem.([]interface{})

		if term, ok := next[1].(*Term); !ok {
			if ordered {
				return nil, fmt.Errorf("expected 'else' keyword")
			}
			rules = append(rules, &Rule{
				Head: prev.Head.Copy(),
				Body: next[3].(Body),
			})
		} else {
			if (rules[0].Head.DocKind() != CompleteDoc) || (i != 0 && !ordered) {
				return nil, fmt.Errorf("unexpected 'else' keyword")
			}
			ordered = true
			curr := &Rule{
				Head: &Head{
					Name:     prev.Head.Name,
					Value:    term,
					Location: term.Location,
				},
				Body: next[3].(Body),
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

func (c *current) onElse1(val interface{}) (interface{}, error) {

	if val == nil {
		term := BooleanTerm(true)
		term.Location = currentLocation(c)
		return term, nil
	}

	sl := val.([]interface{})
	return sl[3].(*Term), nil
}

func (p *parser) callonElse1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onElse1(stack["val"])
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
		body := NewBody(NewExpr(ObjectTerm()))
		body[0].Location = currentLocation(c)
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

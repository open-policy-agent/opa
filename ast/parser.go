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
			pos:  position{line: 94, col: 1, offset: 2868},
			expr: &actionExpr{
				pos: position{line: 94, col: 11, offset: 2878},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 94, col: 11, offset: 2878},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 94, col: 11, offset: 2878},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 94, col: 20, offset: 2887},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 94, col: 23, offset: 2890},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 94, col: 29, offset: 2896},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 94, col: 29, offset: 2896},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 94, col: 35, offset: 2902},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 94, col: 40, offset: 2907},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 94, col: 46, offset: 2913},
								expr: &seqExpr{
									pos: position{line: 94, col: 47, offset: 2914},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 94, col: 47, offset: 2914},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 94, col: 50, offset: 2917},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 94, col: 55, offset: 2922},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 94, col: 58, offset: 2925},
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
			pos:  position{line: 110, col: 1, offset: 3375},
			expr: &choiceExpr{
				pos: position{line: 110, col: 10, offset: 3384},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 110, col: 10, offset: 3384},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 110, col: 25, offset: 3399},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 112, col: 1, offset: 3412},
			expr: &actionExpr{
				pos: position{line: 112, col: 17, offset: 3428},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 112, col: 17, offset: 3428},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 112, col: 17, offset: 3428},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 112, col: 27, offset: 3438},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 112, col: 30, offset: 3441},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 112, col: 35, offset: 3446},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 112, col: 39, offset: 3450},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 112, col: 41, offset: 3452},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 112, col: 45, offset: 3456},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 112, col: 47, offset: 3458},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 112, col: 53, offset: 3464},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 156, col: 1, offset: 4396},
			expr: &actionExpr{
				pos: position{line: 156, col: 16, offset: 4411},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 156, col: 16, offset: 4411},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 156, col: 16, offset: 4411},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 156, col: 21, offset: 4416},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 156, col: 30, offset: 4425},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 156, col: 32, offset: 4427},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 156, col: 35, offset: 4430},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 156, col: 35, offset: 4430},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 156, col: 61, offset: 4456},
										expr: &seqExpr{
											pos: position{line: 156, col: 63, offset: 4458},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 156, col: 63, offset: 4458},
													name: "_",
												},
												&zeroOrOneExpr{
													pos: position{line: 156, col: 65, offset: 4460},
													expr: &ruleRefExpr{
														pos:  position{line: 156, col: 65, offset: 4460},
														name: "Else",
													},
												},
												&ruleRefExpr{
													pos:  position{line: 156, col: 71, offset: 4466},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 156, col: 73, offset: 4468},
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
			pos:  position{line: 207, col: 1, offset: 5666},
			expr: &actionExpr{
				pos: position{line: 207, col: 13, offset: 5678},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 207, col: 13, offset: 5678},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 207, col: 13, offset: 5678},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 207, col: 18, offset: 5683},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 207, col: 22, offset: 5687},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 207, col: 26, offset: 5691},
								expr: &seqExpr{
									pos: position{line: 207, col: 28, offset: 5693},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 207, col: 28, offset: 5693},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 207, col: 30, offset: 5695},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 34, offset: 5699},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 36, offset: 5701},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 41, offset: 5706},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 207, col: 43, offset: 5708},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 47, offset: 5712},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 207, col: 52, offset: 5717},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 207, col: 58, offset: 5723},
								expr: &seqExpr{
									pos: position{line: 207, col: 60, offset: 5725},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 207, col: 60, offset: 5725},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 207, col: 62, offset: 5727},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 66, offset: 5731},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 207, col: 68, offset: 5733},
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
			pos:  position{line: 242, col: 1, offset: 6721},
			expr: &actionExpr{
				pos: position{line: 242, col: 9, offset: 6729},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 242, col: 9, offset: 6729},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 242, col: 9, offset: 6729},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 242, col: 16, offset: 6736},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 242, col: 20, offset: 6740},
								expr: &seqExpr{
									pos: position{line: 242, col: 22, offset: 6742},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 242, col: 22, offset: 6742},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 242, col: 24, offset: 6744},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 242, col: 28, offset: 6748},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 242, col: 30, offset: 6750},
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
			pos:  position{line: 254, col: 1, offset: 6953},
			expr: &choiceExpr{
				pos: position{line: 254, col: 9, offset: 6961},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 254, col: 9, offset: 6961},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 254, col: 29, offset: 6981},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 256, col: 1, offset: 7000},
			expr: &actionExpr{
				pos: position{line: 256, col: 30, offset: 7029},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 256, col: 30, offset: 7029},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 256, col: 30, offset: 7029},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 256, col: 34, offset: 7033},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 256, col: 36, offset: 7035},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 256, col: 40, offset: 7039},
								expr: &ruleRefExpr{
									pos:  position{line: 256, col: 40, offset: 7039},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 256, col: 56, offset: 7055},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 256, col: 58, offset: 7057},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 263, col: 1, offset: 7152},
			expr: &actionExpr{
				pos: position{line: 263, col: 22, offset: 7173},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 263, col: 22, offset: 7173},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 263, col: 22, offset: 7173},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 26, offset: 7177},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 263, col: 28, offset: 7179},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 263, col: 32, offset: 7183},
								expr: &ruleRefExpr{
									pos:  position{line: 263, col: 32, offset: 7183},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 48, offset: 7199},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 263, col: 50, offset: 7201},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 277, col: 1, offset: 7536},
			expr: &actionExpr{
				pos: position{line: 277, col: 19, offset: 7554},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 277, col: 19, offset: 7554},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 277, col: 19, offset: 7554},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 277, col: 24, offset: 7559},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 277, col: 32, offset: 7567},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 277, col: 37, offset: 7572},
								expr: &seqExpr{
									pos: position{line: 277, col: 38, offset: 7573},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 277, col: 38, offset: 7573},
											expr: &charClassMatcher{
												pos:        position{line: 277, col: 38, offset: 7573},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 277, col: 46, offset: 7581},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 277, col: 47, offset: 7582},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 277, col: 47, offset: 7582},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 277, col: 51, offset: 7586},
															expr: &ruleRefExpr{
																pos:  position{line: 277, col: 51, offset: 7586},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 277, col: 64, offset: 7599},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 277, col: 64, offset: 7599},
															expr: &ruleRefExpr{
																pos:  position{line: 277, col: 64, offset: 7599},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 277, col: 73, offset: 7608},
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
											pos:  position{line: 277, col: 82, offset: 7617},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 277, col: 84, offset: 7619},
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
			pos:  position{line: 283, col: 1, offset: 7808},
			expr: &actionExpr{
				pos: position{line: 283, col: 22, offset: 7829},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 283, col: 22, offset: 7829},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 283, col: 22, offset: 7829},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 283, col: 27, offset: 7834},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 283, col: 35, offset: 7842},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 283, col: 40, offset: 7847},
								expr: &seqExpr{
									pos: position{line: 283, col: 42, offset: 7849},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 283, col: 42, offset: 7849},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 283, col: 44, offset: 7851},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 283, col: 48, offset: 7855},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 283, col: 51, offset: 7858},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 283, col: 51, offset: 7858},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 283, col: 61, offset: 7868},
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
			pos:  position{line: 287, col: 1, offset: 7947},
			expr: &actionExpr{
				pos: position{line: 287, col: 12, offset: 7958},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 287, col: 12, offset: 7958},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 287, col: 12, offset: 7958},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 287, col: 16, offset: 7962},
								expr: &seqExpr{
									pos: position{line: 287, col: 18, offset: 7964},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 287, col: 18, offset: 7964},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 287, col: 24, offset: 7970},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 287, col: 30, offset: 7976},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 287, col: 34, offset: 7980},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 287, col: 39, offset: 7985},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 287, col: 44, offset: 7990},
								expr: &seqExpr{
									pos: position{line: 287, col: 46, offset: 7992},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 287, col: 46, offset: 7992},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 287, col: 49, offset: 7995},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 287, col: 54, offset: 8000},
											expr: &seqExpr{
												pos: position{line: 287, col: 55, offset: 8001},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 287, col: 55, offset: 8001},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 287, col: 58, offset: 8004},
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
			pos:  position{line: 310, col: 1, offset: 8576},
			expr: &actionExpr{
				pos: position{line: 310, col: 9, offset: 8584},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 310, col: 9, offset: 8584},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 310, col: 9, offset: 8584},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 310, col: 16, offset: 8591},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 310, col: 19, offset: 8594},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 310, col: 26, offset: 8601},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 310, col: 31, offset: 8606},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 310, col: 34, offset: 8609},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 310, col: 39, offset: 8614},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 310, col: 42, offset: 8617},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 310, col: 48, offset: 8623},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 321, col: 1, offset: 8872},
			expr: &choiceExpr{
				pos: position{line: 321, col: 9, offset: 8880},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 321, col: 10, offset: 8881},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 321, col: 10, offset: 8881},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 321, col: 27, offset: 8898},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 321, col: 52, offset: 8923},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 321, col: 64, offset: 8935},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 321, col: 77, offset: 8948},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 323, col: 1, offset: 8954},
			expr: &actionExpr{
				pos: position{line: 323, col: 19, offset: 8972},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 323, col: 19, offset: 8972},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 323, col: 19, offset: 8972},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 323, col: 26, offset: 8979},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 323, col: 31, offset: 8984},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 323, col: 33, offset: 8986},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 323, col: 37, offset: 8990},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 323, col: 39, offset: 8992},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 323, col: 44, offset: 8997},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 323, col: 49, offset: 9002},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 323, col: 51, offset: 9004},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 323, col: 54, offset: 9007},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 323, col: 67, offset: 9020},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 323, col: 69, offset: 9022},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 323, col: 75, offset: 9028},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 327, col: 1, offset: 9119},
			expr: &actionExpr{
				pos: position{line: 327, col: 26, offset: 9144},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 327, col: 26, offset: 9144},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 327, col: 26, offset: 9144},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 327, col: 31, offset: 9149},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 36, offset: 9154},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 327, col: 38, offset: 9156},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 327, col: 41, offset: 9159},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 54, offset: 9172},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 327, col: 56, offset: 9174},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 327, col: 62, offset: 9180},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 67, offset: 9185},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 327, col: 69, offset: 9187},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 73, offset: 9191},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 327, col: 75, offset: 9193},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 327, col: 82, offset: 9200},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 331, col: 1, offset: 9291},
			expr: &actionExpr{
				pos: position{line: 331, col: 17, offset: 9307},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 331, col: 17, offset: 9307},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 331, col: 22, offset: 9312},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 331, col: 22, offset: 9312},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 28, offset: 9318},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 34, offset: 9324},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 40, offset: 9330},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 46, offset: 9336},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 52, offset: 9342},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 331, col: 58, offset: 9348},
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
			pos:  position{line: 343, col: 1, offset: 9595},
			expr: &actionExpr{
				pos: position{line: 343, col: 14, offset: 9608},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 343, col: 14, offset: 9608},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 343, col: 14, offset: 9608},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 343, col: 19, offset: 9613},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 343, col: 24, offset: 9618},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 343, col: 26, offset: 9620},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 343, col: 29, offset: 9623},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 343, col: 37, offset: 9631},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 343, col: 39, offset: 9633},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 343, col: 45, offset: 9639},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 347, col: 1, offset: 9714},
			expr: &actionExpr{
				pos: position{line: 347, col: 12, offset: 9725},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 347, col: 12, offset: 9725},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 347, col: 17, offset: 9730},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 347, col: 17, offset: 9730},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 347, col: 23, offset: 9736},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 347, col: 30, offset: 9743},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 347, col: 37, offset: 9750},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 347, col: 44, offset: 9757},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 347, col: 50, offset: 9763},
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
			pos:  position{line: 359, col: 1, offset: 10010},
			expr: &choiceExpr{
				pos: position{line: 359, col: 15, offset: 10024},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 359, col: 15, offset: 10024},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 359, col: 26, offset: 10035},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 361, col: 1, offset: 10044},
			expr: &actionExpr{
				pos: position{line: 361, col: 12, offset: 10055},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 361, col: 12, offset: 10055},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 361, col: 12, offset: 10055},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 361, col: 17, offset: 10060},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 361, col: 29, offset: 10072},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 361, col: 33, offset: 10076},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 361, col: 35, offset: 10078},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 361, col: 40, offset: 10083},
								expr: &ruleRefExpr{
									pos:  position{line: 361, col: 40, offset: 10083},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 361, col: 46, offset: 10089},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 361, col: 51, offset: 10094},
								expr: &seqExpr{
									pos: position{line: 361, col: 53, offset: 10096},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 361, col: 53, offset: 10096},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 361, col: 55, offset: 10098},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 361, col: 59, offset: 10102},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 361, col: 61, offset: 10104},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 361, col: 69, offset: 10112},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 361, col: 72, offset: 10115},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 377, col: 1, offset: 10519},
			expr: &actionExpr{
				pos: position{line: 377, col: 16, offset: 10534},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 377, col: 16, offset: 10534},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 377, col: 16, offset: 10534},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 377, col: 21, offset: 10539},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 377, col: 25, offset: 10543},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 377, col: 30, offset: 10548},
								expr: &seqExpr{
									pos: position{line: 377, col: 32, offset: 10550},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 377, col: 32, offset: 10550},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 377, col: 36, offset: 10554},
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
			pos:  position{line: 391, col: 1, offset: 10959},
			expr: &actionExpr{
				pos: position{line: 391, col: 9, offset: 10967},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 391, col: 9, offset: 10967},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 391, col: 15, offset: 10973},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 391, col: 15, offset: 10973},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 391, col: 31, offset: 10989},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 391, col: 43, offset: 11001},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 391, col: 52, offset: 11010},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 391, col: 58, offset: 11016},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 395, col: 1, offset: 11047},
			expr: &ruleRefExpr{
				pos:  position{line: 395, col: 18, offset: 11064},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 397, col: 1, offset: 11084},
			expr: &actionExpr{
				pos: position{line: 397, col: 23, offset: 11106},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 397, col: 23, offset: 11106},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 397, col: 23, offset: 11106},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 397, col: 27, offset: 11110},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 397, col: 29, offset: 11112},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 397, col: 34, offset: 11117},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 397, col: 39, offset: 11122},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 397, col: 41, offset: 11124},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 397, col: 45, offset: 11128},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 397, col: 47, offset: 11130},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 397, col: 52, offset: 11135},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 397, col: 67, offset: 11150},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 397, col: 69, offset: 11152},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 403, col: 1, offset: 11277},
			expr: &choiceExpr{
				pos: position{line: 403, col: 14, offset: 11290},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 403, col: 14, offset: 11290},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 403, col: 23, offset: 11299},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 403, col: 31, offset: 11307},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 405, col: 1, offset: 11312},
			expr: &choiceExpr{
				pos: position{line: 405, col: 11, offset: 11322},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 405, col: 11, offset: 11322},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 405, col: 20, offset: 11331},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 405, col: 29, offset: 11340},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 405, col: 36, offset: 11347},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 407, col: 1, offset: 11353},
			expr: &choiceExpr{
				pos: position{line: 407, col: 8, offset: 11360},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 407, col: 8, offset: 11360},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 407, col: 17, offset: 11369},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 407, col: 23, offset: 11375},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 409, col: 1, offset: 11380},
			expr: &actionExpr{
				pos: position{line: 409, col: 11, offset: 11390},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 409, col: 11, offset: 11390},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 409, col: 11, offset: 11390},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 409, col: 15, offset: 11394},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 409, col: 17, offset: 11396},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 409, col: 22, offset: 11401},
								expr: &seqExpr{
									pos: position{line: 409, col: 23, offset: 11402},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 409, col: 23, offset: 11402},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 27, offset: 11406},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 409, col: 29, offset: 11408},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 33, offset: 11412},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 35, offset: 11414},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 409, col: 42, offset: 11421},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 409, col: 47, offset: 11426},
								expr: &seqExpr{
									pos: position{line: 409, col: 49, offset: 11428},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 409, col: 49, offset: 11428},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 409, col: 51, offset: 11430},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 55, offset: 11434},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 57, offset: 11436},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 61, offset: 11440},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 409, col: 63, offset: 11442},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 67, offset: 11446},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 69, offset: 11448},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 409, col: 77, offset: 11456},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 409, col: 79, offset: 11458},
							expr: &litMatcher{
								pos:        position{line: 409, col: 79, offset: 11458},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 409, col: 84, offset: 11463},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 409, col: 86, offset: 11465},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 433, col: 1, offset: 12244},
			expr: &actionExpr{
				pos: position{line: 433, col: 10, offset: 12253},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 433, col: 10, offset: 12253},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 433, col: 10, offset: 12253},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 433, col: 14, offset: 12257},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 433, col: 17, offset: 12260},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 433, col: 22, offset: 12265},
								expr: &ruleRefExpr{
									pos:  position{line: 433, col: 22, offset: 12265},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 433, col: 28, offset: 12271},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 433, col: 33, offset: 12276},
								expr: &seqExpr{
									pos: position{line: 433, col: 34, offset: 12277},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 433, col: 34, offset: 12277},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 433, col: 36, offset: 12279},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 433, col: 40, offset: 12283},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 433, col: 42, offset: 12285},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 433, col: 49, offset: 12292},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 433, col: 51, offset: 12294},
							expr: &litMatcher{
								pos:        position{line: 433, col: 51, offset: 12294},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 433, col: 56, offset: 12299},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 433, col: 59, offset: 12302},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 457, col: 1, offset: 12875},
			expr: &choiceExpr{
				pos: position{line: 457, col: 8, offset: 12882},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 457, col: 8, offset: 12882},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 457, col: 19, offset: 12893},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 459, col: 1, offset: 12906},
			expr: &actionExpr{
				pos: position{line: 459, col: 13, offset: 12918},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 459, col: 13, offset: 12918},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 459, col: 13, offset: 12918},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 459, col: 20, offset: 12925},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 459, col: 22, offset: 12927},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 465, col: 1, offset: 13015},
			expr: &actionExpr{
				pos: position{line: 465, col: 16, offset: 13030},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 465, col: 16, offset: 13030},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 465, col: 16, offset: 13030},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 20, offset: 13034},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 465, col: 22, offset: 13036},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 465, col: 27, offset: 13041},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 465, col: 32, offset: 13046},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 465, col: 37, offset: 13051},
								expr: &seqExpr{
									pos: position{line: 465, col: 38, offset: 13052},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 465, col: 38, offset: 13052},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 465, col: 40, offset: 13054},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 465, col: 44, offset: 13058},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 465, col: 46, offset: 13060},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 53, offset: 13067},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 465, col: 55, offset: 13069},
							expr: &litMatcher{
								pos:        position{line: 465, col: 55, offset: 13069},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 60, offset: 13074},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 465, col: 62, offset: 13076},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 482, col: 1, offset: 13481},
			expr: &actionExpr{
				pos: position{line: 482, col: 8, offset: 13488},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 482, col: 8, offset: 13488},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 482, col: 8, offset: 13488},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 482, col: 13, offset: 13493},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 482, col: 17, offset: 13497},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 482, col: 22, offset: 13502},
								expr: &choiceExpr{
									pos: position{line: 482, col: 24, offset: 13504},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 482, col: 24, offset: 13504},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 482, col: 33, offset: 13513},
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
			pos:  position{line: 495, col: 1, offset: 13752},
			expr: &actionExpr{
				pos: position{line: 495, col: 11, offset: 13762},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 495, col: 11, offset: 13762},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 495, col: 11, offset: 13762},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 495, col: 15, offset: 13766},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 495, col: 19, offset: 13770},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 502, col: 1, offset: 13989},
			expr: &actionExpr{
				pos: position{line: 502, col: 15, offset: 14003},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 502, col: 15, offset: 14003},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 502, col: 15, offset: 14003},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 502, col: 19, offset: 14007},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 502, col: 24, offset: 14012},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 502, col: 24, offset: 14012},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 502, col: 30, offset: 14018},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 502, col: 39, offset: 14027},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 502, col: 44, offset: 14032},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 506, col: 1, offset: 14061},
			expr: &actionExpr{
				pos: position{line: 506, col: 8, offset: 14068},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 506, col: 8, offset: 14068},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 506, col: 12, offset: 14072},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 511, col: 1, offset: 14194},
			expr: &seqExpr{
				pos: position{line: 511, col: 15, offset: 14208},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 511, col: 15, offset: 14208},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 511, col: 19, offset: 14212},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 511, col: 32, offset: 14225},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 515, col: 1, offset: 14290},
			expr: &actionExpr{
				pos: position{line: 515, col: 17, offset: 14306},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 515, col: 17, offset: 14306},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 515, col: 17, offset: 14306},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 515, col: 29, offset: 14318},
							expr: &choiceExpr{
								pos: position{line: 515, col: 30, offset: 14319},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 515, col: 30, offset: 14319},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 515, col: 44, offset: 14333},
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
			pos:  position{line: 522, col: 1, offset: 14476},
			expr: &actionExpr{
				pos: position{line: 522, col: 11, offset: 14486},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 522, col: 11, offset: 14486},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 522, col: 11, offset: 14486},
							expr: &litMatcher{
								pos:        position{line: 522, col: 11, offset: 14486},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 522, col: 18, offset: 14493},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 522, col: 18, offset: 14493},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 522, col: 26, offset: 14501},
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
			pos:  position{line: 535, col: 1, offset: 14892},
			expr: &choiceExpr{
				pos: position{line: 535, col: 10, offset: 14901},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 535, col: 10, offset: 14901},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 535, col: 26, offset: 14917},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 537, col: 1, offset: 14929},
			expr: &seqExpr{
				pos: position{line: 537, col: 18, offset: 14946},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 537, col: 20, offset: 14948},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 537, col: 20, offset: 14948},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 537, col: 33, offset: 14961},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 537, col: 43, offset: 14971},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 539, col: 1, offset: 14981},
			expr: &seqExpr{
				pos: position{line: 539, col: 15, offset: 14995},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 539, col: 15, offset: 14995},
						expr: &ruleRefExpr{
							pos:  position{line: 539, col: 15, offset: 14995},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 539, col: 24, offset: 15004},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 541, col: 1, offset: 15014},
			expr: &seqExpr{
				pos: position{line: 541, col: 13, offset: 15026},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 541, col: 13, offset: 15026},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 541, col: 17, offset: 15030},
						expr: &ruleRefExpr{
							pos:  position{line: 541, col: 17, offset: 15030},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 543, col: 1, offset: 15045},
			expr: &seqExpr{
				pos: position{line: 543, col: 13, offset: 15057},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 543, col: 13, offset: 15057},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 543, col: 18, offset: 15062},
						expr: &charClassMatcher{
							pos:        position{line: 543, col: 18, offset: 15062},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 543, col: 24, offset: 15068},
						expr: &ruleRefExpr{
							pos:  position{line: 543, col: 24, offset: 15068},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 545, col: 1, offset: 15083},
			expr: &choiceExpr{
				pos: position{line: 545, col: 12, offset: 15094},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 545, col: 12, offset: 15094},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 545, col: 20, offset: 15102},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 545, col: 20, offset: 15102},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 545, col: 40, offset: 15122},
								expr: &ruleRefExpr{
									pos:  position{line: 545, col: 40, offset: 15122},
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
			pos:  position{line: 547, col: 1, offset: 15139},
			expr: &actionExpr{
				pos: position{line: 547, col: 11, offset: 15149},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 547, col: 11, offset: 15149},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 547, col: 11, offset: 15149},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 547, col: 15, offset: 15153},
							expr: &ruleRefExpr{
								pos:  position{line: 547, col: 15, offset: 15153},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 547, col: 21, offset: 15159},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 555, col: 1, offset: 15314},
			expr: &choiceExpr{
				pos: position{line: 555, col: 9, offset: 15322},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 555, col: 9, offset: 15322},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 555, col: 9, offset: 15322},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 559, col: 5, offset: 15422},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 559, col: 5, offset: 15422},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 565, col: 1, offset: 15523},
			expr: &actionExpr{
				pos: position{line: 565, col: 9, offset: 15531},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 565, col: 9, offset: 15531},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 571, col: 1, offset: 15626},
			expr: &charClassMatcher{
				pos:        position{line: 571, col: 16, offset: 15641},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 573, col: 1, offset: 15652},
			expr: &choiceExpr{
				pos: position{line: 573, col: 9, offset: 15660},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 573, col: 11, offset: 15662},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 573, col: 11, offset: 15662},
								expr: &ruleRefExpr{
									pos:  position{line: 573, col: 12, offset: 15663},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 573, col: 24, offset: 15675,
							},
						},
					},
					&seqExpr{
						pos: position{line: 573, col: 32, offset: 15683},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 573, col: 32, offset: 15683},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 573, col: 37, offset: 15688},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 575, col: 1, offset: 15706},
			expr: &charClassMatcher{
				pos:        position{line: 575, col: 16, offset: 15721},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 577, col: 1, offset: 15737},
			expr: &choiceExpr{
				pos: position{line: 577, col: 19, offset: 15755},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 577, col: 19, offset: 15755},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 577, col: 38, offset: 15774},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 579, col: 1, offset: 15789},
			expr: &charClassMatcher{
				pos:        position{line: 579, col: 21, offset: 15809},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 581, col: 1, offset: 15831},
			expr: &seqExpr{
				pos: position{line: 581, col: 18, offset: 15848},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 581, col: 18, offset: 15848},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 581, col: 22, offset: 15852},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 581, col: 31, offset: 15861},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 581, col: 40, offset: 15870},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 581, col: 49, offset: 15879},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 583, col: 1, offset: 15889},
			expr: &charClassMatcher{
				pos:        position{line: 583, col: 17, offset: 15905},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 585, col: 1, offset: 15912},
			expr: &charClassMatcher{
				pos:        position{line: 585, col: 24, offset: 15935},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 587, col: 1, offset: 15942},
			expr: &charClassMatcher{
				pos:        position{line: 587, col: 13, offset: 15954},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 589, col: 1, offset: 15967},
			expr: &oneOrMoreExpr{
				pos: position{line: 589, col: 20, offset: 15986},
				expr: &charClassMatcher{
					pos:        position{line: 589, col: 20, offset: 15986},
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
			pos:         position{line: 591, col: 1, offset: 15998},
			expr: &zeroOrMoreExpr{
				pos: position{line: 591, col: 19, offset: 16016},
				expr: &choiceExpr{
					pos: position{line: 591, col: 21, offset: 16018},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 591, col: 21, offset: 16018},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 591, col: 33, offset: 16030},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 593, col: 1, offset: 16042},
			expr: &actionExpr{
				pos: position{line: 593, col: 12, offset: 16053},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 593, col: 12, offset: 16053},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 593, col: 12, offset: 16053},
							expr: &charClassMatcher{
								pos:        position{line: 593, col: 12, offset: 16053},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 593, col: 19, offset: 16060},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 593, col: 23, offset: 16064},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 593, col: 28, offset: 16069},
								expr: &charClassMatcher{
									pos:        position{line: 593, col: 28, offset: 16069},
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
			pos:  position{line: 599, col: 1, offset: 16204},
			expr: &notExpr{
				pos: position{line: 599, col: 8, offset: 16211},
				expr: &anyMatcher{
					line: 599, col: 9, offset: 16212,
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
	body[0].Location = loc

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

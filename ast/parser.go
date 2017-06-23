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

const commentsKey = "comments"

type program struct {
	buf      []interface{}
	comments interface{}
}

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
			pos:  position{line: 36, col: 1, offset: 706},
			expr: &actionExpr{
				pos: position{line: 36, col: 12, offset: 717},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 36, col: 12, offset: 717},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 36, col: 12, offset: 717},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 36, col: 14, offset: 719},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 36, col: 19, offset: 724},
								expr: &seqExpr{
									pos: position{line: 36, col: 20, offset: 725},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 36, col: 20, offset: 725},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 36, col: 25, offset: 730},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 36, col: 30, offset: 735},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 36, col: 35, offset: 740},
												expr: &seqExpr{
													pos: position{line: 36, col: 36, offset: 741},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 36, col: 37, offset: 742},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 36, col: 37, offset: 742},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 36, col: 42, offset: 747},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 36, col: 54, offset: 759},
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
							pos:  position{line: 36, col: 63, offset: 768},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 36, col: 65, offset: 770},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 54, col: 1, offset: 1144},
			expr: &actionExpr{
				pos: position{line: 54, col: 9, offset: 1152},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 54, col: 9, offset: 1152},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 54, col: 14, offset: 1157},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 54, col: 14, offset: 1157},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 54, col: 24, offset: 1167},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 54, col: 33, offset: 1176},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 54, col: 41, offset: 1184},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 54, col: 48, offset: 1191},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 54, col: 58, offset: 1201},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 63, col: 1, offset: 1565},
			expr: &actionExpr{
				pos: position{line: 63, col: 15, offset: 1579},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 63, col: 15, offset: 1579,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 67, col: 1, offset: 1652},
			expr: &actionExpr{
				pos: position{line: 67, col: 12, offset: 1663},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 67, col: 12, offset: 1663},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 67, col: 12, offset: 1663},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 67, col: 22, offset: 1673},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 67, col: 25, offset: 1676},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 67, col: 30, offset: 1681},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 67, col: 30, offset: 1681},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 67, col: 36, offset: 1687},
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
			pos:  position{line: 101, col: 1, offset: 3003},
			expr: &actionExpr{
				pos: position{line: 101, col: 11, offset: 3013},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 101, col: 11, offset: 3013},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 101, col: 11, offset: 3013},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 101, col: 20, offset: 3022},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 101, col: 23, offset: 3025},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 101, col: 29, offset: 3031},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 101, col: 29, offset: 3031},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 101, col: 35, offset: 3037},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 101, col: 40, offset: 3042},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 101, col: 46, offset: 3048},
								expr: &seqExpr{
									pos: position{line: 101, col: 47, offset: 3049},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 101, col: 47, offset: 3049},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 101, col: 50, offset: 3052},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 101, col: 55, offset: 3057},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 101, col: 58, offset: 3060},
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
			pos:  position{line: 117, col: 1, offset: 3510},
			expr: &choiceExpr{
				pos: position{line: 117, col: 10, offset: 3519},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 117, col: 10, offset: 3519},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 117, col: 25, offset: 3534},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 119, col: 1, offset: 3547},
			expr: &actionExpr{
				pos: position{line: 119, col: 17, offset: 3563},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 119, col: 17, offset: 3563},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 119, col: 17, offset: 3563},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 119, col: 27, offset: 3573},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 119, col: 30, offset: 3576},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 119, col: 35, offset: 3581},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 119, col: 39, offset: 3585},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 119, col: 41, offset: 3587},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 119, col: 45, offset: 3591},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 119, col: 47, offset: 3593},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 119, col: 53, offset: 3599},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 163, col: 1, offset: 4531},
			expr: &actionExpr{
				pos: position{line: 163, col: 16, offset: 4546},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 163, col: 16, offset: 4546},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 163, col: 16, offset: 4546},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 163, col: 21, offset: 4551},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 163, col: 30, offset: 4560},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 163, col: 32, offset: 4562},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 163, col: 35, offset: 4565},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 163, col: 35, offset: 4565},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 163, col: 61, offset: 4591},
										expr: &seqExpr{
											pos: position{line: 163, col: 63, offset: 4593},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 163, col: 63, offset: 4593},
													name: "_",
												},
												&zeroOrOneExpr{
													pos: position{line: 163, col: 65, offset: 4595},
													expr: &ruleRefExpr{
														pos:  position{line: 163, col: 65, offset: 4595},
														name: "Else",
													},
												},
												&ruleRefExpr{
													pos:  position{line: 163, col: 71, offset: 4601},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 163, col: 73, offset: 4603},
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
			pos:  position{line: 214, col: 1, offset: 5801},
			expr: &actionExpr{
				pos: position{line: 214, col: 13, offset: 5813},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 214, col: 13, offset: 5813},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 214, col: 13, offset: 5813},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 214, col: 18, offset: 5818},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 214, col: 22, offset: 5822},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 214, col: 26, offset: 5826},
								expr: &seqExpr{
									pos: position{line: 214, col: 28, offset: 5828},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 214, col: 28, offset: 5828},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 214, col: 30, offset: 5830},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 34, offset: 5834},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 36, offset: 5836},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 41, offset: 5841},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 214, col: 43, offset: 5843},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 47, offset: 5847},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 214, col: 52, offset: 5852},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 214, col: 58, offset: 5858},
								expr: &seqExpr{
									pos: position{line: 214, col: 60, offset: 5860},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 214, col: 60, offset: 5860},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 214, col: 62, offset: 5862},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 66, offset: 5866},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 214, col: 68, offset: 5868},
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
			pos:  position{line: 249, col: 1, offset: 6856},
			expr: &actionExpr{
				pos: position{line: 249, col: 9, offset: 6864},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 249, col: 9, offset: 6864},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 249, col: 9, offset: 6864},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 249, col: 16, offset: 6871},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 249, col: 20, offset: 6875},
								expr: &seqExpr{
									pos: position{line: 249, col: 22, offset: 6877},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 249, col: 22, offset: 6877},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 249, col: 24, offset: 6879},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 249, col: 28, offset: 6883},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 249, col: 30, offset: 6885},
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
			pos:  position{line: 261, col: 1, offset: 7088},
			expr: &choiceExpr{
				pos: position{line: 261, col: 9, offset: 7096},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 261, col: 9, offset: 7096},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 261, col: 29, offset: 7116},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 263, col: 1, offset: 7135},
			expr: &actionExpr{
				pos: position{line: 263, col: 30, offset: 7164},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 263, col: 30, offset: 7164},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 263, col: 30, offset: 7164},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 34, offset: 7168},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 263, col: 36, offset: 7170},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 263, col: 40, offset: 7174},
								expr: &ruleRefExpr{
									pos:  position{line: 263, col: 40, offset: 7174},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 263, col: 56, offset: 7190},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 263, col: 58, offset: 7192},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 270, col: 1, offset: 7287},
			expr: &actionExpr{
				pos: position{line: 270, col: 22, offset: 7308},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 270, col: 22, offset: 7308},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 270, col: 22, offset: 7308},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 270, col: 26, offset: 7312},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 270, col: 28, offset: 7314},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 270, col: 32, offset: 7318},
								expr: &ruleRefExpr{
									pos:  position{line: 270, col: 32, offset: 7318},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 270, col: 48, offset: 7334},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 270, col: 50, offset: 7336},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 284, col: 1, offset: 7671},
			expr: &actionExpr{
				pos: position{line: 284, col: 19, offset: 7689},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 284, col: 19, offset: 7689},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 284, col: 19, offset: 7689},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 284, col: 24, offset: 7694},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 284, col: 32, offset: 7702},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 284, col: 37, offset: 7707},
								expr: &seqExpr{
									pos: position{line: 284, col: 38, offset: 7708},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 284, col: 38, offset: 7708},
											expr: &charClassMatcher{
												pos:        position{line: 284, col: 38, offset: 7708},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 284, col: 46, offset: 7716},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 284, col: 47, offset: 7717},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 284, col: 47, offset: 7717},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 284, col: 51, offset: 7721},
															expr: &ruleRefExpr{
																pos:  position{line: 284, col: 51, offset: 7721},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 284, col: 64, offset: 7734},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 284, col: 64, offset: 7734},
															expr: &ruleRefExpr{
																pos:  position{line: 284, col: 64, offset: 7734},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 284, col: 73, offset: 7743},
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
											pos:  position{line: 284, col: 82, offset: 7752},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 284, col: 84, offset: 7754},
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
			pos:  position{line: 290, col: 1, offset: 7943},
			expr: &actionExpr{
				pos: position{line: 290, col: 22, offset: 7964},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 290, col: 22, offset: 7964},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 290, col: 22, offset: 7964},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 290, col: 27, offset: 7969},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 290, col: 35, offset: 7977},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 290, col: 40, offset: 7982},
								expr: &seqExpr{
									pos: position{line: 290, col: 42, offset: 7984},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 290, col: 42, offset: 7984},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 290, col: 44, offset: 7986},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 290, col: 48, offset: 7990},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 290, col: 51, offset: 7993},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 290, col: 51, offset: 7993},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 290, col: 61, offset: 8003},
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
			pos:  position{line: 294, col: 1, offset: 8082},
			expr: &actionExpr{
				pos: position{line: 294, col: 12, offset: 8093},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 294, col: 12, offset: 8093},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 294, col: 12, offset: 8093},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 294, col: 16, offset: 8097},
								expr: &seqExpr{
									pos: position{line: 294, col: 18, offset: 8099},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 294, col: 18, offset: 8099},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 294, col: 24, offset: 8105},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 294, col: 30, offset: 8111},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 294, col: 34, offset: 8115},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 294, col: 39, offset: 8120},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 294, col: 44, offset: 8125},
								expr: &seqExpr{
									pos: position{line: 294, col: 46, offset: 8127},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 294, col: 46, offset: 8127},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 294, col: 49, offset: 8130},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 294, col: 54, offset: 8135},
											expr: &seqExpr{
												pos: position{line: 294, col: 55, offset: 8136},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 294, col: 55, offset: 8136},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 294, col: 58, offset: 8139},
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
			pos:  position{line: 317, col: 1, offset: 8711},
			expr: &actionExpr{
				pos: position{line: 317, col: 9, offset: 8719},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 317, col: 9, offset: 8719},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 317, col: 9, offset: 8719},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 317, col: 16, offset: 8726},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 317, col: 19, offset: 8729},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 317, col: 26, offset: 8736},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 317, col: 31, offset: 8741},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 317, col: 34, offset: 8744},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 317, col: 39, offset: 8749},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 317, col: 42, offset: 8752},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 317, col: 48, offset: 8758},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 328, col: 1, offset: 9007},
			expr: &choiceExpr{
				pos: position{line: 328, col: 9, offset: 9015},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 328, col: 10, offset: 9016},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 328, col: 10, offset: 9016},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 328, col: 27, offset: 9033},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 328, col: 52, offset: 9058},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 328, col: 64, offset: 9070},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 328, col: 77, offset: 9083},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 330, col: 1, offset: 9089},
			expr: &actionExpr{
				pos: position{line: 330, col: 19, offset: 9107},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 330, col: 19, offset: 9107},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 330, col: 19, offset: 9107},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 330, col: 26, offset: 9114},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 330, col: 31, offset: 9119},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 330, col: 33, offset: 9121},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 330, col: 37, offset: 9125},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 330, col: 39, offset: 9127},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 330, col: 44, offset: 9132},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 330, col: 49, offset: 9137},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 330, col: 51, offset: 9139},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 330, col: 54, offset: 9142},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 330, col: 67, offset: 9155},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 330, col: 69, offset: 9157},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 330, col: 75, offset: 9163},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 334, col: 1, offset: 9254},
			expr: &actionExpr{
				pos: position{line: 334, col: 26, offset: 9279},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 334, col: 26, offset: 9279},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 334, col: 26, offset: 9279},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 334, col: 31, offset: 9284},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 334, col: 36, offset: 9289},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 334, col: 38, offset: 9291},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 334, col: 41, offset: 9294},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 334, col: 54, offset: 9307},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 334, col: 56, offset: 9309},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 334, col: 62, offset: 9315},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 334, col: 67, offset: 9320},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 334, col: 69, offset: 9322},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 334, col: 73, offset: 9326},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 334, col: 75, offset: 9328},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 334, col: 82, offset: 9335},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 338, col: 1, offset: 9426},
			expr: &actionExpr{
				pos: position{line: 338, col: 17, offset: 9442},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 338, col: 17, offset: 9442},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 338, col: 22, offset: 9447},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 338, col: 22, offset: 9447},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 28, offset: 9453},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 34, offset: 9459},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 40, offset: 9465},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 46, offset: 9471},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 52, offset: 9477},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 338, col: 58, offset: 9483},
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
			pos:  position{line: 350, col: 1, offset: 9730},
			expr: &actionExpr{
				pos: position{line: 350, col: 14, offset: 9743},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 350, col: 14, offset: 9743},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 350, col: 14, offset: 9743},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 19, offset: 9748},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 24, offset: 9753},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 350, col: 26, offset: 9755},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 29, offset: 9758},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 37, offset: 9766},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 350, col: 39, offset: 9768},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 45, offset: 9774},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 354, col: 1, offset: 9849},
			expr: &actionExpr{
				pos: position{line: 354, col: 12, offset: 9860},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 354, col: 12, offset: 9860},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 354, col: 17, offset: 9865},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 354, col: 17, offset: 9865},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 354, col: 23, offset: 9871},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 354, col: 30, offset: 9878},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 354, col: 37, offset: 9885},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 354, col: 44, offset: 9892},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 354, col: 50, offset: 9898},
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
			pos:  position{line: 366, col: 1, offset: 10145},
			expr: &choiceExpr{
				pos: position{line: 366, col: 15, offset: 10159},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 366, col: 15, offset: 10159},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 366, col: 26, offset: 10170},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 368, col: 1, offset: 10179},
			expr: &actionExpr{
				pos: position{line: 368, col: 12, offset: 10190},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 368, col: 12, offset: 10190},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 368, col: 12, offset: 10190},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 368, col: 17, offset: 10195},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 368, col: 29, offset: 10207},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 368, col: 33, offset: 10211},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 368, col: 35, offset: 10213},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 368, col: 40, offset: 10218},
								expr: &ruleRefExpr{
									pos:  position{line: 368, col: 40, offset: 10218},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 368, col: 46, offset: 10224},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 368, col: 51, offset: 10229},
								expr: &seqExpr{
									pos: position{line: 368, col: 53, offset: 10231},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 368, col: 53, offset: 10231},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 368, col: 55, offset: 10233},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 368, col: 59, offset: 10237},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 368, col: 61, offset: 10239},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 368, col: 69, offset: 10247},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 368, col: 72, offset: 10250},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 384, col: 1, offset: 10654},
			expr: &actionExpr{
				pos: position{line: 384, col: 16, offset: 10669},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 384, col: 16, offset: 10669},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 384, col: 16, offset: 10669},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 384, col: 21, offset: 10674},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 384, col: 25, offset: 10678},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 384, col: 30, offset: 10683},
								expr: &seqExpr{
									pos: position{line: 384, col: 32, offset: 10685},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 384, col: 32, offset: 10685},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 384, col: 36, offset: 10689},
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
			pos:  position{line: 398, col: 1, offset: 11094},
			expr: &actionExpr{
				pos: position{line: 398, col: 9, offset: 11102},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 398, col: 9, offset: 11102},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 398, col: 15, offset: 11108},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 398, col: 15, offset: 11108},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 398, col: 31, offset: 11124},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 398, col: 43, offset: 11136},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 398, col: 52, offset: 11145},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 398, col: 58, offset: 11151},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 402, col: 1, offset: 11182},
			expr: &ruleRefExpr{
				pos:  position{line: 402, col: 18, offset: 11199},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 404, col: 1, offset: 11219},
			expr: &actionExpr{
				pos: position{line: 404, col: 23, offset: 11241},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 404, col: 23, offset: 11241},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 404, col: 23, offset: 11241},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 404, col: 27, offset: 11245},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 404, col: 29, offset: 11247},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 404, col: 34, offset: 11252},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 404, col: 39, offset: 11257},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 404, col: 41, offset: 11259},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 404, col: 45, offset: 11263},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 404, col: 47, offset: 11265},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 404, col: 52, offset: 11270},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 404, col: 67, offset: 11285},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 404, col: 69, offset: 11287},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 410, col: 1, offset: 11412},
			expr: &choiceExpr{
				pos: position{line: 410, col: 14, offset: 11425},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 410, col: 14, offset: 11425},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 410, col: 23, offset: 11434},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 410, col: 31, offset: 11442},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 412, col: 1, offset: 11447},
			expr: &choiceExpr{
				pos: position{line: 412, col: 11, offset: 11457},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 412, col: 11, offset: 11457},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 412, col: 20, offset: 11466},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 412, col: 29, offset: 11475},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 412, col: 36, offset: 11482},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 414, col: 1, offset: 11488},
			expr: &choiceExpr{
				pos: position{line: 414, col: 8, offset: 11495},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 414, col: 8, offset: 11495},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 414, col: 17, offset: 11504},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 414, col: 23, offset: 11510},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 416, col: 1, offset: 11515},
			expr: &actionExpr{
				pos: position{line: 416, col: 11, offset: 11525},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 416, col: 11, offset: 11525},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 416, col: 11, offset: 11525},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 416, col: 15, offset: 11529},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 416, col: 17, offset: 11531},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 416, col: 22, offset: 11536},
								expr: &seqExpr{
									pos: position{line: 416, col: 23, offset: 11537},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 416, col: 23, offset: 11537},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 27, offset: 11541},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 416, col: 29, offset: 11543},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 33, offset: 11547},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 35, offset: 11549},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 416, col: 42, offset: 11556},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 416, col: 47, offset: 11561},
								expr: &seqExpr{
									pos: position{line: 416, col: 49, offset: 11563},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 416, col: 49, offset: 11563},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 416, col: 51, offset: 11565},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 55, offset: 11569},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 57, offset: 11571},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 61, offset: 11575},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 416, col: 63, offset: 11577},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 67, offset: 11581},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 416, col: 69, offset: 11583},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 416, col: 77, offset: 11591},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 416, col: 79, offset: 11593},
							expr: &litMatcher{
								pos:        position{line: 416, col: 79, offset: 11593},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 416, col: 84, offset: 11598},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 416, col: 86, offset: 11600},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 440, col: 1, offset: 12379},
			expr: &actionExpr{
				pos: position{line: 440, col: 10, offset: 12388},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 440, col: 10, offset: 12388},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 440, col: 10, offset: 12388},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 14, offset: 12392},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 440, col: 17, offset: 12395},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 440, col: 22, offset: 12400},
								expr: &ruleRefExpr{
									pos:  position{line: 440, col: 22, offset: 12400},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 440, col: 28, offset: 12406},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 440, col: 33, offset: 12411},
								expr: &seqExpr{
									pos: position{line: 440, col: 34, offset: 12412},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 440, col: 34, offset: 12412},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 440, col: 36, offset: 12414},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 440, col: 40, offset: 12418},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 440, col: 42, offset: 12420},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 49, offset: 12427},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 440, col: 51, offset: 12429},
							expr: &litMatcher{
								pos:        position{line: 440, col: 51, offset: 12429},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 440, col: 56, offset: 12434},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 440, col: 59, offset: 12437},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 464, col: 1, offset: 13010},
			expr: &choiceExpr{
				pos: position{line: 464, col: 8, offset: 13017},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 464, col: 8, offset: 13017},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 464, col: 19, offset: 13028},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 466, col: 1, offset: 13041},
			expr: &actionExpr{
				pos: position{line: 466, col: 13, offset: 13053},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 466, col: 13, offset: 13053},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 466, col: 13, offset: 13053},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 466, col: 20, offset: 13060},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 466, col: 22, offset: 13062},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 472, col: 1, offset: 13150},
			expr: &actionExpr{
				pos: position{line: 472, col: 16, offset: 13165},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 472, col: 16, offset: 13165},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 472, col: 16, offset: 13165},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 472, col: 20, offset: 13169},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 472, col: 22, offset: 13171},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 472, col: 27, offset: 13176},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 472, col: 32, offset: 13181},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 472, col: 37, offset: 13186},
								expr: &seqExpr{
									pos: position{line: 472, col: 38, offset: 13187},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 472, col: 38, offset: 13187},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 472, col: 40, offset: 13189},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 472, col: 44, offset: 13193},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 472, col: 46, offset: 13195},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 472, col: 53, offset: 13202},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 472, col: 55, offset: 13204},
							expr: &litMatcher{
								pos:        position{line: 472, col: 55, offset: 13204},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 472, col: 60, offset: 13209},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 472, col: 62, offset: 13211},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 489, col: 1, offset: 13616},
			expr: &actionExpr{
				pos: position{line: 489, col: 8, offset: 13623},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 489, col: 8, offset: 13623},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 489, col: 8, offset: 13623},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 489, col: 13, offset: 13628},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 489, col: 17, offset: 13632},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 489, col: 22, offset: 13637},
								expr: &choiceExpr{
									pos: position{line: 489, col: 24, offset: 13639},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 489, col: 24, offset: 13639},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 489, col: 33, offset: 13648},
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
			pos:  position{line: 502, col: 1, offset: 13887},
			expr: &actionExpr{
				pos: position{line: 502, col: 11, offset: 13897},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 502, col: 11, offset: 13897},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 502, col: 11, offset: 13897},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 502, col: 15, offset: 13901},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 502, col: 19, offset: 13905},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 509, col: 1, offset: 14124},
			expr: &actionExpr{
				pos: position{line: 509, col: 15, offset: 14138},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 509, col: 15, offset: 14138},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 509, col: 15, offset: 14138},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 509, col: 19, offset: 14142},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 509, col: 24, offset: 14147},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 509, col: 24, offset: 14147},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 509, col: 30, offset: 14153},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 509, col: 39, offset: 14162},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 509, col: 44, offset: 14167},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 513, col: 1, offset: 14196},
			expr: &actionExpr{
				pos: position{line: 513, col: 8, offset: 14203},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 513, col: 8, offset: 14203},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 513, col: 12, offset: 14207},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 518, col: 1, offset: 14329},
			expr: &seqExpr{
				pos: position{line: 518, col: 15, offset: 14343},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 518, col: 15, offset: 14343},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 518, col: 19, offset: 14347},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 518, col: 32, offset: 14360},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 522, col: 1, offset: 14425},
			expr: &actionExpr{
				pos: position{line: 522, col: 17, offset: 14441},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 522, col: 17, offset: 14441},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 522, col: 17, offset: 14441},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 522, col: 29, offset: 14453},
							expr: &choiceExpr{
								pos: position{line: 522, col: 30, offset: 14454},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 522, col: 30, offset: 14454},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 522, col: 44, offset: 14468},
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
			pos:  position{line: 529, col: 1, offset: 14611},
			expr: &actionExpr{
				pos: position{line: 529, col: 11, offset: 14621},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 529, col: 11, offset: 14621},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 529, col: 11, offset: 14621},
							expr: &litMatcher{
								pos:        position{line: 529, col: 11, offset: 14621},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 529, col: 18, offset: 14628},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 529, col: 18, offset: 14628},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 529, col: 26, offset: 14636},
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
			pos:  position{line: 542, col: 1, offset: 15027},
			expr: &choiceExpr{
				pos: position{line: 542, col: 10, offset: 15036},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 542, col: 10, offset: 15036},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 542, col: 26, offset: 15052},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 544, col: 1, offset: 15064},
			expr: &seqExpr{
				pos: position{line: 544, col: 18, offset: 15081},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 544, col: 20, offset: 15083},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 544, col: 20, offset: 15083},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 544, col: 33, offset: 15096},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 544, col: 43, offset: 15106},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 546, col: 1, offset: 15116},
			expr: &seqExpr{
				pos: position{line: 546, col: 15, offset: 15130},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 546, col: 15, offset: 15130},
						expr: &ruleRefExpr{
							pos:  position{line: 546, col: 15, offset: 15130},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 546, col: 24, offset: 15139},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 548, col: 1, offset: 15149},
			expr: &seqExpr{
				pos: position{line: 548, col: 13, offset: 15161},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 548, col: 13, offset: 15161},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 548, col: 17, offset: 15165},
						expr: &ruleRefExpr{
							pos:  position{line: 548, col: 17, offset: 15165},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 550, col: 1, offset: 15180},
			expr: &seqExpr{
				pos: position{line: 550, col: 13, offset: 15192},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 550, col: 13, offset: 15192},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 550, col: 18, offset: 15197},
						expr: &charClassMatcher{
							pos:        position{line: 550, col: 18, offset: 15197},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 550, col: 24, offset: 15203},
						expr: &ruleRefExpr{
							pos:  position{line: 550, col: 24, offset: 15203},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 552, col: 1, offset: 15218},
			expr: &choiceExpr{
				pos: position{line: 552, col: 12, offset: 15229},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 552, col: 12, offset: 15229},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 552, col: 20, offset: 15237},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 552, col: 20, offset: 15237},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 552, col: 40, offset: 15257},
								expr: &ruleRefExpr{
									pos:  position{line: 552, col: 40, offset: 15257},
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
			pos:  position{line: 554, col: 1, offset: 15274},
			expr: &actionExpr{
				pos: position{line: 554, col: 11, offset: 15284},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 554, col: 11, offset: 15284},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 554, col: 11, offset: 15284},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 554, col: 15, offset: 15288},
							expr: &ruleRefExpr{
								pos:  position{line: 554, col: 15, offset: 15288},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 554, col: 21, offset: 15294},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 562, col: 1, offset: 15449},
			expr: &choiceExpr{
				pos: position{line: 562, col: 9, offset: 15457},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 562, col: 9, offset: 15457},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 562, col: 9, offset: 15457},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 566, col: 5, offset: 15557},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 566, col: 5, offset: 15557},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 572, col: 1, offset: 15658},
			expr: &actionExpr{
				pos: position{line: 572, col: 9, offset: 15666},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 572, col: 9, offset: 15666},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 578, col: 1, offset: 15761},
			expr: &charClassMatcher{
				pos:        position{line: 578, col: 16, offset: 15776},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 580, col: 1, offset: 15787},
			expr: &choiceExpr{
				pos: position{line: 580, col: 9, offset: 15795},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 580, col: 11, offset: 15797},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 580, col: 11, offset: 15797},
								expr: &ruleRefExpr{
									pos:  position{line: 580, col: 12, offset: 15798},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 580, col: 24, offset: 15810,
							},
						},
					},
					&seqExpr{
						pos: position{line: 580, col: 32, offset: 15818},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 580, col: 32, offset: 15818},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 580, col: 37, offset: 15823},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 582, col: 1, offset: 15841},
			expr: &charClassMatcher{
				pos:        position{line: 582, col: 16, offset: 15856},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 584, col: 1, offset: 15872},
			expr: &choiceExpr{
				pos: position{line: 584, col: 19, offset: 15890},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 584, col: 19, offset: 15890},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 584, col: 38, offset: 15909},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 586, col: 1, offset: 15924},
			expr: &charClassMatcher{
				pos:        position{line: 586, col: 21, offset: 15944},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 588, col: 1, offset: 15966},
			expr: &seqExpr{
				pos: position{line: 588, col: 18, offset: 15983},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 588, col: 18, offset: 15983},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 588, col: 22, offset: 15987},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 588, col: 31, offset: 15996},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 588, col: 40, offset: 16005},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 588, col: 49, offset: 16014},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 590, col: 1, offset: 16024},
			expr: &charClassMatcher{
				pos:        position{line: 590, col: 17, offset: 16040},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 592, col: 1, offset: 16047},
			expr: &charClassMatcher{
				pos:        position{line: 592, col: 24, offset: 16070},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 594, col: 1, offset: 16077},
			expr: &charClassMatcher{
				pos:        position{line: 594, col: 13, offset: 16089},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 596, col: 1, offset: 16102},
			expr: &oneOrMoreExpr{
				pos: position{line: 596, col: 20, offset: 16121},
				expr: &charClassMatcher{
					pos:        position{line: 596, col: 20, offset: 16121},
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
			pos:         position{line: 598, col: 1, offset: 16133},
			expr: &zeroOrMoreExpr{
				pos: position{line: 598, col: 19, offset: 16151},
				expr: &choiceExpr{
					pos: position{line: 598, col: 21, offset: 16153},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 598, col: 21, offset: 16153},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 598, col: 33, offset: 16165},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 600, col: 1, offset: 16177},
			expr: &actionExpr{
				pos: position{line: 600, col: 12, offset: 16188},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 600, col: 12, offset: 16188},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 600, col: 12, offset: 16188},
							expr: &charClassMatcher{
								pos:        position{line: 600, col: 12, offset: 16188},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 600, col: 19, offset: 16195},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 600, col: 23, offset: 16199},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 600, col: 28, offset: 16204},
								expr: &charClassMatcher{
									pos:        position{line: 600, col: 28, offset: 16204},
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
			pos:  position{line: 611, col: 1, offset: 16480},
			expr: &notExpr{
				pos: position{line: 611, col: 8, offset: 16487},
				expr: &anyMatcher{
					line: 611, col: 9, offset: 16488,
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

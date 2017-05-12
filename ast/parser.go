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
							label: "val",
							expr: &seqExpr{
								pos: position{line: 157, col: 37, offset: 4466},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 157, col: 37, offset: 4466},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 157, col: 63, offset: 4492},
										expr: &seqExpr{
											pos: position{line: 157, col: 64, offset: 4493},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 157, col: 64, offset: 4493},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 157, col: 66, offset: 4495},
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
			pos:  position{line: 184, col: 1, offset: 5148},
			expr: &actionExpr{
				pos: position{line: 184, col: 13, offset: 5160},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 184, col: 13, offset: 5160},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 184, col: 13, offset: 5160},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 184, col: 18, offset: 5165},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 184, col: 22, offset: 5169},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 184, col: 26, offset: 5173},
								expr: &seqExpr{
									pos: position{line: 184, col: 28, offset: 5175},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 184, col: 28, offset: 5175},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 184, col: 30, offset: 5177},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 34, offset: 5181},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 36, offset: 5183},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 41, offset: 5188},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 184, col: 43, offset: 5190},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 47, offset: 5194},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 184, col: 52, offset: 5199},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 184, col: 58, offset: 5205},
								expr: &seqExpr{
									pos: position{line: 184, col: 60, offset: 5207},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 184, col: 60, offset: 5207},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 184, col: 62, offset: 5209},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 66, offset: 5213},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 184, col: 68, offset: 5215},
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
			pos:  position{line: 218, col: 1, offset: 6159},
			expr: &choiceExpr{
				pos: position{line: 218, col: 9, offset: 6167},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 218, col: 9, offset: 6167},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 218, col: 29, offset: 6187},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 220, col: 1, offset: 6206},
			expr: &actionExpr{
				pos: position{line: 220, col: 30, offset: 6235},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 220, col: 30, offset: 6235},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 220, col: 30, offset: 6235},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 220, col: 34, offset: 6239},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 220, col: 36, offset: 6241},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 220, col: 40, offset: 6245},
								expr: &ruleRefExpr{
									pos:  position{line: 220, col: 40, offset: 6245},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 220, col: 56, offset: 6261},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 220, col: 58, offset: 6263},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 227, col: 1, offset: 6358},
			expr: &actionExpr{
				pos: position{line: 227, col: 22, offset: 6379},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 227, col: 22, offset: 6379},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 227, col: 22, offset: 6379},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 227, col: 26, offset: 6383},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 227, col: 28, offset: 6385},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 227, col: 32, offset: 6389},
								expr: &ruleRefExpr{
									pos:  position{line: 227, col: 32, offset: 6389},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 227, col: 48, offset: 6405},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 227, col: 50, offset: 6407},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 240, col: 1, offset: 6706},
			expr: &actionExpr{
				pos: position{line: 240, col: 19, offset: 6724},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 240, col: 19, offset: 6724},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 240, col: 19, offset: 6724},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 240, col: 24, offset: 6729},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 240, col: 32, offset: 6737},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 240, col: 37, offset: 6742},
								expr: &seqExpr{
									pos: position{line: 240, col: 38, offset: 6743},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 240, col: 38, offset: 6743},
											expr: &charClassMatcher{
												pos:        position{line: 240, col: 38, offset: 6743},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 240, col: 46, offset: 6751},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 240, col: 47, offset: 6752},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 240, col: 47, offset: 6752},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 240, col: 51, offset: 6756},
															expr: &ruleRefExpr{
																pos:  position{line: 240, col: 51, offset: 6756},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 240, col: 64, offset: 6769},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 240, col: 64, offset: 6769},
															expr: &ruleRefExpr{
																pos:  position{line: 240, col: 64, offset: 6769},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 240, col: 73, offset: 6778},
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
											pos:  position{line: 240, col: 82, offset: 6787},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 240, col: 84, offset: 6789},
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
			pos:  position{line: 246, col: 1, offset: 6978},
			expr: &actionExpr{
				pos: position{line: 246, col: 22, offset: 6999},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 246, col: 22, offset: 6999},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 246, col: 22, offset: 6999},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 246, col: 27, offset: 7004},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 246, col: 35, offset: 7012},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 246, col: 40, offset: 7017},
								expr: &seqExpr{
									pos: position{line: 246, col: 42, offset: 7019},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 246, col: 42, offset: 7019},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 246, col: 44, offset: 7021},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 246, col: 48, offset: 7025},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 246, col: 51, offset: 7028},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 246, col: 51, offset: 7028},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 246, col: 61, offset: 7038},
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
			pos:  position{line: 250, col: 1, offset: 7117},
			expr: &actionExpr{
				pos: position{line: 250, col: 12, offset: 7128},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 250, col: 12, offset: 7128},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 250, col: 12, offset: 7128},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 250, col: 16, offset: 7132},
								expr: &seqExpr{
									pos: position{line: 250, col: 18, offset: 7134},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 250, col: 18, offset: 7134},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 250, col: 24, offset: 7140},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 250, col: 30, offset: 7146},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 250, col: 34, offset: 7150},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 250, col: 39, offset: 7155},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 250, col: 44, offset: 7160},
								expr: &seqExpr{
									pos: position{line: 250, col: 46, offset: 7162},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 250, col: 46, offset: 7162},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 250, col: 49, offset: 7165},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 250, col: 54, offset: 7170},
											expr: &seqExpr{
												pos: position{line: 250, col: 55, offset: 7171},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 250, col: 55, offset: 7171},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 250, col: 58, offset: 7174},
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
			pos:  position{line: 273, col: 1, offset: 7746},
			expr: &actionExpr{
				pos: position{line: 273, col: 9, offset: 7754},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 273, col: 9, offset: 7754},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 273, col: 9, offset: 7754},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 273, col: 16, offset: 7761},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 273, col: 19, offset: 7764},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 273, col: 26, offset: 7771},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 273, col: 31, offset: 7776},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 273, col: 34, offset: 7779},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 273, col: 39, offset: 7784},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 273, col: 42, offset: 7787},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 273, col: 48, offset: 7793},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 284, col: 1, offset: 8042},
			expr: &choiceExpr{
				pos: position{line: 284, col: 9, offset: 8050},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 284, col: 10, offset: 8051},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 284, col: 10, offset: 8051},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 284, col: 27, offset: 8068},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 284, col: 52, offset: 8093},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 284, col: 64, offset: 8105},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 284, col: 77, offset: 8118},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 286, col: 1, offset: 8124},
			expr: &actionExpr{
				pos: position{line: 286, col: 19, offset: 8142},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 286, col: 19, offset: 8142},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 286, col: 19, offset: 8142},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 286, col: 26, offset: 8149},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 31, offset: 8154},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 286, col: 33, offset: 8156},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 37, offset: 8160},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 286, col: 39, offset: 8162},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 286, col: 44, offset: 8167},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 49, offset: 8172},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 286, col: 51, offset: 8174},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 286, col: 54, offset: 8177},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 67, offset: 8190},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 286, col: 69, offset: 8192},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 286, col: 75, offset: 8198},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 290, col: 1, offset: 8289},
			expr: &actionExpr{
				pos: position{line: 290, col: 26, offset: 8314},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 290, col: 26, offset: 8314},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 290, col: 26, offset: 8314},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 290, col: 31, offset: 8319},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 36, offset: 8324},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 290, col: 38, offset: 8326},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 290, col: 41, offset: 8329},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 54, offset: 8342},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 290, col: 56, offset: 8344},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 290, col: 62, offset: 8350},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 67, offset: 8355},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 290, col: 69, offset: 8357},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 73, offset: 8361},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 290, col: 75, offset: 8363},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 290, col: 82, offset: 8370},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 294, col: 1, offset: 8461},
			expr: &actionExpr{
				pos: position{line: 294, col: 17, offset: 8477},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 294, col: 17, offset: 8477},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 294, col: 22, offset: 8482},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 294, col: 22, offset: 8482},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 28, offset: 8488},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 34, offset: 8494},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 40, offset: 8500},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 46, offset: 8506},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 52, offset: 8512},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 294, col: 58, offset: 8518},
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
			pos:  position{line: 306, col: 1, offset: 8765},
			expr: &actionExpr{
				pos: position{line: 306, col: 14, offset: 8778},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 306, col: 14, offset: 8778},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 306, col: 14, offset: 8778},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 19, offset: 8783},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 306, col: 24, offset: 8788},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 306, col: 26, offset: 8790},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 29, offset: 8793},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 306, col: 37, offset: 8801},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 306, col: 39, offset: 8803},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 45, offset: 8809},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 310, col: 1, offset: 8884},
			expr: &actionExpr{
				pos: position{line: 310, col: 12, offset: 8895},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 310, col: 12, offset: 8895},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 310, col: 17, offset: 8900},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 310, col: 17, offset: 8900},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 23, offset: 8906},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 30, offset: 8913},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 37, offset: 8920},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 44, offset: 8927},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 50, offset: 8933},
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
			pos:  position{line: 322, col: 1, offset: 9180},
			expr: &choiceExpr{
				pos: position{line: 322, col: 15, offset: 9194},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 322, col: 15, offset: 9194},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 322, col: 26, offset: 9205},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 324, col: 1, offset: 9214},
			expr: &actionExpr{
				pos: position{line: 324, col: 12, offset: 9225},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 324, col: 12, offset: 9225},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 324, col: 12, offset: 9225},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 324, col: 17, offset: 9230},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 324, col: 29, offset: 9242},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 324, col: 33, offset: 9246},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 324, col: 35, offset: 9248},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 324, col: 40, offset: 9253},
								expr: &ruleRefExpr{
									pos:  position{line: 324, col: 40, offset: 9253},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 324, col: 46, offset: 9259},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 324, col: 51, offset: 9264},
								expr: &seqExpr{
									pos: position{line: 324, col: 53, offset: 9266},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 324, col: 53, offset: 9266},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 324, col: 55, offset: 9268},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 324, col: 59, offset: 9272},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 324, col: 61, offset: 9274},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 324, col: 69, offset: 9282},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 324, col: 72, offset: 9285},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 340, col: 1, offset: 9689},
			expr: &actionExpr{
				pos: position{line: 340, col: 16, offset: 9704},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 340, col: 16, offset: 9704},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 340, col: 16, offset: 9704},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 340, col: 21, offset: 9709},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 340, col: 25, offset: 9713},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 340, col: 30, offset: 9718},
								expr: &seqExpr{
									pos: position{line: 340, col: 32, offset: 9720},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 340, col: 32, offset: 9720},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 340, col: 36, offset: 9724},
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
			pos:  position{line: 354, col: 1, offset: 10129},
			expr: &actionExpr{
				pos: position{line: 354, col: 9, offset: 10137},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 354, col: 9, offset: 10137},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 354, col: 15, offset: 10143},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 354, col: 15, offset: 10143},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 354, col: 31, offset: 10159},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 354, col: 43, offset: 10171},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 354, col: 52, offset: 10180},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 354, col: 58, offset: 10186},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 358, col: 1, offset: 10217},
			expr: &ruleRefExpr{
				pos:  position{line: 358, col: 18, offset: 10234},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 360, col: 1, offset: 10254},
			expr: &actionExpr{
				pos: position{line: 360, col: 23, offset: 10276},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 360, col: 23, offset: 10276},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 360, col: 23, offset: 10276},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 27, offset: 10280},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 360, col: 29, offset: 10282},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 360, col: 34, offset: 10287},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 39, offset: 10292},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 360, col: 41, offset: 10294},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 45, offset: 10298},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 360, col: 47, offset: 10300},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 360, col: 52, offset: 10305},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 360, col: 67, offset: 10320},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 360, col: 69, offset: 10322},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 366, col: 1, offset: 10447},
			expr: &choiceExpr{
				pos: position{line: 366, col: 14, offset: 10460},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 366, col: 14, offset: 10460},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 366, col: 23, offset: 10469},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 366, col: 31, offset: 10477},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 368, col: 1, offset: 10482},
			expr: &choiceExpr{
				pos: position{line: 368, col: 11, offset: 10492},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 368, col: 11, offset: 10492},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 368, col: 20, offset: 10501},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 368, col: 29, offset: 10510},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 368, col: 36, offset: 10517},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 370, col: 1, offset: 10523},
			expr: &choiceExpr{
				pos: position{line: 370, col: 8, offset: 10530},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 370, col: 8, offset: 10530},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 370, col: 17, offset: 10539},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 370, col: 23, offset: 10545},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 372, col: 1, offset: 10550},
			expr: &actionExpr{
				pos: position{line: 372, col: 11, offset: 10560},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 372, col: 11, offset: 10560},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 372, col: 11, offset: 10560},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 372, col: 15, offset: 10564},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 372, col: 17, offset: 10566},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 372, col: 22, offset: 10571},
								expr: &seqExpr{
									pos: position{line: 372, col: 23, offset: 10572},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 372, col: 23, offset: 10572},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 27, offset: 10576},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 372, col: 29, offset: 10578},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 33, offset: 10582},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 35, offset: 10584},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 372, col: 42, offset: 10591},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 372, col: 47, offset: 10596},
								expr: &seqExpr{
									pos: position{line: 372, col: 49, offset: 10598},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 372, col: 49, offset: 10598},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 372, col: 51, offset: 10600},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 55, offset: 10604},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 57, offset: 10606},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 61, offset: 10610},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 372, col: 63, offset: 10612},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 67, offset: 10616},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 372, col: 69, offset: 10618},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 372, col: 77, offset: 10626},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 372, col: 79, offset: 10628},
							expr: &litMatcher{
								pos:        position{line: 372, col: 79, offset: 10628},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 372, col: 84, offset: 10633},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 372, col: 86, offset: 10635},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 396, col: 1, offset: 11414},
			expr: &actionExpr{
				pos: position{line: 396, col: 10, offset: 11423},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 396, col: 10, offset: 11423},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 396, col: 10, offset: 11423},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 14, offset: 11427},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 396, col: 17, offset: 11430},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 396, col: 22, offset: 11435},
								expr: &ruleRefExpr{
									pos:  position{line: 396, col: 22, offset: 11435},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 396, col: 28, offset: 11441},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 396, col: 33, offset: 11446},
								expr: &seqExpr{
									pos: position{line: 396, col: 34, offset: 11447},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 396, col: 34, offset: 11447},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 396, col: 36, offset: 11449},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 396, col: 40, offset: 11453},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 396, col: 42, offset: 11455},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 49, offset: 11462},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 396, col: 51, offset: 11464},
							expr: &litMatcher{
								pos:        position{line: 396, col: 51, offset: 11464},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 396, col: 56, offset: 11469},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 396, col: 59, offset: 11472},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 420, col: 1, offset: 12045},
			expr: &choiceExpr{
				pos: position{line: 420, col: 8, offset: 12052},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 420, col: 8, offset: 12052},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 420, col: 19, offset: 12063},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 422, col: 1, offset: 12076},
			expr: &actionExpr{
				pos: position{line: 422, col: 13, offset: 12088},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 422, col: 13, offset: 12088},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 422, col: 13, offset: 12088},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 422, col: 20, offset: 12095},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 422, col: 22, offset: 12097},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 428, col: 1, offset: 12185},
			expr: &actionExpr{
				pos: position{line: 428, col: 16, offset: 12200},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 428, col: 16, offset: 12200},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 428, col: 16, offset: 12200},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 428, col: 20, offset: 12204},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 428, col: 22, offset: 12206},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 428, col: 27, offset: 12211},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 428, col: 32, offset: 12216},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 428, col: 37, offset: 12221},
								expr: &seqExpr{
									pos: position{line: 428, col: 38, offset: 12222},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 428, col: 38, offset: 12222},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 428, col: 40, offset: 12224},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 428, col: 44, offset: 12228},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 428, col: 46, offset: 12230},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 428, col: 53, offset: 12237},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 428, col: 55, offset: 12239},
							expr: &litMatcher{
								pos:        position{line: 428, col: 55, offset: 12239},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 428, col: 60, offset: 12244},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 428, col: 62, offset: 12246},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 445, col: 1, offset: 12651},
			expr: &actionExpr{
				pos: position{line: 445, col: 8, offset: 12658},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 445, col: 8, offset: 12658},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 445, col: 8, offset: 12658},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 13, offset: 12663},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 445, col: 17, offset: 12667},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 445, col: 22, offset: 12672},
								expr: &choiceExpr{
									pos: position{line: 445, col: 24, offset: 12674},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 445, col: 24, offset: 12674},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 445, col: 33, offset: 12683},
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
			pos:  position{line: 458, col: 1, offset: 12922},
			expr: &actionExpr{
				pos: position{line: 458, col: 11, offset: 12932},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 458, col: 11, offset: 12932},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 458, col: 11, offset: 12932},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 458, col: 15, offset: 12936},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 458, col: 19, offset: 12940},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 465, col: 1, offset: 13159},
			expr: &actionExpr{
				pos: position{line: 465, col: 15, offset: 13173},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 465, col: 15, offset: 13173},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 465, col: 15, offset: 13173},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 465, col: 19, offset: 13177},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 465, col: 24, offset: 13182},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 465, col: 24, offset: 13182},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 465, col: 30, offset: 13188},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 465, col: 39, offset: 13197},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 465, col: 44, offset: 13202},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 469, col: 1, offset: 13231},
			expr: &actionExpr{
				pos: position{line: 469, col: 8, offset: 13238},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 469, col: 8, offset: 13238},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 469, col: 12, offset: 13242},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 474, col: 1, offset: 13364},
			expr: &seqExpr{
				pos: position{line: 474, col: 15, offset: 13378},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 474, col: 15, offset: 13378},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 474, col: 19, offset: 13382},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 474, col: 32, offset: 13395},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 478, col: 1, offset: 13460},
			expr: &actionExpr{
				pos: position{line: 478, col: 17, offset: 13476},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 478, col: 17, offset: 13476},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 478, col: 17, offset: 13476},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 478, col: 29, offset: 13488},
							expr: &choiceExpr{
								pos: position{line: 478, col: 30, offset: 13489},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 478, col: 30, offset: 13489},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 478, col: 44, offset: 13503},
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
			pos:  position{line: 485, col: 1, offset: 13646},
			expr: &actionExpr{
				pos: position{line: 485, col: 11, offset: 13656},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 485, col: 11, offset: 13656},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 485, col: 11, offset: 13656},
							expr: &litMatcher{
								pos:        position{line: 485, col: 11, offset: 13656},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 485, col: 18, offset: 13663},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 485, col: 18, offset: 13663},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 485, col: 26, offset: 13671},
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
			pos:  position{line: 498, col: 1, offset: 14062},
			expr: &choiceExpr{
				pos: position{line: 498, col: 10, offset: 14071},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 498, col: 10, offset: 14071},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 498, col: 26, offset: 14087},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 500, col: 1, offset: 14099},
			expr: &seqExpr{
				pos: position{line: 500, col: 18, offset: 14116},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 500, col: 20, offset: 14118},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 500, col: 20, offset: 14118},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 500, col: 33, offset: 14131},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 500, col: 43, offset: 14141},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 502, col: 1, offset: 14151},
			expr: &seqExpr{
				pos: position{line: 502, col: 15, offset: 14165},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 502, col: 15, offset: 14165},
						expr: &ruleRefExpr{
							pos:  position{line: 502, col: 15, offset: 14165},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 502, col: 24, offset: 14174},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 504, col: 1, offset: 14184},
			expr: &seqExpr{
				pos: position{line: 504, col: 13, offset: 14196},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 504, col: 13, offset: 14196},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 504, col: 17, offset: 14200},
						expr: &ruleRefExpr{
							pos:  position{line: 504, col: 17, offset: 14200},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 506, col: 1, offset: 14215},
			expr: &seqExpr{
				pos: position{line: 506, col: 13, offset: 14227},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 506, col: 13, offset: 14227},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 506, col: 18, offset: 14232},
						expr: &charClassMatcher{
							pos:        position{line: 506, col: 18, offset: 14232},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 506, col: 24, offset: 14238},
						expr: &ruleRefExpr{
							pos:  position{line: 506, col: 24, offset: 14238},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 508, col: 1, offset: 14253},
			expr: &choiceExpr{
				pos: position{line: 508, col: 12, offset: 14264},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 508, col: 12, offset: 14264},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 508, col: 20, offset: 14272},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 508, col: 20, offset: 14272},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 508, col: 40, offset: 14292},
								expr: &ruleRefExpr{
									pos:  position{line: 508, col: 40, offset: 14292},
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
			pos:  position{line: 510, col: 1, offset: 14309},
			expr: &actionExpr{
				pos: position{line: 510, col: 11, offset: 14319},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 510, col: 11, offset: 14319},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 510, col: 11, offset: 14319},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 510, col: 15, offset: 14323},
							expr: &ruleRefExpr{
								pos:  position{line: 510, col: 15, offset: 14323},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 510, col: 21, offset: 14329},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 518, col: 1, offset: 14484},
			expr: &choiceExpr{
				pos: position{line: 518, col: 9, offset: 14492},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 518, col: 9, offset: 14492},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 518, col: 9, offset: 14492},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 522, col: 5, offset: 14592},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 522, col: 5, offset: 14592},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 528, col: 1, offset: 14693},
			expr: &actionExpr{
				pos: position{line: 528, col: 9, offset: 14701},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 528, col: 9, offset: 14701},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 534, col: 1, offset: 14796},
			expr: &charClassMatcher{
				pos:        position{line: 534, col: 16, offset: 14811},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 536, col: 1, offset: 14822},
			expr: &choiceExpr{
				pos: position{line: 536, col: 9, offset: 14830},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 536, col: 11, offset: 14832},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 536, col: 11, offset: 14832},
								expr: &ruleRefExpr{
									pos:  position{line: 536, col: 12, offset: 14833},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 536, col: 24, offset: 14845,
							},
						},
					},
					&seqExpr{
						pos: position{line: 536, col: 32, offset: 14853},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 536, col: 32, offset: 14853},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 536, col: 37, offset: 14858},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 538, col: 1, offset: 14876},
			expr: &charClassMatcher{
				pos:        position{line: 538, col: 16, offset: 14891},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 540, col: 1, offset: 14907},
			expr: &choiceExpr{
				pos: position{line: 540, col: 19, offset: 14925},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 540, col: 19, offset: 14925},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 540, col: 38, offset: 14944},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 542, col: 1, offset: 14959},
			expr: &charClassMatcher{
				pos:        position{line: 542, col: 21, offset: 14979},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 544, col: 1, offset: 15001},
			expr: &seqExpr{
				pos: position{line: 544, col: 18, offset: 15018},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 544, col: 18, offset: 15018},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 544, col: 22, offset: 15022},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 544, col: 31, offset: 15031},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 544, col: 40, offset: 15040},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 544, col: 49, offset: 15049},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 546, col: 1, offset: 15059},
			expr: &charClassMatcher{
				pos:        position{line: 546, col: 17, offset: 15075},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 548, col: 1, offset: 15082},
			expr: &charClassMatcher{
				pos:        position{line: 548, col: 24, offset: 15105},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 550, col: 1, offset: 15112},
			expr: &charClassMatcher{
				pos:        position{line: 550, col: 13, offset: 15124},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 552, col: 1, offset: 15137},
			expr: &oneOrMoreExpr{
				pos: position{line: 552, col: 20, offset: 15156},
				expr: &charClassMatcher{
					pos:        position{line: 552, col: 20, offset: 15156},
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
			pos:         position{line: 554, col: 1, offset: 15168},
			expr: &zeroOrMoreExpr{
				pos: position{line: 554, col: 19, offset: 15186},
				expr: &choiceExpr{
					pos: position{line: 554, col: 21, offset: 15188},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 554, col: 21, offset: 15188},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 554, col: 33, offset: 15200},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 556, col: 1, offset: 15212},
			expr: &actionExpr{
				pos: position{line: 556, col: 12, offset: 15223},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 556, col: 12, offset: 15223},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 556, col: 12, offset: 15223},
							expr: &charClassMatcher{
								pos:        position{line: 556, col: 12, offset: 15223},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 556, col: 19, offset: 15230},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 556, col: 23, offset: 15234},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 556, col: 28, offset: 15239},
								expr: &charClassMatcher{
									pos:        position{line: 556, col: 28, offset: 15239},
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
			pos:  position{line: 562, col: 1, offset: 15374},
			expr: &notExpr{
				pos: position{line: 562, col: 8, offset: 15381},
				expr: &anyMatcher{
					line: 562, col: 9, offset: 15382,
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

func (c *current) onNormalRules1(head, val interface{}) (interface{}, error) {

	if head == nil {
		return nil, nil
	}

	// Parser is expected to return []Statement. If the author has chained
	// rule bodies together (disjunction) then multiple rules must be returned.
	bodies := []Body{}

	sl := val.([]interface{})
	bodies = append(bodies, sl[0].(Body))
	for _, x := range sl[1].([]interface{}) {
		bodies = append(bodies, x.([]interface{})[1].(Body))
	}

	rules := make([]*Rule, len(bodies))
	for i := range bodies {
		rules[i] = &Rule{
			Head: head.(*Head).Copy(),
			Body: bodies[i],
		}
	}

	return rules, nil
}

func (p *parser) callonNormalRules1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onNormalRules1(stack["head"], stack["val"])
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

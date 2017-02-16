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
			pos:  position{line: 306, col: 1, offset: 8762},
			expr: &actionExpr{
				pos: position{line: 306, col: 14, offset: 8775},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 306, col: 14, offset: 8775},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 306, col: 14, offset: 8775},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 19, offset: 8780},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 306, col: 24, offset: 8785},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 306, col: 26, offset: 8787},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 29, offset: 8790},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 306, col: 37, offset: 8798},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 306, col: 39, offset: 8800},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 306, col: 45, offset: 8806},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 310, col: 1, offset: 8881},
			expr: &actionExpr{
				pos: position{line: 310, col: 12, offset: 8892},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 310, col: 12, offset: 8892},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 310, col: 17, offset: 8897},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 310, col: 17, offset: 8897},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 23, offset: 8903},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 30, offset: 8910},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 37, offset: 8917},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 44, offset: 8924},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 310, col: 50, offset: 8930},
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
			pos:  position{line: 322, col: 1, offset: 9174},
			expr: &choiceExpr{
				pos: position{line: 322, col: 15, offset: 9188},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 322, col: 15, offset: 9188},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 322, col: 26, offset: 9199},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 324, col: 1, offset: 9208},
			expr: &actionExpr{
				pos: position{line: 324, col: 12, offset: 9219},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 324, col: 12, offset: 9219},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 324, col: 12, offset: 9219},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 324, col: 15, offset: 9222},
								name: "Var",
							},
						},
						&litMatcher{
							pos:        position{line: 324, col: 19, offset: 9226},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 324, col: 23, offset: 9230},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 324, col: 25, offset: 9232},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 324, col: 30, offset: 9237},
								expr: &ruleRefExpr{
									pos:  position{line: 324, col: 30, offset: 9237},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 324, col: 36, offset: 9243},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 324, col: 41, offset: 9248},
								expr: &seqExpr{
									pos: position{line: 324, col: 43, offset: 9250},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 324, col: 43, offset: 9250},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 324, col: 45, offset: 9252},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 324, col: 49, offset: 9256},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 324, col: 51, offset: 9258},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 324, col: 59, offset: 9266},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 324, col: 62, offset: 9269},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 340, col: 1, offset: 9671},
			expr: &actionExpr{
				pos: position{line: 340, col: 9, offset: 9679},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 340, col: 9, offset: 9679},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 340, col: 15, offset: 9685},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 340, col: 15, offset: 9685},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 340, col: 31, offset: 9701},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 340, col: 43, offset: 9713},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 340, col: 52, offset: 9722},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 340, col: 58, offset: 9728},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 344, col: 1, offset: 9759},
			expr: &ruleRefExpr{
				pos:  position{line: 344, col: 18, offset: 9776},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 346, col: 1, offset: 9796},
			expr: &actionExpr{
				pos: position{line: 346, col: 23, offset: 9818},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 346, col: 23, offset: 9818},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 346, col: 23, offset: 9818},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 346, col: 27, offset: 9822},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 346, col: 29, offset: 9824},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 346, col: 34, offset: 9829},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 346, col: 39, offset: 9834},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 346, col: 41, offset: 9836},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 346, col: 45, offset: 9840},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 346, col: 47, offset: 9842},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 346, col: 52, offset: 9847},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 346, col: 67, offset: 9862},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 346, col: 69, offset: 9864},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 352, col: 1, offset: 9989},
			expr: &choiceExpr{
				pos: position{line: 352, col: 14, offset: 10002},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 352, col: 14, offset: 10002},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 352, col: 23, offset: 10011},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 352, col: 31, offset: 10019},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 354, col: 1, offset: 10024},
			expr: &choiceExpr{
				pos: position{line: 354, col: 11, offset: 10034},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 354, col: 11, offset: 10034},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 354, col: 20, offset: 10043},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 354, col: 29, offset: 10052},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 354, col: 36, offset: 10059},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 356, col: 1, offset: 10065},
			expr: &choiceExpr{
				pos: position{line: 356, col: 8, offset: 10072},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 356, col: 8, offset: 10072},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 356, col: 17, offset: 10081},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 356, col: 23, offset: 10087},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 358, col: 1, offset: 10092},
			expr: &actionExpr{
				pos: position{line: 358, col: 11, offset: 10102},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 358, col: 11, offset: 10102},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 358, col: 11, offset: 10102},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 358, col: 15, offset: 10106},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 358, col: 17, offset: 10108},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 358, col: 22, offset: 10113},
								expr: &seqExpr{
									pos: position{line: 358, col: 23, offset: 10114},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 358, col: 23, offset: 10114},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 27, offset: 10118},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 358, col: 29, offset: 10120},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 33, offset: 10124},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 35, offset: 10126},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 358, col: 42, offset: 10133},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 358, col: 47, offset: 10138},
								expr: &seqExpr{
									pos: position{line: 358, col: 49, offset: 10140},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 358, col: 49, offset: 10140},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 358, col: 51, offset: 10142},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 55, offset: 10146},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 57, offset: 10148},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 61, offset: 10152},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 358, col: 63, offset: 10154},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 67, offset: 10158},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 358, col: 69, offset: 10160},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 358, col: 77, offset: 10168},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 358, col: 79, offset: 10170},
							expr: &litMatcher{
								pos:        position{line: 358, col: 79, offset: 10170},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 358, col: 84, offset: 10175},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 358, col: 86, offset: 10177},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 382, col: 1, offset: 10956},
			expr: &actionExpr{
				pos: position{line: 382, col: 10, offset: 10965},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 382, col: 10, offset: 10965},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 382, col: 10, offset: 10965},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 382, col: 14, offset: 10969},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 382, col: 17, offset: 10972},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 382, col: 22, offset: 10977},
								expr: &ruleRefExpr{
									pos:  position{line: 382, col: 22, offset: 10977},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 382, col: 28, offset: 10983},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 382, col: 33, offset: 10988},
								expr: &seqExpr{
									pos: position{line: 382, col: 34, offset: 10989},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 382, col: 34, offset: 10989},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 382, col: 36, offset: 10991},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 382, col: 40, offset: 10995},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 382, col: 42, offset: 10997},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 382, col: 49, offset: 11004},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 382, col: 51, offset: 11006},
							expr: &litMatcher{
								pos:        position{line: 382, col: 51, offset: 11006},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 382, col: 56, offset: 11011},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 382, col: 59, offset: 11014},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 406, col: 1, offset: 11587},
			expr: &choiceExpr{
				pos: position{line: 406, col: 8, offset: 11594},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 406, col: 8, offset: 11594},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 406, col: 19, offset: 11605},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 408, col: 1, offset: 11618},
			expr: &actionExpr{
				pos: position{line: 408, col: 13, offset: 11630},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 408, col: 13, offset: 11630},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 408, col: 13, offset: 11630},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 408, col: 20, offset: 11637},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 408, col: 22, offset: 11639},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 414, col: 1, offset: 11727},
			expr: &actionExpr{
				pos: position{line: 414, col: 16, offset: 11742},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 414, col: 16, offset: 11742},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 414, col: 16, offset: 11742},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 414, col: 20, offset: 11746},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 414, col: 22, offset: 11748},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 414, col: 27, offset: 11753},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 414, col: 32, offset: 11758},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 414, col: 37, offset: 11763},
								expr: &seqExpr{
									pos: position{line: 414, col: 38, offset: 11764},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 414, col: 38, offset: 11764},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 414, col: 40, offset: 11766},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 414, col: 44, offset: 11770},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 414, col: 46, offset: 11772},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 414, col: 53, offset: 11779},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 414, col: 55, offset: 11781},
							expr: &litMatcher{
								pos:        position{line: 414, col: 55, offset: 11781},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 414, col: 60, offset: 11786},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 414, col: 62, offset: 11788},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 431, col: 1, offset: 12193},
			expr: &actionExpr{
				pos: position{line: 431, col: 8, offset: 12200},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 431, col: 8, offset: 12200},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 431, col: 8, offset: 12200},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 431, col: 13, offset: 12205},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 431, col: 17, offset: 12209},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 431, col: 22, offset: 12214},
								expr: &choiceExpr{
									pos: position{line: 431, col: 24, offset: 12216},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 431, col: 24, offset: 12216},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 431, col: 33, offset: 12225},
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
			pos:  position{line: 444, col: 1, offset: 12464},
			expr: &actionExpr{
				pos: position{line: 444, col: 11, offset: 12474},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 444, col: 11, offset: 12474},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 444, col: 11, offset: 12474},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 444, col: 15, offset: 12478},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 444, col: 19, offset: 12482},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 451, col: 1, offset: 12701},
			expr: &actionExpr{
				pos: position{line: 451, col: 15, offset: 12715},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 451, col: 15, offset: 12715},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 451, col: 15, offset: 12715},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 451, col: 19, offset: 12719},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 451, col: 24, offset: 12724},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 451, col: 24, offset: 12724},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 451, col: 30, offset: 12730},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 451, col: 39, offset: 12739},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 451, col: 44, offset: 12744},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 455, col: 1, offset: 12773},
			expr: &actionExpr{
				pos: position{line: 455, col: 8, offset: 12780},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 455, col: 8, offset: 12780},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 455, col: 12, offset: 12784},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 460, col: 1, offset: 12906},
			expr: &seqExpr{
				pos: position{line: 460, col: 15, offset: 12920},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 460, col: 15, offset: 12920},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 460, col: 19, offset: 12924},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 460, col: 32, offset: 12937},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 464, col: 1, offset: 13002},
			expr: &actionExpr{
				pos: position{line: 464, col: 17, offset: 13018},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 464, col: 17, offset: 13018},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 464, col: 17, offset: 13018},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 464, col: 29, offset: 13030},
							expr: &choiceExpr{
								pos: position{line: 464, col: 30, offset: 13031},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 464, col: 30, offset: 13031},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 464, col: 44, offset: 13045},
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
			pos:  position{line: 471, col: 1, offset: 13188},
			expr: &actionExpr{
				pos: position{line: 471, col: 11, offset: 13198},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 471, col: 11, offset: 13198},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 471, col: 11, offset: 13198},
							expr: &litMatcher{
								pos:        position{line: 471, col: 11, offset: 13198},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 471, col: 18, offset: 13205},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 471, col: 18, offset: 13205},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 471, col: 26, offset: 13213},
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
			pos:  position{line: 484, col: 1, offset: 13604},
			expr: &choiceExpr{
				pos: position{line: 484, col: 10, offset: 13613},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 484, col: 10, offset: 13613},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 484, col: 26, offset: 13629},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 486, col: 1, offset: 13641},
			expr: &seqExpr{
				pos: position{line: 486, col: 18, offset: 13658},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 486, col: 20, offset: 13660},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 486, col: 20, offset: 13660},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 486, col: 33, offset: 13673},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 486, col: 43, offset: 13683},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 488, col: 1, offset: 13693},
			expr: &seqExpr{
				pos: position{line: 488, col: 15, offset: 13707},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 488, col: 15, offset: 13707},
						expr: &ruleRefExpr{
							pos:  position{line: 488, col: 15, offset: 13707},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 488, col: 24, offset: 13716},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 490, col: 1, offset: 13726},
			expr: &seqExpr{
				pos: position{line: 490, col: 13, offset: 13738},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 490, col: 13, offset: 13738},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 490, col: 17, offset: 13742},
						expr: &ruleRefExpr{
							pos:  position{line: 490, col: 17, offset: 13742},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 492, col: 1, offset: 13757},
			expr: &seqExpr{
				pos: position{line: 492, col: 13, offset: 13769},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 492, col: 13, offset: 13769},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 492, col: 18, offset: 13774},
						expr: &charClassMatcher{
							pos:        position{line: 492, col: 18, offset: 13774},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 492, col: 24, offset: 13780},
						expr: &ruleRefExpr{
							pos:  position{line: 492, col: 24, offset: 13780},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 494, col: 1, offset: 13795},
			expr: &choiceExpr{
				pos: position{line: 494, col: 12, offset: 13806},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 494, col: 12, offset: 13806},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 494, col: 20, offset: 13814},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 494, col: 20, offset: 13814},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 494, col: 40, offset: 13834},
								expr: &ruleRefExpr{
									pos:  position{line: 494, col: 40, offset: 13834},
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
			pos:  position{line: 496, col: 1, offset: 13851},
			expr: &actionExpr{
				pos: position{line: 496, col: 11, offset: 13861},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 496, col: 11, offset: 13861},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 496, col: 11, offset: 13861},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 496, col: 15, offset: 13865},
							expr: &ruleRefExpr{
								pos:  position{line: 496, col: 15, offset: 13865},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 496, col: 21, offset: 13871},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 504, col: 1, offset: 14026},
			expr: &choiceExpr{
				pos: position{line: 504, col: 9, offset: 14034},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 504, col: 9, offset: 14034},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 504, col: 9, offset: 14034},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 508, col: 5, offset: 14134},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 508, col: 5, offset: 14134},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 514, col: 1, offset: 14235},
			expr: &actionExpr{
				pos: position{line: 514, col: 9, offset: 14243},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 514, col: 9, offset: 14243},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 520, col: 1, offset: 14338},
			expr: &charClassMatcher{
				pos:        position{line: 520, col: 16, offset: 14353},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 522, col: 1, offset: 14364},
			expr: &choiceExpr{
				pos: position{line: 522, col: 9, offset: 14372},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 522, col: 11, offset: 14374},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 522, col: 11, offset: 14374},
								expr: &ruleRefExpr{
									pos:  position{line: 522, col: 12, offset: 14375},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 522, col: 24, offset: 14387,
							},
						},
					},
					&seqExpr{
						pos: position{line: 522, col: 32, offset: 14395},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 522, col: 32, offset: 14395},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 522, col: 37, offset: 14400},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 524, col: 1, offset: 14418},
			expr: &charClassMatcher{
				pos:        position{line: 524, col: 16, offset: 14433},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 526, col: 1, offset: 14449},
			expr: &choiceExpr{
				pos: position{line: 526, col: 19, offset: 14467},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 526, col: 19, offset: 14467},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 526, col: 38, offset: 14486},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 528, col: 1, offset: 14501},
			expr: &charClassMatcher{
				pos:        position{line: 528, col: 21, offset: 14521},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 530, col: 1, offset: 14543},
			expr: &seqExpr{
				pos: position{line: 530, col: 18, offset: 14560},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 530, col: 18, offset: 14560},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 530, col: 22, offset: 14564},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 530, col: 31, offset: 14573},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 530, col: 40, offset: 14582},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 530, col: 49, offset: 14591},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 532, col: 1, offset: 14601},
			expr: &charClassMatcher{
				pos:        position{line: 532, col: 17, offset: 14617},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 534, col: 1, offset: 14624},
			expr: &charClassMatcher{
				pos:        position{line: 534, col: 24, offset: 14647},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 536, col: 1, offset: 14654},
			expr: &charClassMatcher{
				pos:        position{line: 536, col: 13, offset: 14666},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 538, col: 1, offset: 14679},
			expr: &oneOrMoreExpr{
				pos: position{line: 538, col: 20, offset: 14698},
				expr: &charClassMatcher{
					pos:        position{line: 538, col: 20, offset: 14698},
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
			pos:         position{line: 540, col: 1, offset: 14710},
			expr: &zeroOrMoreExpr{
				pos: position{line: 540, col: 19, offset: 14728},
				expr: &choiceExpr{
					pos: position{line: 540, col: 21, offset: 14730},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 540, col: 21, offset: 14730},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 540, col: 33, offset: 14742},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 542, col: 1, offset: 14754},
			expr: &actionExpr{
				pos: position{line: 542, col: 12, offset: 14765},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 542, col: 12, offset: 14765},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 542, col: 12, offset: 14765},
							expr: &charClassMatcher{
								pos:        position{line: 542, col: 12, offset: 14765},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 542, col: 19, offset: 14772},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 542, col: 23, offset: 14776},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 542, col: 28, offset: 14781},
								expr: &charClassMatcher{
									pos:        position{line: 542, col: 28, offset: 14781},
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
			pos:  position{line: 548, col: 1, offset: 14916},
			expr: &notExpr{
				pos: position{line: 548, col: 8, offset: 14923},
				expr: &anyMatcher{
					line: 548, col: 9, offset: 14924,
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

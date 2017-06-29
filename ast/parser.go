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

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 49, col: 1, offset: 962},
			expr: &actionExpr{
				pos: position{line: 49, col: 12, offset: 973},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 49, col: 12, offset: 973},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 49, col: 12, offset: 973},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 49, col: 14, offset: 975},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 49, col: 19, offset: 980},
								expr: &seqExpr{
									pos: position{line: 49, col: 20, offset: 981},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 49, col: 20, offset: 981},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 49, col: 25, offset: 986},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 49, col: 30, offset: 991},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 49, col: 35, offset: 996},
												expr: &seqExpr{
													pos: position{line: 49, col: 36, offset: 997},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 49, col: 37, offset: 998},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 49, col: 37, offset: 998},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 49, col: 42, offset: 1003},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 49, col: 54, offset: 1015},
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
							pos:  position{line: 49, col: 63, offset: 1024},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 49, col: 65, offset: 1026},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 67, col: 1, offset: 1400},
			expr: &actionExpr{
				pos: position{line: 67, col: 9, offset: 1408},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 67, col: 9, offset: 1408},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 67, col: 14, offset: 1413},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 67, col: 14, offset: 1413},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 67, col: 24, offset: 1423},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 67, col: 33, offset: 1432},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 67, col: 41, offset: 1440},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 67, col: 48, offset: 1447},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 67, col: 58, offset: 1457},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 76, col: 1, offset: 1821},
			expr: &actionExpr{
				pos: position{line: 76, col: 15, offset: 1835},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 76, col: 15, offset: 1835,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 80, col: 1, offset: 1908},
			expr: &actionExpr{
				pos: position{line: 80, col: 12, offset: 1919},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 80, col: 12, offset: 1919},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 80, col: 12, offset: 1919},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 80, col: 22, offset: 1929},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 80, col: 25, offset: 1932},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 80, col: 30, offset: 1937},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 80, col: 30, offset: 1937},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 80, col: 36, offset: 1943},
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
			pos:  position{line: 114, col: 1, offset: 3259},
			expr: &actionExpr{
				pos: position{line: 114, col: 11, offset: 3269},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 114, col: 11, offset: 3269},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 114, col: 11, offset: 3269},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 114, col: 20, offset: 3278},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 114, col: 23, offset: 3281},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 114, col: 29, offset: 3287},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 114, col: 29, offset: 3287},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 114, col: 35, offset: 3293},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 114, col: 40, offset: 3298},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 114, col: 46, offset: 3304},
								expr: &seqExpr{
									pos: position{line: 114, col: 47, offset: 3305},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 114, col: 47, offset: 3305},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 114, col: 50, offset: 3308},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 114, col: 55, offset: 3313},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 114, col: 58, offset: 3316},
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
			pos:  position{line: 130, col: 1, offset: 3766},
			expr: &choiceExpr{
				pos: position{line: 130, col: 10, offset: 3775},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 130, col: 10, offset: 3775},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 130, col: 25, offset: 3790},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 132, col: 1, offset: 3803},
			expr: &actionExpr{
				pos: position{line: 132, col: 17, offset: 3819},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 132, col: 17, offset: 3819},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 132, col: 17, offset: 3819},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 132, col: 27, offset: 3829},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 132, col: 30, offset: 3832},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 132, col: 35, offset: 3837},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 132, col: 39, offset: 3841},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 132, col: 41, offset: 3843},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 132, col: 45, offset: 3847},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 132, col: 47, offset: 3849},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 132, col: 53, offset: 3855},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 175, col: 1, offset: 4783},
			expr: &actionExpr{
				pos: position{line: 175, col: 16, offset: 4798},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 175, col: 16, offset: 4798},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 175, col: 16, offset: 4798},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 175, col: 21, offset: 4803},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 175, col: 30, offset: 4812},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 175, col: 32, offset: 4814},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 175, col: 35, offset: 4817},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 175, col: 35, offset: 4817},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 175, col: 61, offset: 4843},
										expr: &seqExpr{
											pos: position{line: 175, col: 63, offset: 4845},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 175, col: 63, offset: 4845},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 175, col: 65, offset: 4847},
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
			name: "RuleHead",
			pos:  position{line: 230, col: 1, offset: 6143},
			expr: &actionExpr{
				pos: position{line: 230, col: 13, offset: 6155},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 230, col: 13, offset: 6155},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 230, col: 13, offset: 6155},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 230, col: 18, offset: 6160},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 230, col: 22, offset: 6164},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 230, col: 26, offset: 6168},
								expr: &seqExpr{
									pos: position{line: 230, col: 28, offset: 6170},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 230, col: 28, offset: 6170},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 230, col: 30, offset: 6172},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 34, offset: 6176},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 36, offset: 6178},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 41, offset: 6183},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 230, col: 43, offset: 6185},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 47, offset: 6189},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 230, col: 52, offset: 6194},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 230, col: 58, offset: 6200},
								expr: &seqExpr{
									pos: position{line: 230, col: 60, offset: 6202},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 230, col: 60, offset: 6202},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 230, col: 62, offset: 6204},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 66, offset: 6208},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 230, col: 68, offset: 6210},
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
			pos:  position{line: 265, col: 1, offset: 7198},
			expr: &actionExpr{
				pos: position{line: 265, col: 9, offset: 7206},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 265, col: 9, offset: 7206},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 265, col: 9, offset: 7206},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 265, col: 16, offset: 7213},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 265, col: 20, offset: 7217},
								expr: &seqExpr{
									pos: position{line: 265, col: 22, offset: 7219},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 265, col: 22, offset: 7219},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 265, col: 24, offset: 7221},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 265, col: 28, offset: 7225},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 265, col: 30, offset: 7227},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 265, col: 38, offset: 7235},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 265, col: 42, offset: 7239},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 265, col: 42, offset: 7239},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 265, col: 44, offset: 7241},
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
			pos:  position{line: 280, col: 1, offset: 7593},
			expr: &actionExpr{
				pos: position{line: 280, col: 12, offset: 7604},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 280, col: 12, offset: 7604},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 280, col: 14, offset: 7606},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 284, col: 1, offset: 7702},
			expr: &choiceExpr{
				pos: position{line: 284, col: 12, offset: 7713},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 284, col: 12, offset: 7713},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 284, col: 19, offset: 7720},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 286, col: 1, offset: 7729},
			expr: &choiceExpr{
				pos: position{line: 286, col: 9, offset: 7737},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 286, col: 9, offset: 7737},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 286, col: 29, offset: 7757},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 288, col: 1, offset: 7776},
			expr: &actionExpr{
				pos: position{line: 288, col: 30, offset: 7805},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 288, col: 30, offset: 7805},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 288, col: 30, offset: 7805},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 288, col: 34, offset: 7809},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 288, col: 36, offset: 7811},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 288, col: 40, offset: 7815},
								expr: &ruleRefExpr{
									pos:  position{line: 288, col: 40, offset: 7815},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 288, col: 56, offset: 7831},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 288, col: 58, offset: 7833},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 295, col: 1, offset: 7928},
			expr: &actionExpr{
				pos: position{line: 295, col: 22, offset: 7949},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 295, col: 22, offset: 7949},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 295, col: 22, offset: 7949},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 295, col: 26, offset: 7953},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 295, col: 28, offset: 7955},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 295, col: 32, offset: 7959},
								expr: &ruleRefExpr{
									pos:  position{line: 295, col: 32, offset: 7959},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 295, col: 48, offset: 7975},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 295, col: 50, offset: 7977},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 309, col: 1, offset: 8312},
			expr: &actionExpr{
				pos: position{line: 309, col: 19, offset: 8330},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 309, col: 19, offset: 8330},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 309, col: 19, offset: 8330},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 309, col: 24, offset: 8335},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 309, col: 32, offset: 8343},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 309, col: 37, offset: 8348},
								expr: &seqExpr{
									pos: position{line: 309, col: 38, offset: 8349},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 309, col: 38, offset: 8349},
											expr: &charClassMatcher{
												pos:        position{line: 309, col: 38, offset: 8349},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 309, col: 46, offset: 8357},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 309, col: 47, offset: 8358},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 309, col: 47, offset: 8358},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 309, col: 51, offset: 8362},
															expr: &ruleRefExpr{
																pos:  position{line: 309, col: 51, offset: 8362},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 309, col: 64, offset: 8375},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 309, col: 64, offset: 8375},
															expr: &ruleRefExpr{
																pos:  position{line: 309, col: 64, offset: 8375},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 309, col: 73, offset: 8384},
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
											pos:  position{line: 309, col: 82, offset: 8393},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 309, col: 84, offset: 8395},
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
			pos:  position{line: 315, col: 1, offset: 8584},
			expr: &actionExpr{
				pos: position{line: 315, col: 22, offset: 8605},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 315, col: 22, offset: 8605},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 315, col: 22, offset: 8605},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 315, col: 27, offset: 8610},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 315, col: 35, offset: 8618},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 315, col: 40, offset: 8623},
								expr: &seqExpr{
									pos: position{line: 315, col: 42, offset: 8625},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 315, col: 42, offset: 8625},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 315, col: 44, offset: 8627},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 315, col: 48, offset: 8631},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 315, col: 51, offset: 8634},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 315, col: 51, offset: 8634},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 315, col: 61, offset: 8644},
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
			pos:  position{line: 319, col: 1, offset: 8723},
			expr: &actionExpr{
				pos: position{line: 319, col: 12, offset: 8734},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 319, col: 12, offset: 8734},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 319, col: 12, offset: 8734},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 319, col: 16, offset: 8738},
								expr: &seqExpr{
									pos: position{line: 319, col: 18, offset: 8740},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 319, col: 18, offset: 8740},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 319, col: 24, offset: 8746},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 319, col: 30, offset: 8752},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 319, col: 34, offset: 8756},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 319, col: 39, offset: 8761},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 319, col: 44, offset: 8766},
								expr: &seqExpr{
									pos: position{line: 319, col: 46, offset: 8768},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 319, col: 46, offset: 8768},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 319, col: 49, offset: 8771},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 319, col: 54, offset: 8776},
											expr: &seqExpr{
												pos: position{line: 319, col: 55, offset: 8777},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 319, col: 55, offset: 8777},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 319, col: 58, offset: 8780},
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
			pos:  position{line: 342, col: 1, offset: 9352},
			expr: &actionExpr{
				pos: position{line: 342, col: 9, offset: 9360},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 342, col: 9, offset: 9360},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 342, col: 9, offset: 9360},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 342, col: 16, offset: 9367},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 342, col: 19, offset: 9370},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 342, col: 26, offset: 9377},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 342, col: 31, offset: 9382},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 342, col: 34, offset: 9385},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 342, col: 39, offset: 9390},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 342, col: 42, offset: 9393},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 342, col: 48, offset: 9399},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 353, col: 1, offset: 9648},
			expr: &choiceExpr{
				pos: position{line: 353, col: 9, offset: 9656},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 353, col: 10, offset: 9657},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 353, col: 10, offset: 9657},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 353, col: 27, offset: 9674},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 353, col: 52, offset: 9699},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 353, col: 64, offset: 9711},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 353, col: 77, offset: 9724},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 355, col: 1, offset: 9730},
			expr: &actionExpr{
				pos: position{line: 355, col: 19, offset: 9748},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 355, col: 19, offset: 9748},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 355, col: 19, offset: 9748},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 355, col: 26, offset: 9755},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 355, col: 31, offset: 9760},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 355, col: 33, offset: 9762},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 355, col: 37, offset: 9766},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 355, col: 39, offset: 9768},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 355, col: 44, offset: 9773},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 355, col: 49, offset: 9778},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 355, col: 51, offset: 9780},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 355, col: 54, offset: 9783},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 355, col: 67, offset: 9796},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 355, col: 69, offset: 9798},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 355, col: 75, offset: 9804},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 359, col: 1, offset: 9895},
			expr: &actionExpr{
				pos: position{line: 359, col: 26, offset: 9920},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 359, col: 26, offset: 9920},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 359, col: 26, offset: 9920},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 359, col: 31, offset: 9925},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 359, col: 36, offset: 9930},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 359, col: 38, offset: 9932},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 359, col: 41, offset: 9935},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 359, col: 54, offset: 9948},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 359, col: 56, offset: 9950},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 359, col: 62, offset: 9956},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 359, col: 67, offset: 9961},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 359, col: 69, offset: 9963},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 359, col: 73, offset: 9967},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 359, col: 75, offset: 9969},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 359, col: 82, offset: 9976},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 363, col: 1, offset: 10067},
			expr: &actionExpr{
				pos: position{line: 363, col: 17, offset: 10083},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 363, col: 17, offset: 10083},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 363, col: 22, offset: 10088},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 363, col: 22, offset: 10088},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 28, offset: 10094},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 34, offset: 10100},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 40, offset: 10106},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 46, offset: 10112},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 52, offset: 10118},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 363, col: 58, offset: 10124},
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
			pos:  position{line: 375, col: 1, offset: 10371},
			expr: &actionExpr{
				pos: position{line: 375, col: 14, offset: 10384},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 375, col: 14, offset: 10384},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 375, col: 14, offset: 10384},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 375, col: 19, offset: 10389},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 375, col: 24, offset: 10394},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 375, col: 26, offset: 10396},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 375, col: 29, offset: 10399},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 375, col: 37, offset: 10407},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 375, col: 39, offset: 10409},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 375, col: 45, offset: 10415},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 379, col: 1, offset: 10490},
			expr: &actionExpr{
				pos: position{line: 379, col: 12, offset: 10501},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 379, col: 12, offset: 10501},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 379, col: 17, offset: 10506},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 379, col: 17, offset: 10506},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 379, col: 23, offset: 10512},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 379, col: 30, offset: 10519},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 379, col: 37, offset: 10526},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 379, col: 44, offset: 10533},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 379, col: 50, offset: 10539},
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
			pos:  position{line: 391, col: 1, offset: 10786},
			expr: &choiceExpr{
				pos: position{line: 391, col: 15, offset: 10800},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 391, col: 15, offset: 10800},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 391, col: 26, offset: 10811},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 393, col: 1, offset: 10820},
			expr: &actionExpr{
				pos: position{line: 393, col: 12, offset: 10831},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 393, col: 12, offset: 10831},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 393, col: 12, offset: 10831},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 393, col: 17, offset: 10836},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 393, col: 29, offset: 10848},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 393, col: 33, offset: 10852},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 393, col: 35, offset: 10854},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 393, col: 40, offset: 10859},
								expr: &ruleRefExpr{
									pos:  position{line: 393, col: 40, offset: 10859},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 393, col: 46, offset: 10865},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 393, col: 51, offset: 10870},
								expr: &seqExpr{
									pos: position{line: 393, col: 53, offset: 10872},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 393, col: 53, offset: 10872},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 393, col: 55, offset: 10874},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 393, col: 59, offset: 10878},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 393, col: 61, offset: 10880},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 393, col: 69, offset: 10888},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 393, col: 72, offset: 10891},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 409, col: 1, offset: 11295},
			expr: &actionExpr{
				pos: position{line: 409, col: 16, offset: 11310},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 409, col: 16, offset: 11310},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 409, col: 16, offset: 11310},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 409, col: 21, offset: 11315},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 409, col: 25, offset: 11319},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 409, col: 30, offset: 11324},
								expr: &seqExpr{
									pos: position{line: 409, col: 32, offset: 11326},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 409, col: 32, offset: 11326},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 409, col: 36, offset: 11330},
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
			pos:  position{line: 423, col: 1, offset: 11735},
			expr: &actionExpr{
				pos: position{line: 423, col: 9, offset: 11743},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 423, col: 9, offset: 11743},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 423, col: 15, offset: 11749},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 423, col: 15, offset: 11749},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 423, col: 31, offset: 11765},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 423, col: 43, offset: 11777},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 423, col: 52, offset: 11786},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 423, col: 58, offset: 11792},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 427, col: 1, offset: 11823},
			expr: &ruleRefExpr{
				pos:  position{line: 427, col: 18, offset: 11840},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 429, col: 1, offset: 11860},
			expr: &actionExpr{
				pos: position{line: 429, col: 23, offset: 11882},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 429, col: 23, offset: 11882},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 429, col: 23, offset: 11882},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 429, col: 27, offset: 11886},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 429, col: 29, offset: 11888},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 429, col: 34, offset: 11893},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 429, col: 39, offset: 11898},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 429, col: 41, offset: 11900},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 429, col: 45, offset: 11904},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 429, col: 47, offset: 11906},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 429, col: 52, offset: 11911},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 429, col: 67, offset: 11926},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 429, col: 69, offset: 11928},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 435, col: 1, offset: 12053},
			expr: &choiceExpr{
				pos: position{line: 435, col: 14, offset: 12066},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 435, col: 14, offset: 12066},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 435, col: 23, offset: 12075},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 435, col: 31, offset: 12083},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 437, col: 1, offset: 12088},
			expr: &choiceExpr{
				pos: position{line: 437, col: 11, offset: 12098},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 437, col: 11, offset: 12098},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 437, col: 20, offset: 12107},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 437, col: 29, offset: 12116},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 437, col: 36, offset: 12123},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 439, col: 1, offset: 12129},
			expr: &choiceExpr{
				pos: position{line: 439, col: 8, offset: 12136},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 439, col: 8, offset: 12136},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 439, col: 17, offset: 12145},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 439, col: 23, offset: 12151},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 441, col: 1, offset: 12156},
			expr: &actionExpr{
				pos: position{line: 441, col: 11, offset: 12166},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 441, col: 11, offset: 12166},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 441, col: 11, offset: 12166},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 15, offset: 12170},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 441, col: 17, offset: 12172},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 441, col: 22, offset: 12177},
								expr: &seqExpr{
									pos: position{line: 441, col: 23, offset: 12178},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 441, col: 23, offset: 12178},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 27, offset: 12182},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 441, col: 29, offset: 12184},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 33, offset: 12188},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 35, offset: 12190},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 441, col: 42, offset: 12197},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 441, col: 47, offset: 12202},
								expr: &seqExpr{
									pos: position{line: 441, col: 49, offset: 12204},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 441, col: 49, offset: 12204},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 441, col: 51, offset: 12206},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 55, offset: 12210},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 57, offset: 12212},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 61, offset: 12216},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 441, col: 63, offset: 12218},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 67, offset: 12222},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 441, col: 69, offset: 12224},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 77, offset: 12232},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 441, col: 79, offset: 12234},
							expr: &litMatcher{
								pos:        position{line: 441, col: 79, offset: 12234},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 84, offset: 12239},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 441, col: 86, offset: 12241},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 465, col: 1, offset: 13020},
			expr: &actionExpr{
				pos: position{line: 465, col: 10, offset: 13029},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 465, col: 10, offset: 13029},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 465, col: 10, offset: 13029},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 14, offset: 13033},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 465, col: 17, offset: 13036},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 465, col: 22, offset: 13041},
								expr: &ruleRefExpr{
									pos:  position{line: 465, col: 22, offset: 13041},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 465, col: 28, offset: 13047},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 465, col: 33, offset: 13052},
								expr: &seqExpr{
									pos: position{line: 465, col: 34, offset: 13053},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 465, col: 34, offset: 13053},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 465, col: 36, offset: 13055},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 465, col: 40, offset: 13059},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 465, col: 42, offset: 13061},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 49, offset: 13068},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 465, col: 51, offset: 13070},
							expr: &litMatcher{
								pos:        position{line: 465, col: 51, offset: 13070},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 465, col: 56, offset: 13075},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 465, col: 59, offset: 13078},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 489, col: 1, offset: 13651},
			expr: &choiceExpr{
				pos: position{line: 489, col: 8, offset: 13658},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 489, col: 8, offset: 13658},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 489, col: 19, offset: 13669},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 491, col: 1, offset: 13682},
			expr: &actionExpr{
				pos: position{line: 491, col: 13, offset: 13694},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 491, col: 13, offset: 13694},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 491, col: 13, offset: 13694},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 491, col: 20, offset: 13701},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 491, col: 22, offset: 13703},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 497, col: 1, offset: 13791},
			expr: &actionExpr{
				pos: position{line: 497, col: 16, offset: 13806},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 497, col: 16, offset: 13806},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 497, col: 16, offset: 13806},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 497, col: 20, offset: 13810},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 497, col: 22, offset: 13812},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 497, col: 27, offset: 13817},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 497, col: 32, offset: 13822},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 497, col: 37, offset: 13827},
								expr: &seqExpr{
									pos: position{line: 497, col: 38, offset: 13828},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 497, col: 38, offset: 13828},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 497, col: 40, offset: 13830},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 497, col: 44, offset: 13834},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 497, col: 46, offset: 13836},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 497, col: 53, offset: 13843},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 497, col: 55, offset: 13845},
							expr: &litMatcher{
								pos:        position{line: 497, col: 55, offset: 13845},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 497, col: 60, offset: 13850},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 497, col: 62, offset: 13852},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 514, col: 1, offset: 14257},
			expr: &actionExpr{
				pos: position{line: 514, col: 8, offset: 14264},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 514, col: 8, offset: 14264},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 514, col: 8, offset: 14264},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 514, col: 13, offset: 14269},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 514, col: 17, offset: 14273},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 514, col: 22, offset: 14278},
								expr: &choiceExpr{
									pos: position{line: 514, col: 24, offset: 14280},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 514, col: 24, offset: 14280},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 514, col: 33, offset: 14289},
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
			pos:  position{line: 527, col: 1, offset: 14528},
			expr: &actionExpr{
				pos: position{line: 527, col: 11, offset: 14538},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 527, col: 11, offset: 14538},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 527, col: 11, offset: 14538},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 527, col: 15, offset: 14542},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 527, col: 19, offset: 14546},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 534, col: 1, offset: 14765},
			expr: &actionExpr{
				pos: position{line: 534, col: 15, offset: 14779},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 534, col: 15, offset: 14779},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 534, col: 15, offset: 14779},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 534, col: 19, offset: 14783},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 534, col: 24, offset: 14788},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 534, col: 24, offset: 14788},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 534, col: 30, offset: 14794},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 534, col: 39, offset: 14803},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 534, col: 44, offset: 14808},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 538, col: 1, offset: 14837},
			expr: &actionExpr{
				pos: position{line: 538, col: 8, offset: 14844},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 538, col: 8, offset: 14844},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 538, col: 12, offset: 14848},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 543, col: 1, offset: 14970},
			expr: &seqExpr{
				pos: position{line: 543, col: 15, offset: 14984},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 543, col: 15, offset: 14984},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 543, col: 19, offset: 14988},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 543, col: 32, offset: 15001},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 547, col: 1, offset: 15066},
			expr: &actionExpr{
				pos: position{line: 547, col: 17, offset: 15082},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 547, col: 17, offset: 15082},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 547, col: 17, offset: 15082},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 547, col: 29, offset: 15094},
							expr: &choiceExpr{
								pos: position{line: 547, col: 30, offset: 15095},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 547, col: 30, offset: 15095},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 547, col: 44, offset: 15109},
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
			pos:  position{line: 554, col: 1, offset: 15252},
			expr: &actionExpr{
				pos: position{line: 554, col: 11, offset: 15262},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 554, col: 11, offset: 15262},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 554, col: 11, offset: 15262},
							expr: &litMatcher{
								pos:        position{line: 554, col: 11, offset: 15262},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 554, col: 18, offset: 15269},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 554, col: 18, offset: 15269},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 554, col: 26, offset: 15277},
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
			pos:  position{line: 567, col: 1, offset: 15668},
			expr: &choiceExpr{
				pos: position{line: 567, col: 10, offset: 15677},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 567, col: 10, offset: 15677},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 567, col: 26, offset: 15693},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 569, col: 1, offset: 15705},
			expr: &seqExpr{
				pos: position{line: 569, col: 18, offset: 15722},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 569, col: 20, offset: 15724},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 569, col: 20, offset: 15724},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 569, col: 33, offset: 15737},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 569, col: 43, offset: 15747},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 571, col: 1, offset: 15757},
			expr: &seqExpr{
				pos: position{line: 571, col: 15, offset: 15771},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 571, col: 15, offset: 15771},
						expr: &ruleRefExpr{
							pos:  position{line: 571, col: 15, offset: 15771},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 571, col: 24, offset: 15780},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 573, col: 1, offset: 15790},
			expr: &seqExpr{
				pos: position{line: 573, col: 13, offset: 15802},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 573, col: 13, offset: 15802},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 573, col: 17, offset: 15806},
						expr: &ruleRefExpr{
							pos:  position{line: 573, col: 17, offset: 15806},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 575, col: 1, offset: 15821},
			expr: &seqExpr{
				pos: position{line: 575, col: 13, offset: 15833},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 575, col: 13, offset: 15833},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 575, col: 18, offset: 15838},
						expr: &charClassMatcher{
							pos:        position{line: 575, col: 18, offset: 15838},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 575, col: 24, offset: 15844},
						expr: &ruleRefExpr{
							pos:  position{line: 575, col: 24, offset: 15844},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 577, col: 1, offset: 15859},
			expr: &choiceExpr{
				pos: position{line: 577, col: 12, offset: 15870},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 577, col: 12, offset: 15870},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 577, col: 20, offset: 15878},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 577, col: 20, offset: 15878},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 577, col: 40, offset: 15898},
								expr: &ruleRefExpr{
									pos:  position{line: 577, col: 40, offset: 15898},
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
			pos:  position{line: 579, col: 1, offset: 15915},
			expr: &actionExpr{
				pos: position{line: 579, col: 11, offset: 15925},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 579, col: 11, offset: 15925},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 579, col: 11, offset: 15925},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 579, col: 15, offset: 15929},
							expr: &ruleRefExpr{
								pos:  position{line: 579, col: 15, offset: 15929},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 579, col: 21, offset: 15935},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 587, col: 1, offset: 16090},
			expr: &choiceExpr{
				pos: position{line: 587, col: 9, offset: 16098},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 587, col: 9, offset: 16098},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 587, col: 9, offset: 16098},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 591, col: 5, offset: 16198},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 591, col: 5, offset: 16198},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 597, col: 1, offset: 16299},
			expr: &actionExpr{
				pos: position{line: 597, col: 9, offset: 16307},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 597, col: 9, offset: 16307},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 603, col: 1, offset: 16402},
			expr: &charClassMatcher{
				pos:        position{line: 603, col: 16, offset: 16417},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 605, col: 1, offset: 16428},
			expr: &choiceExpr{
				pos: position{line: 605, col: 9, offset: 16436},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 605, col: 11, offset: 16438},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 605, col: 11, offset: 16438},
								expr: &ruleRefExpr{
									pos:  position{line: 605, col: 12, offset: 16439},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 605, col: 24, offset: 16451,
							},
						},
					},
					&seqExpr{
						pos: position{line: 605, col: 32, offset: 16459},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 605, col: 32, offset: 16459},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 605, col: 37, offset: 16464},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 607, col: 1, offset: 16482},
			expr: &charClassMatcher{
				pos:        position{line: 607, col: 16, offset: 16497},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 609, col: 1, offset: 16513},
			expr: &choiceExpr{
				pos: position{line: 609, col: 19, offset: 16531},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 609, col: 19, offset: 16531},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 609, col: 38, offset: 16550},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 611, col: 1, offset: 16565},
			expr: &charClassMatcher{
				pos:        position{line: 611, col: 21, offset: 16585},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 613, col: 1, offset: 16607},
			expr: &seqExpr{
				pos: position{line: 613, col: 18, offset: 16624},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 613, col: 18, offset: 16624},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 613, col: 22, offset: 16628},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 613, col: 31, offset: 16637},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 613, col: 40, offset: 16646},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 613, col: 49, offset: 16655},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 615, col: 1, offset: 16665},
			expr: &charClassMatcher{
				pos:        position{line: 615, col: 17, offset: 16681},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 617, col: 1, offset: 16688},
			expr: &charClassMatcher{
				pos:        position{line: 617, col: 24, offset: 16711},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 619, col: 1, offset: 16718},
			expr: &charClassMatcher{
				pos:        position{line: 619, col: 13, offset: 16730},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 621, col: 1, offset: 16743},
			expr: &oneOrMoreExpr{
				pos: position{line: 621, col: 20, offset: 16762},
				expr: &charClassMatcher{
					pos:        position{line: 621, col: 20, offset: 16762},
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
			pos:         position{line: 623, col: 1, offset: 16774},
			expr: &zeroOrMoreExpr{
				pos: position{line: 623, col: 19, offset: 16792},
				expr: &choiceExpr{
					pos: position{line: 623, col: 21, offset: 16794},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 623, col: 21, offset: 16794},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 623, col: 33, offset: 16806},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 625, col: 1, offset: 16818},
			expr: &actionExpr{
				pos: position{line: 625, col: 12, offset: 16829},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 625, col: 12, offset: 16829},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 625, col: 12, offset: 16829},
							expr: &charClassMatcher{
								pos:        position{line: 625, col: 12, offset: 16829},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 625, col: 19, offset: 16836},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 625, col: 23, offset: 16840},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 625, col: 28, offset: 16845},
								expr: &charClassMatcher{
									pos:        position{line: 625, col: 28, offset: 16845},
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
			pos:  position{line: 636, col: 1, offset: 17121},
			expr: &notExpr{
				pos: position{line: 636, col: 8, offset: 17128},
				expr: &anyMatcher{
					line: 636, col: 9, offset: 17129,
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

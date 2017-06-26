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

// commentsKey is the global map key for the comments slice.
const commentsKey = "comments"

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
			pos:  position{line: 43, col: 1, offset: 839},
			expr: &actionExpr{
				pos: position{line: 43, col: 12, offset: 850},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 43, col: 12, offset: 850},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 43, col: 12, offset: 850},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 43, col: 14, offset: 852},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 43, col: 19, offset: 857},
								expr: &seqExpr{
									pos: position{line: 43, col: 20, offset: 858},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 43, col: 20, offset: 858},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 43, col: 25, offset: 863},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 43, col: 30, offset: 868},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 43, col: 35, offset: 873},
												expr: &seqExpr{
													pos: position{line: 43, col: 36, offset: 874},
													exprs: []interface{}{
														&choiceExpr{
															pos: position{line: 43, col: 37, offset: 875},
															alternatives: []interface{}{
																&ruleRefExpr{
																	pos:  position{line: 43, col: 37, offset: 875},
																	name: "ws",
																},
																&ruleRefExpr{
																	pos:  position{line: 43, col: 42, offset: 880},
																	name: "ParseError",
																},
															},
														},
														&ruleRefExpr{
															pos:  position{line: 43, col: 54, offset: 892},
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
							pos:  position{line: 43, col: 63, offset: 901},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 43, col: 65, offset: 903},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 61, col: 1, offset: 1277},
			expr: &actionExpr{
				pos: position{line: 61, col: 9, offset: 1285},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 61, col: 9, offset: 1285},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 61, col: 14, offset: 1290},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 61, col: 14, offset: 1290},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 61, col: 24, offset: 1300},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 61, col: 33, offset: 1309},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 61, col: 41, offset: 1317},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 61, col: 48, offset: 1324},
								name: "Comment",
							},
							&ruleRefExpr{
								pos:  position{line: 61, col: 58, offset: 1334},
								name: "ParseError",
							},
						},
					},
				},
			},
		},
		{
			name: "ParseError",
			pos:  position{line: 70, col: 1, offset: 1698},
			expr: &actionExpr{
				pos: position{line: 70, col: 15, offset: 1712},
				run: (*parser).callonParseError1,
				expr: &anyMatcher{
					line: 70, col: 15, offset: 1712,
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 74, col: 1, offset: 1785},
			expr: &actionExpr{
				pos: position{line: 74, col: 12, offset: 1796},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 74, col: 12, offset: 1796},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 74, col: 12, offset: 1796},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 74, col: 22, offset: 1806},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 74, col: 25, offset: 1809},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 74, col: 30, offset: 1814},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 74, col: 30, offset: 1814},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 74, col: 36, offset: 1820},
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
			pos:  position{line: 108, col: 1, offset: 3136},
			expr: &actionExpr{
				pos: position{line: 108, col: 11, offset: 3146},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 108, col: 11, offset: 3146},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 108, col: 11, offset: 3146},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 108, col: 20, offset: 3155},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 108, col: 23, offset: 3158},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 108, col: 29, offset: 3164},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 108, col: 29, offset: 3164},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 108, col: 35, offset: 3170},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 108, col: 40, offset: 3175},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 108, col: 46, offset: 3181},
								expr: &seqExpr{
									pos: position{line: 108, col: 47, offset: 3182},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 108, col: 47, offset: 3182},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 108, col: 50, offset: 3185},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 108, col: 55, offset: 3190},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 108, col: 58, offset: 3193},
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
			pos:  position{line: 124, col: 1, offset: 3643},
			expr: &choiceExpr{
				pos: position{line: 124, col: 10, offset: 3652},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 124, col: 10, offset: 3652},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 124, col: 25, offset: 3667},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 126, col: 1, offset: 3680},
			expr: &actionExpr{
				pos: position{line: 126, col: 17, offset: 3696},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 126, col: 17, offset: 3696},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 126, col: 17, offset: 3696},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 126, col: 27, offset: 3706},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 126, col: 30, offset: 3709},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 126, col: 35, offset: 3714},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 126, col: 39, offset: 3718},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 126, col: 41, offset: 3720},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 126, col: 45, offset: 3724},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 126, col: 47, offset: 3726},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 126, col: 53, offset: 3732},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 170, col: 1, offset: 4664},
			expr: &actionExpr{
				pos: position{line: 170, col: 16, offset: 4679},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 170, col: 16, offset: 4679},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 170, col: 16, offset: 4679},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 170, col: 21, offset: 4684},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 170, col: 30, offset: 4693},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 170, col: 32, offset: 4695},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 170, col: 35, offset: 4698},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 170, col: 35, offset: 4698},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 170, col: 61, offset: 4724},
										expr: &seqExpr{
											pos: position{line: 170, col: 63, offset: 4726},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 170, col: 63, offset: 4726},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 170, col: 65, offset: 4728},
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
			pos:  position{line: 225, col: 1, offset: 6024},
			expr: &actionExpr{
				pos: position{line: 225, col: 13, offset: 6036},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 225, col: 13, offset: 6036},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 225, col: 13, offset: 6036},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 225, col: 18, offset: 6041},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 225, col: 22, offset: 6045},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 225, col: 26, offset: 6049},
								expr: &seqExpr{
									pos: position{line: 225, col: 28, offset: 6051},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 225, col: 28, offset: 6051},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 225, col: 30, offset: 6053},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 34, offset: 6057},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 36, offset: 6059},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 41, offset: 6064},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 225, col: 43, offset: 6066},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 47, offset: 6070},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 225, col: 52, offset: 6075},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 225, col: 58, offset: 6081},
								expr: &seqExpr{
									pos: position{line: 225, col: 60, offset: 6083},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 225, col: 60, offset: 6083},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 225, col: 62, offset: 6085},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 66, offset: 6089},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 225, col: 68, offset: 6091},
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
			pos:  position{line: 260, col: 1, offset: 7079},
			expr: &actionExpr{
				pos: position{line: 260, col: 9, offset: 7087},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 260, col: 9, offset: 7087},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 260, col: 9, offset: 7087},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 260, col: 16, offset: 7094},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 260, col: 20, offset: 7098},
								expr: &seqExpr{
									pos: position{line: 260, col: 22, offset: 7100},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 260, col: 22, offset: 7100},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 260, col: 24, offset: 7102},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 260, col: 28, offset: 7106},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 260, col: 30, offset: 7108},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 260, col: 38, offset: 7116},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 260, col: 42, offset: 7120},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 260, col: 42, offset: 7120},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 260, col: 44, offset: 7122},
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
			pos:  position{line: 275, col: 1, offset: 7474},
			expr: &actionExpr{
				pos: position{line: 275, col: 12, offset: 7485},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 275, col: 12, offset: 7485},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 275, col: 14, offset: 7487},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 279, col: 1, offset: 7583},
			expr: &choiceExpr{
				pos: position{line: 279, col: 12, offset: 7594},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 279, col: 12, offset: 7594},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 279, col: 19, offset: 7601},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 281, col: 1, offset: 7610},
			expr: &choiceExpr{
				pos: position{line: 281, col: 9, offset: 7618},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 281, col: 9, offset: 7618},
						name: "BraceEnclosedBody",
					},
					&ruleRefExpr{
						pos:  position{line: 281, col: 29, offset: 7638},
						name: "NonWhitespaceBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 283, col: 1, offset: 7657},
			expr: &actionExpr{
				pos: position{line: 283, col: 30, offset: 7686},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 283, col: 30, offset: 7686},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 283, col: 30, offset: 7686},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 283, col: 34, offset: 7690},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 283, col: 36, offset: 7692},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 283, col: 40, offset: 7696},
								expr: &ruleRefExpr{
									pos:  position{line: 283, col: 40, offset: 7696},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 283, col: 56, offset: 7712},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 283, col: 58, offset: 7714},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 290, col: 1, offset: 7809},
			expr: &actionExpr{
				pos: position{line: 290, col: 22, offset: 7830},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 290, col: 22, offset: 7830},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 290, col: 22, offset: 7830},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 26, offset: 7834},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 290, col: 28, offset: 7836},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 290, col: 32, offset: 7840},
								expr: &ruleRefExpr{
									pos:  position{line: 290, col: 32, offset: 7840},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 290, col: 48, offset: 7856},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 290, col: 50, offset: 7858},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 304, col: 1, offset: 8193},
			expr: &actionExpr{
				pos: position{line: 304, col: 19, offset: 8211},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 304, col: 19, offset: 8211},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 304, col: 19, offset: 8211},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 304, col: 24, offset: 8216},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 304, col: 32, offset: 8224},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 304, col: 37, offset: 8229},
								expr: &seqExpr{
									pos: position{line: 304, col: 38, offset: 8230},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 304, col: 38, offset: 8230},
											expr: &charClassMatcher{
												pos:        position{line: 304, col: 38, offset: 8230},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 304, col: 46, offset: 8238},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 304, col: 47, offset: 8239},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 304, col: 47, offset: 8239},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 304, col: 51, offset: 8243},
															expr: &ruleRefExpr{
																pos:  position{line: 304, col: 51, offset: 8243},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 304, col: 64, offset: 8256},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 304, col: 64, offset: 8256},
															expr: &ruleRefExpr{
																pos:  position{line: 304, col: 64, offset: 8256},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 304, col: 73, offset: 8265},
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
											pos:  position{line: 304, col: 82, offset: 8274},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 304, col: 84, offset: 8276},
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
			pos:  position{line: 310, col: 1, offset: 8465},
			expr: &actionExpr{
				pos: position{line: 310, col: 22, offset: 8486},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 310, col: 22, offset: 8486},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 310, col: 22, offset: 8486},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 310, col: 27, offset: 8491},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 310, col: 35, offset: 8499},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 310, col: 40, offset: 8504},
								expr: &seqExpr{
									pos: position{line: 310, col: 42, offset: 8506},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 310, col: 42, offset: 8506},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 310, col: 44, offset: 8508},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 310, col: 48, offset: 8512},
											name: "_",
										},
										&choiceExpr{
											pos: position{line: 310, col: 51, offset: 8515},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 310, col: 51, offset: 8515},
													name: "Literal",
												},
												&ruleRefExpr{
													pos:  position{line: 310, col: 61, offset: 8525},
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
			pos:  position{line: 314, col: 1, offset: 8604},
			expr: &actionExpr{
				pos: position{line: 314, col: 12, offset: 8615},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 314, col: 12, offset: 8615},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 314, col: 12, offset: 8615},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 314, col: 16, offset: 8619},
								expr: &seqExpr{
									pos: position{line: 314, col: 18, offset: 8621},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 314, col: 18, offset: 8621},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 314, col: 24, offset: 8627},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 314, col: 30, offset: 8633},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 314, col: 34, offset: 8637},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 314, col: 39, offset: 8642},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 314, col: 44, offset: 8647},
								expr: &seqExpr{
									pos: position{line: 314, col: 46, offset: 8649},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 314, col: 46, offset: 8649},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 314, col: 49, offset: 8652},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 314, col: 54, offset: 8657},
											expr: &seqExpr{
												pos: position{line: 314, col: 55, offset: 8658},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 314, col: 55, offset: 8658},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 314, col: 58, offset: 8661},
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
			pos:  position{line: 337, col: 1, offset: 9233},
			expr: &actionExpr{
				pos: position{line: 337, col: 9, offset: 9241},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 337, col: 9, offset: 9241},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 337, col: 9, offset: 9241},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 337, col: 16, offset: 9248},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 337, col: 19, offset: 9251},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 337, col: 26, offset: 9258},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 337, col: 31, offset: 9263},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 337, col: 34, offset: 9266},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 337, col: 39, offset: 9271},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 337, col: 42, offset: 9274},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 337, col: 48, offset: 9280},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 348, col: 1, offset: 9529},
			expr: &choiceExpr{
				pos: position{line: 348, col: 9, offset: 9537},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 348, col: 10, offset: 9538},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 348, col: 10, offset: 9538},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 348, col: 27, offset: 9555},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 348, col: 52, offset: 9580},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 348, col: 64, offset: 9592},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 348, col: 77, offset: 9605},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 350, col: 1, offset: 9611},
			expr: &actionExpr{
				pos: position{line: 350, col: 19, offset: 9629},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 350, col: 19, offset: 9629},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 350, col: 19, offset: 9629},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 26, offset: 9636},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 31, offset: 9641},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 350, col: 33, offset: 9643},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 37, offset: 9647},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 350, col: 39, offset: 9649},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 44, offset: 9654},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 49, offset: 9659},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 350, col: 51, offset: 9661},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 54, offset: 9664},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 350, col: 67, offset: 9677},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 350, col: 69, offset: 9679},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 350, col: 75, offset: 9685},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 354, col: 1, offset: 9776},
			expr: &actionExpr{
				pos: position{line: 354, col: 26, offset: 9801},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 354, col: 26, offset: 9801},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 354, col: 26, offset: 9801},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 354, col: 31, offset: 9806},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 354, col: 36, offset: 9811},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 354, col: 38, offset: 9813},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 354, col: 41, offset: 9816},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 354, col: 54, offset: 9829},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 354, col: 56, offset: 9831},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 354, col: 62, offset: 9837},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 354, col: 67, offset: 9842},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 354, col: 69, offset: 9844},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 354, col: 73, offset: 9848},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 354, col: 75, offset: 9850},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 354, col: 82, offset: 9857},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 358, col: 1, offset: 9948},
			expr: &actionExpr{
				pos: position{line: 358, col: 17, offset: 9964},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 358, col: 17, offset: 9964},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 358, col: 22, offset: 9969},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 358, col: 22, offset: 9969},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 28, offset: 9975},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 34, offset: 9981},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 40, offset: 9987},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 46, offset: 9993},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 52, offset: 9999},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 358, col: 58, offset: 10005},
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
			pos:  position{line: 370, col: 1, offset: 10252},
			expr: &actionExpr{
				pos: position{line: 370, col: 14, offset: 10265},
				run: (*parser).callonInfixExpr1,
				expr: &seqExpr{
					pos: position{line: 370, col: 14, offset: 10265},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 370, col: 14, offset: 10265},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 370, col: 19, offset: 10270},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 370, col: 24, offset: 10275},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 370, col: 26, offset: 10277},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 370, col: 29, offset: 10280},
								name: "InfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 370, col: 37, offset: 10288},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 370, col: 39, offset: 10290},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 370, col: 45, offset: 10296},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixOp",
			pos:  position{line: 374, col: 1, offset: 10371},
			expr: &actionExpr{
				pos: position{line: 374, col: 12, offset: 10382},
				run: (*parser).callonInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 374, col: 12, offset: 10382},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 374, col: 17, offset: 10387},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 374, col: 17, offset: 10387},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 374, col: 23, offset: 10393},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 374, col: 30, offset: 10400},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 374, col: 37, offset: 10407},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 374, col: 44, offset: 10414},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 374, col: 50, offset: 10420},
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
			pos:  position{line: 386, col: 1, offset: 10667},
			expr: &choiceExpr{
				pos: position{line: 386, col: 15, offset: 10681},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 386, col: 15, offset: 10681},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 386, col: 26, offset: 10692},
						name: "Builtin",
					},
				},
			},
		},
		{
			name: "Builtin",
			pos:  position{line: 388, col: 1, offset: 10701},
			expr: &actionExpr{
				pos: position{line: 388, col: 12, offset: 10712},
				run: (*parser).callonBuiltin1,
				expr: &seqExpr{
					pos: position{line: 388, col: 12, offset: 10712},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 388, col: 12, offset: 10712},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 388, col: 17, offset: 10717},
								name: "BuiltinName",
							},
						},
						&litMatcher{
							pos:        position{line: 388, col: 29, offset: 10729},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 388, col: 33, offset: 10733},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 388, col: 35, offset: 10735},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 388, col: 40, offset: 10740},
								expr: &ruleRefExpr{
									pos:  position{line: 388, col: 40, offset: 10740},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 388, col: 46, offset: 10746},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 388, col: 51, offset: 10751},
								expr: &seqExpr{
									pos: position{line: 388, col: 53, offset: 10753},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 388, col: 53, offset: 10753},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 388, col: 55, offset: 10755},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 388, col: 59, offset: 10759},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 388, col: 61, offset: 10761},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 388, col: 69, offset: 10769},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 388, col: 72, offset: 10772},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BuiltinName",
			pos:  position{line: 404, col: 1, offset: 11176},
			expr: &actionExpr{
				pos: position{line: 404, col: 16, offset: 11191},
				run: (*parser).callonBuiltinName1,
				expr: &seqExpr{
					pos: position{line: 404, col: 16, offset: 11191},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 404, col: 16, offset: 11191},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 404, col: 21, offset: 11196},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 404, col: 25, offset: 11200},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 404, col: 30, offset: 11205},
								expr: &seqExpr{
									pos: position{line: 404, col: 32, offset: 11207},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 404, col: 32, offset: 11207},
											val:        ".",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 404, col: 36, offset: 11211},
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
			pos:  position{line: 418, col: 1, offset: 11616},
			expr: &actionExpr{
				pos: position{line: 418, col: 9, offset: 11624},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 418, col: 9, offset: 11624},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 418, col: 15, offset: 11630},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 418, col: 15, offset: 11630},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 418, col: 31, offset: 11646},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 418, col: 43, offset: 11658},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 418, col: 52, offset: 11667},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 418, col: 58, offset: 11673},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 422, col: 1, offset: 11704},
			expr: &ruleRefExpr{
				pos:  position{line: 422, col: 18, offset: 11721},
				name: "ArrayComprehension",
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 424, col: 1, offset: 11741},
			expr: &actionExpr{
				pos: position{line: 424, col: 23, offset: 11763},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 424, col: 23, offset: 11763},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 424, col: 23, offset: 11763},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 424, col: 27, offset: 11767},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 424, col: 29, offset: 11769},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 424, col: 34, offset: 11774},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 424, col: 39, offset: 11779},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 424, col: 41, offset: 11781},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 424, col: 45, offset: 11785},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 424, col: 47, offset: 11787},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 424, col: 52, offset: 11792},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 424, col: 67, offset: 11807},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 424, col: 69, offset: 11809},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 430, col: 1, offset: 11934},
			expr: &choiceExpr{
				pos: position{line: 430, col: 14, offset: 11947},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 430, col: 14, offset: 11947},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 430, col: 23, offset: 11956},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 430, col: 31, offset: 11964},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 432, col: 1, offset: 11969},
			expr: &choiceExpr{
				pos: position{line: 432, col: 11, offset: 11979},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 432, col: 11, offset: 11979},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 432, col: 20, offset: 11988},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 432, col: 29, offset: 11997},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 432, col: 36, offset: 12004},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 434, col: 1, offset: 12010},
			expr: &choiceExpr{
				pos: position{line: 434, col: 8, offset: 12017},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 434, col: 8, offset: 12017},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 434, col: 17, offset: 12026},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 434, col: 23, offset: 12032},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 436, col: 1, offset: 12037},
			expr: &actionExpr{
				pos: position{line: 436, col: 11, offset: 12047},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 436, col: 11, offset: 12047},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 436, col: 11, offset: 12047},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 436, col: 15, offset: 12051},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 436, col: 17, offset: 12053},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 436, col: 22, offset: 12058},
								expr: &seqExpr{
									pos: position{line: 436, col: 23, offset: 12059},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 436, col: 23, offset: 12059},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 27, offset: 12063},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 436, col: 29, offset: 12065},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 33, offset: 12069},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 35, offset: 12071},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 436, col: 42, offset: 12078},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 436, col: 47, offset: 12083},
								expr: &seqExpr{
									pos: position{line: 436, col: 49, offset: 12085},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 436, col: 49, offset: 12085},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 436, col: 51, offset: 12087},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 55, offset: 12091},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 57, offset: 12093},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 61, offset: 12097},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 436, col: 63, offset: 12099},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 67, offset: 12103},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 436, col: 69, offset: 12105},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 436, col: 77, offset: 12113},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 436, col: 79, offset: 12115},
							expr: &litMatcher{
								pos:        position{line: 436, col: 79, offset: 12115},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 436, col: 84, offset: 12120},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 436, col: 86, offset: 12122},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 460, col: 1, offset: 12901},
			expr: &actionExpr{
				pos: position{line: 460, col: 10, offset: 12910},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 460, col: 10, offset: 12910},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 460, col: 10, offset: 12910},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 460, col: 14, offset: 12914},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 460, col: 17, offset: 12917},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 460, col: 22, offset: 12922},
								expr: &ruleRefExpr{
									pos:  position{line: 460, col: 22, offset: 12922},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 460, col: 28, offset: 12928},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 460, col: 33, offset: 12933},
								expr: &seqExpr{
									pos: position{line: 460, col: 34, offset: 12934},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 460, col: 34, offset: 12934},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 460, col: 36, offset: 12936},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 460, col: 40, offset: 12940},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 460, col: 42, offset: 12942},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 460, col: 49, offset: 12949},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 460, col: 51, offset: 12951},
							expr: &litMatcher{
								pos:        position{line: 460, col: 51, offset: 12951},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 460, col: 56, offset: 12956},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 460, col: 59, offset: 12959},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 484, col: 1, offset: 13532},
			expr: &choiceExpr{
				pos: position{line: 484, col: 8, offset: 13539},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 484, col: 8, offset: 13539},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 484, col: 19, offset: 13550},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 486, col: 1, offset: 13563},
			expr: &actionExpr{
				pos: position{line: 486, col: 13, offset: 13575},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 486, col: 13, offset: 13575},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 486, col: 13, offset: 13575},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 486, col: 20, offset: 13582},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 486, col: 22, offset: 13584},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 492, col: 1, offset: 13672},
			expr: &actionExpr{
				pos: position{line: 492, col: 16, offset: 13687},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 492, col: 16, offset: 13687},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 492, col: 16, offset: 13687},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 492, col: 20, offset: 13691},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 492, col: 22, offset: 13693},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 492, col: 27, offset: 13698},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 492, col: 32, offset: 13703},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 492, col: 37, offset: 13708},
								expr: &seqExpr{
									pos: position{line: 492, col: 38, offset: 13709},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 492, col: 38, offset: 13709},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 492, col: 40, offset: 13711},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 492, col: 44, offset: 13715},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 492, col: 46, offset: 13717},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 492, col: 53, offset: 13724},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 492, col: 55, offset: 13726},
							expr: &litMatcher{
								pos:        position{line: 492, col: 55, offset: 13726},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 492, col: 60, offset: 13731},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 492, col: 62, offset: 13733},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 509, col: 1, offset: 14138},
			expr: &actionExpr{
				pos: position{line: 509, col: 8, offset: 14145},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 509, col: 8, offset: 14145},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 509, col: 8, offset: 14145},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 509, col: 13, offset: 14150},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 509, col: 17, offset: 14154},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 509, col: 22, offset: 14159},
								expr: &choiceExpr{
									pos: position{line: 509, col: 24, offset: 14161},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 509, col: 24, offset: 14161},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 509, col: 33, offset: 14170},
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
			pos:  position{line: 522, col: 1, offset: 14409},
			expr: &actionExpr{
				pos: position{line: 522, col: 11, offset: 14419},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 522, col: 11, offset: 14419},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 522, col: 11, offset: 14419},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 522, col: 15, offset: 14423},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 522, col: 19, offset: 14427},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 529, col: 1, offset: 14646},
			expr: &actionExpr{
				pos: position{line: 529, col: 15, offset: 14660},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 529, col: 15, offset: 14660},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 529, col: 15, offset: 14660},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 529, col: 19, offset: 14664},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 529, col: 24, offset: 14669},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 529, col: 24, offset: 14669},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 529, col: 30, offset: 14675},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 529, col: 39, offset: 14684},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 529, col: 44, offset: 14689},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 533, col: 1, offset: 14718},
			expr: &actionExpr{
				pos: position{line: 533, col: 8, offset: 14725},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 533, col: 8, offset: 14725},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 533, col: 12, offset: 14729},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 538, col: 1, offset: 14851},
			expr: &seqExpr{
				pos: position{line: 538, col: 15, offset: 14865},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 538, col: 15, offset: 14865},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 538, col: 19, offset: 14869},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 538, col: 32, offset: 14882},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 542, col: 1, offset: 14947},
			expr: &actionExpr{
				pos: position{line: 542, col: 17, offset: 14963},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 542, col: 17, offset: 14963},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 542, col: 17, offset: 14963},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 542, col: 29, offset: 14975},
							expr: &choiceExpr{
								pos: position{line: 542, col: 30, offset: 14976},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 542, col: 30, offset: 14976},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 542, col: 44, offset: 14990},
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
			pos:  position{line: 549, col: 1, offset: 15133},
			expr: &actionExpr{
				pos: position{line: 549, col: 11, offset: 15143},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 549, col: 11, offset: 15143},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 549, col: 11, offset: 15143},
							expr: &litMatcher{
								pos:        position{line: 549, col: 11, offset: 15143},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 549, col: 18, offset: 15150},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 549, col: 18, offset: 15150},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 549, col: 26, offset: 15158},
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
			pos:  position{line: 562, col: 1, offset: 15549},
			expr: &choiceExpr{
				pos: position{line: 562, col: 10, offset: 15558},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 562, col: 10, offset: 15558},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 562, col: 26, offset: 15574},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 564, col: 1, offset: 15586},
			expr: &seqExpr{
				pos: position{line: 564, col: 18, offset: 15603},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 564, col: 20, offset: 15605},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 564, col: 20, offset: 15605},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 564, col: 33, offset: 15618},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 564, col: 43, offset: 15628},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 566, col: 1, offset: 15638},
			expr: &seqExpr{
				pos: position{line: 566, col: 15, offset: 15652},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 566, col: 15, offset: 15652},
						expr: &ruleRefExpr{
							pos:  position{line: 566, col: 15, offset: 15652},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 566, col: 24, offset: 15661},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 568, col: 1, offset: 15671},
			expr: &seqExpr{
				pos: position{line: 568, col: 13, offset: 15683},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 568, col: 13, offset: 15683},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 568, col: 17, offset: 15687},
						expr: &ruleRefExpr{
							pos:  position{line: 568, col: 17, offset: 15687},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 570, col: 1, offset: 15702},
			expr: &seqExpr{
				pos: position{line: 570, col: 13, offset: 15714},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 570, col: 13, offset: 15714},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 570, col: 18, offset: 15719},
						expr: &charClassMatcher{
							pos:        position{line: 570, col: 18, offset: 15719},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 570, col: 24, offset: 15725},
						expr: &ruleRefExpr{
							pos:  position{line: 570, col: 24, offset: 15725},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 572, col: 1, offset: 15740},
			expr: &choiceExpr{
				pos: position{line: 572, col: 12, offset: 15751},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 572, col: 12, offset: 15751},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 572, col: 20, offset: 15759},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 572, col: 20, offset: 15759},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 572, col: 40, offset: 15779},
								expr: &ruleRefExpr{
									pos:  position{line: 572, col: 40, offset: 15779},
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
			pos:  position{line: 574, col: 1, offset: 15796},
			expr: &actionExpr{
				pos: position{line: 574, col: 11, offset: 15806},
				run: (*parser).callonString1,
				expr: &seqExpr{
					pos: position{line: 574, col: 11, offset: 15806},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 574, col: 11, offset: 15806},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 574, col: 15, offset: 15810},
							expr: &ruleRefExpr{
								pos:  position{line: 574, col: 15, offset: 15810},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 574, col: 21, offset: 15816},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 582, col: 1, offset: 15971},
			expr: &choiceExpr{
				pos: position{line: 582, col: 9, offset: 15979},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 582, col: 9, offset: 15979},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 582, col: 9, offset: 15979},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 586, col: 5, offset: 16079},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 586, col: 5, offset: 16079},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 592, col: 1, offset: 16180},
			expr: &actionExpr{
				pos: position{line: 592, col: 9, offset: 16188},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 592, col: 9, offset: 16188},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 598, col: 1, offset: 16283},
			expr: &charClassMatcher{
				pos:        position{line: 598, col: 16, offset: 16298},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 600, col: 1, offset: 16309},
			expr: &choiceExpr{
				pos: position{line: 600, col: 9, offset: 16317},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 600, col: 11, offset: 16319},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 600, col: 11, offset: 16319},
								expr: &ruleRefExpr{
									pos:  position{line: 600, col: 12, offset: 16320},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 600, col: 24, offset: 16332,
							},
						},
					},
					&seqExpr{
						pos: position{line: 600, col: 32, offset: 16340},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 600, col: 32, offset: 16340},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 600, col: 37, offset: 16345},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 602, col: 1, offset: 16363},
			expr: &charClassMatcher{
				pos:        position{line: 602, col: 16, offset: 16378},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 604, col: 1, offset: 16394},
			expr: &choiceExpr{
				pos: position{line: 604, col: 19, offset: 16412},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 604, col: 19, offset: 16412},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 604, col: 38, offset: 16431},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 606, col: 1, offset: 16446},
			expr: &charClassMatcher{
				pos:        position{line: 606, col: 21, offset: 16466},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 608, col: 1, offset: 16488},
			expr: &seqExpr{
				pos: position{line: 608, col: 18, offset: 16505},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 608, col: 18, offset: 16505},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 608, col: 22, offset: 16509},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 608, col: 31, offset: 16518},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 608, col: 40, offset: 16527},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 608, col: 49, offset: 16536},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 610, col: 1, offset: 16546},
			expr: &charClassMatcher{
				pos:        position{line: 610, col: 17, offset: 16562},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 612, col: 1, offset: 16569},
			expr: &charClassMatcher{
				pos:        position{line: 612, col: 24, offset: 16592},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 614, col: 1, offset: 16599},
			expr: &charClassMatcher{
				pos:        position{line: 614, col: 13, offset: 16611},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 616, col: 1, offset: 16624},
			expr: &oneOrMoreExpr{
				pos: position{line: 616, col: 20, offset: 16643},
				expr: &charClassMatcher{
					pos:        position{line: 616, col: 20, offset: 16643},
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
			pos:         position{line: 618, col: 1, offset: 16655},
			expr: &zeroOrMoreExpr{
				pos: position{line: 618, col: 19, offset: 16673},
				expr: &choiceExpr{
					pos: position{line: 618, col: 21, offset: 16675},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 618, col: 21, offset: 16675},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 618, col: 33, offset: 16687},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 620, col: 1, offset: 16699},
			expr: &actionExpr{
				pos: position{line: 620, col: 12, offset: 16710},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 620, col: 12, offset: 16710},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 620, col: 12, offset: 16710},
							expr: &charClassMatcher{
								pos:        position{line: 620, col: 12, offset: 16710},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 620, col: 19, offset: 16717},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 620, col: 23, offset: 16721},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 620, col: 28, offset: 16726},
								expr: &charClassMatcher{
									pos:        position{line: 620, col: 28, offset: 16726},
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
			pos:  position{line: 631, col: 1, offset: 17002},
			expr: &notExpr{
				pos: position{line: 631, col: 8, offset: 17009},
				expr: &anyMatcher{
					line: 631, col: 9, offset: 17010,
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

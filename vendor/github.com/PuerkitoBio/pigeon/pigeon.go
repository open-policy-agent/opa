package main

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

	"github.com/PuerkitoBio/pigeon/ast"
)

var g = &grammar{
	rules: []*rule{
		{
			name: "Grammar",
			pos:  position{line: 5, col: 1, offset: 18},
			expr: &actionExpr{
				pos: position{line: 5, col: 11, offset: 30},
				run: (*parser).callonGrammar1,
				expr: &seqExpr{
					pos: position{line: 5, col: 11, offset: 30},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 5, col: 11, offset: 30},
							name: "__",
						},
						&labeledExpr{
							pos:   position{line: 5, col: 14, offset: 33},
							label: "initializer",
							expr: &zeroOrOneExpr{
								pos: position{line: 5, col: 26, offset: 45},
								expr: &seqExpr{
									pos: position{line: 5, col: 28, offset: 47},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 5, col: 28, offset: 47},
											name: "Initializer",
										},
										&ruleRefExpr{
											pos:  position{line: 5, col: 40, offset: 59},
											name: "__",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 5, col: 46, offset: 65},
							label: "rules",
							expr: &oneOrMoreExpr{
								pos: position{line: 5, col: 52, offset: 71},
								expr: &seqExpr{
									pos: position{line: 5, col: 54, offset: 73},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 5, col: 54, offset: 73},
											name: "Rule",
										},
										&ruleRefExpr{
											pos:  position{line: 5, col: 59, offset: 78},
											name: "__",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 5, col: 65, offset: 84},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Initializer",
			pos:  position{line: 24, col: 1, offset: 525},
			expr: &actionExpr{
				pos: position{line: 24, col: 15, offset: 541},
				run: (*parser).callonInitializer1,
				expr: &seqExpr{
					pos: position{line: 24, col: 15, offset: 541},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 24, col: 15, offset: 541},
							label: "code",
							expr: &ruleRefExpr{
								pos:  position{line: 24, col: 20, offset: 546},
								name: "CodeBlock",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 24, col: 30, offset: 556},
							name: "EOS",
						},
					},
				},
			},
		},
		{
			name: "Rule",
			pos:  position{line: 28, col: 1, offset: 586},
			expr: &actionExpr{
				pos: position{line: 28, col: 8, offset: 595},
				run: (*parser).callonRule1,
				expr: &seqExpr{
					pos: position{line: 28, col: 8, offset: 595},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 28, col: 8, offset: 595},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 28, col: 13, offset: 600},
								name: "IdentifierName",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 28, col: 28, offset: 615},
							name: "__",
						},
						&labeledExpr{
							pos:   position{line: 28, col: 31, offset: 618},
							label: "display",
							expr: &zeroOrOneExpr{
								pos: position{line: 28, col: 39, offset: 626},
								expr: &seqExpr{
									pos: position{line: 28, col: 41, offset: 628},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 28, col: 41, offset: 628},
											name: "StringLiteral",
										},
										&ruleRefExpr{
											pos:  position{line: 28, col: 55, offset: 642},
											name: "__",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 28, col: 61, offset: 648},
							name: "RuleDefOp",
						},
						&ruleRefExpr{
							pos:  position{line: 28, col: 71, offset: 658},
							name: "__",
						},
						&labeledExpr{
							pos:   position{line: 28, col: 74, offset: 661},
							label: "expr",
							expr: &ruleRefExpr{
								pos:  position{line: 28, col: 79, offset: 666},
								name: "Expression",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 28, col: 90, offset: 677},
							name: "EOS",
						},
					},
				},
			},
		},
		{
			name: "Expression",
			pos:  position{line: 41, col: 1, offset: 961},
			expr: &ruleRefExpr{
				pos:  position{line: 41, col: 14, offset: 976},
				name: "ChoiceExpr",
			},
		},
		{
			name: "ChoiceExpr",
			pos:  position{line: 43, col: 1, offset: 988},
			expr: &actionExpr{
				pos: position{line: 43, col: 14, offset: 1003},
				run: (*parser).callonChoiceExpr1,
				expr: &seqExpr{
					pos: position{line: 43, col: 14, offset: 1003},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 43, col: 14, offset: 1003},
							label: "first",
							expr: &ruleRefExpr{
								pos:  position{line: 43, col: 20, offset: 1009},
								name: "ActionExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 43, col: 31, offset: 1020},
							label: "rest",
							expr: &zeroOrMoreExpr{
								pos: position{line: 43, col: 36, offset: 1025},
								expr: &seqExpr{
									pos: position{line: 43, col: 38, offset: 1027},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 43, col: 38, offset: 1027},
											name: "__",
										},
										&litMatcher{
											pos:        position{line: 43, col: 41, offset: 1030},
											val:        "/",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 45, offset: 1034},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 48, offset: 1037},
											name: "ActionExpr",
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
			name: "ActionExpr",
			pos:  position{line: 58, col: 1, offset: 1442},
			expr: &actionExpr{
				pos: position{line: 58, col: 14, offset: 1457},
				run: (*parser).callonActionExpr1,
				expr: &seqExpr{
					pos: position{line: 58, col: 14, offset: 1457},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 58, col: 14, offset: 1457},
							label: "expr",
							expr: &ruleRefExpr{
								pos:  position{line: 58, col: 19, offset: 1462},
								name: "SeqExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 58, col: 27, offset: 1470},
							label: "code",
							expr: &zeroOrOneExpr{
								pos: position{line: 58, col: 32, offset: 1475},
								expr: &seqExpr{
									pos: position{line: 58, col: 34, offset: 1477},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 58, col: 34, offset: 1477},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 58, col: 37, offset: 1480},
											name: "CodeBlock",
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
			name: "SeqExpr",
			pos:  position{line: 72, col: 1, offset: 1746},
			expr: &actionExpr{
				pos: position{line: 72, col: 11, offset: 1758},
				run: (*parser).callonSeqExpr1,
				expr: &seqExpr{
					pos: position{line: 72, col: 11, offset: 1758},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 72, col: 11, offset: 1758},
							label: "first",
							expr: &ruleRefExpr{
								pos:  position{line: 72, col: 17, offset: 1764},
								name: "LabeledExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 72, col: 29, offset: 1776},
							label: "rest",
							expr: &zeroOrMoreExpr{
								pos: position{line: 72, col: 34, offset: 1781},
								expr: &seqExpr{
									pos: position{line: 72, col: 36, offset: 1783},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 72, col: 36, offset: 1783},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 72, col: 39, offset: 1786},
											name: "LabeledExpr",
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
			name: "LabeledExpr",
			pos:  position{line: 85, col: 1, offset: 2137},
			expr: &choiceExpr{
				pos: position{line: 85, col: 15, offset: 2153},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 85, col: 15, offset: 2153},
						run: (*parser).callonLabeledExpr2,
						expr: &seqExpr{
							pos: position{line: 85, col: 15, offset: 2153},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 85, col: 15, offset: 2153},
									label: "label",
									expr: &ruleRefExpr{
										pos:  position{line: 85, col: 21, offset: 2159},
										name: "Identifier",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 85, col: 32, offset: 2170},
									name: "__",
								},
								&litMatcher{
									pos:        position{line: 85, col: 35, offset: 2173},
									val:        ":",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 85, col: 39, offset: 2177},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 85, col: 42, offset: 2180},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 85, col: 47, offset: 2185},
										name: "PrefixedExpr",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 91, col: 5, offset: 2358},
						name: "PrefixedExpr",
					},
				},
			},
		},
		{
			name: "PrefixedExpr",
			pos:  position{line: 93, col: 1, offset: 2372},
			expr: &choiceExpr{
				pos: position{line: 93, col: 16, offset: 2389},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 93, col: 16, offset: 2389},
						run: (*parser).callonPrefixedExpr2,
						expr: &seqExpr{
							pos: position{line: 93, col: 16, offset: 2389},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 93, col: 16, offset: 2389},
									label: "op",
									expr: &ruleRefExpr{
										pos:  position{line: 93, col: 19, offset: 2392},
										name: "PrefixedOp",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 93, col: 30, offset: 2403},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 93, col: 33, offset: 2406},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 93, col: 38, offset: 2411},
										name: "SuffixedExpr",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 104, col: 5, offset: 2693},
						name: "SuffixedExpr",
					},
				},
			},
		},
		{
			name: "PrefixedOp",
			pos:  position{line: 106, col: 1, offset: 2707},
			expr: &actionExpr{
				pos: position{line: 106, col: 14, offset: 2722},
				run: (*parser).callonPrefixedOp1,
				expr: &choiceExpr{
					pos: position{line: 106, col: 16, offset: 2724},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 106, col: 16, offset: 2724},
							val:        "&",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 106, col: 22, offset: 2730},
							val:        "!",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SuffixedExpr",
			pos:  position{line: 110, col: 1, offset: 2772},
			expr: &choiceExpr{
				pos: position{line: 110, col: 16, offset: 2789},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 110, col: 16, offset: 2789},
						run: (*parser).callonSuffixedExpr2,
						expr: &seqExpr{
							pos: position{line: 110, col: 16, offset: 2789},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 110, col: 16, offset: 2789},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 110, col: 21, offset: 2794},
										name: "PrimaryExpr",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 110, col: 33, offset: 2806},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 110, col: 36, offset: 2809},
									label: "op",
									expr: &ruleRefExpr{
										pos:  position{line: 110, col: 39, offset: 2812},
										name: "SuffixedOp",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 129, col: 5, offset: 3342},
						name: "PrimaryExpr",
					},
				},
			},
		},
		{
			name: "SuffixedOp",
			pos:  position{line: 131, col: 1, offset: 3356},
			expr: &actionExpr{
				pos: position{line: 131, col: 14, offset: 3371},
				run: (*parser).callonSuffixedOp1,
				expr: &choiceExpr{
					pos: position{line: 131, col: 16, offset: 3373},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 131, col: 16, offset: 3373},
							val:        "?",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 131, col: 22, offset: 3379},
							val:        "*",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 131, col: 28, offset: 3385},
							val:        "+",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "PrimaryExpr",
			pos:  position{line: 135, col: 1, offset: 3427},
			expr: &choiceExpr{
				pos: position{line: 135, col: 15, offset: 3443},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 135, col: 15, offset: 3443},
						name: "LitMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 135, col: 28, offset: 3456},
						name: "CharClassMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 135, col: 47, offset: 3475},
						name: "AnyMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 135, col: 60, offset: 3488},
						name: "RuleRefExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 135, col: 74, offset: 3502},
						name: "SemanticPredExpr",
					},
					&actionExpr{
						pos: position{line: 135, col: 93, offset: 3521},
						run: (*parser).callonPrimaryExpr7,
						expr: &seqExpr{
							pos: position{line: 135, col: 93, offset: 3521},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 135, col: 93, offset: 3521},
									val:        "(",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 135, col: 97, offset: 3525},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 135, col: 100, offset: 3528},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 135, col: 105, offset: 3533},
										name: "Expression",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 135, col: 116, offset: 3544},
									name: "__",
								},
								&litMatcher{
									pos:        position{line: 135, col: 119, offset: 3547},
									val:        ")",
									ignoreCase: false,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "RuleRefExpr",
			pos:  position{line: 138, col: 1, offset: 3576},
			expr: &actionExpr{
				pos: position{line: 138, col: 15, offset: 3592},
				run: (*parser).callonRuleRefExpr1,
				expr: &seqExpr{
					pos: position{line: 138, col: 15, offset: 3592},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 138, col: 15, offset: 3592},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 138, col: 20, offset: 3597},
								name: "IdentifierName",
							},
						},
						&notExpr{
							pos: position{line: 138, col: 35, offset: 3612},
							expr: &seqExpr{
								pos: position{line: 138, col: 38, offset: 3615},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 138, col: 38, offset: 3615},
										name: "__",
									},
									&zeroOrOneExpr{
										pos: position{line: 138, col: 41, offset: 3618},
										expr: &seqExpr{
											pos: position{line: 138, col: 43, offset: 3620},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 138, col: 43, offset: 3620},
													name: "StringLiteral",
												},
												&ruleRefExpr{
													pos:  position{line: 138, col: 57, offset: 3634},
													name: "__",
												},
											},
										},
									},
									&ruleRefExpr{
										pos:  position{line: 138, col: 63, offset: 3640},
										name: "RuleDefOp",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SemanticPredExpr",
			pos:  position{line: 143, col: 1, offset: 3756},
			expr: &actionExpr{
				pos: position{line: 143, col: 20, offset: 3777},
				run: (*parser).callonSemanticPredExpr1,
				expr: &seqExpr{
					pos: position{line: 143, col: 20, offset: 3777},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 143, col: 20, offset: 3777},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 143, col: 23, offset: 3780},
								name: "SemanticPredOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 143, col: 38, offset: 3795},
							name: "__",
						},
						&labeledExpr{
							pos:   position{line: 143, col: 41, offset: 3798},
							label: "code",
							expr: &ruleRefExpr{
								pos:  position{line: 143, col: 46, offset: 3803},
								name: "CodeBlock",
							},
						},
					},
				},
			},
		},
		{
			name: "SemanticPredOp",
			pos:  position{line: 154, col: 1, offset: 4080},
			expr: &actionExpr{
				pos: position{line: 154, col: 18, offset: 4099},
				run: (*parser).callonSemanticPredOp1,
				expr: &choiceExpr{
					pos: position{line: 154, col: 20, offset: 4101},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 154, col: 20, offset: 4101},
							val:        "&",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 154, col: 26, offset: 4107},
							val:        "!",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "RuleDefOp",
			pos:  position{line: 158, col: 1, offset: 4149},
			expr: &choiceExpr{
				pos: position{line: 158, col: 13, offset: 4163},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 158, col: 13, offset: 4163},
						val:        "=",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 158, col: 19, offset: 4169},
						val:        "<-",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 158, col: 26, offset: 4176},
						val:        "←",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 158, col: 37, offset: 4187},
						val:        "⟵",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "SourceChar",
			pos:  position{line: 160, col: 1, offset: 4197},
			expr: &anyMatcher{
				line: 160, col: 14, offset: 4212,
			},
		},
		{
			name: "Comment",
			pos:  position{line: 161, col: 1, offset: 4214},
			expr: &choiceExpr{
				pos: position{line: 161, col: 11, offset: 4226},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 161, col: 11, offset: 4226},
						name: "MultiLineComment",
					},
					&ruleRefExpr{
						pos:  position{line: 161, col: 30, offset: 4245},
						name: "SingleLineComment",
					},
				},
			},
		},
		{
			name: "MultiLineComment",
			pos:  position{line: 162, col: 1, offset: 4263},
			expr: &seqExpr{
				pos: position{line: 162, col: 20, offset: 4284},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 162, col: 20, offset: 4284},
						val:        "/*",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 162, col: 25, offset: 4289},
						expr: &seqExpr{
							pos: position{line: 162, col: 27, offset: 4291},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 162, col: 27, offset: 4291},
									expr: &litMatcher{
										pos:        position{line: 162, col: 28, offset: 4292},
										val:        "*/",
										ignoreCase: false,
									},
								},
								&ruleRefExpr{
									pos:  position{line: 162, col: 33, offset: 4297},
									name: "SourceChar",
								},
							},
						},
					},
					&litMatcher{
						pos:        position{line: 162, col: 47, offset: 4311},
						val:        "*/",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "MultiLineCommentNoLineTerminator",
			pos:  position{line: 163, col: 1, offset: 4316},
			expr: &seqExpr{
				pos: position{line: 163, col: 36, offset: 4353},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 163, col: 36, offset: 4353},
						val:        "/*",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 163, col: 41, offset: 4358},
						expr: &seqExpr{
							pos: position{line: 163, col: 43, offset: 4360},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 163, col: 43, offset: 4360},
									expr: &choiceExpr{
										pos: position{line: 163, col: 46, offset: 4363},
										alternatives: []interface{}{
											&litMatcher{
												pos:        position{line: 163, col: 46, offset: 4363},
												val:        "*/",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 163, col: 53, offset: 4370},
												name: "EOL",
											},
										},
									},
								},
								&ruleRefExpr{
									pos:  position{line: 163, col: 59, offset: 4376},
									name: "SourceChar",
								},
							},
						},
					},
					&litMatcher{
						pos:        position{line: 163, col: 73, offset: 4390},
						val:        "*/",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "SingleLineComment",
			pos:  position{line: 164, col: 1, offset: 4395},
			expr: &seqExpr{
				pos: position{line: 164, col: 21, offset: 4417},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 164, col: 21, offset: 4417},
						val:        "//",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 164, col: 26, offset: 4422},
						expr: &seqExpr{
							pos: position{line: 164, col: 28, offset: 4424},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 164, col: 28, offset: 4424},
									expr: &ruleRefExpr{
										pos:  position{line: 164, col: 29, offset: 4425},
										name: "EOL",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 164, col: 33, offset: 4429},
									name: "SourceChar",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Identifier",
			pos:  position{line: 166, col: 1, offset: 4444},
			expr: &actionExpr{
				pos: position{line: 166, col: 14, offset: 4459},
				run: (*parser).callonIdentifier1,
				expr: &labeledExpr{
					pos:   position{line: 166, col: 14, offset: 4459},
					label: "ident",
					expr: &ruleRefExpr{
						pos:  position{line: 166, col: 20, offset: 4465},
						name: "IdentifierName",
					},
				},
			},
		},
		{
			name: "IdentifierName",
			pos:  position{line: 174, col: 1, offset: 4684},
			expr: &actionExpr{
				pos: position{line: 174, col: 18, offset: 4703},
				run: (*parser).callonIdentifierName1,
				expr: &seqExpr{
					pos: position{line: 174, col: 18, offset: 4703},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 174, col: 18, offset: 4703},
							name: "IdentifierStart",
						},
						&zeroOrMoreExpr{
							pos: position{line: 174, col: 34, offset: 4719},
							expr: &ruleRefExpr{
								pos:  position{line: 174, col: 34, offset: 4719},
								name: "IdentifierPart",
							},
						},
					},
				},
			},
		},
		{
			name: "IdentifierStart",
			pos:  position{line: 177, col: 1, offset: 4801},
			expr: &charClassMatcher{
				pos:        position{line: 177, col: 19, offset: 4821},
				val:        "[\\pL_]",
				chars:      []rune{'_'},
				classes:    []*unicode.RangeTable{rangeTable("L")},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "IdentifierPart",
			pos:  position{line: 178, col: 1, offset: 4828},
			expr: &choiceExpr{
				pos: position{line: 178, col: 18, offset: 4847},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 178, col: 18, offset: 4847},
						name: "IdentifierStart",
					},
					&charClassMatcher{
						pos:        position{line: 178, col: 36, offset: 4865},
						val:        "[\\p{Nd}]",
						classes:    []*unicode.RangeTable{rangeTable("Nd")},
						ignoreCase: false,
						inverted:   false,
					},
				},
			},
		},
		{
			name: "LitMatcher",
			pos:  position{line: 180, col: 1, offset: 4875},
			expr: &actionExpr{
				pos: position{line: 180, col: 14, offset: 4890},
				run: (*parser).callonLitMatcher1,
				expr: &seqExpr{
					pos: position{line: 180, col: 14, offset: 4890},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 180, col: 14, offset: 4890},
							label: "lit",
							expr: &ruleRefExpr{
								pos:  position{line: 180, col: 18, offset: 4894},
								name: "StringLiteral",
							},
						},
						&labeledExpr{
							pos:   position{line: 180, col: 32, offset: 4908},
							label: "ignore",
							expr: &zeroOrOneExpr{
								pos: position{line: 180, col: 39, offset: 4915},
								expr: &litMatcher{
									pos:        position{line: 180, col: 39, offset: 4915},
									val:        "i",
									ignoreCase: false,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "StringLiteral",
			pos:  position{line: 193, col: 1, offset: 5314},
			expr: &choiceExpr{
				pos: position{line: 193, col: 17, offset: 5332},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 193, col: 17, offset: 5332},
						run: (*parser).callonStringLiteral2,
						expr: &choiceExpr{
							pos: position{line: 193, col: 19, offset: 5334},
							alternatives: []interface{}{
								&seqExpr{
									pos: position{line: 193, col: 19, offset: 5334},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 193, col: 19, offset: 5334},
											val:        "\"",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 193, col: 23, offset: 5338},
											expr: &ruleRefExpr{
												pos:  position{line: 193, col: 23, offset: 5338},
												name: "DoubleStringChar",
											},
										},
										&litMatcher{
											pos:        position{line: 193, col: 41, offset: 5356},
											val:        "\"",
											ignoreCase: false,
										},
									},
								},
								&seqExpr{
									pos: position{line: 193, col: 47, offset: 5362},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 193, col: 47, offset: 5362},
											val:        "'",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 193, col: 51, offset: 5366},
											name: "SingleStringChar",
										},
										&litMatcher{
											pos:        position{line: 193, col: 68, offset: 5383},
											val:        "'",
											ignoreCase: false,
										},
									},
								},
								&seqExpr{
									pos: position{line: 193, col: 74, offset: 5389},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 193, col: 74, offset: 5389},
											val:        "`",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 193, col: 78, offset: 5393},
											expr: &ruleRefExpr{
												pos:  position{line: 193, col: 78, offset: 5393},
												name: "RawStringChar",
											},
										},
										&litMatcher{
											pos:        position{line: 193, col: 93, offset: 5408},
											val:        "`",
											ignoreCase: false,
										},
									},
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 195, col: 5, offset: 5481},
						run: (*parser).callonStringLiteral18,
						expr: &choiceExpr{
							pos: position{line: 195, col: 7, offset: 5483},
							alternatives: []interface{}{
								&seqExpr{
									pos: position{line: 195, col: 9, offset: 5485},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 195, col: 9, offset: 5485},
											val:        "\"",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 195, col: 13, offset: 5489},
											expr: &ruleRefExpr{
												pos:  position{line: 195, col: 13, offset: 5489},
												name: "DoubleStringChar",
											},
										},
										&choiceExpr{
											pos: position{line: 195, col: 33, offset: 5509},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 195, col: 33, offset: 5509},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 195, col: 39, offset: 5515},
													name: "EOF",
												},
											},
										},
									},
								},
								&seqExpr{
									pos: position{line: 195, col: 51, offset: 5527},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 195, col: 51, offset: 5527},
											val:        "'",
											ignoreCase: false,
										},
										&zeroOrOneExpr{
											pos: position{line: 195, col: 55, offset: 5531},
											expr: &ruleRefExpr{
												pos:  position{line: 195, col: 55, offset: 5531},
												name: "SingleStringChar",
											},
										},
										&choiceExpr{
											pos: position{line: 195, col: 75, offset: 5551},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 195, col: 75, offset: 5551},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 195, col: 81, offset: 5557},
													name: "EOF",
												},
											},
										},
									},
								},
								&seqExpr{
									pos: position{line: 195, col: 91, offset: 5567},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 195, col: 91, offset: 5567},
											val:        "`",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 195, col: 95, offset: 5571},
											expr: &ruleRefExpr{
												pos:  position{line: 195, col: 95, offset: 5571},
												name: "RawStringChar",
											},
										},
										&ruleRefExpr{
											pos:  position{line: 195, col: 110, offset: 5586},
											name: "EOF",
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
			name: "DoubleStringChar",
			pos:  position{line: 199, col: 1, offset: 5688},
			expr: &choiceExpr{
				pos: position{line: 199, col: 20, offset: 5709},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 199, col: 20, offset: 5709},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 199, col: 20, offset: 5709},
								expr: &choiceExpr{
									pos: position{line: 199, col: 23, offset: 5712},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 199, col: 23, offset: 5712},
											val:        "\"",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 199, col: 29, offset: 5718},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 199, col: 36, offset: 5725},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 199, col: 42, offset: 5731},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 199, col: 55, offset: 5744},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 199, col: 55, offset: 5744},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 199, col: 60, offset: 5749},
								name: "DoubleStringEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "SingleStringChar",
			pos:  position{line: 200, col: 1, offset: 5768},
			expr: &choiceExpr{
				pos: position{line: 200, col: 20, offset: 5789},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 200, col: 20, offset: 5789},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 200, col: 20, offset: 5789},
								expr: &choiceExpr{
									pos: position{line: 200, col: 23, offset: 5792},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 200, col: 23, offset: 5792},
											val:        "'",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 200, col: 29, offset: 5798},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 200, col: 36, offset: 5805},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 200, col: 42, offset: 5811},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 200, col: 55, offset: 5824},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 200, col: 55, offset: 5824},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 200, col: 60, offset: 5829},
								name: "SingleStringEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "RawStringChar",
			pos:  position{line: 201, col: 1, offset: 5848},
			expr: &seqExpr{
				pos: position{line: 201, col: 17, offset: 5866},
				exprs: []interface{}{
					&notExpr{
						pos: position{line: 201, col: 17, offset: 5866},
						expr: &litMatcher{
							pos:        position{line: 201, col: 18, offset: 5867},
							val:        "`",
							ignoreCase: false,
						},
					},
					&ruleRefExpr{
						pos:  position{line: 201, col: 22, offset: 5871},
						name: "SourceChar",
					},
				},
			},
		},
		{
			name: "DoubleStringEscape",
			pos:  position{line: 203, col: 1, offset: 5883},
			expr: &choiceExpr{
				pos: position{line: 203, col: 22, offset: 5906},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 203, col: 24, offset: 5908},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 203, col: 24, offset: 5908},
								val:        "\"",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 203, col: 30, offset: 5914},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 204, col: 7, offset: 5943},
						run: (*parser).callonDoubleStringEscape5,
						expr: &choiceExpr{
							pos: position{line: 204, col: 9, offset: 5945},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 204, col: 9, offset: 5945},
									name: "SourceChar",
								},
								&ruleRefExpr{
									pos:  position{line: 204, col: 22, offset: 5958},
									name: "EOL",
								},
								&ruleRefExpr{
									pos:  position{line: 204, col: 28, offset: 5964},
									name: "EOF",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "SingleStringEscape",
			pos:  position{line: 207, col: 1, offset: 6029},
			expr: &choiceExpr{
				pos: position{line: 207, col: 22, offset: 6052},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 207, col: 24, offset: 6054},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 207, col: 24, offset: 6054},
								val:        "'",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 207, col: 30, offset: 6060},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 208, col: 7, offset: 6089},
						run: (*parser).callonSingleStringEscape5,
						expr: &choiceExpr{
							pos: position{line: 208, col: 9, offset: 6091},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 208, col: 9, offset: 6091},
									name: "SourceChar",
								},
								&ruleRefExpr{
									pos:  position{line: 208, col: 22, offset: 6104},
									name: "EOL",
								},
								&ruleRefExpr{
									pos:  position{line: 208, col: 28, offset: 6110},
									name: "EOF",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "CommonEscapeSequence",
			pos:  position{line: 212, col: 1, offset: 6176},
			expr: &choiceExpr{
				pos: position{line: 212, col: 24, offset: 6201},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 212, col: 24, offset: 6201},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 212, col: 43, offset: 6220},
						name: "OctalEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 212, col: 57, offset: 6234},
						name: "HexEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 212, col: 69, offset: 6246},
						name: "LongUnicodeEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 212, col: 89, offset: 6266},
						name: "ShortUnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 213, col: 1, offset: 6285},
			expr: &choiceExpr{
				pos: position{line: 213, col: 20, offset: 6306},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 213, col: 20, offset: 6306},
						val:        "a",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 26, offset: 6312},
						val:        "b",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 32, offset: 6318},
						val:        "n",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 38, offset: 6324},
						val:        "f",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 44, offset: 6330},
						val:        "r",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 50, offset: 6336},
						val:        "t",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 56, offset: 6342},
						val:        "v",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 213, col: 62, offset: 6348},
						val:        "\\",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "OctalEscape",
			pos:  position{line: 214, col: 1, offset: 6353},
			expr: &choiceExpr{
				pos: position{line: 214, col: 15, offset: 6369},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 214, col: 15, offset: 6369},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 214, col: 15, offset: 6369},
								name: "OctalDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 214, col: 26, offset: 6380},
								name: "OctalDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 214, col: 37, offset: 6391},
								name: "OctalDigit",
							},
						},
					},
					&actionExpr{
						pos: position{line: 215, col: 7, offset: 6408},
						run: (*parser).callonOctalEscape6,
						expr: &seqExpr{
							pos: position{line: 215, col: 7, offset: 6408},
							exprs: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 215, col: 7, offset: 6408},
									name: "OctalDigit",
								},
								&choiceExpr{
									pos: position{line: 215, col: 20, offset: 6421},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 215, col: 20, offset: 6421},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 215, col: 33, offset: 6434},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 215, col: 39, offset: 6440},
											name: "EOF",
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
			name: "HexEscape",
			pos:  position{line: 218, col: 1, offset: 6501},
			expr: &choiceExpr{
				pos: position{line: 218, col: 13, offset: 6515},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 218, col: 13, offset: 6515},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 218, col: 13, offset: 6515},
								val:        "x",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 218, col: 17, offset: 6519},
								name: "HexDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 218, col: 26, offset: 6528},
								name: "HexDigit",
							},
						},
					},
					&actionExpr{
						pos: position{line: 219, col: 7, offset: 6543},
						run: (*parser).callonHexEscape6,
						expr: &seqExpr{
							pos: position{line: 219, col: 7, offset: 6543},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 219, col: 7, offset: 6543},
									val:        "x",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 219, col: 13, offset: 6549},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 219, col: 13, offset: 6549},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 219, col: 26, offset: 6562},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 219, col: 32, offset: 6568},
											name: "EOF",
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
			name: "LongUnicodeEscape",
			pos:  position{line: 222, col: 1, offset: 6635},
			expr: &choiceExpr{
				pos: position{line: 223, col: 5, offset: 6662},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 223, col: 5, offset: 6662},
						run: (*parser).callonLongUnicodeEscape2,
						expr: &seqExpr{
							pos: position{line: 223, col: 5, offset: 6662},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 223, col: 5, offset: 6662},
									val:        "U",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 9, offset: 6666},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 18, offset: 6675},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 27, offset: 6684},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 36, offset: 6693},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 45, offset: 6702},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 54, offset: 6711},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 63, offset: 6720},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 223, col: 72, offset: 6729},
									name: "HexDigit",
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 226, col: 7, offset: 6831},
						run: (*parser).callonLongUnicodeEscape13,
						expr: &seqExpr{
							pos: position{line: 226, col: 7, offset: 6831},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 226, col: 7, offset: 6831},
									val:        "U",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 226, col: 13, offset: 6837},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 226, col: 13, offset: 6837},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 226, col: 26, offset: 6850},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 226, col: 32, offset: 6856},
											name: "EOF",
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
			name: "ShortUnicodeEscape",
			pos:  position{line: 229, col: 1, offset: 6919},
			expr: &choiceExpr{
				pos: position{line: 230, col: 5, offset: 6947},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 230, col: 5, offset: 6947},
						run: (*parser).callonShortUnicodeEscape2,
						expr: &seqExpr{
							pos: position{line: 230, col: 5, offset: 6947},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 230, col: 5, offset: 6947},
									val:        "u",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 230, col: 9, offset: 6951},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 230, col: 18, offset: 6960},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 230, col: 27, offset: 6969},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 230, col: 36, offset: 6978},
									name: "HexDigit",
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 233, col: 7, offset: 7080},
						run: (*parser).callonShortUnicodeEscape9,
						expr: &seqExpr{
							pos: position{line: 233, col: 7, offset: 7080},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 233, col: 7, offset: 7080},
									val:        "u",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 233, col: 13, offset: 7086},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 233, col: 13, offset: 7086},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 26, offset: 7099},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 32, offset: 7105},
											name: "EOF",
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
			name: "OctalDigit",
			pos:  position{line: 237, col: 1, offset: 7169},
			expr: &charClassMatcher{
				pos:        position{line: 237, col: 14, offset: 7184},
				val:        "[0-7]",
				ranges:     []rune{'0', '7'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 238, col: 1, offset: 7190},
			expr: &charClassMatcher{
				pos:        position{line: 238, col: 16, offset: 7207},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 239, col: 1, offset: 7213},
			expr: &charClassMatcher{
				pos:        position{line: 239, col: 12, offset: 7226},
				val:        "[0-9a-f]i",
				ranges:     []rune{'0', '9', 'a', 'f'},
				ignoreCase: true,
				inverted:   false,
			},
		},
		{
			name: "CharClassMatcher",
			pos:  position{line: 241, col: 1, offset: 7237},
			expr: &choiceExpr{
				pos: position{line: 241, col: 20, offset: 7258},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 241, col: 20, offset: 7258},
						run: (*parser).callonCharClassMatcher2,
						expr: &seqExpr{
							pos: position{line: 241, col: 20, offset: 7258},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 241, col: 20, offset: 7258},
									val:        "[",
									ignoreCase: false,
								},
								&zeroOrMoreExpr{
									pos: position{line: 241, col: 24, offset: 7262},
									expr: &choiceExpr{
										pos: position{line: 241, col: 26, offset: 7264},
										alternatives: []interface{}{
											&ruleRefExpr{
												pos:  position{line: 241, col: 26, offset: 7264},
												name: "ClassCharRange",
											},
											&ruleRefExpr{
												pos:  position{line: 241, col: 43, offset: 7281},
												name: "ClassChar",
											},
											&seqExpr{
												pos: position{line: 241, col: 55, offset: 7293},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 241, col: 55, offset: 7293},
														val:        "\\",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 241, col: 60, offset: 7298},
														name: "UnicodeClassEscape",
													},
												},
											},
										},
									},
								},
								&litMatcher{
									pos:        position{line: 241, col: 82, offset: 7320},
									val:        "]",
									ignoreCase: false,
								},
								&zeroOrOneExpr{
									pos: position{line: 241, col: 86, offset: 7324},
									expr: &litMatcher{
										pos:        position{line: 241, col: 86, offset: 7324},
										val:        "i",
										ignoreCase: false,
									},
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 245, col: 5, offset: 7431},
						run: (*parser).callonCharClassMatcher15,
						expr: &seqExpr{
							pos: position{line: 245, col: 5, offset: 7431},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 245, col: 5, offset: 7431},
									val:        "[",
									ignoreCase: false,
								},
								&zeroOrMoreExpr{
									pos: position{line: 245, col: 9, offset: 7435},
									expr: &seqExpr{
										pos: position{line: 245, col: 11, offset: 7437},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 245, col: 11, offset: 7437},
												expr: &ruleRefExpr{
													pos:  position{line: 245, col: 14, offset: 7440},
													name: "EOL",
												},
											},
											&ruleRefExpr{
												pos:  position{line: 245, col: 20, offset: 7446},
												name: "SourceChar",
											},
										},
									},
								},
								&choiceExpr{
									pos: position{line: 245, col: 36, offset: 7462},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 245, col: 36, offset: 7462},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 245, col: 42, offset: 7468},
											name: "EOF",
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
			name: "ClassCharRange",
			pos:  position{line: 249, col: 1, offset: 7578},
			expr: &seqExpr{
				pos: position{line: 249, col: 18, offset: 7597},
				exprs: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 249, col: 18, offset: 7597},
						name: "ClassChar",
					},
					&litMatcher{
						pos:        position{line: 249, col: 28, offset: 7607},
						val:        "-",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 249, col: 32, offset: 7611},
						name: "ClassChar",
					},
				},
			},
		},
		{
			name: "ClassChar",
			pos:  position{line: 250, col: 1, offset: 7621},
			expr: &choiceExpr{
				pos: position{line: 250, col: 13, offset: 7635},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 250, col: 13, offset: 7635},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 250, col: 13, offset: 7635},
								expr: &choiceExpr{
									pos: position{line: 250, col: 16, offset: 7638},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 250, col: 16, offset: 7638},
											val:        "]",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 250, col: 22, offset: 7644},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 250, col: 29, offset: 7651},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 250, col: 35, offset: 7657},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 250, col: 48, offset: 7670},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 250, col: 48, offset: 7670},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 250, col: 53, offset: 7675},
								name: "CharClassEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "CharClassEscape",
			pos:  position{line: 251, col: 1, offset: 7691},
			expr: &choiceExpr{
				pos: position{line: 251, col: 19, offset: 7711},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 251, col: 21, offset: 7713},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 251, col: 21, offset: 7713},
								val:        "]",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 251, col: 27, offset: 7719},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 252, col: 7, offset: 7748},
						run: (*parser).callonCharClassEscape5,
						expr: &seqExpr{
							pos: position{line: 252, col: 7, offset: 7748},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 252, col: 7, offset: 7748},
									expr: &litMatcher{
										pos:        position{line: 252, col: 8, offset: 7749},
										val:        "p",
										ignoreCase: false,
									},
								},
								&choiceExpr{
									pos: position{line: 252, col: 14, offset: 7755},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 252, col: 14, offset: 7755},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 27, offset: 7768},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 33, offset: 7774},
											name: "EOF",
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
			name: "UnicodeClassEscape",
			pos:  position{line: 256, col: 1, offset: 7840},
			expr: &seqExpr{
				pos: position{line: 256, col: 22, offset: 7863},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 256, col: 22, offset: 7863},
						val:        "p",
						ignoreCase: false,
					},
					&choiceExpr{
						pos: position{line: 257, col: 7, offset: 7876},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 257, col: 7, offset: 7876},
								name: "SingleCharUnicodeClass",
							},
							&actionExpr{
								pos: position{line: 258, col: 7, offset: 7905},
								run: (*parser).callonUnicodeClassEscape5,
								expr: &seqExpr{
									pos: position{line: 258, col: 7, offset: 7905},
									exprs: []interface{}{
										&notExpr{
											pos: position{line: 258, col: 7, offset: 7905},
											expr: &litMatcher{
												pos:        position{line: 258, col: 8, offset: 7906},
												val:        "{",
												ignoreCase: false,
											},
										},
										&choiceExpr{
											pos: position{line: 258, col: 14, offset: 7912},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 258, col: 14, offset: 7912},
													name: "SourceChar",
												},
												&ruleRefExpr{
													pos:  position{line: 258, col: 27, offset: 7925},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 258, col: 33, offset: 7931},
													name: "EOF",
												},
											},
										},
									},
								},
							},
							&actionExpr{
								pos: position{line: 259, col: 7, offset: 8002},
								run: (*parser).callonUnicodeClassEscape13,
								expr: &seqExpr{
									pos: position{line: 259, col: 7, offset: 8002},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 259, col: 7, offset: 8002},
											val:        "{",
											ignoreCase: false,
										},
										&labeledExpr{
											pos:   position{line: 259, col: 11, offset: 8006},
											label: "ident",
											expr: &ruleRefExpr{
												pos:  position{line: 259, col: 17, offset: 8012},
												name: "IdentifierName",
											},
										},
										&litMatcher{
											pos:        position{line: 259, col: 32, offset: 8027},
											val:        "}",
											ignoreCase: false,
										},
									},
								},
							},
							&actionExpr{
								pos: position{line: 265, col: 7, offset: 8204},
								run: (*parser).callonUnicodeClassEscape19,
								expr: &seqExpr{
									pos: position{line: 265, col: 7, offset: 8204},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 265, col: 7, offset: 8204},
											val:        "{",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 265, col: 11, offset: 8208},
											name: "IdentifierName",
										},
										&choiceExpr{
											pos: position{line: 265, col: 28, offset: 8225},
											alternatives: []interface{}{
												&litMatcher{
													pos:        position{line: 265, col: 28, offset: 8225},
													val:        "]",
													ignoreCase: false,
												},
												&ruleRefExpr{
													pos:  position{line: 265, col: 34, offset: 8231},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 265, col: 40, offset: 8237},
													name: "EOF",
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
			name: "SingleCharUnicodeClass",
			pos:  position{line: 269, col: 1, offset: 8320},
			expr: &charClassMatcher{
				pos:        position{line: 269, col: 26, offset: 8347},
				val:        "[LMNCPZS]",
				chars:      []rune{'L', 'M', 'N', 'C', 'P', 'Z', 'S'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "AnyMatcher",
			pos:  position{line: 271, col: 1, offset: 8358},
			expr: &actionExpr{
				pos: position{line: 271, col: 14, offset: 8373},
				run: (*parser).callonAnyMatcher1,
				expr: &litMatcher{
					pos:        position{line: 271, col: 14, offset: 8373},
					val:        ".",
					ignoreCase: false,
				},
			},
		},
		{
			name: "CodeBlock",
			pos:  position{line: 276, col: 1, offset: 8448},
			expr: &choiceExpr{
				pos: position{line: 276, col: 13, offset: 8462},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 276, col: 13, offset: 8462},
						run: (*parser).callonCodeBlock2,
						expr: &seqExpr{
							pos: position{line: 276, col: 13, offset: 8462},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 276, col: 13, offset: 8462},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 276, col: 17, offset: 8466},
									name: "Code",
								},
								&litMatcher{
									pos:        position{line: 276, col: 22, offset: 8471},
									val:        "}",
									ignoreCase: false,
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 280, col: 5, offset: 8570},
						run: (*parser).callonCodeBlock7,
						expr: &seqExpr{
							pos: position{line: 280, col: 5, offset: 8570},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 280, col: 5, offset: 8570},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 280, col: 9, offset: 8574},
									name: "Code",
								},
								&ruleRefExpr{
									pos:  position{line: 280, col: 14, offset: 8579},
									name: "EOF",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Code",
			pos:  position{line: 284, col: 1, offset: 8644},
			expr: &zeroOrMoreExpr{
				pos: position{line: 284, col: 8, offset: 8653},
				expr: &choiceExpr{
					pos: position{line: 284, col: 10, offset: 8655},
					alternatives: []interface{}{
						&oneOrMoreExpr{
							pos: position{line: 284, col: 10, offset: 8655},
							expr: &seqExpr{
								pos: position{line: 284, col: 12, offset: 8657},
								exprs: []interface{}{
									&notExpr{
										pos: position{line: 284, col: 12, offset: 8657},
										expr: &charClassMatcher{
											pos:        position{line: 284, col: 13, offset: 8658},
											val:        "[{}]",
											chars:      []rune{'{', '}'},
											ignoreCase: false,
											inverted:   false,
										},
									},
									&ruleRefExpr{
										pos:  position{line: 284, col: 18, offset: 8663},
										name: "SourceChar",
									},
								},
							},
						},
						&seqExpr{
							pos: position{line: 284, col: 34, offset: 8679},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 284, col: 34, offset: 8679},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 284, col: 38, offset: 8683},
									name: "Code",
								},
								&litMatcher{
									pos:        position{line: 284, col: 43, offset: 8688},
									val:        "}",
									ignoreCase: false,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "__",
			pos:  position{line: 286, col: 1, offset: 8696},
			expr: &zeroOrMoreExpr{
				pos: position{line: 286, col: 6, offset: 8703},
				expr: &choiceExpr{
					pos: position{line: 286, col: 8, offset: 8705},
					alternatives: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 286, col: 8, offset: 8705},
							name: "Whitespace",
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 21, offset: 8718},
							name: "EOL",
						},
						&ruleRefExpr{
							pos:  position{line: 286, col: 27, offset: 8724},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "_",
			pos:  position{line: 287, col: 1, offset: 8735},
			expr: &zeroOrMoreExpr{
				pos: position{line: 287, col: 5, offset: 8741},
				expr: &choiceExpr{
					pos: position{line: 287, col: 7, offset: 8743},
					alternatives: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 287, col: 7, offset: 8743},
							name: "Whitespace",
						},
						&ruleRefExpr{
							pos:  position{line: 287, col: 20, offset: 8756},
							name: "MultiLineCommentNoLineTerminator",
						},
					},
				},
			},
		},
		{
			name: "Whitespace",
			pos:  position{line: 289, col: 1, offset: 8793},
			expr: &charClassMatcher{
				pos:        position{line: 289, col: 14, offset: 8808},
				val:        "[ \\t\\r]",
				chars:      []rune{' ', '\t', '\r'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EOL",
			pos:  position{line: 290, col: 1, offset: 8816},
			expr: &litMatcher{
				pos:        position{line: 290, col: 7, offset: 8824},
				val:        "\n",
				ignoreCase: false,
			},
		},
		{
			name: "EOS",
			pos:  position{line: 291, col: 1, offset: 8829},
			expr: &choiceExpr{
				pos: position{line: 291, col: 7, offset: 8837},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 291, col: 7, offset: 8837},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 291, col: 7, offset: 8837},
								name: "__",
							},
							&litMatcher{
								pos:        position{line: 291, col: 10, offset: 8840},
								val:        ";",
								ignoreCase: false,
							},
						},
					},
					&seqExpr{
						pos: position{line: 291, col: 16, offset: 8846},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 291, col: 16, offset: 8846},
								name: "_",
							},
							&zeroOrOneExpr{
								pos: position{line: 291, col: 18, offset: 8848},
								expr: &ruleRefExpr{
									pos:  position{line: 291, col: 18, offset: 8848},
									name: "SingleLineComment",
								},
							},
							&ruleRefExpr{
								pos:  position{line: 291, col: 37, offset: 8867},
								name: "EOL",
							},
						},
					},
					&seqExpr{
						pos: position{line: 291, col: 43, offset: 8873},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 291, col: 43, offset: 8873},
								name: "__",
							},
							&ruleRefExpr{
								pos:  position{line: 291, col: 46, offset: 8876},
								name: "EOF",
							},
						},
					},
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 293, col: 1, offset: 8881},
			expr: &notExpr{
				pos: position{line: 293, col: 7, offset: 8889},
				expr: &anyMatcher{
					line: 293, col: 8, offset: 8890,
				},
			},
		},
	},
}

func (c *current) onGrammar1(initializer, rules interface{}) (interface{}, error) {
	pos := c.astPos()

	// create the grammar, assign its initializer
	g := ast.NewGrammar(pos)
	initSlice := toIfaceSlice(initializer)
	if len(initSlice) > 0 {
		g.Init = initSlice[0].(*ast.CodeBlock)
	}

	rulesSlice := toIfaceSlice(rules)
	g.Rules = make([]*ast.Rule, len(rulesSlice))
	for i, duo := range rulesSlice {
		g.Rules[i] = duo.([]interface{})[0].(*ast.Rule)
	}

	return g, nil
}

func (p *parser) callonGrammar1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onGrammar1(stack["initializer"], stack["rules"])
}

func (c *current) onInitializer1(code interface{}) (interface{}, error) {
	return code, nil
}

func (p *parser) callonInitializer1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInitializer1(stack["code"])
}

func (c *current) onRule1(name, display, expr interface{}) (interface{}, error) {
	pos := c.astPos()

	rule := ast.NewRule(pos, name.(*ast.Identifier))
	displaySlice := toIfaceSlice(display)
	if len(displaySlice) > 0 {
		rule.DisplayName = displaySlice[0].(*ast.StringLit)
	}
	rule.Expr = expr.(ast.Expression)

	return rule, nil
}

func (p *parser) callonRule1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRule1(stack["name"], stack["display"], stack["expr"])
}

func (c *current) onChoiceExpr1(first, rest interface{}) (interface{}, error) {
	restSlice := toIfaceSlice(rest)
	if len(restSlice) == 0 {
		return first, nil
	}

	pos := c.astPos()
	choice := ast.NewChoiceExpr(pos)
	choice.Alternatives = []ast.Expression{first.(ast.Expression)}
	for _, sl := range restSlice {
		choice.Alternatives = append(choice.Alternatives, sl.([]interface{})[3].(ast.Expression))
	}
	return choice, nil
}

func (p *parser) callonChoiceExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onChoiceExpr1(stack["first"], stack["rest"])
}

func (c *current) onActionExpr1(expr, code interface{}) (interface{}, error) {
	if code == nil {
		return expr, nil
	}

	pos := c.astPos()
	act := ast.NewActionExpr(pos)
	act.Expr = expr.(ast.Expression)
	codeSlice := toIfaceSlice(code)
	act.Code = codeSlice[1].(*ast.CodeBlock)

	return act, nil
}

func (p *parser) callonActionExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onActionExpr1(stack["expr"], stack["code"])
}

func (c *current) onSeqExpr1(first, rest interface{}) (interface{}, error) {
	restSlice := toIfaceSlice(rest)
	if len(restSlice) == 0 {
		return first, nil
	}
	seq := ast.NewSeqExpr(c.astPos())
	seq.Exprs = []ast.Expression{first.(ast.Expression)}
	for _, sl := range restSlice {
		seq.Exprs = append(seq.Exprs, sl.([]interface{})[1].(ast.Expression))
	}
	return seq, nil
}

func (p *parser) callonSeqExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSeqExpr1(stack["first"], stack["rest"])
}

func (c *current) onLabeledExpr2(label, expr interface{}) (interface{}, error) {
	pos := c.astPos()
	lab := ast.NewLabeledExpr(pos)
	lab.Label = label.(*ast.Identifier)
	lab.Expr = expr.(ast.Expression)
	return lab, nil
}

func (p *parser) callonLabeledExpr2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLabeledExpr2(stack["label"], stack["expr"])
}

func (c *current) onPrefixedExpr2(op, expr interface{}) (interface{}, error) {
	pos := c.astPos()
	opStr := op.(string)
	if opStr == "&" {
		and := ast.NewAndExpr(pos)
		and.Expr = expr.(ast.Expression)
		return and, nil
	}
	not := ast.NewNotExpr(pos)
	not.Expr = expr.(ast.Expression)
	return not, nil
}

func (p *parser) callonPrefixedExpr2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPrefixedExpr2(stack["op"], stack["expr"])
}

func (c *current) onPrefixedOp1() (interface{}, error) {
	return string(c.text), nil
}

func (p *parser) callonPrefixedOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPrefixedOp1()
}

func (c *current) onSuffixedExpr2(expr, op interface{}) (interface{}, error) {
	pos := c.astPos()
	opStr := op.(string)
	switch opStr {
	case "?":
		zero := ast.NewZeroOrOneExpr(pos)
		zero.Expr = expr.(ast.Expression)
		return zero, nil
	case "*":
		zero := ast.NewZeroOrMoreExpr(pos)
		zero.Expr = expr.(ast.Expression)
		return zero, nil
	case "+":
		one := ast.NewOneOrMoreExpr(pos)
		one.Expr = expr.(ast.Expression)
		return one, nil
	default:
		return nil, errors.New("unknown operator: " + opStr)
	}
}

func (p *parser) callonSuffixedExpr2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSuffixedExpr2(stack["expr"], stack["op"])
}

func (c *current) onSuffixedOp1() (interface{}, error) {
	return string(c.text), nil
}

func (p *parser) callonSuffixedOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSuffixedOp1()
}

func (c *current) onPrimaryExpr7(expr interface{}) (interface{}, error) {
	return expr, nil
}

func (p *parser) callonPrimaryExpr7() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onPrimaryExpr7(stack["expr"])
}

func (c *current) onRuleRefExpr1(name interface{}) (interface{}, error) {
	ref := ast.NewRuleRefExpr(c.astPos())
	ref.Name = name.(*ast.Identifier)
	return ref, nil
}

func (p *parser) callonRuleRefExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRuleRefExpr1(stack["name"])
}

func (c *current) onSemanticPredExpr1(op, code interface{}) (interface{}, error) {
	opStr := op.(string)
	if opStr == "&" {
		and := ast.NewAndCodeExpr(c.astPos())
		and.Code = code.(*ast.CodeBlock)
		return and, nil
	}
	not := ast.NewNotCodeExpr(c.astPos())
	not.Code = code.(*ast.CodeBlock)
	return not, nil
}

func (p *parser) callonSemanticPredExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSemanticPredExpr1(stack["op"], stack["code"])
}

func (c *current) onSemanticPredOp1() (interface{}, error) {
	return string(c.text), nil
}

func (p *parser) callonSemanticPredOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSemanticPredOp1()
}

func (c *current) onIdentifier1(ident interface{}) (interface{}, error) {
	astIdent := ast.NewIdentifier(c.astPos(), string(c.text))
	if reservedWords[astIdent.Val] {
		return astIdent, errors.New("identifier is a reserved word")
	}
	return astIdent, nil
}

func (p *parser) callonIdentifier1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onIdentifier1(stack["ident"])
}

func (c *current) onIdentifierName1() (interface{}, error) {
	return ast.NewIdentifier(c.astPos(), string(c.text)), nil
}

func (p *parser) callonIdentifierName1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onIdentifierName1()
}

func (c *current) onLitMatcher1(lit, ignore interface{}) (interface{}, error) {
	rawStr := lit.(*ast.StringLit).Val
	s, err := strconv.Unquote(rawStr)
	if err != nil {
		// an invalid string literal raises an error in the escape rules,
		// so simply replace the literal with an empty string here to
		// avoid a cascade of errors.
		s = ""
	}
	m := ast.NewLitMatcher(c.astPos(), s)
	m.IgnoreCase = ignore != nil
	return m, nil
}

func (p *parser) callonLitMatcher1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLitMatcher1(stack["lit"], stack["ignore"])
}

func (c *current) onStringLiteral2() (interface{}, error) {
	return ast.NewStringLit(c.astPos(), string(c.text)), nil
}

func (p *parser) callonStringLiteral2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onStringLiteral2()
}

func (c *current) onStringLiteral18() (interface{}, error) {
	return ast.NewStringLit(c.astPos(), "``"), errors.New("string literal not terminated")
}

func (p *parser) callonStringLiteral18() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onStringLiteral18()
}

func (c *current) onDoubleStringEscape5() (interface{}, error) {
	return nil, errors.New("invalid escape character")
}

func (p *parser) callonDoubleStringEscape5() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onDoubleStringEscape5()
}

func (c *current) onSingleStringEscape5() (interface{}, error) {
	return nil, errors.New("invalid escape character")
}

func (p *parser) callonSingleStringEscape5() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSingleStringEscape5()
}

func (c *current) onOctalEscape6() (interface{}, error) {
	return nil, errors.New("invalid octal escape")
}

func (p *parser) callonOctalEscape6() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onOctalEscape6()
}

func (c *current) onHexEscape6() (interface{}, error) {
	return nil, errors.New("invalid hexadecimal escape")
}

func (p *parser) callonHexEscape6() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onHexEscape6()
}

func (c *current) onLongUnicodeEscape2() (interface{}, error) {
	return validateUnicodeEscape(string(c.text), "invalid Unicode escape")

}

func (p *parser) callonLongUnicodeEscape2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLongUnicodeEscape2()
}

func (c *current) onLongUnicodeEscape13() (interface{}, error) {
	return nil, errors.New("invalid Unicode escape")
}

func (p *parser) callonLongUnicodeEscape13() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLongUnicodeEscape13()
}

func (c *current) onShortUnicodeEscape2() (interface{}, error) {
	return validateUnicodeEscape(string(c.text), "invalid Unicode escape")

}

func (p *parser) callonShortUnicodeEscape2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onShortUnicodeEscape2()
}

func (c *current) onShortUnicodeEscape9() (interface{}, error) {
	return nil, errors.New("invalid Unicode escape")
}

func (p *parser) callonShortUnicodeEscape9() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onShortUnicodeEscape9()
}

func (c *current) onCharClassMatcher2() (interface{}, error) {
	pos := c.astPos()
	cc := ast.NewCharClassMatcher(pos, string(c.text))
	return cc, nil
}

func (p *parser) callonCharClassMatcher2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCharClassMatcher2()
}

func (c *current) onCharClassMatcher15() (interface{}, error) {
	return ast.NewCharClassMatcher(c.astPos(), "[]"), errors.New("character class not terminated")
}

func (p *parser) callonCharClassMatcher15() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCharClassMatcher15()
}

func (c *current) onCharClassEscape5() (interface{}, error) {
	return nil, errors.New("invalid escape character")
}

func (p *parser) callonCharClassEscape5() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCharClassEscape5()
}

func (c *current) onUnicodeClassEscape5() (interface{}, error) {
	return nil, errors.New("invalid Unicode class escape")
}

func (p *parser) callonUnicodeClassEscape5() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onUnicodeClassEscape5()
}

func (c *current) onUnicodeClassEscape13(ident interface{}) (interface{}, error) {
	if !unicodeClasses[ident.(*ast.Identifier).Val] {
		return nil, errors.New("invalid Unicode class escape")
	}
	return nil, nil

}

func (p *parser) callonUnicodeClassEscape13() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onUnicodeClassEscape13(stack["ident"])
}

func (c *current) onUnicodeClassEscape19() (interface{}, error) {
	return nil, errors.New("Unicode class not terminated")

}

func (p *parser) callonUnicodeClassEscape19() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onUnicodeClassEscape19()
}

func (c *current) onAnyMatcher1() (interface{}, error) {
	any := ast.NewAnyMatcher(c.astPos(), ".")
	return any, nil
}

func (p *parser) callonAnyMatcher1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onAnyMatcher1()
}

func (c *current) onCodeBlock2() (interface{}, error) {
	pos := c.astPos()
	cb := ast.NewCodeBlock(pos, string(c.text))
	return cb, nil
}

func (p *parser) callonCodeBlock2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCodeBlock2()
}

func (c *current) onCodeBlock7() (interface{}, error) {
	return nil, errors.New("code block not terminated")
}

func (p *parser) callonCodeBlock7() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCodeBlock7()
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
	pe := &parserError{Inner: err, prefix: buf.String()}
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

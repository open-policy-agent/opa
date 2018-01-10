package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mna/pigeon/ast"
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
				name: "RecoveryExpr",
			},
		},
		{
			name: "RecoveryExpr",
			pos:  position{line: 43, col: 1, offset: 990},
			expr: &actionExpr{
				pos: position{line: 43, col: 16, offset: 1007},
				run: (*parser).callonRecoveryExpr1,
				expr: &seqExpr{
					pos: position{line: 43, col: 16, offset: 1007},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 43, col: 16, offset: 1007},
							label: "expr",
							expr: &ruleRefExpr{
								pos:  position{line: 43, col: 21, offset: 1012},
								name: "ChoiceExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 43, col: 32, offset: 1023},
							label: "recoverExprs",
							expr: &zeroOrMoreExpr{
								pos: position{line: 43, col: 45, offset: 1036},
								expr: &seqExpr{
									pos: position{line: 43, col: 47, offset: 1038},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 43, col: 47, offset: 1038},
											name: "__",
										},
										&litMatcher{
											pos:        position{line: 43, col: 50, offset: 1041},
											val:        "//{",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 56, offset: 1047},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 59, offset: 1050},
											name: "Labels",
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 66, offset: 1057},
											name: "__",
										},
										&litMatcher{
											pos:        position{line: 43, col: 69, offset: 1060},
											val:        "}",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 73, offset: 1064},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 43, col: 76, offset: 1067},
											name: "ChoiceExpr",
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
			name: "Labels",
			pos:  position{line: 58, col: 1, offset: 1481},
			expr: &actionExpr{
				pos: position{line: 58, col: 10, offset: 1492},
				run: (*parser).callonLabels1,
				expr: &seqExpr{
					pos: position{line: 58, col: 10, offset: 1492},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 58, col: 10, offset: 1492},
							label: "label",
							expr: &ruleRefExpr{
								pos:  position{line: 58, col: 16, offset: 1498},
								name: "IdentifierName",
							},
						},
						&labeledExpr{
							pos:   position{line: 58, col: 31, offset: 1513},
							label: "labels",
							expr: &zeroOrMoreExpr{
								pos: position{line: 58, col: 38, offset: 1520},
								expr: &seqExpr{
									pos: position{line: 58, col: 40, offset: 1522},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 58, col: 40, offset: 1522},
											name: "__",
										},
										&litMatcher{
											pos:        position{line: 58, col: 43, offset: 1525},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 58, col: 47, offset: 1529},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 58, col: 50, offset: 1532},
											name: "IdentifierName",
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
			name: "ChoiceExpr",
			pos:  position{line: 67, col: 1, offset: 1861},
			expr: &actionExpr{
				pos: position{line: 67, col: 14, offset: 1876},
				run: (*parser).callonChoiceExpr1,
				expr: &seqExpr{
					pos: position{line: 67, col: 14, offset: 1876},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 67, col: 14, offset: 1876},
							label: "first",
							expr: &ruleRefExpr{
								pos:  position{line: 67, col: 20, offset: 1882},
								name: "ActionExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 67, col: 31, offset: 1893},
							label: "rest",
							expr: &zeroOrMoreExpr{
								pos: position{line: 67, col: 36, offset: 1898},
								expr: &seqExpr{
									pos: position{line: 67, col: 38, offset: 1900},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 67, col: 38, offset: 1900},
											name: "__",
										},
										&litMatcher{
											pos:        position{line: 67, col: 41, offset: 1903},
											val:        "/",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 67, col: 45, offset: 1907},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 67, col: 48, offset: 1910},
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
			pos:  position{line: 82, col: 1, offset: 2315},
			expr: &actionExpr{
				pos: position{line: 82, col: 14, offset: 2330},
				run: (*parser).callonActionExpr1,
				expr: &seqExpr{
					pos: position{line: 82, col: 14, offset: 2330},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 82, col: 14, offset: 2330},
							label: "expr",
							expr: &ruleRefExpr{
								pos:  position{line: 82, col: 19, offset: 2335},
								name: "SeqExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 82, col: 27, offset: 2343},
							label: "code",
							expr: &zeroOrOneExpr{
								pos: position{line: 82, col: 32, offset: 2348},
								expr: &seqExpr{
									pos: position{line: 82, col: 34, offset: 2350},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 82, col: 34, offset: 2350},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 82, col: 37, offset: 2353},
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
			pos:  position{line: 96, col: 1, offset: 2619},
			expr: &actionExpr{
				pos: position{line: 96, col: 11, offset: 2631},
				run: (*parser).callonSeqExpr1,
				expr: &seqExpr{
					pos: position{line: 96, col: 11, offset: 2631},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 96, col: 11, offset: 2631},
							label: "first",
							expr: &ruleRefExpr{
								pos:  position{line: 96, col: 17, offset: 2637},
								name: "LabeledExpr",
							},
						},
						&labeledExpr{
							pos:   position{line: 96, col: 29, offset: 2649},
							label: "rest",
							expr: &zeroOrMoreExpr{
								pos: position{line: 96, col: 34, offset: 2654},
								expr: &seqExpr{
									pos: position{line: 96, col: 36, offset: 2656},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 96, col: 36, offset: 2656},
											name: "__",
										},
										&ruleRefExpr{
											pos:  position{line: 96, col: 39, offset: 2659},
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
			pos:  position{line: 109, col: 1, offset: 3010},
			expr: &choiceExpr{
				pos: position{line: 109, col: 15, offset: 3026},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 109, col: 15, offset: 3026},
						run: (*parser).callonLabeledExpr2,
						expr: &seqExpr{
							pos: position{line: 109, col: 15, offset: 3026},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 109, col: 15, offset: 3026},
									label: "label",
									expr: &ruleRefExpr{
										pos:  position{line: 109, col: 21, offset: 3032},
										name: "Identifier",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 109, col: 32, offset: 3043},
									name: "__",
								},
								&litMatcher{
									pos:        position{line: 109, col: 35, offset: 3046},
									val:        ":",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 109, col: 39, offset: 3050},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 109, col: 42, offset: 3053},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 109, col: 47, offset: 3058},
										name: "PrefixedExpr",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 115, col: 5, offset: 3231},
						name: "PrefixedExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 115, col: 20, offset: 3246},
						name: "ThrowExpr",
					},
				},
			},
		},
		{
			name: "PrefixedExpr",
			pos:  position{line: 117, col: 1, offset: 3257},
			expr: &choiceExpr{
				pos: position{line: 117, col: 16, offset: 3274},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 117, col: 16, offset: 3274},
						run: (*parser).callonPrefixedExpr2,
						expr: &seqExpr{
							pos: position{line: 117, col: 16, offset: 3274},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 117, col: 16, offset: 3274},
									label: "op",
									expr: &ruleRefExpr{
										pos:  position{line: 117, col: 19, offset: 3277},
										name: "PrefixedOp",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 117, col: 30, offset: 3288},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 117, col: 33, offset: 3291},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 117, col: 38, offset: 3296},
										name: "SuffixedExpr",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 128, col: 5, offset: 3578},
						name: "SuffixedExpr",
					},
				},
			},
		},
		{
			name: "PrefixedOp",
			pos:  position{line: 130, col: 1, offset: 3592},
			expr: &actionExpr{
				pos: position{line: 130, col: 14, offset: 3607},
				run: (*parser).callonPrefixedOp1,
				expr: &choiceExpr{
					pos: position{line: 130, col: 16, offset: 3609},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 130, col: 16, offset: 3609},
							val:        "&",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 130, col: 22, offset: 3615},
							val:        "!",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SuffixedExpr",
			pos:  position{line: 134, col: 1, offset: 3657},
			expr: &choiceExpr{
				pos: position{line: 134, col: 16, offset: 3674},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 134, col: 16, offset: 3674},
						run: (*parser).callonSuffixedExpr2,
						expr: &seqExpr{
							pos: position{line: 134, col: 16, offset: 3674},
							exprs: []interface{}{
								&labeledExpr{
									pos:   position{line: 134, col: 16, offset: 3674},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 134, col: 21, offset: 3679},
										name: "PrimaryExpr",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 134, col: 33, offset: 3691},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 134, col: 36, offset: 3694},
									label: "op",
									expr: &ruleRefExpr{
										pos:  position{line: 134, col: 39, offset: 3697},
										name: "SuffixedOp",
									},
								},
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 153, col: 5, offset: 4227},
						name: "PrimaryExpr",
					},
				},
			},
		},
		{
			name: "SuffixedOp",
			pos:  position{line: 155, col: 1, offset: 4241},
			expr: &actionExpr{
				pos: position{line: 155, col: 14, offset: 4256},
				run: (*parser).callonSuffixedOp1,
				expr: &choiceExpr{
					pos: position{line: 155, col: 16, offset: 4258},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 155, col: 16, offset: 4258},
							val:        "?",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 155, col: 22, offset: 4264},
							val:        "*",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 155, col: 28, offset: 4270},
							val:        "+",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "PrimaryExpr",
			pos:  position{line: 159, col: 1, offset: 4312},
			expr: &choiceExpr{
				pos: position{line: 159, col: 15, offset: 4328},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 159, col: 15, offset: 4328},
						name: "LitMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 159, col: 28, offset: 4341},
						name: "CharClassMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 159, col: 47, offset: 4360},
						name: "AnyMatcher",
					},
					&ruleRefExpr{
						pos:  position{line: 159, col: 60, offset: 4373},
						name: "RuleRefExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 159, col: 74, offset: 4387},
						name: "SemanticPredExpr",
					},
					&actionExpr{
						pos: position{line: 159, col: 93, offset: 4406},
						run: (*parser).callonPrimaryExpr7,
						expr: &seqExpr{
							pos: position{line: 159, col: 93, offset: 4406},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 159, col: 93, offset: 4406},
									val:        "(",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 159, col: 97, offset: 4410},
									name: "__",
								},
								&labeledExpr{
									pos:   position{line: 159, col: 100, offset: 4413},
									label: "expr",
									expr: &ruleRefExpr{
										pos:  position{line: 159, col: 105, offset: 4418},
										name: "Expression",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 159, col: 116, offset: 4429},
									name: "__",
								},
								&litMatcher{
									pos:        position{line: 159, col: 119, offset: 4432},
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
			pos:  position{line: 162, col: 1, offset: 4461},
			expr: &actionExpr{
				pos: position{line: 162, col: 15, offset: 4477},
				run: (*parser).callonRuleRefExpr1,
				expr: &seqExpr{
					pos: position{line: 162, col: 15, offset: 4477},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 162, col: 15, offset: 4477},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 162, col: 20, offset: 4482},
								name: "IdentifierName",
							},
						},
						&notExpr{
							pos: position{line: 162, col: 35, offset: 4497},
							expr: &seqExpr{
								pos: position{line: 162, col: 38, offset: 4500},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 162, col: 38, offset: 4500},
										name: "__",
									},
									&zeroOrOneExpr{
										pos: position{line: 162, col: 41, offset: 4503},
										expr: &seqExpr{
											pos: position{line: 162, col: 43, offset: 4505},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 162, col: 43, offset: 4505},
													name: "StringLiteral",
												},
												&ruleRefExpr{
													pos:  position{line: 162, col: 57, offset: 4519},
													name: "__",
												},
											},
										},
									},
									&ruleRefExpr{
										pos:  position{line: 162, col: 63, offset: 4525},
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
			pos:  position{line: 167, col: 1, offset: 4641},
			expr: &actionExpr{
				pos: position{line: 167, col: 20, offset: 4662},
				run: (*parser).callonSemanticPredExpr1,
				expr: &seqExpr{
					pos: position{line: 167, col: 20, offset: 4662},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 167, col: 20, offset: 4662},
							label: "op",
							expr: &ruleRefExpr{
								pos:  position{line: 167, col: 23, offset: 4665},
								name: "SemanticPredOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 167, col: 38, offset: 4680},
							name: "__",
						},
						&labeledExpr{
							pos:   position{line: 167, col: 41, offset: 4683},
							label: "code",
							expr: &ruleRefExpr{
								pos:  position{line: 167, col: 46, offset: 4688},
								name: "CodeBlock",
							},
						},
					},
				},
			},
		},
		{
			name: "SemanticPredOp",
			pos:  position{line: 187, col: 1, offset: 5135},
			expr: &actionExpr{
				pos: position{line: 187, col: 18, offset: 5154},
				run: (*parser).callonSemanticPredOp1,
				expr: &choiceExpr{
					pos: position{line: 187, col: 20, offset: 5156},
					alternatives: []interface{}{
						&litMatcher{
							pos:        position{line: 187, col: 20, offset: 5156},
							val:        "#",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 187, col: 26, offset: 5162},
							val:        "&",
							ignoreCase: false,
						},
						&litMatcher{
							pos:        position{line: 187, col: 32, offset: 5168},
							val:        "!",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "RuleDefOp",
			pos:  position{line: 191, col: 1, offset: 5210},
			expr: &choiceExpr{
				pos: position{line: 191, col: 13, offset: 5224},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 191, col: 13, offset: 5224},
						val:        "=",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 191, col: 19, offset: 5230},
						val:        "<-",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 191, col: 26, offset: 5237},
						val:        "←",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 191, col: 37, offset: 5248},
						val:        "⟵",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "SourceChar",
			pos:  position{line: 193, col: 1, offset: 5258},
			expr: &anyMatcher{
				line: 193, col: 14, offset: 5273,
			},
		},
		{
			name: "Comment",
			pos:  position{line: 194, col: 1, offset: 5275},
			expr: &choiceExpr{
				pos: position{line: 194, col: 11, offset: 5287},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 194, col: 11, offset: 5287},
						name: "MultiLineComment",
					},
					&ruleRefExpr{
						pos:  position{line: 194, col: 30, offset: 5306},
						name: "SingleLineComment",
					},
				},
			},
		},
		{
			name: "MultiLineComment",
			pos:  position{line: 195, col: 1, offset: 5324},
			expr: &seqExpr{
				pos: position{line: 195, col: 20, offset: 5345},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 195, col: 20, offset: 5345},
						val:        "/*",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 195, col: 25, offset: 5350},
						expr: &seqExpr{
							pos: position{line: 195, col: 27, offset: 5352},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 195, col: 27, offset: 5352},
									expr: &litMatcher{
										pos:        position{line: 195, col: 28, offset: 5353},
										val:        "*/",
										ignoreCase: false,
									},
								},
								&ruleRefExpr{
									pos:  position{line: 195, col: 33, offset: 5358},
									name: "SourceChar",
								},
							},
						},
					},
					&litMatcher{
						pos:        position{line: 195, col: 47, offset: 5372},
						val:        "*/",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "MultiLineCommentNoLineTerminator",
			pos:  position{line: 196, col: 1, offset: 5377},
			expr: &seqExpr{
				pos: position{line: 196, col: 36, offset: 5414},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 196, col: 36, offset: 5414},
						val:        "/*",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 196, col: 41, offset: 5419},
						expr: &seqExpr{
							pos: position{line: 196, col: 43, offset: 5421},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 196, col: 43, offset: 5421},
									expr: &choiceExpr{
										pos: position{line: 196, col: 46, offset: 5424},
										alternatives: []interface{}{
											&litMatcher{
												pos:        position{line: 196, col: 46, offset: 5424},
												val:        "*/",
												ignoreCase: false,
											},
											&ruleRefExpr{
												pos:  position{line: 196, col: 53, offset: 5431},
												name: "EOL",
											},
										},
									},
								},
								&ruleRefExpr{
									pos:  position{line: 196, col: 59, offset: 5437},
									name: "SourceChar",
								},
							},
						},
					},
					&litMatcher{
						pos:        position{line: 196, col: 73, offset: 5451},
						val:        "*/",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "SingleLineComment",
			pos:  position{line: 197, col: 1, offset: 5456},
			expr: &seqExpr{
				pos: position{line: 197, col: 21, offset: 5478},
				exprs: []interface{}{
					&notExpr{
						pos: position{line: 197, col: 21, offset: 5478},
						expr: &litMatcher{
							pos:        position{line: 197, col: 23, offset: 5480},
							val:        "//{",
							ignoreCase: false,
						},
					},
					&litMatcher{
						pos:        position{line: 197, col: 30, offset: 5487},
						val:        "//",
						ignoreCase: false,
					},
					&zeroOrMoreExpr{
						pos: position{line: 197, col: 35, offset: 5492},
						expr: &seqExpr{
							pos: position{line: 197, col: 37, offset: 5494},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 197, col: 37, offset: 5494},
									expr: &ruleRefExpr{
										pos:  position{line: 197, col: 38, offset: 5495},
										name: "EOL",
									},
								},
								&ruleRefExpr{
									pos:  position{line: 197, col: 42, offset: 5499},
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
			pos:  position{line: 199, col: 1, offset: 5514},
			expr: &actionExpr{
				pos: position{line: 199, col: 14, offset: 5529},
				run: (*parser).callonIdentifier1,
				expr: &labeledExpr{
					pos:   position{line: 199, col: 14, offset: 5529},
					label: "ident",
					expr: &ruleRefExpr{
						pos:  position{line: 199, col: 20, offset: 5535},
						name: "IdentifierName",
					},
				},
			},
		},
		{
			name: "IdentifierName",
			pos:  position{line: 207, col: 1, offset: 5754},
			expr: &actionExpr{
				pos: position{line: 207, col: 18, offset: 5773},
				run: (*parser).callonIdentifierName1,
				expr: &seqExpr{
					pos: position{line: 207, col: 18, offset: 5773},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 207, col: 18, offset: 5773},
							name: "IdentifierStart",
						},
						&zeroOrMoreExpr{
							pos: position{line: 207, col: 34, offset: 5789},
							expr: &ruleRefExpr{
								pos:  position{line: 207, col: 34, offset: 5789},
								name: "IdentifierPart",
							},
						},
					},
				},
			},
		},
		{
			name: "IdentifierStart",
			pos:  position{line: 210, col: 1, offset: 5871},
			expr: &charClassMatcher{
				pos:        position{line: 210, col: 19, offset: 5891},
				val:        "[\\pL_]",
				chars:      []rune{'_'},
				classes:    []*unicode.RangeTable{rangeTable("L")},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "IdentifierPart",
			pos:  position{line: 211, col: 1, offset: 5898},
			expr: &choiceExpr{
				pos: position{line: 211, col: 18, offset: 5917},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 211, col: 18, offset: 5917},
						name: "IdentifierStart",
					},
					&charClassMatcher{
						pos:        position{line: 211, col: 36, offset: 5935},
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
			pos:  position{line: 213, col: 1, offset: 5945},
			expr: &actionExpr{
				pos: position{line: 213, col: 14, offset: 5960},
				run: (*parser).callonLitMatcher1,
				expr: &seqExpr{
					pos: position{line: 213, col: 14, offset: 5960},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 213, col: 14, offset: 5960},
							label: "lit",
							expr: &ruleRefExpr{
								pos:  position{line: 213, col: 18, offset: 5964},
								name: "StringLiteral",
							},
						},
						&labeledExpr{
							pos:   position{line: 213, col: 32, offset: 5978},
							label: "ignore",
							expr: &zeroOrOneExpr{
								pos: position{line: 213, col: 39, offset: 5985},
								expr: &litMatcher{
									pos:        position{line: 213, col: 39, offset: 5985},
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
			pos:  position{line: 226, col: 1, offset: 6384},
			expr: &choiceExpr{
				pos: position{line: 226, col: 17, offset: 6402},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 226, col: 17, offset: 6402},
						run: (*parser).callonStringLiteral2,
						expr: &choiceExpr{
							pos: position{line: 226, col: 19, offset: 6404},
							alternatives: []interface{}{
								&seqExpr{
									pos: position{line: 226, col: 19, offset: 6404},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 226, col: 19, offset: 6404},
											val:        "\"",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 226, col: 23, offset: 6408},
											expr: &ruleRefExpr{
												pos:  position{line: 226, col: 23, offset: 6408},
												name: "DoubleStringChar",
											},
										},
										&litMatcher{
											pos:        position{line: 226, col: 41, offset: 6426},
											val:        "\"",
											ignoreCase: false,
										},
									},
								},
								&seqExpr{
									pos: position{line: 226, col: 47, offset: 6432},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 226, col: 47, offset: 6432},
											val:        "'",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 226, col: 51, offset: 6436},
											name: "SingleStringChar",
										},
										&litMatcher{
											pos:        position{line: 226, col: 68, offset: 6453},
											val:        "'",
											ignoreCase: false,
										},
									},
								},
								&seqExpr{
									pos: position{line: 226, col: 74, offset: 6459},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 226, col: 74, offset: 6459},
											val:        "`",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 226, col: 78, offset: 6463},
											expr: &ruleRefExpr{
												pos:  position{line: 226, col: 78, offset: 6463},
												name: "RawStringChar",
											},
										},
										&litMatcher{
											pos:        position{line: 226, col: 93, offset: 6478},
											val:        "`",
											ignoreCase: false,
										},
									},
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 228, col: 5, offset: 6551},
						run: (*parser).callonStringLiteral18,
						expr: &choiceExpr{
							pos: position{line: 228, col: 7, offset: 6553},
							alternatives: []interface{}{
								&seqExpr{
									pos: position{line: 228, col: 9, offset: 6555},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 228, col: 9, offset: 6555},
											val:        "\"",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 228, col: 13, offset: 6559},
											expr: &ruleRefExpr{
												pos:  position{line: 228, col: 13, offset: 6559},
												name: "DoubleStringChar",
											},
										},
										&choiceExpr{
											pos: position{line: 228, col: 33, offset: 6579},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 228, col: 33, offset: 6579},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 228, col: 39, offset: 6585},
													name: "EOF",
												},
											},
										},
									},
								},
								&seqExpr{
									pos: position{line: 228, col: 51, offset: 6597},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 228, col: 51, offset: 6597},
											val:        "'",
											ignoreCase: false,
										},
										&zeroOrOneExpr{
											pos: position{line: 228, col: 55, offset: 6601},
											expr: &ruleRefExpr{
												pos:  position{line: 228, col: 55, offset: 6601},
												name: "SingleStringChar",
											},
										},
										&choiceExpr{
											pos: position{line: 228, col: 75, offset: 6621},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 228, col: 75, offset: 6621},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 228, col: 81, offset: 6627},
													name: "EOF",
												},
											},
										},
									},
								},
								&seqExpr{
									pos: position{line: 228, col: 91, offset: 6637},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 228, col: 91, offset: 6637},
											val:        "`",
											ignoreCase: false,
										},
										&zeroOrMoreExpr{
											pos: position{line: 228, col: 95, offset: 6641},
											expr: &ruleRefExpr{
												pos:  position{line: 228, col: 95, offset: 6641},
												name: "RawStringChar",
											},
										},
										&ruleRefExpr{
											pos:  position{line: 228, col: 110, offset: 6656},
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
			pos:  position{line: 232, col: 1, offset: 6758},
			expr: &choiceExpr{
				pos: position{line: 232, col: 20, offset: 6779},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 232, col: 20, offset: 6779},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 232, col: 20, offset: 6779},
								expr: &choiceExpr{
									pos: position{line: 232, col: 23, offset: 6782},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 232, col: 23, offset: 6782},
											val:        "\"",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 232, col: 29, offset: 6788},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 232, col: 36, offset: 6795},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 232, col: 42, offset: 6801},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 232, col: 55, offset: 6814},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 232, col: 55, offset: 6814},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 232, col: 60, offset: 6819},
								name: "DoubleStringEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "SingleStringChar",
			pos:  position{line: 233, col: 1, offset: 6838},
			expr: &choiceExpr{
				pos: position{line: 233, col: 20, offset: 6859},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 233, col: 20, offset: 6859},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 233, col: 20, offset: 6859},
								expr: &choiceExpr{
									pos: position{line: 233, col: 23, offset: 6862},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 233, col: 23, offset: 6862},
											val:        "'",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 233, col: 29, offset: 6868},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 233, col: 36, offset: 6875},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 233, col: 42, offset: 6881},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 233, col: 55, offset: 6894},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 233, col: 55, offset: 6894},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 233, col: 60, offset: 6899},
								name: "SingleStringEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "RawStringChar",
			pos:  position{line: 234, col: 1, offset: 6918},
			expr: &seqExpr{
				pos: position{line: 234, col: 17, offset: 6936},
				exprs: []interface{}{
					&notExpr{
						pos: position{line: 234, col: 17, offset: 6936},
						expr: &litMatcher{
							pos:        position{line: 234, col: 18, offset: 6937},
							val:        "`",
							ignoreCase: false,
						},
					},
					&ruleRefExpr{
						pos:  position{line: 234, col: 22, offset: 6941},
						name: "SourceChar",
					},
				},
			},
		},
		{
			name: "DoubleStringEscape",
			pos:  position{line: 236, col: 1, offset: 6953},
			expr: &choiceExpr{
				pos: position{line: 236, col: 22, offset: 6976},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 236, col: 24, offset: 6978},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 236, col: 24, offset: 6978},
								val:        "\"",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 236, col: 30, offset: 6984},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 237, col: 7, offset: 7013},
						run: (*parser).callonDoubleStringEscape5,
						expr: &choiceExpr{
							pos: position{line: 237, col: 9, offset: 7015},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 237, col: 9, offset: 7015},
									name: "SourceChar",
								},
								&ruleRefExpr{
									pos:  position{line: 237, col: 22, offset: 7028},
									name: "EOL",
								},
								&ruleRefExpr{
									pos:  position{line: 237, col: 28, offset: 7034},
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
			pos:  position{line: 240, col: 1, offset: 7099},
			expr: &choiceExpr{
				pos: position{line: 240, col: 22, offset: 7122},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 240, col: 24, offset: 7124},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 240, col: 24, offset: 7124},
								val:        "'",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 240, col: 30, offset: 7130},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 241, col: 7, offset: 7159},
						run: (*parser).callonSingleStringEscape5,
						expr: &choiceExpr{
							pos: position{line: 241, col: 9, offset: 7161},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 241, col: 9, offset: 7161},
									name: "SourceChar",
								},
								&ruleRefExpr{
									pos:  position{line: 241, col: 22, offset: 7174},
									name: "EOL",
								},
								&ruleRefExpr{
									pos:  position{line: 241, col: 28, offset: 7180},
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
			pos:  position{line: 245, col: 1, offset: 7246},
			expr: &choiceExpr{
				pos: position{line: 245, col: 24, offset: 7271},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 245, col: 24, offset: 7271},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 245, col: 43, offset: 7290},
						name: "OctalEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 245, col: 57, offset: 7304},
						name: "HexEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 245, col: 69, offset: 7316},
						name: "LongUnicodeEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 245, col: 89, offset: 7336},
						name: "ShortUnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 246, col: 1, offset: 7355},
			expr: &choiceExpr{
				pos: position{line: 246, col: 20, offset: 7376},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 246, col: 20, offset: 7376},
						val:        "a",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 26, offset: 7382},
						val:        "b",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 32, offset: 7388},
						val:        "n",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 38, offset: 7394},
						val:        "f",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 44, offset: 7400},
						val:        "r",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 50, offset: 7406},
						val:        "t",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 56, offset: 7412},
						val:        "v",
						ignoreCase: false,
					},
					&litMatcher{
						pos:        position{line: 246, col: 62, offset: 7418},
						val:        "\\",
						ignoreCase: false,
					},
				},
			},
		},
		{
			name: "OctalEscape",
			pos:  position{line: 247, col: 1, offset: 7423},
			expr: &choiceExpr{
				pos: position{line: 247, col: 15, offset: 7439},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 247, col: 15, offset: 7439},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 247, col: 15, offset: 7439},
								name: "OctalDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 247, col: 26, offset: 7450},
								name: "OctalDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 247, col: 37, offset: 7461},
								name: "OctalDigit",
							},
						},
					},
					&actionExpr{
						pos: position{line: 248, col: 7, offset: 7478},
						run: (*parser).callonOctalEscape6,
						expr: &seqExpr{
							pos: position{line: 248, col: 7, offset: 7478},
							exprs: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 248, col: 7, offset: 7478},
									name: "OctalDigit",
								},
								&choiceExpr{
									pos: position{line: 248, col: 20, offset: 7491},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 248, col: 20, offset: 7491},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 33, offset: 7504},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 248, col: 39, offset: 7510},
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
			pos:  position{line: 251, col: 1, offset: 7571},
			expr: &choiceExpr{
				pos: position{line: 251, col: 13, offset: 7585},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 251, col: 13, offset: 7585},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 251, col: 13, offset: 7585},
								val:        "x",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 251, col: 17, offset: 7589},
								name: "HexDigit",
							},
							&ruleRefExpr{
								pos:  position{line: 251, col: 26, offset: 7598},
								name: "HexDigit",
							},
						},
					},
					&actionExpr{
						pos: position{line: 252, col: 7, offset: 7613},
						run: (*parser).callonHexEscape6,
						expr: &seqExpr{
							pos: position{line: 252, col: 7, offset: 7613},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 252, col: 7, offset: 7613},
									val:        "x",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 252, col: 13, offset: 7619},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 252, col: 13, offset: 7619},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 26, offset: 7632},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 252, col: 32, offset: 7638},
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
			pos:  position{line: 255, col: 1, offset: 7705},
			expr: &choiceExpr{
				pos: position{line: 256, col: 5, offset: 7732},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 256, col: 5, offset: 7732},
						run: (*parser).callonLongUnicodeEscape2,
						expr: &seqExpr{
							pos: position{line: 256, col: 5, offset: 7732},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 256, col: 5, offset: 7732},
									val:        "U",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 9, offset: 7736},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 18, offset: 7745},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 27, offset: 7754},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 36, offset: 7763},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 45, offset: 7772},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 54, offset: 7781},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 63, offset: 7790},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 256, col: 72, offset: 7799},
									name: "HexDigit",
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 259, col: 7, offset: 7901},
						run: (*parser).callonLongUnicodeEscape13,
						expr: &seqExpr{
							pos: position{line: 259, col: 7, offset: 7901},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 259, col: 7, offset: 7901},
									val:        "U",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 259, col: 13, offset: 7907},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 259, col: 13, offset: 7907},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 259, col: 26, offset: 7920},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 259, col: 32, offset: 7926},
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
			pos:  position{line: 262, col: 1, offset: 7989},
			expr: &choiceExpr{
				pos: position{line: 263, col: 5, offset: 8017},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 263, col: 5, offset: 8017},
						run: (*parser).callonShortUnicodeEscape2,
						expr: &seqExpr{
							pos: position{line: 263, col: 5, offset: 8017},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 263, col: 5, offset: 8017},
									val:        "u",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 263, col: 9, offset: 8021},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 263, col: 18, offset: 8030},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 263, col: 27, offset: 8039},
									name: "HexDigit",
								},
								&ruleRefExpr{
									pos:  position{line: 263, col: 36, offset: 8048},
									name: "HexDigit",
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 266, col: 7, offset: 8150},
						run: (*parser).callonShortUnicodeEscape9,
						expr: &seqExpr{
							pos: position{line: 266, col: 7, offset: 8150},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 266, col: 7, offset: 8150},
									val:        "u",
									ignoreCase: false,
								},
								&choiceExpr{
									pos: position{line: 266, col: 13, offset: 8156},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 266, col: 13, offset: 8156},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 266, col: 26, offset: 8169},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 266, col: 32, offset: 8175},
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
			pos:  position{line: 270, col: 1, offset: 8239},
			expr: &charClassMatcher{
				pos:        position{line: 270, col: 14, offset: 8254},
				val:        "[0-7]",
				ranges:     []rune{'0', '7'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 271, col: 1, offset: 8260},
			expr: &charClassMatcher{
				pos:        position{line: 271, col: 16, offset: 8277},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 272, col: 1, offset: 8283},
			expr: &charClassMatcher{
				pos:        position{line: 272, col: 12, offset: 8296},
				val:        "[0-9a-f]i",
				ranges:     []rune{'0', '9', 'a', 'f'},
				ignoreCase: true,
				inverted:   false,
			},
		},
		{
			name: "CharClassMatcher",
			pos:  position{line: 274, col: 1, offset: 8307},
			expr: &choiceExpr{
				pos: position{line: 274, col: 20, offset: 8328},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 274, col: 20, offset: 8328},
						run: (*parser).callonCharClassMatcher2,
						expr: &seqExpr{
							pos: position{line: 274, col: 20, offset: 8328},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 274, col: 20, offset: 8328},
									val:        "[",
									ignoreCase: false,
								},
								&zeroOrMoreExpr{
									pos: position{line: 274, col: 24, offset: 8332},
									expr: &choiceExpr{
										pos: position{line: 274, col: 26, offset: 8334},
										alternatives: []interface{}{
											&ruleRefExpr{
												pos:  position{line: 274, col: 26, offset: 8334},
												name: "ClassCharRange",
											},
											&ruleRefExpr{
												pos:  position{line: 274, col: 43, offset: 8351},
												name: "ClassChar",
											},
											&seqExpr{
												pos: position{line: 274, col: 55, offset: 8363},
												exprs: []interface{}{
													&litMatcher{
														pos:        position{line: 274, col: 55, offset: 8363},
														val:        "\\",
														ignoreCase: false,
													},
													&ruleRefExpr{
														pos:  position{line: 274, col: 60, offset: 8368},
														name: "UnicodeClassEscape",
													},
												},
											},
										},
									},
								},
								&litMatcher{
									pos:        position{line: 274, col: 82, offset: 8390},
									val:        "]",
									ignoreCase: false,
								},
								&zeroOrOneExpr{
									pos: position{line: 274, col: 86, offset: 8394},
									expr: &litMatcher{
										pos:        position{line: 274, col: 86, offset: 8394},
										val:        "i",
										ignoreCase: false,
									},
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 278, col: 5, offset: 8501},
						run: (*parser).callonCharClassMatcher15,
						expr: &seqExpr{
							pos: position{line: 278, col: 5, offset: 8501},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 278, col: 5, offset: 8501},
									val:        "[",
									ignoreCase: false,
								},
								&zeroOrMoreExpr{
									pos: position{line: 278, col: 9, offset: 8505},
									expr: &seqExpr{
										pos: position{line: 278, col: 11, offset: 8507},
										exprs: []interface{}{
											&notExpr{
												pos: position{line: 278, col: 11, offset: 8507},
												expr: &ruleRefExpr{
													pos:  position{line: 278, col: 14, offset: 8510},
													name: "EOL",
												},
											},
											&ruleRefExpr{
												pos:  position{line: 278, col: 20, offset: 8516},
												name: "SourceChar",
											},
										},
									},
								},
								&choiceExpr{
									pos: position{line: 278, col: 36, offset: 8532},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 278, col: 36, offset: 8532},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 278, col: 42, offset: 8538},
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
			pos:  position{line: 282, col: 1, offset: 8648},
			expr: &seqExpr{
				pos: position{line: 282, col: 18, offset: 8667},
				exprs: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 282, col: 18, offset: 8667},
						name: "ClassChar",
					},
					&litMatcher{
						pos:        position{line: 282, col: 28, offset: 8677},
						val:        "-",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 282, col: 32, offset: 8681},
						name: "ClassChar",
					},
				},
			},
		},
		{
			name: "ClassChar",
			pos:  position{line: 283, col: 1, offset: 8691},
			expr: &choiceExpr{
				pos: position{line: 283, col: 13, offset: 8705},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 283, col: 13, offset: 8705},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 283, col: 13, offset: 8705},
								expr: &choiceExpr{
									pos: position{line: 283, col: 16, offset: 8708},
									alternatives: []interface{}{
										&litMatcher{
											pos:        position{line: 283, col: 16, offset: 8708},
											val:        "]",
											ignoreCase: false,
										},
										&litMatcher{
											pos:        position{line: 283, col: 22, offset: 8714},
											val:        "\\",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 283, col: 29, offset: 8721},
											name: "EOL",
										},
									},
								},
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 35, offset: 8727},
								name: "SourceChar",
							},
						},
					},
					&seqExpr{
						pos: position{line: 283, col: 48, offset: 8740},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 283, col: 48, offset: 8740},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 283, col: 53, offset: 8745},
								name: "CharClassEscape",
							},
						},
					},
				},
			},
		},
		{
			name: "CharClassEscape",
			pos:  position{line: 284, col: 1, offset: 8761},
			expr: &choiceExpr{
				pos: position{line: 284, col: 19, offset: 8781},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 284, col: 21, offset: 8783},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 284, col: 21, offset: 8783},
								val:        "]",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 284, col: 27, offset: 8789},
								name: "CommonEscapeSequence",
							},
						},
					},
					&actionExpr{
						pos: position{line: 285, col: 7, offset: 8818},
						run: (*parser).callonCharClassEscape5,
						expr: &seqExpr{
							pos: position{line: 285, col: 7, offset: 8818},
							exprs: []interface{}{
								&notExpr{
									pos: position{line: 285, col: 7, offset: 8818},
									expr: &litMatcher{
										pos:        position{line: 285, col: 8, offset: 8819},
										val:        "p",
										ignoreCase: false,
									},
								},
								&choiceExpr{
									pos: position{line: 285, col: 14, offset: 8825},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 285, col: 14, offset: 8825},
											name: "SourceChar",
										},
										&ruleRefExpr{
											pos:  position{line: 285, col: 27, offset: 8838},
											name: "EOL",
										},
										&ruleRefExpr{
											pos:  position{line: 285, col: 33, offset: 8844},
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
			pos:  position{line: 289, col: 1, offset: 8910},
			expr: &seqExpr{
				pos: position{line: 289, col: 22, offset: 8933},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 289, col: 22, offset: 8933},
						val:        "p",
						ignoreCase: false,
					},
					&choiceExpr{
						pos: position{line: 290, col: 7, offset: 8946},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 290, col: 7, offset: 8946},
								name: "SingleCharUnicodeClass",
							},
							&actionExpr{
								pos: position{line: 291, col: 7, offset: 8975},
								run: (*parser).callonUnicodeClassEscape5,
								expr: &seqExpr{
									pos: position{line: 291, col: 7, offset: 8975},
									exprs: []interface{}{
										&notExpr{
											pos: position{line: 291, col: 7, offset: 8975},
											expr: &litMatcher{
												pos:        position{line: 291, col: 8, offset: 8976},
												val:        "{",
												ignoreCase: false,
											},
										},
										&choiceExpr{
											pos: position{line: 291, col: 14, offset: 8982},
											alternatives: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 291, col: 14, offset: 8982},
													name: "SourceChar",
												},
												&ruleRefExpr{
													pos:  position{line: 291, col: 27, offset: 8995},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 291, col: 33, offset: 9001},
													name: "EOF",
												},
											},
										},
									},
								},
							},
							&actionExpr{
								pos: position{line: 292, col: 7, offset: 9072},
								run: (*parser).callonUnicodeClassEscape13,
								expr: &seqExpr{
									pos: position{line: 292, col: 7, offset: 9072},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 292, col: 7, offset: 9072},
											val:        "{",
											ignoreCase: false,
										},
										&labeledExpr{
											pos:   position{line: 292, col: 11, offset: 9076},
											label: "ident",
											expr: &ruleRefExpr{
												pos:  position{line: 292, col: 17, offset: 9082},
												name: "IdentifierName",
											},
										},
										&litMatcher{
											pos:        position{line: 292, col: 32, offset: 9097},
											val:        "}",
											ignoreCase: false,
										},
									},
								},
							},
							&actionExpr{
								pos: position{line: 298, col: 7, offset: 9274},
								run: (*parser).callonUnicodeClassEscape19,
								expr: &seqExpr{
									pos: position{line: 298, col: 7, offset: 9274},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 298, col: 7, offset: 9274},
											val:        "{",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 298, col: 11, offset: 9278},
											name: "IdentifierName",
										},
										&choiceExpr{
											pos: position{line: 298, col: 28, offset: 9295},
											alternatives: []interface{}{
												&litMatcher{
													pos:        position{line: 298, col: 28, offset: 9295},
													val:        "]",
													ignoreCase: false,
												},
												&ruleRefExpr{
													pos:  position{line: 298, col: 34, offset: 9301},
													name: "EOL",
												},
												&ruleRefExpr{
													pos:  position{line: 298, col: 40, offset: 9307},
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
			pos:  position{line: 302, col: 1, offset: 9390},
			expr: &charClassMatcher{
				pos:        position{line: 302, col: 26, offset: 9417},
				val:        "[LMNCPZS]",
				chars:      []rune{'L', 'M', 'N', 'C', 'P', 'Z', 'S'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "AnyMatcher",
			pos:  position{line: 304, col: 1, offset: 9428},
			expr: &actionExpr{
				pos: position{line: 304, col: 14, offset: 9443},
				run: (*parser).callonAnyMatcher1,
				expr: &litMatcher{
					pos:        position{line: 304, col: 14, offset: 9443},
					val:        ".",
					ignoreCase: false,
				},
			},
		},
		{
			name: "ThrowExpr",
			pos:  position{line: 309, col: 1, offset: 9518},
			expr: &choiceExpr{
				pos: position{line: 309, col: 13, offset: 9532},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 309, col: 13, offset: 9532},
						run: (*parser).callonThrowExpr2,
						expr: &seqExpr{
							pos: position{line: 309, col: 13, offset: 9532},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 309, col: 13, offset: 9532},
									val:        "%",
									ignoreCase: false,
								},
								&litMatcher{
									pos:        position{line: 309, col: 17, offset: 9536},
									val:        "{",
									ignoreCase: false,
								},
								&labeledExpr{
									pos:   position{line: 309, col: 21, offset: 9540},
									label: "label",
									expr: &ruleRefExpr{
										pos:  position{line: 309, col: 27, offset: 9546},
										name: "IdentifierName",
									},
								},
								&litMatcher{
									pos:        position{line: 309, col: 42, offset: 9561},
									val:        "}",
									ignoreCase: false,
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 313, col: 5, offset: 9669},
						run: (*parser).callonThrowExpr9,
						expr: &seqExpr{
							pos: position{line: 313, col: 5, offset: 9669},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 313, col: 5, offset: 9669},
									val:        "%",
									ignoreCase: false,
								},
								&litMatcher{
									pos:        position{line: 313, col: 9, offset: 9673},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 313, col: 13, offset: 9677},
									name: "IdentifierName",
								},
								&ruleRefExpr{
									pos:  position{line: 313, col: 28, offset: 9692},
									name: "EOF",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "CodeBlock",
			pos:  position{line: 317, col: 1, offset: 9763},
			expr: &choiceExpr{
				pos: position{line: 317, col: 13, offset: 9777},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 317, col: 13, offset: 9777},
						run: (*parser).callonCodeBlock2,
						expr: &seqExpr{
							pos: position{line: 317, col: 13, offset: 9777},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 317, col: 13, offset: 9777},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 317, col: 17, offset: 9781},
									name: "Code",
								},
								&litMatcher{
									pos:        position{line: 317, col: 22, offset: 9786},
									val:        "}",
									ignoreCase: false,
								},
							},
						},
					},
					&actionExpr{
						pos: position{line: 321, col: 5, offset: 9885},
						run: (*parser).callonCodeBlock7,
						expr: &seqExpr{
							pos: position{line: 321, col: 5, offset: 9885},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 321, col: 5, offset: 9885},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 321, col: 9, offset: 9889},
									name: "Code",
								},
								&ruleRefExpr{
									pos:  position{line: 321, col: 14, offset: 9894},
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
			pos:  position{line: 325, col: 1, offset: 9959},
			expr: &zeroOrMoreExpr{
				pos: position{line: 325, col: 8, offset: 9968},
				expr: &choiceExpr{
					pos: position{line: 325, col: 10, offset: 9970},
					alternatives: []interface{}{
						&oneOrMoreExpr{
							pos: position{line: 325, col: 10, offset: 9970},
							expr: &seqExpr{
								pos: position{line: 325, col: 12, offset: 9972},
								exprs: []interface{}{
									&notExpr{
										pos: position{line: 325, col: 12, offset: 9972},
										expr: &charClassMatcher{
											pos:        position{line: 325, col: 13, offset: 9973},
											val:        "[{}]",
											chars:      []rune{'{', '}'},
											ignoreCase: false,
											inverted:   false,
										},
									},
									&ruleRefExpr{
										pos:  position{line: 325, col: 18, offset: 9978},
										name: "SourceChar",
									},
								},
							},
						},
						&seqExpr{
							pos: position{line: 325, col: 34, offset: 9994},
							exprs: []interface{}{
								&litMatcher{
									pos:        position{line: 325, col: 34, offset: 9994},
									val:        "{",
									ignoreCase: false,
								},
								&ruleRefExpr{
									pos:  position{line: 325, col: 38, offset: 9998},
									name: "Code",
								},
								&litMatcher{
									pos:        position{line: 325, col: 43, offset: 10003},
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
			pos:  position{line: 327, col: 1, offset: 10011},
			expr: &zeroOrMoreExpr{
				pos: position{line: 327, col: 6, offset: 10018},
				expr: &choiceExpr{
					pos: position{line: 327, col: 8, offset: 10020},
					alternatives: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 327, col: 8, offset: 10020},
							name: "Whitespace",
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 21, offset: 10033},
							name: "EOL",
						},
						&ruleRefExpr{
							pos:  position{line: 327, col: 27, offset: 10039},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "_",
			pos:  position{line: 328, col: 1, offset: 10050},
			expr: &zeroOrMoreExpr{
				pos: position{line: 328, col: 5, offset: 10056},
				expr: &choiceExpr{
					pos: position{line: 328, col: 7, offset: 10058},
					alternatives: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 328, col: 7, offset: 10058},
							name: "Whitespace",
						},
						&ruleRefExpr{
							pos:  position{line: 328, col: 20, offset: 10071},
							name: "MultiLineCommentNoLineTerminator",
						},
					},
				},
			},
		},
		{
			name: "Whitespace",
			pos:  position{line: 330, col: 1, offset: 10108},
			expr: &charClassMatcher{
				pos:        position{line: 330, col: 14, offset: 10123},
				val:        "[ \\t\\r]",
				chars:      []rune{' ', '\t', '\r'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EOL",
			pos:  position{line: 331, col: 1, offset: 10131},
			expr: &litMatcher{
				pos:        position{line: 331, col: 7, offset: 10139},
				val:        "\n",
				ignoreCase: false,
			},
		},
		{
			name: "EOS",
			pos:  position{line: 332, col: 1, offset: 10144},
			expr: &choiceExpr{
				pos: position{line: 332, col: 7, offset: 10152},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 332, col: 7, offset: 10152},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 332, col: 7, offset: 10152},
								name: "__",
							},
							&litMatcher{
								pos:        position{line: 332, col: 10, offset: 10155},
								val:        ";",
								ignoreCase: false,
							},
						},
					},
					&seqExpr{
						pos: position{line: 332, col: 16, offset: 10161},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 332, col: 16, offset: 10161},
								name: "_",
							},
							&zeroOrOneExpr{
								pos: position{line: 332, col: 18, offset: 10163},
								expr: &ruleRefExpr{
									pos:  position{line: 332, col: 18, offset: 10163},
									name: "SingleLineComment",
								},
							},
							&ruleRefExpr{
								pos:  position{line: 332, col: 37, offset: 10182},
								name: "EOL",
							},
						},
					},
					&seqExpr{
						pos: position{line: 332, col: 43, offset: 10188},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 332, col: 43, offset: 10188},
								name: "__",
							},
							&ruleRefExpr{
								pos:  position{line: 332, col: 46, offset: 10191},
								name: "EOF",
							},
						},
					},
				},
			},
		},
		{
			name: "EOF",
			pos:  position{line: 334, col: 1, offset: 10196},
			expr: &notExpr{
				pos: position{line: 334, col: 7, offset: 10204},
				expr: &anyMatcher{
					line: 334, col: 8, offset: 10205,
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

func (c *current) onRecoveryExpr1(expr, recoverExprs interface{}) (interface{}, error) {
	recoverExprSlice := toIfaceSlice(recoverExprs)
	recover := expr.(ast.Expression)
	for _, sl := range recoverExprSlice {
		pos := c.astPos()
		r := ast.NewRecoveryExpr(pos)
		r.Expr = recover
		r.RecoverExpr = sl.([]interface{})[7].(ast.Expression)
		r.Labels = sl.([]interface{})[3].([]ast.FailureLabel)

		recover = r
	}
	return recover, nil
}

func (p *parser) callonRecoveryExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRecoveryExpr1(stack["expr"], stack["recoverExprs"])
}

func (c *current) onLabels1(label, labels interface{}) (interface{}, error) {
	failureLabels := []ast.FailureLabel{ast.FailureLabel(label.(*ast.Identifier).Val)}
	labelSlice := toIfaceSlice(labels)
	for _, fl := range labelSlice {
		failureLabels = append(failureLabels, ast.FailureLabel(fl.([]interface{})[3].(*ast.Identifier).Val))
	}
	return failureLabels, nil
}

func (p *parser) callonLabels1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onLabels1(stack["label"], stack["labels"])
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
	switch op.(string) {
	case "#":
		state := ast.NewStateCodeExpr(c.astPos())
		state.Code = code.(*ast.CodeBlock)
		return state, nil

	case "&":
		and := ast.NewAndCodeExpr(c.astPos())
		and.Code = code.(*ast.CodeBlock)
		return and, nil

	// case "!":
	default:
		not := ast.NewNotCodeExpr(c.astPos())
		not.Code = code.(*ast.CodeBlock)
		return not, nil

	}
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

func (c *current) onThrowExpr2(label interface{}) (interface{}, error) {
	t := ast.NewThrowExpr(c.astPos())
	t.Label = label.(*ast.Identifier).Val
	return t, nil
}

func (p *parser) callonThrowExpr2() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onThrowExpr2(stack["label"])
}

func (c *current) onThrowExpr9() (interface{}, error) {
	return nil, errors.New("throw expression not terminated")
}

func (p *parser) callonThrowExpr9() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onThrowExpr9()
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

	// errInvalidEntrypoint is returned when the specified entrypoint rule
	// does not exit.
	errInvalidEntrypoint = errors.New("invalid entrypoint")

	// errInvalidEncoding is returned when the source is not properly
	// utf8-encoded.
	errInvalidEncoding = errors.New("invalid encoding")

	// errMaxExprCnt is used to signal that the maximum number of
	// expressions have been parsed.
	errMaxExprCnt = errors.New("max number of expresssions parsed")
)

// Option is a function that can set an option on the parser. It returns
// the previous setting as an Option.
type Option func(*parser) Option

// MaxExpressions creates an Option to stop parsing after the provided
// number of expressions have been parsed, if the value is 0 then the parser will
// parse for as many steps as needed (possibly an infinite number).
//
// The default for maxExprCnt is 0.
func MaxExpressions(maxExprCnt uint64) Option {
	return func(p *parser) Option {
		oldMaxExprCnt := p.maxExprCnt
		p.maxExprCnt = maxExprCnt
		return MaxExpressions(oldMaxExprCnt)
	}
}

// Entrypoint creates an Option to set the rule name to use as entrypoint.
// The rule name must have been specified in the -alternate-entrypoints
// if generating the parser with the -optimize-grammar flag, otherwise
// it may have been optimized out. Passing an empty string sets the
// entrypoint to the first rule in the grammar.
//
// The default is to start parsing at the first rule in the grammar.
func Entrypoint(ruleName string) Option {
	return func(p *parser) Option {
		oldEntrypoint := p.entrypoint
		p.entrypoint = ruleName
		if ruleName == "" {
			p.entrypoint = g.rules[0].name
		}
		return Entrypoint(oldEntrypoint)
	}
}

// Statistics adds a user provided Stats struct to the parser to allow
// the user to process the results after the parsing has finished.
// Also the key for the "no match" counter is set.
//
// Example usage:
//
//     input := "input"
//     stats := Stats{}
//     _, err := Parse("input-file", []byte(input), Statistics(&stats, "no match"))
//     if err != nil {
//         log.Panicln(err)
//     }
//     b, err := json.MarshalIndent(stats.ChoiceAltCnt, "", "  ")
//     if err != nil {
//         log.Panicln(err)
//     }
//     fmt.Println(string(b))
//
func Statistics(stats *Stats, choiceNoMatch string) Option {
	return func(p *parser) Option {
		oldStats := p.Stats
		p.Stats = stats
		oldChoiceNoMatch := p.choiceNoMatch
		p.choiceNoMatch = choiceNoMatch
		if p.Stats.ChoiceAltCnt == nil {
			p.Stats.ChoiceAltCnt = make(map[string]map[string]int)
		}
		return Statistics(oldStats, oldChoiceNoMatch)
	}
}

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

// AllowInvalidUTF8 creates an Option to allow invalid UTF-8 bytes.
// Every invalid UTF-8 byte is treated as a utf8.RuneError (U+FFFD)
// by character class matchers and is matched by the any matcher.
// The returned matched value, c.text and c.offset are NOT affected.
//
// The default is false.
func AllowInvalidUTF8(b bool) Option {
	return func(p *parser) Option {
		old := p.allowInvalidUTF8
		p.allowInvalidUTF8 = b
		return AllowInvalidUTF8(old)
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

// InitState creates an Option to set a key to a certain value in
// the global "state" store.
func InitState(key string, value interface{}) Option {
	return func(p *parser) Option {
		old := p.cur.state[key]
		p.cur.state[key] = value
		return InitState(key, old)
	}
}

// ParseFile parses the file identified by filename.
func ParseFile(filename string, opts ...Option) (i interface{}, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = closeErr
		}
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

	// state is a store for arbitrary key,value pairs that the user wants to be
	// tied to the backtracking of the parser.
	// This is always rolled back if a parsing rule fails.
	state storeDict

	// globalStore is a general store for the user to store arbitrary key-value
	// pairs that they need to manage and that they do not want tied to the
	// backtracking of the parser. This is only modified by the user and never
	// rolled back by the parser. It is always up to the user to keep this in a
	// consistent state.
	globalStore storeDict
}

type storeDict map[string]interface{}

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

type recoveryExpr struct {
	pos          position
	expr         interface{}
	recoverExpr  interface{}
	failureLabel []string
}

type seqExpr struct {
	pos   position
	exprs []interface{}
}

type throwExpr struct {
	pos   position
	label string
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

type stateCodeExpr struct {
	pos position
	run func(*parser) error
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
	pos             position
	val             string
	basicLatinChars [128]bool
	chars           []rune
	ranges          []rune
	classes         []*unicode.RangeTable
	ignoreCase      bool
	inverted        bool
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
	stats := Stats{
		ChoiceAltCnt: make(map[string]map[string]int),
	}

	p := &parser{
		filename: filename,
		errs:     new(errList),
		data:     b,
		pt:       savepoint{position: position{line: 1}},
		recover:  true,
		cur: current{
			state:       make(storeDict),
			globalStore: make(storeDict),
		},
		maxFailPos:      position{col: 1, line: 1},
		maxFailExpected: make([]string, 0, 20),
		Stats:           &stats,
		// start rule is rule [0] unless an alternate entrypoint is specified
		entrypoint: g.rules[0].name,
		emptyState: make(storeDict),
	}
	p.setOptions(opts)

	if p.maxExprCnt == 0 {
		p.maxExprCnt = math.MaxUint64
	}

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

const choiceNoMatch = -1

// Stats stores some statistics, gathered during parsing
type Stats struct {
	// ExprCnt counts the number of expressions processed during parsing
	// This value is compared to the maximum number of expressions allowed
	// (set by the MaxExpressions option).
	ExprCnt uint64

	// ChoiceAltCnt is used to count for each ordered choice expression,
	// which alternative is used how may times.
	// These numbers allow to optimize the order of the ordered choice expression
	// to increase the performance of the parser
	//
	// The outer key of ChoiceAltCnt is composed of the name of the rule as well
	// as the line and the column of the ordered choice.
	// The inner key of ChoiceAltCnt is the number (one-based) of the matching alternative.
	// For each alternative the number of matches are counted. If an ordered choice does not
	// match, a special counter is incremented. The name of this counter is set with
	// the parser option Statistics.
	// For an alternative to be included in ChoiceAltCnt, it has to match at least once.
	ChoiceAltCnt map[string]map[string]int
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

	// parse fail
	maxFailPos            position
	maxFailExpected       []string
	maxFailInvertExpected bool

	// max number of expressions to be parsed
	maxExprCnt uint64
	// entrypoint for the parser
	entrypoint string

	allowInvalidUTF8 bool

	*Stats

	choiceNoMatch string
	// recovery expression stack, keeps track of the currently available recovery expression, these are traversed in reverse
	recoveryStack []map[string]interface{}

	// emptyState contains an empty storeDict, which is used to optimize cloneState if global "state" store is not used.
	emptyState storeDict
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

// push a recovery expression with its labels to the recoveryStack
func (p *parser) pushRecovery(labels []string, expr interface{}) {
	if cap(p.recoveryStack) == len(p.recoveryStack) {
		// create new empty slot in the stack
		p.recoveryStack = append(p.recoveryStack, nil)
	} else {
		// slice to 1 more
		p.recoveryStack = p.recoveryStack[:len(p.recoveryStack)+1]
	}

	m := make(map[string]interface{}, len(labels))
	for _, fl := range labels {
		m[fl] = expr
	}
	p.recoveryStack[len(p.recoveryStack)-1] = m
}

// pop a recovery expression from the recoveryStack
func (p *parser) popRecovery() {
	// GC that map
	p.recoveryStack[len(p.recoveryStack)-1] = nil

	p.recoveryStack = p.recoveryStack[:len(p.recoveryStack)-1]
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
			p.maxFailExpected = p.maxFailExpected[:0]
		}

		if p.maxFailInvertExpected {
			want = "!" + want
		}
		p.maxFailExpected = append(p.maxFailExpected, want)
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

	if rn == utf8.RuneError && n == 1 { // see utf8.DecodeRune
		if !p.allowInvalidUTF8 {
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

// Cloner is implemented by any value that has a Clone method, which returns a
// copy of the value. This is mainly used for types which are not passed by
// value (e.g map, slice, chan) or structs that contain such types.
//
// This is used in conjunction with the global state feature to create proper
// copies of the state to allow the parser to properly restore the state in
// the case of backtracking.
type Cloner interface {
	Clone() interface{}
}

// clone and return parser current state.
func (p *parser) cloneState() storeDict {
	if p.debug {
		defer p.out(p.in("cloneState"))
	}

	if len(p.cur.state) == 0 {
		if len(p.emptyState) > 0 {
			p.emptyState = make(storeDict)
		}
		return p.emptyState
	}

	state := make(storeDict, len(p.cur.state))
	for k, v := range p.cur.state {
		if c, ok := v.(Cloner); ok {
			state[k] = c.Clone()
		} else {
			state[k] = v
		}
	}
	return state
}

// restore parser current state to the state storeDict.
// every restoreState should applied only one time for every cloned state
func (p *parser) restoreState(state storeDict) {
	if p.debug {
		defer p.out(p.in("restoreState"))
	}
	p.cur.state = state
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

	startRule, ok := p.rules[p.entrypoint]
	if !ok {
		p.addErr(errInvalidEntrypoint)
		return nil, p.errs.err()
	}

	p.read() // advance to first rune
	val, ok = p.parseRule(startRule)
	if !ok {
		if len(*p.errs) == 0 {
			// If parsing fails, but no errors have been recorded, the expected values
			// for the farthest parser position are returned as error.
			maxFailExpectedMap := make(map[string]struct{}, len(p.maxFailExpected))
			for _, v := range p.maxFailExpected {
				maxFailExpectedMap[v] = struct{}{}
			}
			expected := make([]string, 0, len(maxFailExpectedMap))
			eof := false
			if _, ok := maxFailExpectedMap["!."]; ok {
				delete(maxFailExpectedMap, "!.")
				eof = true
			}
			for k := range maxFailExpectedMap {
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

	p.ExprCnt++
	if p.ExprCnt > p.maxExprCnt {
		panic(errMaxExprCnt)
	}

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
	case *recoveryExpr:
		val, ok = p.parseRecoveryExpr(expr)
	case *ruleRefExpr:
		val, ok = p.parseRuleRefExpr(expr)
	case *seqExpr:
		val, ok = p.parseSeqExpr(expr)
	case *stateCodeExpr:
		val, ok = p.parseStateCodeExpr(expr)
	case *throwExpr:
		val, ok = p.parseThrowExpr(expr)
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
		state := p.cloneState()
		actVal, err := act.run(p)
		if err != nil {
			p.addErrAt(err, start.position, []string{})
		}
		p.restoreState(state)

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

	state := p.cloneState()

	ok, err := and.run(p)
	if err != nil {
		p.addErr(err)
	}
	p.restoreState(state)

	return nil, ok
}

func (p *parser) parseAndExpr(and *andExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAndExpr"))
	}

	pt := p.pt
	state := p.cloneState()
	p.pushV()
	_, ok := p.parseExpr(and.expr)
	p.popV()
	p.restoreState(state)
	p.restore(pt)

	return nil, ok
}

func (p *parser) parseAnyMatcher(any *anyMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseAnyMatcher"))
	}

	if p.pt.rn == utf8.RuneError && p.pt.w == 0 {
		// EOF - see utf8.DecodeRune
		p.failAt(false, p.pt.position, ".")
		return nil, false
	}
	start := p.pt
	p.read()
	p.failAt(true, start.position, ".")
	return p.sliceFrom(start), true
}

func (p *parser) parseCharClassMatcher(chr *charClassMatcher) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseCharClassMatcher"))
	}

	cur := p.pt.rn
	start := p.pt

	// can't match EOF
	if cur == utf8.RuneError && p.pt.w == 0 { // see utf8.DecodeRune
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

func (p *parser) incChoiceAltCnt(ch *choiceExpr, altI int) {
	choiceIdent := fmt.Sprintf("%s %d:%d", p.rstack[len(p.rstack)-1].name, ch.pos.line, ch.pos.col)
	m := p.ChoiceAltCnt[choiceIdent]
	if m == nil {
		m = make(map[string]int)
		p.ChoiceAltCnt[choiceIdent] = m
	}
	// We increment altI by 1, so the keys do not start at 0
	alt := strconv.Itoa(altI + 1)
	if altI == choiceNoMatch {
		alt = p.choiceNoMatch
	}
	m[alt]++
}

func (p *parser) parseChoiceExpr(ch *choiceExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseChoiceExpr"))
	}

	for altI, alt := range ch.alternatives {
		// dummy assignment to prevent compile error if optimized
		_ = altI

		state := p.cloneState()

		p.pushV()
		val, ok := p.parseExpr(alt)
		p.popV()
		if ok {
			p.incChoiceAltCnt(ch, altI)
			return val, ok
		}
		p.restoreState(state)
	}
	p.incChoiceAltCnt(ch, choiceNoMatch)
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

	state := p.cloneState()

	ok, err := not.run(p)
	if err != nil {
		p.addErr(err)
	}
	p.restoreState(state)

	return nil, !ok
}

func (p *parser) parseNotExpr(not *notExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseNotExpr"))
	}

	pt := p.pt
	state := p.cloneState()
	p.pushV()
	p.maxFailInvertExpected = !p.maxFailInvertExpected
	_, ok := p.parseExpr(not.expr)
	p.maxFailInvertExpected = !p.maxFailInvertExpected
	p.popV()
	p.restoreState(state)
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

func (p *parser) parseRecoveryExpr(recover *recoveryExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseRecoveryExpr (" + strings.Join(recover.failureLabel, ",") + ")"))
	}

	p.pushRecovery(recover.failureLabel, recover.recoverExpr)
	val, ok := p.parseExpr(recover.expr)
	p.popRecovery()

	return val, ok
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

	vals := make([]interface{}, 0, len(seq.exprs))

	pt := p.pt
	state := p.cloneState()
	for _, expr := range seq.exprs {
		val, ok := p.parseExpr(expr)
		if !ok {
			p.restoreState(state)
			p.restore(pt)
			return nil, false
		}
		vals = append(vals, val)
	}
	return vals, true
}

func (p *parser) parseStateCodeExpr(state *stateCodeExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseStateCodeExpr"))
	}

	err := state.run(p)
	if err != nil {
		p.addErr(err)
	}
	return nil, true
}

func (p *parser) parseThrowExpr(expr *throwExpr) (interface{}, bool) {
	if p.debug {
		defer p.out(p.in("parseThrowExpr"))
	}

	for i := len(p.recoveryStack) - 1; i >= 0; i-- {
		if recoverExpr, ok := p.recoveryStack[i][expr.label]; ok {
			if val, ok := p.parseExpr(recoverExpr); ok {
				return val, ok
			}
		}
	}

	return nil, false
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

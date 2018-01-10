package ast

import (
	"reflect"
	"testing"
	// "github.com/pmezard/go-difflib/difflib"
	// goon "github.com/shurcooL/go-goon"
)

var p Pos

var cases = []struct {
	in  *Grammar
	out *Grammar
}{
	// Case 0
	{
		in: &Grammar{
			Rules: []*Rule{
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "Input",
						},
					},
					Expr: &SeqExpr{
						Exprs: []Expression{
							&LitMatcher{
								posValue: posValue{
									Val: "a",
								},
							},
							&RuleRefExpr{
								Name: &Identifier{
									posValue: posValue{
										Val: "innerseq",
									},
								},
							},
							&AndExpr{
								Expr: &RuleRefExpr{
									Name: &Identifier{
										posValue: posValue{
											Val: "innerseq",
										},
									},
								},
							},
							&ChoiceExpr{
								Alternatives: []Expression{
									&LitMatcher{
										posValue: posValue{
											Val: "c1",
										},
									},
									&ChoiceExpr{
										Alternatives: []Expression{
											&LitMatcher{
												posValue: posValue{
													Val: "c2",
												},
											},
											&LitMatcher{
												posValue: posValue{
													Val: "c3",
												},
											},
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "c4",
										},
									},
								},
							},
							&SeqExpr{
								Exprs: []Expression{
									&LitMatcher{
										posValue: posValue{
											Val: "s1",
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "s2",
										},
									},
								},
							},
							&LitMatcher{
								posValue: posValue{
									Val: "c",
								},
							},
							&ChoiceExpr{
								Alternatives: []Expression{
									&LitMatcher{
										posValue: posValue{
											Val: "\n",
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "\r",
										},
									},
									&CharClassMatcher{
										posValue: posValue{
											Val: "[\t]",
										},
										Chars: []rune{
											'\t',
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "a",
										},
									},
								},
							},
						},
					},
				},
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "innerseq",
						},
					},
					Expr: &LitMatcher{
						posValue: posValue{
							Val: "b",
						},
					},
				},
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "innerseq2",
						},
					},
					Expr: &LitMatcher{
						posValue: posValue{
							Val: "b",
						},
					},
				},
			},
		},
		out: &Grammar{
			Rules: []*Rule{
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "Input",
						},
					},
					Expr: &SeqExpr{
						Exprs: []Expression{
							&LitMatcher{
								posValue: posValue{
									Val: "ab",
								},
							},
							&AndExpr{
								Expr: &LitMatcher{
									posValue: posValue{
										Val: "b",
									},
								},
							},
							&ChoiceExpr{
								Alternatives: []Expression{
									&LitMatcher{
										posValue: posValue{
											Val: "c1",
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "c2",
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "c3",
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: "c4",
										},
									},
								},
							},
							&LitMatcher{
								posValue: posValue{
									Val: "s1s2c",
								},
							},
							&CharClassMatcher{
								posValue: posValue{
									Val: "[\\n\\r\\ta]",
								},
								Chars: []rune{
									'\n', '\r', '\t', 'a',
								},
							},
						},
					},
				},
			},
		},
	},
	// Case 1
	{
		in: &Grammar{
			Rules: []*Rule{
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "x",
						},
					},
					Expr: &ChoiceExpr{
						Alternatives: []Expression{
							&SeqExpr{
								Exprs: []Expression{
									&RuleRefExpr{
										Name: &Identifier{
											posValue: posValue{
												Val: "y",
											},
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: ";",
										},
									},
								},
							},
							&SeqExpr{
								Exprs: []Expression{
									&LitMatcher{
										posValue: posValue{
											Val: "z",
										},
									},
									&RuleRefExpr{
										Name: &Identifier{
											posValue: posValue{
												Val: "b",
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "y",
						},
					},
					Expr: &ChoiceExpr{
						Alternatives: []Expression{
							&LitMatcher{
								posValue: posValue{
									Val: "a",
								},
							},
							&RuleRefExpr{
								Name: &Identifier{
									posValue: posValue{
										Val: "b",
									},
								},
							},
						},
					},
				},
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "b",
						},
					},
					Expr: &LitMatcher{
						posValue: posValue{
							Val: "b",
						},
					},
				},
			},
		},
		out: &Grammar{
			Rules: []*Rule{
				{
					Name: &Identifier{
						posValue: posValue{
							Val: "x",
						},
					},
					Expr: &ChoiceExpr{
						Alternatives: []Expression{
							&SeqExpr{
								Exprs: []Expression{
									&CharClassMatcher{
										posValue: posValue{
											Val: "[ab]",
										},
										Chars: []rune{
											'a', 'b',
										},
									},
									&LitMatcher{
										posValue: posValue{
											Val: ";",
										},
									},
								},
							},
							&LitMatcher{
								posValue: posValue{
									Val: "zb",
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestOptimize(t *testing.T) {
	for i := range cases {
		Optimize(cases[i].in)
		if !reflect.DeepEqual(cases[i].in, cases[i].out) {
			t.Errorf("%d: not equal", i)
			// dumpin := goon.Sdump(cases[i].in)
			// dumpout := goon.Sdump(cases[i].out)
			// fmt.Println("=== want:")
			// fmt.Println(dumpout)
			// fmt.Println("=== got:")
			// fmt.Println(dumpin)
			// fmt.Println("=== diff:")
			// diff := difflib.UnifiedDiff{
			// 	A:        difflib.SplitLines(dumpout),
			// 	B:        difflib.SplitLines(dumpin),
			// 	FromFile: "want",
			// 	ToFile:   "got",
			// 	Context:  3,
			// }
			// text, _ := difflib.GetUnifiedDiffString(diff)
			// fmt.Println(text)
		}
	}
}

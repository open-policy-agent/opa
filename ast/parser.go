package ast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
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

func makeObject(head interface{}, tail interface{}, loc *Location) (*Term, error) {
	term := ObjectTerm()
	term.Location = loc

	// Empty object.
	if head == nil {
		return term, nil
	}

	headSlice := head.([]interface{})
	obj := term.Value.(Object)
	obj.Insert(headSlice[0].(*Term), headSlice[len(headSlice)-1].(*Term))

	// Non-empty object, remaining key/value pairs.
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		obj.Insert(s[3].(*Term), s[len(s)-1].(*Term))
	}

	return term, nil
}

func makeArray(head interface{}, tail interface{}, loc *Location) (*Term, error) {
	arr := ArrayTerm()
	arr.Location = loc

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

func makeArgs(head interface{}, tail interface{}, loc *Location) (Args, error) {
	args := Args{}
	if head == nil {
		return nil, nil
	}
	args = append(args, head.(*Term))
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		args = append(args, s[len(s)-1].(*Term))
	}
	return args, nil
}

func makeInfixCallExpr(operator interface{}, args interface{}, output interface{}) (*Expr, error) {
	expr := &Expr{}
	a := args.(Args)
	terms := make([]*Term, len(a)+2)
	terms[0] = operator.(*Term)
	dst := terms[1:]
	for i := 0; i < len(a); i++ {
		dst[i] = a[i]
	}
	terms[len(terms)-1] = output.(*Term)
	expr.Terms = terms
	expr.Infix = true
	return expr, nil
}

var g = &grammar{
	rules: []*rule{
		{
			name: "Program",
			pos:  position{line: 124, col: 1, offset: 2937},
			expr: &actionExpr{
				pos: position{line: 124, col: 12, offset: 2948},
				run: (*parser).callonProgram1,
				expr: &seqExpr{
					pos: position{line: 124, col: 12, offset: 2948},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 124, col: 12, offset: 2948},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 124, col: 14, offset: 2950},
							label: "vals",
							expr: &zeroOrOneExpr{
								pos: position{line: 124, col: 19, offset: 2955},
								expr: &seqExpr{
									pos: position{line: 124, col: 20, offset: 2956},
									exprs: []interface{}{
										&labeledExpr{
											pos:   position{line: 124, col: 20, offset: 2956},
											label: "head",
											expr: &ruleRefExpr{
												pos:  position{line: 124, col: 25, offset: 2961},
												name: "Stmt",
											},
										},
										&labeledExpr{
											pos:   position{line: 124, col: 30, offset: 2966},
											label: "tail",
											expr: &zeroOrMoreExpr{
												pos: position{line: 124, col: 35, offset: 2971},
												expr: &seqExpr{
													pos: position{line: 124, col: 36, offset: 2972},
													exprs: []interface{}{
														&ruleRefExpr{
															pos:  position{line: 124, col: 36, offset: 2972},
															name: "ws",
														},
														&ruleRefExpr{
															pos:  position{line: 124, col: 39, offset: 2975},
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
							pos:  position{line: 124, col: 48, offset: 2984},
							name: "_",
						},
						&ruleRefExpr{
							pos:  position{line: 124, col: 50, offset: 2986},
							name: "EOF",
						},
					},
				},
			},
		},
		{
			name: "Stmt",
			pos:  position{line: 142, col: 1, offset: 3360},
			expr: &actionExpr{
				pos: position{line: 142, col: 9, offset: 3368},
				run: (*parser).callonStmt1,
				expr: &labeledExpr{
					pos:   position{line: 142, col: 9, offset: 3368},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 142, col: 14, offset: 3373},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 142, col: 14, offset: 3373},
								name: "Package",
							},
							&ruleRefExpr{
								pos:  position{line: 142, col: 24, offset: 3383},
								name: "Import",
							},
							&ruleRefExpr{
								pos:  position{line: 142, col: 33, offset: 3392},
								name: "Rules",
							},
							&ruleRefExpr{
								pos:  position{line: 142, col: 41, offset: 3400},
								name: "Body",
							},
							&ruleRefExpr{
								pos:  position{line: 142, col: 48, offset: 3407},
								name: "Comment",
							},
						},
					},
				},
			},
		},
		{
			name: "Package",
			pos:  position{line: 146, col: 1, offset: 3441},
			expr: &actionExpr{
				pos: position{line: 146, col: 12, offset: 3452},
				run: (*parser).callonPackage1,
				expr: &seqExpr{
					pos: position{line: 146, col: 12, offset: 3452},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 146, col: 12, offset: 3452},
							val:        "package",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 146, col: 22, offset: 3462},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 146, col: 25, offset: 3465},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 146, col: 30, offset: 3470},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 146, col: 30, offset: 3470},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 146, col: 36, offset: 3476},
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
			pos:  position{line: 180, col: 1, offset: 4792},
			expr: &actionExpr{
				pos: position{line: 180, col: 11, offset: 4802},
				run: (*parser).callonImport1,
				expr: &seqExpr{
					pos: position{line: 180, col: 11, offset: 4802},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 180, col: 11, offset: 4802},
							val:        "import",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 180, col: 20, offset: 4811},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 180, col: 23, offset: 4814},
							label: "path",
							expr: &choiceExpr{
								pos: position{line: 180, col: 29, offset: 4820},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 180, col: 29, offset: 4820},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 180, col: 35, offset: 4826},
										name: "Var",
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 180, col: 40, offset: 4831},
							label: "alias",
							expr: &zeroOrOneExpr{
								pos: position{line: 180, col: 46, offset: 4837},
								expr: &seqExpr{
									pos: position{line: 180, col: 47, offset: 4838},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 180, col: 47, offset: 4838},
											name: "ws",
										},
										&litMatcher{
											pos:        position{line: 180, col: 50, offset: 4841},
											val:        "as",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 180, col: 55, offset: 4846},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 180, col: 58, offset: 4849},
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
			pos:  position{line: 196, col: 1, offset: 5299},
			expr: &choiceExpr{
				pos: position{line: 196, col: 10, offset: 5308},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 196, col: 10, offset: 5308},
						name: "DefaultRules",
					},
					&ruleRefExpr{
						pos:  position{line: 196, col: 25, offset: 5323},
						name: "NormalRules",
					},
				},
			},
		},
		{
			name: "DefaultRules",
			pos:  position{line: 198, col: 1, offset: 5336},
			expr: &actionExpr{
				pos: position{line: 198, col: 17, offset: 5352},
				run: (*parser).callonDefaultRules1,
				expr: &seqExpr{
					pos: position{line: 198, col: 17, offset: 5352},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 198, col: 17, offset: 5352},
							val:        "default",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 198, col: 27, offset: 5362},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 198, col: 30, offset: 5365},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 198, col: 35, offset: 5370},
								name: "Var",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 198, col: 39, offset: 5374},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 198, col: 41, offset: 5376},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 198, col: 45, offset: 5380},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 198, col: 47, offset: 5382},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 198, col: 53, offset: 5388},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "NormalRules",
			pos:  position{line: 241, col: 1, offset: 6357},
			expr: &actionExpr{
				pos: position{line: 241, col: 16, offset: 6372},
				run: (*parser).callonNormalRules1,
				expr: &seqExpr{
					pos: position{line: 241, col: 16, offset: 6372},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 241, col: 16, offset: 6372},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 241, col: 21, offset: 6377},
								name: "RuleHead",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 241, col: 30, offset: 6386},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 241, col: 32, offset: 6388},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 241, col: 35, offset: 6391},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 241, col: 35, offset: 6391},
										name: "NonEmptyBraceEnclosedBody",
									},
									&zeroOrMoreExpr{
										pos: position{line: 241, col: 61, offset: 6417},
										expr: &seqExpr{
											pos: position{line: 241, col: 63, offset: 6419},
											exprs: []interface{}{
												&ruleRefExpr{
													pos:  position{line: 241, col: 63, offset: 6419},
													name: "_",
												},
												&ruleRefExpr{
													pos:  position{line: 241, col: 65, offset: 6421},
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
			pos:  position{line: 297, col: 1, offset: 7751},
			expr: &actionExpr{
				pos: position{line: 297, col: 13, offset: 7763},
				run: (*parser).callonRuleHead1,
				expr: &seqExpr{
					pos: position{line: 297, col: 13, offset: 7763},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 297, col: 13, offset: 7763},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 297, col: 18, offset: 7768},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 297, col: 22, offset: 7772},
							label: "args",
							expr: &zeroOrOneExpr{
								pos: position{line: 297, col: 27, offset: 7777},
								expr: &seqExpr{
									pos: position{line: 297, col: 29, offset: 7779},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 297, col: 29, offset: 7779},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 297, col: 31, offset: 7781},
											val:        "(",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 35, offset: 7785},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 37, offset: 7787},
											name: "Args",
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 42, offset: 7792},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 297, col: 44, offset: 7794},
											val:        ")",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 48, offset: 7798},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 297, col: 53, offset: 7803},
							label: "key",
							expr: &zeroOrOneExpr{
								pos: position{line: 297, col: 57, offset: 7807},
								expr: &seqExpr{
									pos: position{line: 297, col: 59, offset: 7809},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 297, col: 59, offset: 7809},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 297, col: 61, offset: 7811},
											val:        "[",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 65, offset: 7815},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 67, offset: 7817},
											name: "Term",
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 72, offset: 7822},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 297, col: 74, offset: 7824},
											val:        "]",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 78, offset: 7828},
											name: "_",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 297, col: 83, offset: 7833},
							label: "value",
							expr: &zeroOrOneExpr{
								pos: position{line: 297, col: 89, offset: 7839},
								expr: &seqExpr{
									pos: position{line: 297, col: 91, offset: 7841},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 297, col: 91, offset: 7841},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 297, col: 93, offset: 7843},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 97, offset: 7847},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 297, col: 99, offset: 7849},
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
			name: "Args",
			pos:  position{line: 340, col: 1, offset: 9057},
			expr: &actionExpr{
				pos: position{line: 340, col: 9, offset: 9065},
				run: (*parser).callonArgs1,
				expr: &seqExpr{
					pos: position{line: 340, col: 9, offset: 9065},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 340, col: 9, offset: 9065},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 340, col: 11, offset: 9067},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 340, col: 16, offset: 9072},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 340, col: 21, offset: 9077},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 340, col: 26, offset: 9082},
								expr: &seqExpr{
									pos: position{line: 340, col: 27, offset: 9083},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 340, col: 27, offset: 9083},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 340, col: 29, offset: 9085},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 340, col: 33, offset: 9089},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 340, col: 35, offset: 9091},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 340, col: 42, offset: 9098},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 340, col: 44, offset: 9100},
							expr: &litMatcher{
								pos:        position{line: 340, col: 44, offset: 9100},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 340, col: 49, offset: 9105},
							name: "_",
						},
					},
				},
			},
		},
		{
			name: "Else",
			pos:  position{line: 344, col: 1, offset: 9161},
			expr: &actionExpr{
				pos: position{line: 344, col: 9, offset: 9169},
				run: (*parser).callonElse1,
				expr: &seqExpr{
					pos: position{line: 344, col: 9, offset: 9169},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 344, col: 9, offset: 9169},
							val:        "else",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 344, col: 16, offset: 9176},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 344, col: 20, offset: 9180},
								expr: &seqExpr{
									pos: position{line: 344, col: 22, offset: 9182},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 344, col: 22, offset: 9182},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 344, col: 24, offset: 9184},
											val:        "=",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 344, col: 28, offset: 9188},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 344, col: 30, offset: 9190},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 344, col: 38, offset: 9198},
							label: "b",
							expr: &seqExpr{
								pos: position{line: 344, col: 42, offset: 9202},
								exprs: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 344, col: 42, offset: 9202},
										name: "_",
									},
									&ruleRefExpr{
										pos:  position{line: 344, col: 44, offset: 9204},
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
			pos:  position{line: 359, col: 1, offset: 9556},
			expr: &actionExpr{
				pos: position{line: 359, col: 12, offset: 9567},
				run: (*parser).callonRuleDup1,
				expr: &labeledExpr{
					pos:   position{line: 359, col: 12, offset: 9567},
					label: "b",
					expr: &ruleRefExpr{
						pos:  position{line: 359, col: 14, offset: 9569},
						name: "NonEmptyBraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "RuleExt",
			pos:  position{line: 363, col: 1, offset: 9665},
			expr: &choiceExpr{
				pos: position{line: 363, col: 12, offset: 9676},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 363, col: 12, offset: 9676},
						name: "Else",
					},
					&ruleRefExpr{
						pos:  position{line: 363, col: 19, offset: 9683},
						name: "RuleDup",
					},
				},
			},
		},
		{
			name: "Body",
			pos:  position{line: 365, col: 1, offset: 9692},
			expr: &choiceExpr{
				pos: position{line: 365, col: 9, offset: 9700},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 365, col: 9, offset: 9700},
						name: "NonWhitespaceBody",
					},
					&ruleRefExpr{
						pos:  position{line: 365, col: 29, offset: 9720},
						name: "BraceEnclosedBody",
					},
				},
			},
		},
		{
			name: "NonEmptyBraceEnclosedBody",
			pos:  position{line: 367, col: 1, offset: 9739},
			expr: &actionExpr{
				pos: position{line: 367, col: 30, offset: 9768},
				run: (*parser).callonNonEmptyBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 367, col: 30, offset: 9768},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 367, col: 30, offset: 9768},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 367, col: 34, offset: 9772},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 367, col: 36, offset: 9774},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 367, col: 40, offset: 9778},
								expr: &ruleRefExpr{
									pos:  position{line: 367, col: 40, offset: 9778},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 367, col: 56, offset: 9794},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 367, col: 58, offset: 9796},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "BraceEnclosedBody",
			pos:  position{line: 374, col: 1, offset: 9891},
			expr: &actionExpr{
				pos: position{line: 374, col: 22, offset: 9912},
				run: (*parser).callonBraceEnclosedBody1,
				expr: &seqExpr{
					pos: position{line: 374, col: 22, offset: 9912},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 374, col: 22, offset: 9912},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 374, col: 26, offset: 9916},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 374, col: 28, offset: 9918},
							label: "val",
							expr: &zeroOrOneExpr{
								pos: position{line: 374, col: 32, offset: 9922},
								expr: &ruleRefExpr{
									pos:  position{line: 374, col: 32, offset: 9922},
									name: "WhitespaceBody",
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 374, col: 48, offset: 9938},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 374, col: 50, offset: 9940},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "WhitespaceBody",
			pos:  position{line: 388, col: 1, offset: 10292},
			expr: &actionExpr{
				pos: position{line: 388, col: 19, offset: 10310},
				run: (*parser).callonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 388, col: 19, offset: 10310},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 388, col: 19, offset: 10310},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 388, col: 24, offset: 10315},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 388, col: 32, offset: 10323},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 388, col: 37, offset: 10328},
								expr: &seqExpr{
									pos: position{line: 388, col: 38, offset: 10329},
									exprs: []interface{}{
										&zeroOrMoreExpr{
											pos: position{line: 388, col: 38, offset: 10329},
											expr: &charClassMatcher{
												pos:        position{line: 388, col: 38, offset: 10329},
												val:        "[ \\t]",
												chars:      []rune{' ', '\t'},
												ignoreCase: false,
												inverted:   false,
											},
										},
										&choiceExpr{
											pos: position{line: 388, col: 46, offset: 10337},
											alternatives: []interface{}{
												&seqExpr{
													pos: position{line: 388, col: 47, offset: 10338},
													exprs: []interface{}{
														&litMatcher{
															pos:        position{line: 388, col: 47, offset: 10338},
															val:        ";",
															ignoreCase: false,
														},
														&zeroOrOneExpr{
															pos: position{line: 388, col: 51, offset: 10342},
															expr: &ruleRefExpr{
																pos:  position{line: 388, col: 51, offset: 10342},
																name: "Comment",
															},
														},
													},
												},
												&seqExpr{
													pos: position{line: 388, col: 64, offset: 10355},
													exprs: []interface{}{
														&zeroOrOneExpr{
															pos: position{line: 388, col: 64, offset: 10355},
															expr: &ruleRefExpr{
																pos:  position{line: 388, col: 64, offset: 10355},
																name: "Comment",
															},
														},
														&charClassMatcher{
															pos:        position{line: 388, col: 73, offset: 10364},
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
											pos:  position{line: 388, col: 82, offset: 10373},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 388, col: 84, offset: 10375},
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
			pos:  position{line: 394, col: 1, offset: 10564},
			expr: &actionExpr{
				pos: position{line: 394, col: 22, offset: 10585},
				run: (*parser).callonNonWhitespaceBody1,
				expr: &seqExpr{
					pos: position{line: 394, col: 22, offset: 10585},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 394, col: 22, offset: 10585},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 394, col: 27, offset: 10590},
								name: "Literal",
							},
						},
						&labeledExpr{
							pos:   position{line: 394, col: 35, offset: 10598},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 394, col: 40, offset: 10603},
								expr: &seqExpr{
									pos: position{line: 394, col: 42, offset: 10605},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 394, col: 42, offset: 10605},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 394, col: 44, offset: 10607},
											val:        ";",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 394, col: 48, offset: 10611},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 394, col: 50, offset: 10613},
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
			name: "Literal",
			pos:  position{line: 398, col: 1, offset: 10688},
			expr: &actionExpr{
				pos: position{line: 398, col: 12, offset: 10699},
				run: (*parser).callonLiteral1,
				expr: &seqExpr{
					pos: position{line: 398, col: 12, offset: 10699},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 398, col: 12, offset: 10699},
							label: "neg",
							expr: &zeroOrOneExpr{
								pos: position{line: 398, col: 16, offset: 10703},
								expr: &seqExpr{
									pos: position{line: 398, col: 18, offset: 10705},
									exprs: []interface{}{
										&litMatcher{
											pos:        position{line: 398, col: 18, offset: 10705},
											val:        "not",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 398, col: 24, offset: 10711},
											name: "ws",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 398, col: 30, offset: 10717},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 398, col: 34, offset: 10721},
								name: "Expr",
							},
						},
						&labeledExpr{
							pos:   position{line: 398, col: 39, offset: 10726},
							label: "with",
							expr: &zeroOrOneExpr{
								pos: position{line: 398, col: 44, offset: 10731},
								expr: &seqExpr{
									pos: position{line: 398, col: 46, offset: 10733},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 398, col: 46, offset: 10733},
											name: "ws",
										},
										&ruleRefExpr{
											pos:  position{line: 398, col: 49, offset: 10736},
											name: "With",
										},
										&zeroOrMoreExpr{
											pos: position{line: 398, col: 54, offset: 10741},
											expr: &seqExpr{
												pos: position{line: 398, col: 55, offset: 10742},
												exprs: []interface{}{
													&ruleRefExpr{
														pos:  position{line: 398, col: 55, offset: 10742},
														name: "ws",
													},
													&ruleRefExpr{
														pos:  position{line: 398, col: 58, offset: 10745},
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
			pos:  position{line: 426, col: 1, offset: 11416},
			expr: &actionExpr{
				pos: position{line: 426, col: 9, offset: 11424},
				run: (*parser).callonWith1,
				expr: &seqExpr{
					pos: position{line: 426, col: 9, offset: 11424},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 426, col: 9, offset: 11424},
							val:        "with",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 426, col: 16, offset: 11431},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 426, col: 19, offset: 11434},
							label: "target",
							expr: &ruleRefExpr{
								pos:  position{line: 426, col: 26, offset: 11441},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 426, col: 31, offset: 11446},
							name: "ws",
						},
						&litMatcher{
							pos:        position{line: 426, col: 34, offset: 11449},
							val:        "as",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 426, col: 39, offset: 11454},
							name: "ws",
						},
						&labeledExpr{
							pos:   position{line: 426, col: 42, offset: 11457},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 426, col: 48, offset: 11463},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "Expr",
			pos:  position{line: 437, col: 1, offset: 11712},
			expr: &choiceExpr{
				pos: position{line: 437, col: 9, offset: 11720},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 437, col: 9, offset: 11720},
						name: "InfixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 437, col: 21, offset: 11732},
						name: "PrefixExpr",
					},
					&ruleRefExpr{
						pos:  position{line: 437, col: 34, offset: 11745},
						name: "Term",
					},
				},
			},
		},
		{
			name: "InfixExpr",
			pos:  position{line: 439, col: 1, offset: 11751},
			expr: &choiceExpr{
				pos: position{line: 439, col: 14, offset: 11764},
				alternatives: []interface{}{
					&choiceExpr{
						pos: position{line: 439, col: 15, offset: 11765},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 439, col: 15, offset: 11765},
								name: "InfixCallExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 439, col: 31, offset: 11781},
								name: "InfixCallExprReverse",
							},
						},
					},
					&choiceExpr{
						pos: position{line: 439, col: 56, offset: 11806},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 439, col: 56, offset: 11806},
								name: "InfixArithExpr",
							},
							&ruleRefExpr{
								pos:  position{line: 439, col: 73, offset: 11823},
								name: "InfixArithExprReverse",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 439, col: 98, offset: 11848},
						name: "InfixRelationExpr",
					},
				},
			},
		},
		{
			name: "InfixCallExpr",
			pos:  position{line: 441, col: 1, offset: 11867},
			expr: &actionExpr{
				pos: position{line: 441, col: 18, offset: 11884},
				run: (*parser).callonInfixCallExpr1,
				expr: &seqExpr{
					pos: position{line: 441, col: 18, offset: 11884},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 441, col: 18, offset: 11884},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 441, col: 25, offset: 11891},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 30, offset: 11896},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 441, col: 32, offset: 11898},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 36, offset: 11902},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 441, col: 38, offset: 11904},
							label: "operator",
							expr: &ruleRefExpr{
								pos:  position{line: 441, col: 47, offset: 11913},
								name: "Operator",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 56, offset: 11922},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 441, col: 58, offset: 11924},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 62, offset: 11928},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 441, col: 64, offset: 11930},
							label: "args",
							expr: &ruleRefExpr{
								pos:  position{line: 441, col: 69, offset: 11935},
								name: "Args",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 441, col: 74, offset: 11940},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 441, col: 76, offset: 11942},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "InfixCallExprReverse",
			pos:  position{line: 445, col: 1, offset: 12004},
			expr: &actionExpr{
				pos: position{line: 445, col: 25, offset: 12028},
				run: (*parser).callonInfixCallExprReverse1,
				expr: &seqExpr{
					pos: position{line: 445, col: 25, offset: 12028},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 445, col: 25, offset: 12028},
							label: "operator",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 34, offset: 12037},
								name: "Operator",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 43, offset: 12046},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 445, col: 45, offset: 12048},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 49, offset: 12052},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 445, col: 51, offset: 12054},
							label: "args",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 56, offset: 12059},
								name: "Args",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 61, offset: 12064},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 445, col: 63, offset: 12066},
							val:        ")",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 67, offset: 12070},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 445, col: 69, offset: 12072},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 445, col: 73, offset: 12076},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 445, col: 75, offset: 12078},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 445, col: 82, offset: 12085},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExpr",
			pos:  position{line: 449, col: 1, offset: 12148},
			expr: &actionExpr{
				pos: position{line: 449, col: 19, offset: 12166},
				run: (*parser).callonInfixArithExpr1,
				expr: &seqExpr{
					pos: position{line: 449, col: 19, offset: 12166},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 449, col: 19, offset: 12166},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 26, offset: 12173},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 31, offset: 12178},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 449, col: 33, offset: 12180},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 37, offset: 12184},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 39, offset: 12186},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 44, offset: 12191},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 49, offset: 12196},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 51, offset: 12198},
							label: "operator",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 60, offset: 12207},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 449, col: 73, offset: 12220},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 449, col: 75, offset: 12222},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 449, col: 81, offset: 12228},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixArithExprReverse",
			pos:  position{line: 453, col: 1, offset: 12320},
			expr: &actionExpr{
				pos: position{line: 453, col: 26, offset: 12345},
				run: (*parser).callonInfixArithExprReverse1,
				expr: &seqExpr{
					pos: position{line: 453, col: 26, offset: 12345},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 453, col: 26, offset: 12345},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 31, offset: 12350},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 36, offset: 12355},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 38, offset: 12357},
							label: "operator",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 47, offset: 12366},
								name: "ArithInfixOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 60, offset: 12379},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 62, offset: 12381},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 68, offset: 12387},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 73, offset: 12392},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 453, col: 75, offset: 12394},
							val:        "=",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 453, col: 79, offset: 12398},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 453, col: 81, offset: 12400},
							label: "output",
							expr: &ruleRefExpr{
								pos:  position{line: 453, col: 88, offset: 12407},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "ArithInfixOp",
			pos:  position{line: 457, col: 1, offset: 12499},
			expr: &actionExpr{
				pos: position{line: 457, col: 17, offset: 12515},
				run: (*parser).callonArithInfixOp1,
				expr: &labeledExpr{
					pos:   position{line: 457, col: 17, offset: 12515},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 457, col: 22, offset: 12520},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 457, col: 22, offset: 12520},
								val:        "+",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 28, offset: 12526},
								val:        "-",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 34, offset: 12532},
								val:        "*",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 40, offset: 12538},
								val:        "/",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 46, offset: 12544},
								val:        "&",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 52, offset: 12550},
								val:        "|",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 457, col: 58, offset: 12556},
								val:        "-",
								ignoreCase: false,
							},
						},
					},
				},
			},
		},
		{
			name: "InfixRelationExpr",
			pos:  position{line: 469, col: 1, offset: 12830},
			expr: &actionExpr{
				pos: position{line: 469, col: 22, offset: 12851},
				run: (*parser).callonInfixRelationExpr1,
				expr: &seqExpr{
					pos: position{line: 469, col: 22, offset: 12851},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 469, col: 22, offset: 12851},
							label: "left",
							expr: &ruleRefExpr{
								pos:  position{line: 469, col: 27, offset: 12856},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 469, col: 32, offset: 12861},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 469, col: 34, offset: 12863},
							label: "operator",
							expr: &ruleRefExpr{
								pos:  position{line: 469, col: 43, offset: 12872},
								name: "InfixRelationOp",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 469, col: 59, offset: 12888},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 469, col: 61, offset: 12890},
							label: "right",
							expr: &ruleRefExpr{
								pos:  position{line: 469, col: 67, offset: 12896},
								name: "Term",
							},
						},
					},
				},
			},
		},
		{
			name: "InfixRelationOp",
			pos:  position{line: 480, col: 1, offset: 13074},
			expr: &actionExpr{
				pos: position{line: 480, col: 20, offset: 13093},
				run: (*parser).callonInfixRelationOp1,
				expr: &labeledExpr{
					pos:   position{line: 480, col: 20, offset: 13093},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 480, col: 25, offset: 13098},
						alternatives: []interface{}{
							&litMatcher{
								pos:        position{line: 480, col: 25, offset: 13098},
								val:        "=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 480, col: 31, offset: 13104},
								val:        "!=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 480, col: 38, offset: 13111},
								val:        "<=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 480, col: 45, offset: 13118},
								val:        ">=",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 480, col: 52, offset: 13125},
								val:        "<",
								ignoreCase: false,
							},
							&litMatcher{
								pos:        position{line: 480, col: 58, offset: 13131},
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
			pos:  position{line: 492, col: 1, offset: 13405},
			expr: &choiceExpr{
				pos: position{line: 492, col: 15, offset: 13419},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 492, col: 15, offset: 13419},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 492, col: 26, offset: 13430},
						name: "Call",
					},
				},
			},
		},
		{
			name: "Call",
			pos:  position{line: 494, col: 1, offset: 13436},
			expr: &actionExpr{
				pos: position{line: 494, col: 9, offset: 13444},
				run: (*parser).callonCall1,
				expr: &seqExpr{
					pos: position{line: 494, col: 9, offset: 13444},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 494, col: 9, offset: 13444},
							label: "name",
							expr: &ruleRefExpr{
								pos:  position{line: 494, col: 14, offset: 13449},
								name: "Operator",
							},
						},
						&litMatcher{
							pos:        position{line: 494, col: 23, offset: 13458},
							val:        "(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 494, col: 27, offset: 13462},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 494, col: 29, offset: 13464},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 494, col: 34, offset: 13469},
								expr: &ruleRefExpr{
									pos:  position{line: 494, col: 34, offset: 13469},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 494, col: 40, offset: 13475},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 494, col: 45, offset: 13480},
								expr: &seqExpr{
									pos: position{line: 494, col: 47, offset: 13482},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 494, col: 47, offset: 13482},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 494, col: 49, offset: 13484},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 494, col: 53, offset: 13488},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 494, col: 55, offset: 13490},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 494, col: 63, offset: 13498},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 494, col: 66, offset: 13501},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Operator",
			pos:  position{line: 512, col: 1, offset: 13935},
			expr: &actionExpr{
				pos: position{line: 512, col: 13, offset: 13947},
				run: (*parser).callonOperator1,
				expr: &labeledExpr{
					pos:   position{line: 512, col: 13, offset: 13947},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 512, col: 18, offset: 13952},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 512, col: 18, offset: 13952},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 512, col: 24, offset: 13958},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Term",
			pos:  position{line: 524, col: 1, offset: 14189},
			expr: &actionExpr{
				pos: position{line: 524, col: 9, offset: 14197},
				run: (*parser).callonTerm1,
				expr: &labeledExpr{
					pos:   position{line: 524, col: 9, offset: 14197},
					label: "val",
					expr: &choiceExpr{
						pos: position{line: 524, col: 15, offset: 14203},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 524, col: 15, offset: 14203},
								name: "Comprehension",
							},
							&ruleRefExpr{
								pos:  position{line: 524, col: 31, offset: 14219},
								name: "Composite",
							},
							&ruleRefExpr{
								pos:  position{line: 524, col: 43, offset: 14231},
								name: "Scalar",
							},
							&ruleRefExpr{
								pos:  position{line: 524, col: 52, offset: 14240},
								name: "Ref",
							},
							&ruleRefExpr{
								pos:  position{line: 524, col: 58, offset: 14246},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "Comprehension",
			pos:  position{line: 528, col: 1, offset: 14277},
			expr: &choiceExpr{
				pos: position{line: 528, col: 18, offset: 14294},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 528, col: 18, offset: 14294},
						name: "ArrayComprehension",
					},
					&ruleRefExpr{
						pos:  position{line: 528, col: 39, offset: 14315},
						name: "ObjectComprehension",
					},
					&ruleRefExpr{
						pos:  position{line: 528, col: 61, offset: 14337},
						name: "SetComprehension",
					},
				},
			},
		},
		{
			name: "ArrayComprehension",
			pos:  position{line: 530, col: 1, offset: 14355},
			expr: &actionExpr{
				pos: position{line: 530, col: 23, offset: 14377},
				run: (*parser).callonArrayComprehension1,
				expr: &seqExpr{
					pos: position{line: 530, col: 23, offset: 14377},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 530, col: 23, offset: 14377},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 530, col: 27, offset: 14381},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 530, col: 29, offset: 14383},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 530, col: 34, offset: 14388},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 530, col: 39, offset: 14393},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 530, col: 41, offset: 14395},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 530, col: 45, offset: 14399},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 530, col: 47, offset: 14401},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 530, col: 52, offset: 14406},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 530, col: 67, offset: 14421},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 530, col: 69, offset: 14423},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "ObjectComprehension",
			pos:  position{line: 536, col: 1, offset: 14548},
			expr: &actionExpr{
				pos: position{line: 536, col: 24, offset: 14571},
				run: (*parser).callonObjectComprehension1,
				expr: &seqExpr{
					pos: position{line: 536, col: 24, offset: 14571},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 536, col: 24, offset: 14571},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 28, offset: 14575},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 536, col: 30, offset: 14577},
							label: "key",
							expr: &ruleRefExpr{
								pos:  position{line: 536, col: 34, offset: 14581},
								name: "Key",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 38, offset: 14585},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 536, col: 40, offset: 14587},
							val:        ":",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 44, offset: 14591},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 536, col: 46, offset: 14593},
							label: "value",
							expr: &ruleRefExpr{
								pos:  position{line: 536, col: 52, offset: 14599},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 58, offset: 14605},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 536, col: 60, offset: 14607},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 64, offset: 14611},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 536, col: 66, offset: 14613},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 536, col: 71, offset: 14618},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 536, col: 86, offset: 14633},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 536, col: 88, offset: 14635},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetComprehension",
			pos:  position{line: 542, col: 1, offset: 14775},
			expr: &actionExpr{
				pos: position{line: 542, col: 21, offset: 14795},
				run: (*parser).callonSetComprehension1,
				expr: &seqExpr{
					pos: position{line: 542, col: 21, offset: 14795},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 542, col: 21, offset: 14795},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 542, col: 25, offset: 14799},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 542, col: 27, offset: 14801},
							label: "term",
							expr: &ruleRefExpr{
								pos:  position{line: 542, col: 32, offset: 14806},
								name: "Term",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 542, col: 37, offset: 14811},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 542, col: 39, offset: 14813},
							val:        "|",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 542, col: 43, offset: 14817},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 542, col: 45, offset: 14819},
							label: "body",
							expr: &ruleRefExpr{
								pos:  position{line: 542, col: 50, offset: 14824},
								name: "WhitespaceBody",
							},
						},
						&ruleRefExpr{
							pos:  position{line: 542, col: 65, offset: 14839},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 542, col: 67, offset: 14841},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Composite",
			pos:  position{line: 548, col: 1, offset: 14964},
			expr: &choiceExpr{
				pos: position{line: 548, col: 14, offset: 14977},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 548, col: 14, offset: 14977},
						name: "Object",
					},
					&ruleRefExpr{
						pos:  position{line: 548, col: 23, offset: 14986},
						name: "Array",
					},
					&ruleRefExpr{
						pos:  position{line: 548, col: 31, offset: 14994},
						name: "Set",
					},
				},
			},
		},
		{
			name: "Scalar",
			pos:  position{line: 550, col: 1, offset: 14999},
			expr: &choiceExpr{
				pos: position{line: 550, col: 11, offset: 15009},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 550, col: 11, offset: 15009},
						name: "Number",
					},
					&ruleRefExpr{
						pos:  position{line: 550, col: 20, offset: 15018},
						name: "String",
					},
					&ruleRefExpr{
						pos:  position{line: 550, col: 29, offset: 15027},
						name: "Bool",
					},
					&ruleRefExpr{
						pos:  position{line: 550, col: 36, offset: 15034},
						name: "Null",
					},
				},
			},
		},
		{
			name: "Key",
			pos:  position{line: 552, col: 1, offset: 15040},
			expr: &choiceExpr{
				pos: position{line: 552, col: 8, offset: 15047},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 552, col: 8, offset: 15047},
						name: "Scalar",
					},
					&ruleRefExpr{
						pos:  position{line: 552, col: 17, offset: 15056},
						name: "Ref",
					},
					&ruleRefExpr{
						pos:  position{line: 552, col: 23, offset: 15062},
						name: "Var",
					},
				},
			},
		},
		{
			name: "Object",
			pos:  position{line: 554, col: 1, offset: 15067},
			expr: &actionExpr{
				pos: position{line: 554, col: 11, offset: 15077},
				run: (*parser).callonObject1,
				expr: &seqExpr{
					pos: position{line: 554, col: 11, offset: 15077},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 554, col: 11, offset: 15077},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 554, col: 15, offset: 15081},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 554, col: 17, offset: 15083},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 554, col: 22, offset: 15088},
								expr: &seqExpr{
									pos: position{line: 554, col: 23, offset: 15089},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 554, col: 23, offset: 15089},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 27, offset: 15093},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 554, col: 29, offset: 15095},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 33, offset: 15099},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 35, offset: 15101},
											name: "Term",
										},
									},
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 554, col: 42, offset: 15108},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 554, col: 47, offset: 15113},
								expr: &seqExpr{
									pos: position{line: 554, col: 49, offset: 15115},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 554, col: 49, offset: 15115},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 554, col: 51, offset: 15117},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 55, offset: 15121},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 57, offset: 15123},
											name: "Key",
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 61, offset: 15127},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 554, col: 63, offset: 15129},
											val:        ":",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 67, offset: 15133},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 554, col: 69, offset: 15135},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 554, col: 77, offset: 15143},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 554, col: 79, offset: 15145},
							expr: &litMatcher{
								pos:        position{line: 554, col: 79, offset: 15145},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 554, col: 84, offset: 15150},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 554, col: 86, offset: 15152},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Array",
			pos:  position{line: 558, col: 1, offset: 15215},
			expr: &actionExpr{
				pos: position{line: 558, col: 10, offset: 15224},
				run: (*parser).callonArray1,
				expr: &seqExpr{
					pos: position{line: 558, col: 10, offset: 15224},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 558, col: 10, offset: 15224},
							val:        "[",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 558, col: 14, offset: 15228},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 558, col: 17, offset: 15231},
							label: "head",
							expr: &zeroOrOneExpr{
								pos: position{line: 558, col: 22, offset: 15236},
								expr: &ruleRefExpr{
									pos:  position{line: 558, col: 22, offset: 15236},
									name: "Term",
								},
							},
						},
						&labeledExpr{
							pos:   position{line: 558, col: 28, offset: 15242},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 558, col: 33, offset: 15247},
								expr: &seqExpr{
									pos: position{line: 558, col: 34, offset: 15248},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 558, col: 34, offset: 15248},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 558, col: 36, offset: 15250},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 558, col: 40, offset: 15254},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 558, col: 42, offset: 15256},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 558, col: 49, offset: 15263},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 558, col: 51, offset: 15265},
							expr: &litMatcher{
								pos:        position{line: 558, col: 51, offset: 15265},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 558, col: 56, offset: 15270},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 558, col: 59, offset: 15273},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Set",
			pos:  position{line: 562, col: 1, offset: 15335},
			expr: &choiceExpr{
				pos: position{line: 562, col: 8, offset: 15342},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 562, col: 8, offset: 15342},
						name: "SetEmpty",
					},
					&ruleRefExpr{
						pos:  position{line: 562, col: 19, offset: 15353},
						name: "SetNonEmpty",
					},
				},
			},
		},
		{
			name: "SetEmpty",
			pos:  position{line: 564, col: 1, offset: 15366},
			expr: &actionExpr{
				pos: position{line: 564, col: 13, offset: 15378},
				run: (*parser).callonSetEmpty1,
				expr: &seqExpr{
					pos: position{line: 564, col: 13, offset: 15378},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 564, col: 13, offset: 15378},
							val:        "set(",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 564, col: 20, offset: 15385},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 564, col: 22, offset: 15387},
							val:        ")",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "SetNonEmpty",
			pos:  position{line: 570, col: 1, offset: 15475},
			expr: &actionExpr{
				pos: position{line: 570, col: 16, offset: 15490},
				run: (*parser).callonSetNonEmpty1,
				expr: &seqExpr{
					pos: position{line: 570, col: 16, offset: 15490},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 570, col: 16, offset: 15490},
							val:        "{",
							ignoreCase: false,
						},
						&ruleRefExpr{
							pos:  position{line: 570, col: 20, offset: 15494},
							name: "_",
						},
						&labeledExpr{
							pos:   position{line: 570, col: 22, offset: 15496},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 570, col: 27, offset: 15501},
								name: "Term",
							},
						},
						&labeledExpr{
							pos:   position{line: 570, col: 32, offset: 15506},
							label: "tail",
							expr: &zeroOrMoreExpr{
								pos: position{line: 570, col: 37, offset: 15511},
								expr: &seqExpr{
									pos: position{line: 570, col: 38, offset: 15512},
									exprs: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 570, col: 38, offset: 15512},
											name: "_",
										},
										&litMatcher{
											pos:        position{line: 570, col: 40, offset: 15514},
											val:        ",",
											ignoreCase: false,
										},
										&ruleRefExpr{
											pos:  position{line: 570, col: 44, offset: 15518},
											name: "_",
										},
										&ruleRefExpr{
											pos:  position{line: 570, col: 46, offset: 15520},
											name: "Term",
										},
									},
								},
							},
						},
						&ruleRefExpr{
							pos:  position{line: 570, col: 53, offset: 15527},
							name: "_",
						},
						&zeroOrOneExpr{
							pos: position{line: 570, col: 55, offset: 15529},
							expr: &litMatcher{
								pos:        position{line: 570, col: 55, offset: 15529},
								val:        ",",
								ignoreCase: false,
							},
						},
						&ruleRefExpr{
							pos:  position{line: 570, col: 60, offset: 15534},
							name: "_",
						},
						&litMatcher{
							pos:        position{line: 570, col: 62, offset: 15536},
							val:        "}",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Ref",
			pos:  position{line: 587, col: 1, offset: 15940},
			expr: &actionExpr{
				pos: position{line: 587, col: 8, offset: 15947},
				run: (*parser).callonRef1,
				expr: &seqExpr{
					pos: position{line: 587, col: 8, offset: 15947},
					exprs: []interface{}{
						&labeledExpr{
							pos:   position{line: 587, col: 8, offset: 15947},
							label: "head",
							expr: &ruleRefExpr{
								pos:  position{line: 587, col: 13, offset: 15952},
								name: "Var",
							},
						},
						&labeledExpr{
							pos:   position{line: 587, col: 17, offset: 15956},
							label: "tail",
							expr: &oneOrMoreExpr{
								pos: position{line: 587, col: 22, offset: 15961},
								expr: &choiceExpr{
									pos: position{line: 587, col: 24, offset: 15963},
									alternatives: []interface{}{
										&ruleRefExpr{
											pos:  position{line: 587, col: 24, offset: 15963},
											name: "RefDot",
										},
										&ruleRefExpr{
											pos:  position{line: 587, col: 33, offset: 15972},
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
			pos:  position{line: 600, col: 1, offset: 16211},
			expr: &actionExpr{
				pos: position{line: 600, col: 11, offset: 16221},
				run: (*parser).callonRefDot1,
				expr: &seqExpr{
					pos: position{line: 600, col: 11, offset: 16221},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 600, col: 11, offset: 16221},
							val:        ".",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 600, col: 15, offset: 16225},
							label: "val",
							expr: &ruleRefExpr{
								pos:  position{line: 600, col: 19, offset: 16229},
								name: "Var",
							},
						},
					},
				},
			},
		},
		{
			name: "RefBracket",
			pos:  position{line: 607, col: 1, offset: 16448},
			expr: &actionExpr{
				pos: position{line: 607, col: 15, offset: 16462},
				run: (*parser).callonRefBracket1,
				expr: &seqExpr{
					pos: position{line: 607, col: 15, offset: 16462},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 607, col: 15, offset: 16462},
							val:        "[",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 607, col: 19, offset: 16466},
							label: "val",
							expr: &choiceExpr{
								pos: position{line: 607, col: 24, offset: 16471},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 607, col: 24, offset: 16471},
										name: "Composite",
									},
									&ruleRefExpr{
										pos:  position{line: 607, col: 36, offset: 16483},
										name: "Ref",
									},
									&ruleRefExpr{
										pos:  position{line: 607, col: 42, offset: 16489},
										name: "Scalar",
									},
									&ruleRefExpr{
										pos:  position{line: 607, col: 51, offset: 16498},
										name: "Var",
									},
								},
							},
						},
						&litMatcher{
							pos:        position{line: 607, col: 56, offset: 16503},
							val:        "]",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Var",
			pos:  position{line: 611, col: 1, offset: 16532},
			expr: &actionExpr{
				pos: position{line: 611, col: 8, offset: 16539},
				run: (*parser).callonVar1,
				expr: &labeledExpr{
					pos:   position{line: 611, col: 8, offset: 16539},
					label: "val",
					expr: &ruleRefExpr{
						pos:  position{line: 611, col: 12, offset: 16543},
						name: "VarChecked",
					},
				},
			},
		},
		{
			name: "VarChecked",
			pos:  position{line: 616, col: 1, offset: 16665},
			expr: &seqExpr{
				pos: position{line: 616, col: 15, offset: 16679},
				exprs: []interface{}{
					&labeledExpr{
						pos:   position{line: 616, col: 15, offset: 16679},
						label: "val",
						expr: &ruleRefExpr{
							pos:  position{line: 616, col: 19, offset: 16683},
							name: "VarUnchecked",
						},
					},
					&notCodeExpr{
						pos: position{line: 616, col: 32, offset: 16696},
						run: (*parser).callonVarChecked4,
					},
				},
			},
		},
		{
			name: "VarUnchecked",
			pos:  position{line: 620, col: 1, offset: 16761},
			expr: &actionExpr{
				pos: position{line: 620, col: 17, offset: 16777},
				run: (*parser).callonVarUnchecked1,
				expr: &seqExpr{
					pos: position{line: 620, col: 17, offset: 16777},
					exprs: []interface{}{
						&ruleRefExpr{
							pos:  position{line: 620, col: 17, offset: 16777},
							name: "AsciiLetter",
						},
						&zeroOrMoreExpr{
							pos: position{line: 620, col: 29, offset: 16789},
							expr: &choiceExpr{
								pos: position{line: 620, col: 30, offset: 16790},
								alternatives: []interface{}{
									&ruleRefExpr{
										pos:  position{line: 620, col: 30, offset: 16790},
										name: "AsciiLetter",
									},
									&ruleRefExpr{
										pos:  position{line: 620, col: 44, offset: 16804},
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
			pos:  position{line: 627, col: 1, offset: 16947},
			expr: &actionExpr{
				pos: position{line: 627, col: 11, offset: 16957},
				run: (*parser).callonNumber1,
				expr: &seqExpr{
					pos: position{line: 627, col: 11, offset: 16957},
					exprs: []interface{}{
						&zeroOrOneExpr{
							pos: position{line: 627, col: 11, offset: 16957},
							expr: &litMatcher{
								pos:        position{line: 627, col: 11, offset: 16957},
								val:        "-",
								ignoreCase: false,
							},
						},
						&choiceExpr{
							pos: position{line: 627, col: 18, offset: 16964},
							alternatives: []interface{}{
								&ruleRefExpr{
									pos:  position{line: 627, col: 18, offset: 16964},
									name: "Float",
								},
								&ruleRefExpr{
									pos:  position{line: 627, col: 26, offset: 16972},
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
			pos:  position{line: 640, col: 1, offset: 17363},
			expr: &choiceExpr{
				pos: position{line: 640, col: 10, offset: 17372},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 640, col: 10, offset: 17372},
						name: "ExponentFloat",
					},
					&ruleRefExpr{
						pos:  position{line: 640, col: 26, offset: 17388},
						name: "PointFloat",
					},
				},
			},
		},
		{
			name: "ExponentFloat",
			pos:  position{line: 642, col: 1, offset: 17400},
			expr: &seqExpr{
				pos: position{line: 642, col: 18, offset: 17417},
				exprs: []interface{}{
					&choiceExpr{
						pos: position{line: 642, col: 20, offset: 17419},
						alternatives: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 642, col: 20, offset: 17419},
								name: "PointFloat",
							},
							&ruleRefExpr{
								pos:  position{line: 642, col: 33, offset: 17432},
								name: "Integer",
							},
						},
					},
					&ruleRefExpr{
						pos:  position{line: 642, col: 43, offset: 17442},
						name: "Exponent",
					},
				},
			},
		},
		{
			name: "PointFloat",
			pos:  position{line: 644, col: 1, offset: 17452},
			expr: &seqExpr{
				pos: position{line: 644, col: 15, offset: 17466},
				exprs: []interface{}{
					&zeroOrOneExpr{
						pos: position{line: 644, col: 15, offset: 17466},
						expr: &ruleRefExpr{
							pos:  position{line: 644, col: 15, offset: 17466},
							name: "Integer",
						},
					},
					&ruleRefExpr{
						pos:  position{line: 644, col: 24, offset: 17475},
						name: "Fraction",
					},
				},
			},
		},
		{
			name: "Fraction",
			pos:  position{line: 646, col: 1, offset: 17485},
			expr: &seqExpr{
				pos: position{line: 646, col: 13, offset: 17497},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 646, col: 13, offset: 17497},
						val:        ".",
						ignoreCase: false,
					},
					&oneOrMoreExpr{
						pos: position{line: 646, col: 17, offset: 17501},
						expr: &ruleRefExpr{
							pos:  position{line: 646, col: 17, offset: 17501},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Exponent",
			pos:  position{line: 648, col: 1, offset: 17516},
			expr: &seqExpr{
				pos: position{line: 648, col: 13, offset: 17528},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 648, col: 13, offset: 17528},
						val:        "e",
						ignoreCase: true,
					},
					&zeroOrOneExpr{
						pos: position{line: 648, col: 18, offset: 17533},
						expr: &charClassMatcher{
							pos:        position{line: 648, col: 18, offset: 17533},
							val:        "[+-]",
							chars:      []rune{'+', '-'},
							ignoreCase: false,
							inverted:   false,
						},
					},
					&oneOrMoreExpr{
						pos: position{line: 648, col: 24, offset: 17539},
						expr: &ruleRefExpr{
							pos:  position{line: 648, col: 24, offset: 17539},
							name: "DecimalDigit",
						},
					},
				},
			},
		},
		{
			name: "Integer",
			pos:  position{line: 650, col: 1, offset: 17554},
			expr: &choiceExpr{
				pos: position{line: 650, col: 12, offset: 17565},
				alternatives: []interface{}{
					&litMatcher{
						pos:        position{line: 650, col: 12, offset: 17565},
						val:        "0",
						ignoreCase: false,
					},
					&seqExpr{
						pos: position{line: 650, col: 20, offset: 17573},
						exprs: []interface{}{
							&ruleRefExpr{
								pos:  position{line: 650, col: 20, offset: 17573},
								name: "NonZeroDecimalDigit",
							},
							&zeroOrMoreExpr{
								pos: position{line: 650, col: 40, offset: 17593},
								expr: &ruleRefExpr{
									pos:  position{line: 650, col: 40, offset: 17593},
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
			pos:  position{line: 652, col: 1, offset: 17610},
			expr: &choiceExpr{
				pos: position{line: 652, col: 11, offset: 17620},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 652, col: 11, offset: 17620},
						name: "QuotedString",
					},
					&ruleRefExpr{
						pos:  position{line: 652, col: 26, offset: 17635},
						name: "RawString",
					},
				},
			},
		},
		{
			name: "QuotedString",
			pos:  position{line: 654, col: 1, offset: 17646},
			expr: &actionExpr{
				pos: position{line: 654, col: 17, offset: 17662},
				run: (*parser).callonQuotedString1,
				expr: &seqExpr{
					pos: position{line: 654, col: 17, offset: 17662},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 654, col: 17, offset: 17662},
							val:        "\"",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 654, col: 21, offset: 17666},
							expr: &ruleRefExpr{
								pos:  position{line: 654, col: 21, offset: 17666},
								name: "Char",
							},
						},
						&litMatcher{
							pos:        position{line: 654, col: 27, offset: 17672},
							val:        "\"",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "RawString",
			pos:  position{line: 662, col: 1, offset: 17827},
			expr: &actionExpr{
				pos: position{line: 662, col: 14, offset: 17840},
				run: (*parser).callonRawString1,
				expr: &seqExpr{
					pos: position{line: 662, col: 14, offset: 17840},
					exprs: []interface{}{
						&litMatcher{
							pos:        position{line: 662, col: 14, offset: 17840},
							val:        "`",
							ignoreCase: false,
						},
						&zeroOrMoreExpr{
							pos: position{line: 662, col: 18, offset: 17844},
							expr: &charClassMatcher{
								pos:        position{line: 662, col: 18, offset: 17844},
								val:        "[^`]",
								chars:      []rune{'`'},
								ignoreCase: false,
								inverted:   true,
							},
						},
						&litMatcher{
							pos:        position{line: 662, col: 24, offset: 17850},
							val:        "`",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Bool",
			pos:  position{line: 671, col: 1, offset: 18017},
			expr: &choiceExpr{
				pos: position{line: 671, col: 9, offset: 18025},
				alternatives: []interface{}{
					&actionExpr{
						pos: position{line: 671, col: 9, offset: 18025},
						run: (*parser).callonBool2,
						expr: &litMatcher{
							pos:        position{line: 671, col: 9, offset: 18025},
							val:        "true",
							ignoreCase: false,
						},
					},
					&actionExpr{
						pos: position{line: 675, col: 5, offset: 18125},
						run: (*parser).callonBool4,
						expr: &litMatcher{
							pos:        position{line: 675, col: 5, offset: 18125},
							val:        "false",
							ignoreCase: false,
						},
					},
				},
			},
		},
		{
			name: "Null",
			pos:  position{line: 681, col: 1, offset: 18226},
			expr: &actionExpr{
				pos: position{line: 681, col: 9, offset: 18234},
				run: (*parser).callonNull1,
				expr: &litMatcher{
					pos:        position{line: 681, col: 9, offset: 18234},
					val:        "null",
					ignoreCase: false,
				},
			},
		},
		{
			name: "AsciiLetter",
			pos:  position{line: 687, col: 1, offset: 18329},
			expr: &charClassMatcher{
				pos:        position{line: 687, col: 16, offset: 18344},
				val:        "[A-Za-z_]",
				chars:      []rune{'_'},
				ranges:     []rune{'A', 'Z', 'a', 'z'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "Char",
			pos:  position{line: 689, col: 1, offset: 18355},
			expr: &choiceExpr{
				pos: position{line: 689, col: 9, offset: 18363},
				alternatives: []interface{}{
					&seqExpr{
						pos: position{line: 689, col: 11, offset: 18365},
						exprs: []interface{}{
							&notExpr{
								pos: position{line: 689, col: 11, offset: 18365},
								expr: &ruleRefExpr{
									pos:  position{line: 689, col: 12, offset: 18366},
									name: "EscapedChar",
								},
							},
							&anyMatcher{
								line: 689, col: 24, offset: 18378,
							},
						},
					},
					&seqExpr{
						pos: position{line: 689, col: 32, offset: 18386},
						exprs: []interface{}{
							&litMatcher{
								pos:        position{line: 689, col: 32, offset: 18386},
								val:        "\\",
								ignoreCase: false,
							},
							&ruleRefExpr{
								pos:  position{line: 689, col: 37, offset: 18391},
								name: "EscapeSequence",
							},
						},
					},
				},
			},
		},
		{
			name: "EscapedChar",
			pos:  position{line: 691, col: 1, offset: 18409},
			expr: &charClassMatcher{
				pos:        position{line: 691, col: 16, offset: 18424},
				val:        "[\\x00-\\x1f\"\\\\]",
				chars:      []rune{'"', '\\'},
				ranges:     []rune{'\x00', '\x1f'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "EscapeSequence",
			pos:  position{line: 693, col: 1, offset: 18440},
			expr: &choiceExpr{
				pos: position{line: 693, col: 19, offset: 18458},
				alternatives: []interface{}{
					&ruleRefExpr{
						pos:  position{line: 693, col: 19, offset: 18458},
						name: "SingleCharEscape",
					},
					&ruleRefExpr{
						pos:  position{line: 693, col: 38, offset: 18477},
						name: "UnicodeEscape",
					},
				},
			},
		},
		{
			name: "SingleCharEscape",
			pos:  position{line: 695, col: 1, offset: 18492},
			expr: &charClassMatcher{
				pos:        position{line: 695, col: 21, offset: 18512},
				val:        "[ \" \\\\ / b f n r t ]",
				chars:      []rune{' ', '"', ' ', '\\', ' ', '/', ' ', 'b', ' ', 'f', ' ', 'n', ' ', 'r', ' ', 't', ' '},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "UnicodeEscape",
			pos:  position{line: 697, col: 1, offset: 18534},
			expr: &seqExpr{
				pos: position{line: 697, col: 18, offset: 18551},
				exprs: []interface{}{
					&litMatcher{
						pos:        position{line: 697, col: 18, offset: 18551},
						val:        "u",
						ignoreCase: false,
					},
					&ruleRefExpr{
						pos:  position{line: 697, col: 22, offset: 18555},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 697, col: 31, offset: 18564},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 697, col: 40, offset: 18573},
						name: "HexDigit",
					},
					&ruleRefExpr{
						pos:  position{line: 697, col: 49, offset: 18582},
						name: "HexDigit",
					},
				},
			},
		},
		{
			name: "DecimalDigit",
			pos:  position{line: 699, col: 1, offset: 18592},
			expr: &charClassMatcher{
				pos:        position{line: 699, col: 17, offset: 18608},
				val:        "[0-9]",
				ranges:     []rune{'0', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "NonZeroDecimalDigit",
			pos:  position{line: 701, col: 1, offset: 18615},
			expr: &charClassMatcher{
				pos:        position{line: 701, col: 24, offset: 18638},
				val:        "[1-9]",
				ranges:     []rune{'1', '9'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name: "HexDigit",
			pos:  position{line: 703, col: 1, offset: 18645},
			expr: &charClassMatcher{
				pos:        position{line: 703, col: 13, offset: 18657},
				val:        "[0-9a-fA-F]",
				ranges:     []rune{'0', '9', 'a', 'f', 'A', 'F'},
				ignoreCase: false,
				inverted:   false,
			},
		},
		{
			name:        "ws",
			displayName: "\"whitespace\"",
			pos:         position{line: 705, col: 1, offset: 18670},
			expr: &oneOrMoreExpr{
				pos: position{line: 705, col: 20, offset: 18689},
				expr: &charClassMatcher{
					pos:        position{line: 705, col: 20, offset: 18689},
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
			pos:         position{line: 707, col: 1, offset: 18701},
			expr: &zeroOrMoreExpr{
				pos: position{line: 707, col: 19, offset: 18719},
				expr: &choiceExpr{
					pos: position{line: 707, col: 21, offset: 18721},
					alternatives: []interface{}{
						&charClassMatcher{
							pos:        position{line: 707, col: 21, offset: 18721},
							val:        "[ \\t\\r\\n]",
							chars:      []rune{' ', '\t', '\r', '\n'},
							ignoreCase: false,
							inverted:   false,
						},
						&ruleRefExpr{
							pos:  position{line: 707, col: 33, offset: 18733},
							name: "Comment",
						},
					},
				},
			},
		},
		{
			name: "Comment",
			pos:  position{line: 709, col: 1, offset: 18745},
			expr: &actionExpr{
				pos: position{line: 709, col: 12, offset: 18756},
				run: (*parser).callonComment1,
				expr: &seqExpr{
					pos: position{line: 709, col: 12, offset: 18756},
					exprs: []interface{}{
						&zeroOrMoreExpr{
							pos: position{line: 709, col: 12, offset: 18756},
							expr: &charClassMatcher{
								pos:        position{line: 709, col: 12, offset: 18756},
								val:        "[ \\t]",
								chars:      []rune{' ', '\t'},
								ignoreCase: false,
								inverted:   false,
							},
						},
						&litMatcher{
							pos:        position{line: 709, col: 19, offset: 18763},
							val:        "#",
							ignoreCase: false,
						},
						&labeledExpr{
							pos:   position{line: 709, col: 23, offset: 18767},
							label: "text",
							expr: &zeroOrMoreExpr{
								pos: position{line: 709, col: 28, offset: 18772},
								expr: &charClassMatcher{
									pos:        position{line: 709, col: 28, offset: 18772},
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
			pos:  position{line: 720, col: 1, offset: 19048},
			expr: &notExpr{
				pos: position{line: 720, col: 8, offset: 19055},
				expr: &anyMatcher{
					line: 720, col: 9, offset: 19056,
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
		case *ArrayComprehension, *ObjectComprehension, *SetComprehension: // skip closures
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
					Args:     prev.Head.Args.Copy(),
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

func (c *current) onRuleHead1(name, args, key, value interface{}) (interface{}, error) {

	head := &Head{}

	head.Location = currentLocation(c)
	head.Name = name.(*Term).Value.(Var)

	if args != nil && key != nil {
		return nil, fmt.Errorf("partial %v/%v %vs cannot take arguments", SetTypeName, ObjectTypeName, RuleTypeName)
	}

	if args != nil {
		argSlice := args.([]interface{})
		head.Args = argSlice[3].(Args)
	}

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
		head.Value = BooleanTerm(true).SetLocation(head.Location)
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
	return p.cur.onRuleHead1(stack["name"], stack["args"], stack["key"], stack["value"])
}

func (c *current) onArgs1(head, tail interface{}) (interface{}, error) {
	return makeArgs(head, tail, currentLocation(c))
}

func (p *parser) callonArgs1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArgs1(stack["head"], stack["tail"])
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
	var expr *Expr
	switch val := val.(type) {
	case *Expr:
		expr = val
	case *Term:
		expr = &Expr{Terms: val}
	}
	expr.Location = currentLocation(c)
	expr.Negated = neg != nil

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

func (c *current) onInfixCallExpr1(output, operator, args interface{}) (interface{}, error) {
	return makeInfixCallExpr(operator, args, output)
}

func (p *parser) callonInfixCallExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixCallExpr1(stack["output"], stack["operator"], stack["args"])
}

func (c *current) onInfixCallExprReverse1(operator, args, output interface{}) (interface{}, error) {
	return makeInfixCallExpr(operator, args, output)
}

func (p *parser) callonInfixCallExprReverse1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixCallExprReverse1(stack["operator"], stack["args"], stack["output"])
}

func (c *current) onInfixArithExpr1(output, left, operator, right interface{}) (interface{}, error) {
	return makeInfixCallExpr(operator, Args{left.(*Term), right.(*Term)}, output)
}

func (p *parser) callonInfixArithExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixArithExpr1(stack["output"], stack["left"], stack["operator"], stack["right"])
}

func (c *current) onInfixArithExprReverse1(left, operator, right, output interface{}) (interface{}, error) {
	return makeInfixCallExpr(operator, Args{left.(*Term), right.(*Term)}, output)
}

func (p *parser) callonInfixArithExprReverse1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixArithExprReverse1(stack["left"], stack["operator"], stack["right"], stack["output"])
}

func (c *current) onArithInfixOp1(val interface{}) (interface{}, error) {
	op := string(c.text)
	for _, b := range Builtins {
		if string(b.Infix) == op {
			op = string(b.Name)
		}
	}
	loc := currentLocation(c)
	operator := RefTerm(VarTerm(op).SetLocation(loc)).SetLocation(loc)
	return operator, nil
}

func (p *parser) callonArithInfixOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onArithInfixOp1(stack["val"])
}

func (c *current) onInfixRelationExpr1(left, operator, right interface{}) (interface{}, error) {
	return &Expr{
		Terms: []*Term{
			operator.(*Term),
			left.(*Term),
			right.(*Term),
		},
		Infix: true,
	}, nil
}

func (p *parser) callonInfixRelationExpr1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixRelationExpr1(stack["left"], stack["operator"], stack["right"])
}

func (c *current) onInfixRelationOp1(val interface{}) (interface{}, error) {
	op := string(c.text)
	for _, b := range Builtins {
		if string(b.Infix) == op {
			op = string(b.Name)
		}
	}
	loc := currentLocation(c)
	operator := RefTerm(VarTerm(op).SetLocation(loc)).SetLocation(loc)
	return operator, nil
}

func (p *parser) callonInfixRelationOp1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onInfixRelationOp1(stack["val"])
}

func (c *current) onCall1(name, head, tail interface{}) (interface{}, error) {
	buf := []*Term{name.(*Term)}
	if head == nil {
		return &Expr{Terms: buf}, nil
	}

	buf = append(buf, head.(*Term))

	// PrefixExpr above describes the "tail" structure. We only care about the "Term" elements.
	tailSlice := tail.([]interface{})
	for _, v := range tailSlice {
		s := v.([]interface{})
		buf = append(buf, s[len(s)-1].(*Term))
	}

	return &Expr{Terms: buf}, nil
}

func (p *parser) callonCall1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onCall1(stack["name"], stack["head"], stack["tail"])
}

func (c *current) onOperator1(val interface{}) (interface{}, error) {
	term := val.(*Term)
	switch term.Value.(type) {
	case Ref:
		return val, nil
	case Var:
		return RefTerm(term).SetLocation(currentLocation(c)), nil
	default:
		panic("unreachable")
	}
}

func (p *parser) callonOperator1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onOperator1(stack["val"])
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

func (c *current) onObjectComprehension1(key, value, body interface{}) (interface{}, error) {
	oc := ObjectComprehensionTerm(key.(*Term), value.(*Term), body.(Body))
	oc.Location = currentLocation(c)
	return oc, nil
}

func (p *parser) callonObjectComprehension1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObjectComprehension1(stack["key"], stack["value"], stack["body"])
}

func (c *current) onSetComprehension1(term, body interface{}) (interface{}, error) {
	sc := SetComprehensionTerm(term.(*Term), body.(Body))
	sc.Location = currentLocation(c)
	return sc, nil
}

func (p *parser) callonSetComprehension1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onSetComprehension1(stack["term"], stack["body"])
}

func (c *current) onObject1(head, tail interface{}) (interface{}, error) {
	return makeObject(head, tail, currentLocation(c))
}

func (p *parser) callonObject1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onObject1(stack["head"], stack["tail"])
}

func (c *current) onArray1(head, tail interface{}) (interface{}, error) {
	return makeArray(head, tail, currentLocation(c))
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

	val := set.Value.(Set)
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

func (c *current) onQuotedString1() (interface{}, error) {
	var v string
	err := json.Unmarshal([]byte(c.text), &v)
	str := StringTerm(v)
	str.Location = currentLocation(c)
	return str, err
}

func (p *parser) callonQuotedString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onQuotedString1()
}

func (c *current) onRawString1() (interface{}, error) {
	s := string(c.text)
	s = s[1 : len(s)-1] // Trim surrounding quotes.

	str := StringTerm(s)
	str.Location = currentLocation(c)
	return str, nil
}

func (p *parser) callonRawString1() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.onRawString1()
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

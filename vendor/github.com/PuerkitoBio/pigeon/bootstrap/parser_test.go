package bootstrap

import (
	"strings"
	"testing"
)

var parseValidCases = []string{
	"",
	"\n",
	"\n{code}",
	"\nR <- 'c'",
	"\n\nR <- 'c'\n\n",
	`
A = ident:B / C+ / D?;`,
	`{ code }

R "name" <- "abc"i
R2 = 'd'i
R3 = ( R2+ ![;] )`,
}

var parseExpRes = []string{
	`1:0 (0): *ast.Grammar{Init: <nil>, Rules: [
]}`,
	`2:0 (0): *ast.Grammar{Init: <nil>, Rules: [
]}`,
	`2:0 (0): *ast.Grammar{Init: 2:1 (1): *ast.CodeBlock{Val: "{code}"}, Rules: [
]}`,
	`2:0 (0): *ast.Grammar{Init: <nil>, Rules: [
2:1 (1): *ast.Rule{Name: 2:1 (1): *ast.Identifier{Val: "R"}, DisplayName: <nil>, Expr: 2:6 (6): *ast.LitMatcher{Val: "c", IgnoreCase: false}},
]}`,
	`2:0 (0): *ast.Grammar{Init: <nil>, Rules: [
3:1 (2): *ast.Rule{Name: 3:1 (2): *ast.Identifier{Val: "R"}, DisplayName: <nil>, Expr: 3:6 (7): *ast.LitMatcher{Val: "c", IgnoreCase: false}},
]}`,
	`2:0 (0): *ast.Grammar{Init: <nil>, Rules: [
2:1 (1): *ast.Rule{Name: 2:1 (1): *ast.Identifier{Val: "A"}, DisplayName: <nil>, Expr: 2:5 (5): *ast.ChoiceExpr{Alternatives: [
2:5 (5): *ast.LabeledExpr{Label: 2:5 (5): *ast.Identifier{Val: "ident"}, Expr: 2:11 (11): *ast.RuleRefExpr{Name: 2:11 (11): *ast.Identifier{Val: "B"}}},
2:15 (15): *ast.OneOrMoreExpr{Expr: 2:15 (15): *ast.RuleRefExpr{Name: 2:15 (15): *ast.Identifier{Val: "C"}}},
2:20 (20): *ast.ZeroOrOneExpr{Expr: 2:20 (20): *ast.RuleRefExpr{Name: 2:20 (20): *ast.Identifier{Val: "D"}}},
]}},
]}`,
	`1:1 (0): *ast.Grammar{Init: 1:1 (0): *ast.CodeBlock{Val: "{ code }"}, Rules: [
3:1 (10): *ast.Rule{Name: 3:1 (10): *ast.Identifier{Val: "R"}, DisplayName: 3:3 (12): *ast.StringLit{Val: "name"}, Expr: 3:13 (22): *ast.LitMatcher{Val: "abc", IgnoreCase: true}},
4:1 (29): *ast.Rule{Name: 4:1 (29): *ast.Identifier{Val: "R2"}, DisplayName: <nil>, Expr: 4:6 (34): *ast.LitMatcher{Val: "d", IgnoreCase: true}},
5:1 (39): *ast.Rule{Name: 5:1 (39): *ast.Identifier{Val: "R3"}, DisplayName: <nil>, Expr: 5:8 (46): *ast.SeqExpr{Exprs: [
5:8 (46): *ast.OneOrMoreExpr{Expr: 5:8 (46): *ast.RuleRefExpr{Name: 5:8 (46): *ast.Identifier{Val: "R2"}}},
5:12 (50): *ast.NotExpr{Expr: 5:13 (51): *ast.CharClassMatcher{Val: "[;]", IgnoreCase: false, Inverted: false}},
]}},
]}`,
}

func TestParseValid(t *testing.T) {
	p := NewParser()
	for i, c := range parseValidCases {
		g, err := p.Parse("", strings.NewReader(c))
		if err != nil {
			t.Errorf("%d: got error %v", i, err)
			continue
		}

		want := parseExpRes[i]
		got := g.String()
		if want != got {
			t.Errorf("%d: want \n%s\n, got \n%s\n", i, want, got)
		}
	}
}

var parseInvalidCases = []string{
	"a",
	`R = )`,
}

var parseExpErrs = [][]string{
	{"1:1 (0): expected ruledef, got eof"},
	{"1:5 (4): no expression in sequence", "1:5 (4): no expression in choice", "1:5 (4): missing expression"},
}

func TestParseInvalid(t *testing.T) {
	p := NewParser()
	for i, c := range parseInvalidCases {
		_, err := p.Parse("", strings.NewReader(c))
		el := *(err.(*errList))
		if len(el) != len(parseExpErrs[i]) {
			t.Errorf("%d: want %d errors, got %d", i, len(parseExpErrs[i]), len(el))
			continue
		}
		for j, err := range el {
			want := parseExpErrs[i][j]
			got := err.Error()
			if want != got {
				t.Errorf("%d: error %d: want %q, got %q", i, j, want, got)
			}
		}
	}
}

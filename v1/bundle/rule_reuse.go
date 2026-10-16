package bundle

import (
	"bytes"

	"github.com/open-policy-agent/opa/v1/ast"
)

// reuseModuleParser puts rules the current compiler has compiled from the same
// source in place of parsing them. A statement the scan cuts wrong matches no
// compiled rule, and is parsed.
func reuseModuleParser(current func() *ast.Compiler) moduleParser {
	return func(path string, raw []byte, opts ast.ParserOptions) (*ast.Module, error) {
		full := func() (*ast.Module, error) { return ast.ParseModuleWithOpts(path, string(raw), opts) }

		// Document-scoped annotations attach to rules in other statements too.
		c := current()
		if c == nil || bytes.Contains(raw, []byte("scope: document")) {
			return full()
		}

		lines := bytes.SplitAfter(raw, []byte("\n"))
		skip := make([]bool, len(lines))
		var pkg string
		var reused []*ast.Rule
		for _, s := range statements(lines) {
			rule := bytes.TrimSpace(bytes.Join(lines[s.start:s.end], nil))
			if pkg == "" {
				if p, err := ast.ParsePackage(string(rule)); err == nil {
					pkg = p.Path.String()
				}
				continue
			}
			texts := [][]byte{rule}
			if s.annotation < s.start && opts.ProcessAnnotation {
				texts = [][]byte{bytes.TrimRight(bytes.Join(lines[s.annotation:s.start], nil), "\r\n"), rule}
			}
			if r := c.ReusableRule(ast.ReuseKey(pkg, texts...)); r != nil {
				reused = append(reused, r)
				for i := s.annotation; i < s.end; i++ {
					skip[i] = true
				}
			}
		}
		if len(reused) == 0 {
			return full()
		}

		// Newlines in place of skipped lines keep the parsed rules' locations.
		out := make([]byte, 0, len(raw))
		for i, l := range lines {
			if !skip[i] {
				out = append(out, l...)
			} else if bytes.HasSuffix(l, []byte("\n")) {
				out = append(out, '\n')
			}
		}
		mod, err := ast.ParseModuleWithOpts(path, string(out), opts)
		if err != nil {
			return full()
		}
		mod.Rules = append(mod.Rules, reused...)
		return mod, nil
	}
}

// A statement's METADATA comment spans lines [annotation, start).
type statement struct {
	annotation, start, end int
}

func statements(lines [][]byte) []statement {
	var out []statement
	depth, inRaw := 0, false
	last := -1 // last line of the current statement
	for i, l := range lines {
		atTop := depth == 0 && !inRaw
		t := bytes.TrimSpace(l)
		depth, inRaw = scanLine(l, depth, inRaw)
		if !atTop || len(t) == 0 || t[0] == '#' {
			if !atTop && len(t) > 0 {
				last = i
			}
			continue
		}
		if bytes.HasPrefix(t, []byte("else")) && (len(t) == 4 || !isIdentByte(t[4])) {
			last = i
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].end = last + 1
		}
		a := i
		for a > 0 && isComment(lines[a-1]) {
			a--
		}
		for a < i && !bytes.HasPrefix(lines[a], []byte("# METADATA")) {
			a++
		}
		out = append(out, statement{annotation: a, start: i})
		last = i
	}
	if len(out) > 0 {
		out[len(out)-1].end = last + 1
	}
	return out
}

func scanLine(l []byte, depth int, inRaw bool) (int, bool) {
	for i := 0; i < len(l); i++ {
		switch b := l[i]; {
		case inRaw:
			inRaw = b != '`'
		case b == '`':
			inRaw = true
		case b == '"':
			for i++; i < len(l) && l[i] != '"'; i++ {
				if l[i] == '\\' {
					i++
				}
			}
		case b == '#':
			return depth, inRaw
		case b == '(' || b == '[' || b == '{':
			depth++
		case b == ')' || b == ']' || b == '}':
			depth--
		}
	}
	return depth, inRaw
}

func isComment(l []byte) bool {
	t := bytes.TrimSpace(l)
	return len(t) > 0 && t[0] == '#'
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

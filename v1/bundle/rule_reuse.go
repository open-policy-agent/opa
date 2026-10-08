// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"iter"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// reuseModuleParser puts rules the current compiler has compiled from the same
// source in place of parsing them. A statement the scan cuts wrong matches no
// compiled rule, and is parsed.
func reuseModuleParser(current func() *ast.Compiler) moduleParser {
	return func(path string, raw []byte, opts ast.ParserOptions) (*ast.Module, error) {
		c := current()
		// Document-scoped annotations attach to rules in other statements too.
		if c == nil || bytes.Contains(raw, []byte("scope: document")) {
			return parseModule(path, raw, opts)
		}

		var out []byte
		var reused []*ast.Rule
		copied := 0
		for s, key := range keyedStatements(raw, opts.ProcessAnnotation) {
			r := c.ReusableRule(key)
			if r == nil {
				continue
			}
			if out == nil {
				out = make([]byte, 0, len(raw))
			}
			// Newlines in place of skipped lines keep the parsed rules' locations.
			out = append(out, raw[copied:s.annotation]...)
			for range bytes.Count(raw[s.annotation:s.end], []byte("\n")) {
				out = append(out, '\n')
			}
			copied = s.end
			reused = append(reused, r)
		}
		if len(reused) == 0 {
			return parseModule(path, raw, opts)
		}
		out = append(out, raw[copied:]...)
		mod, err := parseModule(path, out, opts)
		if err != nil {
			return parseModule(path, raw, opts)
		}
		mod.Rules = append(mod.Rules, reused...)
		return mod, nil
	}
}

// A statement spans the bytes [start, end), its METADATA comment [annotation, start).
type statement struct {
	annotation, start, end int
}

// keyedStatements yields the statements after the package that can be keyed,
// keyed like the compiler keys the rules parsed from them.
func keyedStatements(raw []byte, processAnnotation bool) iter.Seq2[statement, ast.RuleKey] {
	return func(yield func(statement, ast.RuleKey) bool) {
		var pkg string
		var meta []byte
		for s := range statements(raw) {
			rule := bytes.TrimSpace(raw[s.start:s.end])
			if pkg == "" {
				if p, err := ast.ParsePackage(util.ByteSliceToString(rule)); err == nil {
					pkg = p.Path.String()
				}
				continue
			}
			key := ast.ReuseKey(pkg, rule)
			if s.annotation < s.start && processAnnotation {
				var ok bool
				if meta, ok = metadataText(meta, raw[s.annotation:s.start]); !ok {
					continue
				}
				key = ast.ReuseKey(pkg, meta, rule)
			}
			if !yield(s, key) {
				return
			}
		}
	}
}

// metadataText rebuilds the text the parser gives a METADATA block into dst,
// unless the block isn't one to the parser.
func metadataText(dst, block []byte) ([]byte, bool) {
	dst = append(dst[:0], "# METADATA"...)
	_, rest, _ := bytes.Cut(block, []byte("\n"))
	for l := range bytes.Lines(rest) {
		c, ok := bytes.CutPrefix(bytes.TrimSuffix(l, []byte("\n")), []byte("#"))
		if !ok || bytes.HasPrefix(bytes.TrimSpace(c), []byte("METADATA")) {
			return dst, false
		}
		// Carriage returns are trimmed as in ast's commentFromLocText.
		if len(c) == 1 && c[0] == '\r' {
			c = nil
		}
		for len(c) > 1 && c[len(c)-1] == '\r' {
			c = c[:len(c)-1]
		}
		dst = append(append(dst, "\n# "...), bytes.TrimPrefix(c, []byte(" "))...)
	}
	return dst, true
}

func statements(raw []byte) iter.Seq[statement] {
	return func(yield func(statement) bool) {
		var cur statement
		started := false
		depth, inRaw := 0, false
		meta := -1 // start of the first METADATA line in the current run of comment lines
		off := 0
		for l := range bytes.Lines(raw) {
			start := off
			off += len(l)
			atTop := depth == 0 && !inRaw
			t := bytes.TrimSpace(l)
			depth, inRaw = scanLine(l, depth, inRaw)
			if len(t) > 0 && t[0] == '#' {
				if meta < 0 && bytes.HasPrefix(l, []byte("# METADATA")) {
					meta = start
				}
				if !atTop {
					cur.end = off
				}
				continue
			}
			annotation := start
			if meta >= 0 {
				annotation = meta
			}
			meta = -1
			if len(t) == 0 {
				continue
			}
			if !atTop || bytes.HasPrefix(t, []byte("else")) && (len(t) == 4 || !isIdentByte(t[4])) {
				cur.end = off
				continue
			}
			if started && !yield(cur) {
				return
			}
			cur, started = statement{annotation, start, off}, true
		}
		if started {
			yield(cur)
		}
	}
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

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package ucast

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// LuceneMatchAll is the query that selects every document.
const LuceneMatchAll = "*:*"

// luceneClause is a rendered (sub)query, or with neg its negation: a purely
// negative clause needs an anchor, so the NOT is only written where it is placed.
type luceneClause struct {
	text string
	neg  bool
}

// AsLucene renders the tree in the syntax of Solr's standard query parser. The
// first segment of a field names the document, and isn't part of the field name.
func (u *UCASTNode) AsLucene() (string, error) {
	if docs := u.documents(map[string]struct{}{}); len(docs) > 1 {
		return "", fmt.Errorf("a lucene query filters one document, got fields of: %s",
			strings.Join(slices.Sorted(maps.Keys(docs)), ", "))
	}
	c, err := u.asLucene()
	if err != nil {
		return "", err
	}
	return c.standalone(), nil
}

// documents gathers the document (first segment) of every field in the tree.
func (u *UCASTNode) documents(into map[string]struct{}) map[string]struct{} {
	if u.Type == "field" {
		doc, _, _ := strings.Cut(u.Field, ".")
		into[doc] = struct{}{}
	}
	if nodes, ok := u.Value.([]UCASTNode); ok {
		for i := range nodes {
			nodes[i].documents(into)
		}
	}
	return into
}

// standalone renders the clause so that it is valid wherever it is placed.
func (c luceneClause) standalone() string {
	if c.neg {
		return "(" + LuceneMatchAll + " AND NOT " + c.text + ")"
	}
	return c.text
}

func (u *UCASTNode) asLucene() (luceneClause, error) {
	switch {
	case u.Type == "field":
		return u.fieldAsLucene()
	case u.Type == "compound":
		return u.compoundAsLucene()
	default:
		return luceneClause{}, fmt.Errorf("unsupported node type for lucene: %q", u.Type)
	}
}

func (u *UCASTNode) fieldAsLucene() (luceneClause, error) {
	field, err := luceneField(u.Field)
	if err != nil {
		return luceneClause{}, err
	}

	if _, ok := u.Value.(Null); ok {
		switch u.Op {
		case "eq":
			return luceneClause{text: field + ":[* TO *]", neg: true}, nil
		case "ne":
			return luceneClause{text: field + ":[* TO *]"}, nil
		default:
			return luceneClause{}, errors.New("null value can only be used with 'eq' or 'ne' operators")
		}
	}

	switch u.Op {
	case "eq", "ne":
		v, err := luceneScalar(u.Value)
		if err != nil {
			return luceneClause{}, err
		}
		if u.Op == "ne" {
			// A document without the field has no value to differ: in Rego, `f != v` is
			// undefined there, but a bare NOT would match it.
			return luceneClause{text: "(" + field + ":[* TO *] AND NOT " + field + ":" + v + ")"}, nil
		}
		return luceneClause{text: field + ":" + v}, nil
	case "gt", "gte", "lt", "lte":
		v, err := luceneBound(u.Value)
		if err != nil {
			return luceneClause{}, err
		}
		switch u.Op {
		case "gt":
			return luceneClause{text: field + ":{" + v + " TO *}"}, nil
		case "gte":
			return luceneClause{text: field + ":[" + v + " TO *]"}, nil
		case "lt":
			return luceneClause{text: field + ":{* TO " + v + "}"}, nil
		default:
			return luceneClause{text: field + ":[* TO " + v + "]"}, nil
		}
	case "in":
		arr, ok := u.Value.([]any)
		if !ok {
			return luceneClause{}, errors.New("field operator 'in' requires collection argument")
		}
		if len(arr) == 0 {
			return luceneClause{text: LuceneMatchAll, neg: true}, nil
		}
		vs := make([]string, len(arr))
		for i := range arr {
			if vs[i], err = luceneScalar(arr[i]); err != nil {
				return luceneClause{}, err
			}
		}
		return luceneClause{text: field + ":(" + strings.Join(vs, " OR ") + ")"}, nil
	case "startswith", "endswith", "contains":
		patterns, ok := u.Value.([]any)
		if s, isString := u.Value.(string); isString {
			patterns, ok = []any{s}, true
		}
		if !ok {
			return luceneClause{}, fmt.Errorf("'%s' pattern requires string argument, got %v %[2]T", u.Op, u.Value)
		}
		if len(patterns) == 0 {
			return luceneClause{text: LuceneMatchAll, neg: true}, nil
		}
		terms := make([]string, len(patterns))
		for i := range patterns {
			p, ok := patterns[i].(string)
			if !ok {
				return luceneClause{}, fmt.Errorf("'%s' pattern requires string argument, got %v %[2]T", u.Op, patterns[i])
			}
			if p == "" { // every string starts with, ends with and contains ""
				return luceneClause{text: field + ":[* TO *]"}, nil
			}
			switch t := luceneWildcardLiteral(p); u.Op {
			case "startswith":
				terms[i] = t + "*"
			case "endswith":
				terms[i] = "*" + t
			default:
				terms[i] = "*" + t + "*"
			}
		}
		if len(terms) == 1 {
			return luceneClause{text: field + ":" + terms[0]}, nil
		}
		return luceneClause{text: field + ":(" + strings.Join(terms, " OR ") + ")"}, nil
	case "outside": // some value is none of the values, which are in ascending order
		arr, ok := u.Value.([]any)
		if !ok {
			return luceneClause{}, errors.New("field operator 'outside' requires collection argument")
		}
		if len(arr) == 0 {
			return luceneClause{text: field + ":[* TO *]"}, nil
		}
		ranges := make([]string, 0, len(arr)+1)
		lower := "*"
		for i := range arr {
			v, err := luceneBound(arr[i])
			if err != nil {
				return luceneClause{}, err
			}
			ranges = append(ranges, "{"+lower+" TO "+v+"}")
			lower = v
		}
		ranges = append(ranges, "{"+lower+" TO *}")
		return luceneClause{text: field + ":(" + strings.Join(ranges, " OR ") + ")"}, nil
	default:
		return luceneClause{}, fmt.Errorf("unsupported operator for lucene: %s", u.Op)
	}
}

func (u *UCASTNode) compoundAsLucene() (luceneClause, error) {
	nodes, ok := u.Value.([]UCASTNode)
	if !ok {
		return luceneClause{}, fmt.Errorf("compound expression '%s' requires an array of nodes, got %T", u.Op, u.Value)
	}
	switch u.Op {
	case "not":
		if len(nodes) != 1 {
			return luceneClause{}, errors.New("compound expression 'not' requires exactly one value")
		}
		c, err := nodes[0].asLucene()
		if err != nil {
			return luceneClause{}, err
		}
		c.neg = !c.neg
		return c, nil
	case "and", "or":
		if len(nodes) == 0 {
			return luceneClause{}, fmt.Errorf("compound expression '%s' requires a value", u.Op)
		}
		return junctionAsLucene(nodes, u.Op == "and")
	default:
		return luceneClause{}, fmt.Errorf("unrecognized operator: %s", u.Op)
	}
}

func junctionAsLucene(nodes []UCASTNode, conj bool) (luceneClause, error) {
	var pos, neg []string
	all := make([]string, 0, len(nodes))
	for i := range nodes {
		c, err := nodes[i].asLucene()
		if err != nil {
			return luceneClause{}, err
		}
		switch {
		case conj && c.neg:
			neg = append(neg, "NOT "+c.text)
		case conj:
			pos = append(pos, c.text)
		default:
			all = append(all, c.standalone())
		}
	}

	if !conj {
		return luceneClause{text: group(all, " OR ")}, nil
	}
	if len(pos) == 0 {
		pos = append(pos, LuceneMatchAll)
	}
	return luceneClause{text: group(append(pos, neg...), " AND ")}, nil
}

func group(parts []string, sep string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, sep) + ")"
}

// luceneField drops the document from the field, and checks that the name, which
// can come from the request, is safe to emit as it is.
func luceneField(f string) (string, error) {
	if _, after, ok := strings.Cut(f, "."); ok {
		f = after
	}
	if f == "" {
		return "", errors.New("empty field name")
	}
	for i := range len(f) {
		switch c := f[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case i > 0 && (c >= '0' && c <= '9' || c == '-'):
		case i > 0 && c == '.' && i < len(f)-1 && f[i+1] != '.':
		default:
			return "", fmt.Errorf("invalid lucene field name: %q", f)
		}
	}
	return f, nil
}

// luceneScalar renders a term value: strings are quoted, with the two
// characters that are special in a quoted phrase escaped, and a leading minus
// sign on a number is escaped.
func luceneScalar(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`, nil
	case bool:
		return strconv.FormatBool(v), nil
	case json.Number:
		s := v.String()
		if strings.HasPrefix(s, "-") {
			return `\` + s, nil
		}
		return s, nil
	case FieldRef:
		return "", errors.New("references to other fields are not supported for lucene")
	default:
		return "", fmt.Errorf("unsupported value for lucene: %v (%[1]T)", v)
	}
}

// luceneWildcardLiteral escapes s for a wildcard term, which can't be quoted: a
// backslash makes any character literal.
func luceneWildcardLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// luceneBound renders a range bound; only numbers and strings are ordered.
func luceneBound(v any) (string, error) {
	if _, ok := v.(bool); ok {
		return "", errors.New("range bound must be a number or string")
	}
	return luceneScalar(v)
}

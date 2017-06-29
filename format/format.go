// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/open-policy-agent/opa/ast"
)

// Bytes formats Rego source code. The bytes provided do not have to be an entire
// source file, but they must be parse-able. If the bytes are not parse-able, Bytes
// will return an error resulting from the attempt to parse them.
func Bytes(src []byte) ([]byte, error) {
	astElem, err := ast.Parse("", src, ast.CommentsOption())
	if err != nil {
		return nil, err
	}
	return Ast(astElem)
}

// Source formats a Rego source file. The bytes provided must decribe a complete
// Rego module. If they don't, Source will return an error resulting from the attempt
// to parse the bytes.
func Source(filename string, src []byte) ([]byte, error) {
	module, err := ast.ParseModule(filename, string(src))
	if err != nil {
		return nil, err
	}
	formatted, err := Ast(module)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", filename, err)
	}
	return formatted, nil
}

// Ast formats a Rego AST element. If the passed value is not a valid AST element,
// Ast returns nil and an error. Ast relies on all AST elements having non-nil
// Location values, and will return an error if this is not the case.
func Ast(x interface{}) (formatted []byte, err error) {
	defer func() {
		// Ast relies on all terms in the ast element having non-nil Location
		// values. If a location is nil, Ast will panic, so we need to recover
		// gracefully.
		if r := recover(); r != nil {
			formatted = nil
			switch r := r.(type) {
			case nilLocationErr:
				err = r
			default:
				panic(r)
			}
		}
	}()

	// Check all elements in the Ast interface have a location.
	ast.Walk(ast.NewGenericVisitor(func(x interface{}) bool {
		switch x := x.(type) {
		case *ast.Module, ast.Value: // Pass, they don't have locations.
		case *ast.Term:
			switch v := x.Value.(type) {
			case ast.Ref:
				if h := v[0]; !ast.RootDocumentNames.Contains(h) {
					assertHasLocation(x)
				}
			case ast.Var:
				if vt := ast.VarTerm(string(v)); !ast.RootDocumentNames.Contains(vt) {
					assertHasLocation(x)
				}
			default:
				assertHasLocation(x)
			}
		case *ast.Package, *ast.Import, *ast.Rule, *ast.Head, ast.Body, *ast.Expr, *ast.With, *ast.Comment:
			assertHasLocation(x)
		}
		return false
	}), x)

	w := &writer{indent: "\t"}
	switch x := x.(type) {
	case *ast.Module:
		w.writeModule(x)
	case *ast.Package:
		w.writePackage(x, nil)
	case *ast.Import:
		w.writeImports([]*ast.Import{x}, nil)
	case *ast.Rule:
		w.writeRule(x, nil)
	case *ast.Head:
		w.write(x.String())
	case ast.Body:
		w.writeBody(x, nil)
	case *ast.Expr:
		w.writeExpr(x, nil)
	case *ast.With:
		w.writeWith(x, nil)
	case *ast.Term:
		w.writeTerm(x, nil)
	case ast.Value:
		w.writeTerm(&ast.Term{Value: x, Location: &ast.Location{}}, nil)
	case *ast.Comment:
		w.writeComments([]*ast.Comment{x})
	default:
		return nil, fmt.Errorf("not an ast element: %v", x)
	}
	return w.buf.Bytes(), nil
}

type writer struct {
	buf bytes.Buffer

	indent    string
	level     int
	inline    bool
	beforeEnd *ast.Comment
	delay     bool
}

func (w *writer) writeModule(module *ast.Module) {
	var pkg *ast.Package
	var others []interface{}
	var comments []*ast.Comment
	visitor := ast.NewGenericVisitor(func(x interface{}) bool {
		switch x := x.(type) {
		case *ast.Comment:
			comments = append(comments, x)
			return true
		case *ast.Import, *ast.Rule:
			others = append(others, x)
			return true
		case *ast.Package:
			pkg = x
			return true
		default:
			return false
		}
	})
	ast.Walk(visitor, module)

	sort.Slice(comments, func(i, j int) bool {
		return locLess(comments[i], comments[j])
	})

	// XXX: The parser currently duplicates comments for some reason, so we need
	// to remove duplicates here.
	comments = dedupComments(comments)
	sort.Slice(others, func(i, j int) bool {
		return locLess(others[i], others[j])
	})

	comments = w.writePackage(pkg, comments)
	var imports []*ast.Import
	var rules []*ast.Rule
	for len(others) > 0 {
		imports, others = gatherImports(others)
		comments = w.writeImports(imports, comments)
		rules, others = gatherRules(others)
		comments = w.writeRules(rules, comments)
	}

	for _, c := range comments {
		w.writeLine(c.String())
	}
}

func (w *writer) writePackage(pkg *ast.Package, comments []*ast.Comment) []*ast.Comment {
	comments = w.insertComments(comments, pkg.Location)

	w.startLine()
	w.write(pkg.String())
	w.blankLine()

	return comments
}

func (w *writer) writeComments(comments []*ast.Comment) {
	for i := 0; i < len(comments); i++ {
		if i > 0 && locCmp(comments[i], comments[i-1]) > 1 {
			w.blankLine()
		}
		w.writeLine(comments[i].String())
	}
}

func (w *writer) writeRules(rules []*ast.Rule, comments []*ast.Comment) []*ast.Comment {
	for _, rule := range rules {
		comments = w.insertComments(comments, rule.Location)
		comments = w.writeRule(rule, comments)
		w.blankLine()
	}
	return comments
}

func (w *writer) writeRule(rule *ast.Rule, comments []*ast.Comment) []*ast.Comment {
	if rule == nil {
		return comments
	}

	w.startLine()
	if rule.Default {
		w.write("default ")
	}
	comments = w.writeHead(rule.Head, comments)

	// OPA transforms lone bodies like `foo = {"a": "b"}` into rules of the form
	// `foo = {"a": "b"} { true }` in the AST. We want to preserve that notation
	// in the formatted code instead of expanding the bodies into rules, so we
	// pretend that the rule has no body in this case.
	isExpandedConst := rule.Head.DocKind() == ast.CompleteDoc && rule.Body.Equal(ast.NewBody(ast.NewExpr(ast.BooleanTerm(true))))
	if len(rule.Body) == 0 || isExpandedConst {
		w.endLine()
		return comments
	}

	w.write(" {")
	w.endLine()
	w.up()

	comments = w.writeBody(rule.Body, comments)
	comments = w.insertComments(comments, closingLoc('{', '}', rule.Location))

	w.down()
	w.startLine()
	w.write("}")
	if rule.Else != nil {
		w.blankLine()
		rule.Else.Head.Name = ast.Var("else")
		comments = w.insertComments(comments, rule.Else.Loc())
		comments = w.writeRule(rule.Else, comments)
	}
	return comments
}

func (w *writer) writeHead(head *ast.Head, comments []*ast.Comment) []*ast.Comment {
	w.write(head.Name.String())
	if head.Key != nil {
		w.write("[")
		comments = w.writeTerm(head.Key, comments)
		w.write("]")
	}
	if head.Value != nil {
		w.write(" = ")
		comments = w.writeTerm(head.Value, comments)
	}
	return comments
}

func (w *writer) insertComments(comments []*ast.Comment, loc *ast.Location) []*ast.Comment {
	before, at, comments := partitionComments(comments, loc)
	w.writeComments(before)
	if len(before) > 0 && loc.Row-before[len(before)-1].Location.Row > 1 {
		w.blankLine()
	}

	w.beforeLineEnd(at)
	return comments
}

func (w *writer) writeBody(body ast.Body, comments []*ast.Comment) []*ast.Comment {
	comments = w.insertComments(comments, body.Loc())
	offset := 0
	for i, expr := range body {
		if i > 0 && expr.Location.Row-body[i-1].Location.Row-offset > 1 {
			w.blankLine()
		}
		w.startLine()

		comments = w.writeExpr(expr, comments)
		w.endLine()
	}
	return comments
}

func (w *writer) writeExpr(expr *ast.Expr, comments []*ast.Comment) []*ast.Comment {
	comments = w.insertComments(comments, expr.Location)
	if expr.Negated {
		w.write("not ")
	}

	switch t := expr.Terms.(type) {
	case []*ast.Term:
		comments = w.writeFunctionCall(t, comments)
	case *ast.Term:
		comments = w.writeTerm(t, comments)
	}

	var indented bool
	for i, with := range expr.With {
		if i > 0 && with.Location.Row-expr.With[i-1].Location.Row > 0 {
			if !indented {
				indented = true

				w.up()
				defer w.down()
			}
			w.endLine()
			w.startLine()
		}
		comments = w.writeWith(with, comments)
	}

	return comments
}

func (w *writer) writeFunctionCall(t []*ast.Term, comments []*ast.Comment) []*ast.Comment {
	name := t[0].Value.(ast.String)
	bi := ast.BuiltinMap[name]
	if bi != nil && len(bi.Infix) > 0 {
		switch len(bi.Args) {
		case 3:
			comments = w.writeTerm(t[3], comments)
			w.write(" = ")
			fallthrough
		case 2:
			comments = w.writeTerm(t[1], comments)
			w.write(" %s ", string(bi.Infix))
			return w.writeTerm(t[2], comments)
		}
	}

	w.write("%s(", string(t[0].Value.(ast.String)))
	for _, v := range t[1 : len(t)-1] {
		comments = w.writeTerm(v, comments)
		w.write(", ")
	}
	comments = w.writeTerm(t[len(t)-1], comments)
	w.write(")")
	return comments
}

func (w *writer) writeWith(with *ast.With, comments []*ast.Comment) []*ast.Comment {
	comments = w.insertComments(comments, with.Location)
	w.write(" with ")
	comments = w.writeTerm(with.Target, comments)
	w.write(" as ")
	return w.writeTerm(with.Value, comments)
}

// writeTerm assumes that the current line has been started. it must leave the
// current line started as well.
func (w *writer) writeTerm(term *ast.Term, comments []*ast.Comment) []*ast.Comment {
	if !w.inline {
		panic("writing term on unstarted line")
	}

	comments = w.insertComments(comments, term.Location)
	if !w.inline {
		// If any comments were written, we need to start the line again.
		w.startLine()
	}

	switch x := term.Value.(type) {
	case ast.Ref:
		w.write(x.String())
		return comments
	case ast.Object:
		comments = w.writeObject(x, term.Location, comments)
	case ast.Array:
		comments = w.writeArray(x, term.Location, comments)
	case *ast.Set:
		comments = w.writeSet(x, term.Location, comments)
	case *ast.ArrayComprehension:
		comments = w.writeArrayComprehension(x, term.Location, comments)
	case fmt.Stringer:
		w.write(x.String())
	}

	if !w.inline {
		panic("writeTerm did not leave the line in started state")
	}
	return comments
}

func (w *writer) writeObject(obj ast.Object, loc *ast.Location, comments []*ast.Comment) []*ast.Comment {
	w.write("{")
	defer w.write("}")

	var s []interface{}
	for _, t := range obj {
		s = append(s, t)
	}
	comments = w.writeIterable(s, loc, comments, w.objectWriter())
	return w.insertComments(comments, closingLoc('{', '}', loc))
}

func (w *writer) writeArray(arr ast.Array, loc *ast.Location, comments []*ast.Comment) []*ast.Comment {
	w.write("[")
	defer w.write("]")

	var s []interface{}
	for _, t := range arr {
		s = append(s, t)
	}
	comments = w.writeIterable(s, loc, comments, w.listWriter())
	return w.insertComments(comments, closingLoc('[', ']', loc))
}

func (w *writer) writeSet(set *ast.Set, loc *ast.Location, comments []*ast.Comment) []*ast.Comment {
	w.write("{")
	defer w.write("}")

	var s []interface{}
	for _, t := range *set {
		s = append(s, t)
	}
	comments = w.writeIterable(s, loc, comments, w.listWriter())
	return w.insertComments(comments, closingLoc('{', '}', loc))
}

func (w *writer) writeArrayComprehension(arr *ast.ArrayComprehension, loc *ast.Location, comments []*ast.Comment) []*ast.Comment {
	w.write("[")
	defer w.write("]")

	if arr.Term.Location.Row-loc.Row > 1 {
		w.endLine()
		w.startLine()
	}

	comments = w.writeTerm(arr.Term, comments)
	w.write(" |")

	var exprs []interface{}
	for _, expr := range arr.Body {
		exprs = append(exprs, expr)
	}
	lines := groupIterable(exprs, arr.Term.Location)

	if arr.Body.Loc().Row-loc.Row > 0 || len(lines) > 1 {
		w.endLine()
		w.up()
		defer w.startLine()
		defer w.down()

		comments = w.writeBody(arr.Body, comments)
	} else {
		w.write(" ")
		i := 0
		for ; i < len(arr.Body)-1; i++ {
			comments = w.writeExpr(arr.Body[i], comments)
			w.write("; ")
		}
		comments = w.writeExpr(arr.Body[i], comments)
	}

	return w.insertComments(comments, closingLoc('[', ']', loc))
}

func (w *writer) writeImports(imports []*ast.Import, comments []*ast.Comment) []*ast.Comment {
	m, comments := mapImportsToComments(imports, comments)

	groups := groupImports(imports)
	for _, group := range groups {
		comments = w.insertComments(comments, group[0].Loc())

		// Sort imports within a newline grouping.
		sort.Slice(group, func(i, j int) bool {
			a := group[i]
			b := group[j]
			return a.Compare(b) < 0
		})
		for _, i := range group {
			w.startLine()
			w.write(i.String())
			if c, ok := m[i]; ok {
				w.write(" " + c.String())
			}
			w.endLine()
		}
		w.blankLine()
	}

	return comments
}

type entryWriter func(interface{}, []*ast.Comment) []*ast.Comment

func (w *writer) writeIterable(elements []interface{}, last *ast.Location, comments []*ast.Comment, fn entryWriter) []*ast.Comment {
	lines := groupIterable(elements, last)
	if len(lines) > 1 {
		w.delayBeforeEnd()
		w.startMultilineSeq()
		defer w.endMultilineSeq()
	}

	i := 0
	for ; i < len(lines)-1; i++ {
		comments = w.writeIterableLine(lines[i], comments, fn)
		w.write(",")

		w.endLine()
		w.startLine()
	}
	comments = w.writeIterableLine(lines[i], comments, fn)
	return comments
}

func (w *writer) writeIterableLine(elements []interface{}, comments []*ast.Comment, fn entryWriter) []*ast.Comment {
	if len(elements) == 0 {
		return comments
	}

	i := 0
	for ; i < len(elements)-1; i++ {
		comments = fn(elements[i], comments)
		w.write(", ")
	}

	return fn(elements[i], comments)
}

func (w *writer) objectWriter() entryWriter {
	return func(x interface{}, comments []*ast.Comment) []*ast.Comment {
		entry := x.([2]*ast.Term)
		comments = w.writeTerm(entry[0], comments)
		w.write(": ")
		return w.writeTerm(entry[1], comments)
	}
}

func (w *writer) listWriter() entryWriter {
	return func(x interface{}, comments []*ast.Comment) []*ast.Comment {
		return w.writeTerm(x.(*ast.Term), comments)
	}
}

func groupIterable(elements []interface{}, last *ast.Location) (lines [][]interface{}) {
	var cur []interface{}
	for i, t := range elements {
		loc := getLoc(t)
		lineDiff := loc.Row - last.Row
		if lineDiff > 0 && i > 0 {
			lines = append(lines, cur)
			cur = nil
		}

		last = loc
		cur = append(cur, t)
	}
	return append(lines, cur)
}

func mapImportsToComments(imports []*ast.Import, comments []*ast.Comment) (map[*ast.Import]*ast.Comment, []*ast.Comment) {
	var leftovers []*ast.Comment
	m := map[*ast.Import]*ast.Comment{}

	for _, c := range comments {
		matched := false
		for _, i := range imports {
			if c.Loc().Row == i.Loc().Row {
				m[i] = c
				matched = true
				break
			}
		}
		if !matched {
			leftovers = append(leftovers, c)
		}
	}

	return m, leftovers
}

func groupImports(imports []*ast.Import) (groups [][]*ast.Import) {
	if len(imports) == 0 {
		return nil
	}

	last := imports[0]
	var group []*ast.Import
	for _, i := range imports {
		if i.Loc().Row-last.Loc().Row > 1 {
			groups = append(groups, group)
			group = []*ast.Import{}
		}
		group = append(group, i)
		last = i
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}

	return groups
}

func partitionComments(comments []*ast.Comment, l *ast.Location) (before []*ast.Comment, at *ast.Comment, after []*ast.Comment) {
	for _, c := range comments {
		switch cmp := c.Location.Row - l.Row; {
		case cmp < 0:
			before = append(before, c)
		case cmp > 0:
			after = append(after, c)
		case cmp == 0:
			at = c
		}
	}

	return before, at, after
}

func gatherImports(others []interface{}) (imports []*ast.Import, rest []interface{}) {
	i := 0
loop:
	for ; i < len(others); i++ {
		switch x := others[i].(type) {
		case *ast.Import:
			imports = append(imports, x)
		case *ast.Rule:
			break loop
		}
	}
	return imports, others[i:]
}

func gatherRules(others []interface{}) (rules []*ast.Rule, rest []interface{}) {
	i := 0
loop:
	for ; i < len(others); i++ {
		switch x := others[i].(type) {
		case *ast.Rule:
			rules = append(rules, x)
		case *ast.Import:
			break loop
		}
	}
	return rules, others[i:]
}

func locLess(a, b interface{}) bool {
	return locCmp(a, b) < 0
}

func locCmp(a, b interface{}) int {
	al := getLoc(a)
	bl := getLoc(b)
	if cmp := al.Row - bl.Row; cmp != 0 {
		return cmp
	}
	return al.Col - bl.Col
}

func getLoc(x interface{}) *ast.Location {
	switch x := x.(type) {
	case ast.Statement:
		return x.Loc()
	case *ast.Head:
		return x.Location
	case *ast.Expr:
		return x.Location
	case *ast.With:
		return x.Location
	case *ast.Term:
		return x.Location
	case *ast.Location:
		return x
	case [2]*ast.Term:
		// Special case to allow for easy printing of objects.
		return x[0].Location
	default:
		panic("Not reached")
	}
}

func closingLoc(open, close byte, loc *ast.Location) *ast.Location {
	i := 0
	for ; i < len(loc.Text) && loc.Text[i] != open; i++ {
	}

	if i >= len(loc.Text) {
		return &ast.Location{Row: -1}
	}

	state := 1
	offset := 0
	for state > 0 {
		i++
		if i >= len(loc.Text) {
			return &ast.Location{Row: -1}
		}

		switch loc.Text[i] {
		case open:
			state++
		case close:
			state--
		case '\n':
			offset++
		}
	}

	return &ast.Location{Row: loc.Row + offset}
}

func dedupComments(comments []*ast.Comment) []*ast.Comment {
	if len(comments) == 0 {
		return nil
	}

	filtered := []*ast.Comment{comments[0]}
	for i := 1; i < len(comments); i++ {
		if comments[i].Location.Equal(comments[i-1].Location) {
			continue
		}
		filtered = append(filtered, comments[i])
	}
	return filtered
}

// startLine begins a line with the current indentation level.
func (w *writer) startLine() {
	if w.inline {
		panic("currently in a line")
	}
	w.inline = true
	for i := 0; i < w.level; i++ {
		w.write(w.indent)
	}
}

// endLine ends a line with a newline.
func (w *writer) endLine() {
	if !w.inline {
		panic("not in a line")
	}
	w.inline = false
	if w.beforeEnd != nil && !w.delay {
		w.write(" " + w.beforeEnd.String())
		w.beforeEnd = nil
	}
	w.delay = false
	w.write("\n")
}

// beforeLineEnd registers a comment to be printed at the end of the current line.
func (w *writer) beforeLineEnd(c *ast.Comment) {
	if w.beforeEnd != nil {
		if c == nil {
			return
		}
		panic("overwriting non-nil beforeEnd")
	}
	w.beforeEnd = c
}

func (w *writer) delayBeforeEnd() {
	w.delay = true
}

// line prints a blank line. If the writer is currently in the middle of a line,
// line ends it and then prints a blank one.
func (w *writer) blankLine() {
	if w.inline {
		w.endLine()
	}
	w.write("\n")
}

// write formats the input string and writes it to the buffer.
func (w *writer) write(s string, a ...interface{}) {
	w.buf.WriteString(fmt.Sprintf(s, a...))
}

// writeLine writes the formatted string on a newly started line, then terminates
// the line.
func (w *writer) writeLine(s string, a ...interface{}) {
	if !w.inline {
		w.startLine()
	}
	w.write(s, a...)
	w.endLine()
}

func (w *writer) startMultilineSeq() {
	w.endLine()
	w.up()
	w.startLine()
}

func (w *writer) endMultilineSeq() {
	w.write(",")
	w.endLine()
	w.down()
	w.startLine()
}

// up increases the indentation level
func (w *writer) up() {
	w.level++
}

// down decreases the indentation level
func (w *writer) down() {
	if w.level == 0 {
		panic("negative indentation level")
	}
	w.level--
}

func assertHasLocation(xs ...interface{}) {
	for _, x := range xs {
		if getLoc(x) == nil {
			panic(nilLocationErr{fmt.Errorf("nil location: %v", x)})
		}
	}
}

type nilLocationErr struct {
	err error
}

func (err nilLocationErr) Error() string {
	return err.err.Error()
}

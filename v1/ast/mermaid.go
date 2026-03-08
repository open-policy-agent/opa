// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
)

// mermaidFormatter is implemented by AST nodes that can render themselves as a
// Mermaid flowchart fragment. Each call writes node and edge declarations to b
// and returns the ID of the node it emitted.
type mermaidFormatter interface {
	mermaidFormat(b *mermaidBuilder) string
}

// mermaidBuilder accumulates Mermaid flowchart lines and issues unique node IDs.
type mermaidBuilder struct {
	buf     strings.Builder
	counter int
}

func (b *mermaidBuilder) newID() string {
	b.counter++
	return fmt.Sprintf("n%d", b.counter)
}

// node writes a Mermaid node declaration and returns its ID.
// Supported shapes: "rect" (default), "round", "hex", "cyl", "stadium", "trap".
func (b *mermaidBuilder) node(shape, label string) string {
	id := b.newID()
	label = mermaidEscapeLabel(label)
	b.buf.WriteString("  ")
	b.buf.WriteString(id)
	switch shape {
	case "round":
		b.buf.WriteString("(\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\")\n")
	case "hex":
		b.buf.WriteString("{{\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\"}}\n")
	case "cyl":
		b.buf.WriteString("[(\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\")]\n")
	case "stadium":
		b.buf.WriteString("([\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\"])\n")
	case "trap":
		b.buf.WriteString("[/\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\"/]\n")
	default: // "rect"
		b.buf.WriteString("[\"")
		b.buf.WriteString(label)
		b.buf.WriteString("\"]\n")
	}
	return id
}

// edge writes a plain directed edge.
func (b *mermaidBuilder) edge(from, to string) {
	b.buf.WriteString("  ")
	b.buf.WriteString(from)
	b.buf.WriteString(" --> ")
	b.buf.WriteString(to)
	b.buf.WriteString("\n")
}

// edgeLabeled writes a directed edge with a text label.
func (b *mermaidBuilder) edgeLabeled(from, to, label string) {
	b.buf.WriteString("  ")
	b.buf.WriteString(from)
	b.buf.WriteString(" -->|")
	b.buf.WriteString(label)
	b.buf.WriteString("| ")
	b.buf.WriteString(to)
	b.buf.WriteString("\n")
}

// mermaidEscapeLabel replaces characters that break Mermaid quoted node labels.
func mermaidEscapeLabel(s string) string {
	return strings.ReplaceAll(s, `"`, "#quot;")
}

// mermaidGraph returns a Mermaid flowchart string representing the structure of
// the given module.
func mermaidGraph(module *Module) string {
	b := &mermaidBuilder{}
	module.mermaidFormat(b)
	var out strings.Builder
	out.WriteString("flowchart TD\n")
	out.WriteString(b.buf.String())
	return out.String()
}

// --- Module ---

func (mod *Module) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("rect", "Module")
	pkgID := mod.Package.mermaidFormat(b)
	b.edgeLabeled(id, pkgID, "package")
	for _, imp := range mod.Imports {
		impID := imp.mermaidFormat(b)
		b.edgeLabeled(id, impID, "import")
	}
	for _, rule := range mod.Rules {
		ruleID := rule.mermaidFormat(b)
		b.edgeLabeled(id, ruleID, "rule")
	}
	return id
}

// --- Package ---

func (pkg *Package) mermaidFormat(b *mermaidBuilder) string {
	return b.node("rect", "Package: "+pkg.Path.String())
}

// --- Import ---

func (imp *Import) mermaidFormat(b *mermaidBuilder) string {
	label := "Import: " + imp.Path.String()
	if imp.Alias != "" {
		label += " as " + string(imp.Alias)
	}
	return b.node("rect", label)
}

// --- Rule ---

func (rule *Rule) mermaidFormat(b *mermaidBuilder) string {
	ref := rule.Head.Ref().String()
	label := "Rule: " + ref
	if rule.Default {
		label = "Rule: default " + ref
	}
	id := b.node("rect", label)

	headID := mermaidFormatHead(rule.Head, b)
	b.edge(id, headID)

	if len(rule.Body) > 0 {
		bodyID := b.node("rect", "Body")
		b.edge(id, bodyID)
		for i, expr := range rule.Body {
			exprID := mermaidFormatExpr(expr, i, b)
			b.edgeLabeled(bodyID, exprID, fmt.Sprintf("%d", i))
		}
	}

	if rule.Else != nil {
		elseID := rule.Else.mermaidFormat(b)
		b.edgeLabeled(id, elseID, "else")
	}
	return id
}

func mermaidFormatHead(head *Head, b *mermaidBuilder) string {
	id := b.node("rect", "Head")

	refId := head.Ref().mermaidFormat(b)
	b.edgeLabeled(id, refId, "ref")

	for i, arg := range head.Args {
		argID := arg.mermaidFormat(b)
		b.edgeLabeled(id, argID, fmt.Sprintf(`"arg[%d]"`, i))
	}

	if head.Key != nil {
		keyID := head.Key.mermaidFormat(b)
		b.edgeLabeled(id, keyID, "key")
	}

	if head.Value != nil {
		valID := head.Value.mermaidFormat(b)
		b.edgeLabeled(id, valID, "value")
	}

	return id
}

func mermaidFormatExpr(expr *Expr, index int, b *mermaidBuilder) string {
	//label := fmt.Sprintf("Expr[%d]", index)
	//if expr.Negated {
	//	label = "not " + label
	//}

	label := expr.String()
	//var label string
	//if expr.Location != nil {
	//	label = expr.Location.String()
	//} else {
	//	label = expr.String()
	//}

	id := b.node("hex", label)

	switch terms := expr.Terms.(type) {
	case *Term:
		termID := terms.mermaidFormat(b)
		b.edge(id, termID)
	case []*Term:
		// terms[0] is the operator; remaining are arguments.
		for i, t := range terms {
			tID := t.mermaidFormat(b)
			if i == 0 {
				b.edgeLabeled(id, tID, "op")
			} else {
				b.edgeLabeled(id, tID, fmt.Sprintf(`"arg[%d]"`, i-1))
			}
		}
	case *SomeDecl:
		for _, sym := range terms.Symbols {
			symID := sym.mermaidFormat(b)
			b.edgeLabeled(id, symID, "symbol")
		}
	case *Every:
		everyID := mermaidFormatEvery(terms, b)
		b.edge(id, everyID)
	}

	for _, w := range expr.With {
		withID := mermaidFormatWith(w, b)
		b.edgeLabeled(id, withID, "with")
	}
	return id
}

func mermaidFormatEvery(every *Every, b *mermaidBuilder) string {
	id := b.node("rect", "every")
	if every.Key != nil {
		keyID := every.Key.mermaidFormat(b)
		b.edgeLabeled(id, keyID, "key")
	}
	valID := every.Value.mermaidFormat(b)
	b.edgeLabeled(id, valID, "value")
	domainID := every.Domain.mermaidFormat(b)
	b.edgeLabeled(id, domainID, "domain")
	bodyID := b.node("rect", "Body")
	b.edge(id, bodyID)
	for i, expr := range every.Body {
		exprID := mermaidFormatExpr(expr, i, b)
		b.edge(bodyID, exprID)
	}
	return id
}

func mermaidFormatWith(w *With, b *mermaidBuilder) string {
	id := b.node("rect", "with")
	targetID := w.Target.mermaidFormat(b)
	b.edgeLabeled(id, targetID, "target")
	valID := w.Value.mermaidFormat(b)
	b.edgeLabeled(id, valID, "value")
	return id
}

// --- Not ---

func (not *Not) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("stadium", "not")
	for i, expr := range not.Body {
		exprID := mermaidFormatExpr(expr, i, b)
		b.edgeLabeled(id, exprID, fmt.Sprintf("%d", i))
	}
	return id
}

// --- Term ---

// mermaidFormat delegates to the underlying Value. Term itself does not emit a
// node; the Value determines the node shape and label.
func (term *Term) mermaidFormat(b *mermaidBuilder) string {
	switch v := term.Value.(type) {
	case mermaidFormatter:
		return v.mermaidFormat(b)
	case Set:
		return mermaidFormatSet(v, b)
	case Object:
		return mermaidFormatObject(v, b)
	default:
		return b.node("round", term.String())
	}
}

// --- Scalar Values ---

func (Null) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", "null")
}

func (bol Boolean) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", bol.String())
}

func (num Number) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", num.String())
}

func (str String) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", str.String())
}

func (v Var) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", string(v))
}

// --- Ref ---

func (ref Ref) mermaidFormat(b *mermaidBuilder) string {
	return b.node("round", "ref: "+ref.String())
}

// --- Array ---

func (arr *Array) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("cyl", fmt.Sprintf("array[%d]", arr.Len()))
	arr.Foreach(func(t *Term) {
		elemID := t.mermaidFormat(b)
		b.edge(id, elemID)
	})
	return id
}

// --- Set ---

func mermaidFormatSet(s Set, b *mermaidBuilder) string {
	id := b.node("cyl", fmt.Sprintf("set{%d}", s.Len()))
	s.Foreach(func(t *Term) {
		elemID := t.mermaidFormat(b)
		b.edge(id, elemID)
	})
	return id
}

// --- Object ---

func mermaidFormatObject(o Object, b *mermaidBuilder) string {
	id := b.node("cyl", fmt.Sprintf("object{%d}", o.Len()))
	_ = o.Iter(func(k, v *Term) error {
		kvID := b.node("rect", "kv")
		b.edge(id, kvID)
		kID := k.mermaidFormat(b)
		b.edgeLabeled(kvID, kID, "key")
		vID := v.mermaidFormat(b)
		b.edgeLabeled(kvID, vID, "value")
		return nil
	})
	return id
}

// --- Comprehensions ---

func (ac *ArrayComprehension) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("stadium", "[_ | ...]")
	termID := ac.Term.mermaidFormat(b)
	b.edgeLabeled(id, termID, "term")
	bodyID := b.node("rect", "Body")
	b.edge(id, bodyID)
	for i, expr := range ac.Body {
		exprID := mermaidFormatExpr(expr, i, b)
		b.edge(bodyID, exprID)
	}
	return id
}

func (oc *ObjectComprehension) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("stadium", "{_:_ | ...}")
	kID := oc.Key.mermaidFormat(b)
	b.edgeLabeled(id, kID, "key")
	vID := oc.Value.mermaidFormat(b)
	b.edgeLabeled(id, vID, "value")
	bodyID := b.node("rect", "Body")
	b.edge(id, bodyID)
	for i, expr := range oc.Body {
		exprID := mermaidFormatExpr(expr, i, b)
		b.edge(bodyID, exprID)
	}
	return id
}

func (sc *SetComprehension) mermaidFormat(b *mermaidBuilder) string {
	id := b.node("stadium", "{_ | ...}")
	termID := sc.Term.mermaidFormat(b)
	b.edgeLabeled(id, termID, "term")
	bodyID := b.node("rect", "Body")
	b.edge(id, bodyID)
	for i, expr := range sc.Body {
		exprID := mermaidFormatExpr(expr, i, b)
		b.edge(bodyID, exprID)
	}
	return id
}

// --- Call ---

func (c Call) mermaidFormat(b *mermaidBuilder) string {
	opLabel := "call"
	if len(c) > 0 {
		opLabel = "call: " + c[0].String()
	}
	id := b.node("trap", opLabel)
	for i, arg := range c[1:] {
		argID := arg.mermaidFormat(b)
		b.edgeLabeled(id, argID, fmt.Sprintf(`"arg[%d]"`, i))
	}
	return id
}

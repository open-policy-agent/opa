// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/util"
)

// mermaid renders the trie, naming the rules by their bodies -- which the index
// holds, and the nodes only the ids of.
func (i *baseDocEqIndex) mermaid() string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")
	nodeCounter := 0
	nodeIDs := make(map[*trieNode]string)
	i.root.mermaidFormat(&sb, &nodeCounter, nodeIDs, "", i.rules)
	return sb.String()
}

func (node *trieNode) mermaidFormat(sb *strings.Builder, counter *int, nodeIDs map[*trieNode]string, parentID string, rules []*Rule) {
	currentID, exists := nodeIDs[node]
	if !exists {
		currentID = fmt.Sprintf("n%d", *counter)
		*counter++
		nodeIDs[node] = currentID
		fmt.Fprintf(sb, "  %s[\"%s\"]\n", currentID, node.mermaidLabel(rules))
	}

	if parentID != "" {
		fmt.Fprintf(sb, "  %s --> %s\n", parentID, currentID)
	}

	if exists {
		return
	}

	node.next.mermaidFormat(sb, counter, nodeIDs, currentID, rules)
}

// mermaidEdge draws one way of matching the level's reference, emitting the
// child first if this is where it is reached from.
func mermaidEdge(sb *strings.Builder, counter *int, nodeIDs map[*trieNode]string, from, label string, child *trieNode, rules []*Rule) {
	if child == nil {
		return
	}
	if _, exists := nodeIDs[child]; !exists {
		child.mermaidFormat(sb, counter, nodeIDs, "", rules)
	}
	fmt.Fprintf(sb, "  %s -->|\"%s\"| %s\n", from, mermaidEscape(label), nodeIDs[child])
}

func (d *levelDetail) mermaidFormat(sb *strings.Builder, counter *int, nodeIDs map[*trieNode]string, from string, rules []*Rule) {
	if d == nil {
		return
	}

	mermaidEdge(sb, counter, nodeIDs, from, "undefined", d.undefined, rules)
	mermaidEdge(sb, counter, nodeIDs, from, "any", d.any, rules)

	d.scalars.Iter(func(key Value, child *trieNode) bool {
		mermaidEdge(sb, counter, nodeIDs, from, key.String(), child, rules)
		return false
	})

	if d.alternatives != nil {
		d.alternatives.members.Iter(func(key Value, nodes []*trieNode) bool {
			for _, child := range nodes {
				mermaidEdge(sb, counter, nodeIDs, from, key.String(), child, rules)
			}
			return false
		})
	}

	for _, p := range d.prefixes.walk() {
		mermaidEdge(sb, counter, nodeIDs, from, p.prefix+"*", p.node, rules)
	}

	// A suffix trie holds its bases reversed (see affixTries), so what it
	// walks back is what was written.
	for _, p := range d.suffixes.walk() {
		mermaidEdge(sb, counter, nodeIDs, from, "*"+reverseString(p.prefix), p.node, rules)
	}

	d.array.mermaidFormat(sb, counter, nodeIDs, from, "array", rules)
}

func (a *arrayTrie) mermaidFormat(sb *strings.Builder, counter *int, nodeIDs map[*trieNode]string, from, label string, rules []*Rule) {
	if a == nil {
		return
	}

	mermaidEdge(sb, counter, nodeIDs, from, label, a.end, rules)
	a.any.mermaidFormat(sb, counter, nodeIDs, from, label+" any", rules)
	a.scalars.Iter(func(key Value, child *arrayTrie) bool {
		child.mermaidFormat(sb, counter, nodeIDs, from, label+" "+key.String(), rules)
		return false
	})
}

func (node *trieNode) mermaidLabel(rules []*Rule) string {
	var parts []string

	if node.next != nil && len(node.next.ref) > 0 {
		parts = append(parts, node.next.ref.String())
	}

	for _, id := range node.rules {
		bodyStr := ""
		if rule := rules[id]; rule.Body != nil {
			bodyStr = rule.Body.String()
			if len(bodyStr) > 50 {
				bodyStr = bodyStr[:50] + "..."
			}
		}
		parts = append(parts, mermaidEscape(bodyStr))
	}

	if node.next != nil && len(node.next.mappers) > 0 {
		parts = append(parts, fmt.Sprintf("%d mapper(s)", len(node.next.mappers)))
	}
	if node.multiple {
		parts = append(parts, "multiple")
	}

	if len(parts) == 0 {
		return "\u00b7"
	}

	return strings.Join(parts, "<br/>")
}

func mermaidEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	return s
}

func (node *trieNode) String() string {
	var sb strings.Builder
	node.format(&sb, 0)
	return sb.String()
}

func (node *trieNode) format(sb *strings.Builder, depth int) {
	if node == nil {
		return
	}

	indent := strings.Repeat("  ", depth)

	if depth == 0 {
		sb.WriteString("root")
	} else {
		sb.WriteString(indent)
	}
	if len(node.rules) > 0 {
		sb.WriteString(" [")
		util.WriteInt(sb, len(node.rules))
		sb.WriteString(" rule(s)]")
	}
	if node.value != nil {
		sb.WriteString(" value=")
		sb.WriteString(node.value.String())
	}
	if node.multiple {
		sb.WriteString(" [multiple]")
	}
	sb.WriteByte('\n')

	node.next.format(sb, depth)
}

// format prints the level below a node: the reference it resolves, and a line
// per way of matching it.
func (d *levelDetail) format(sb *strings.Builder, depth int) {
	if d == nil {
		return
	}

	indent := strings.Repeat("  ", depth)

	if len(d.ref) > 0 {
		sb.WriteString(indent)
		sb.WriteString(d.ref.String())
		if len(d.mappers) > 0 {
			sb.WriteString(" [")
			util.WriteInt(sb, len(d.mappers))
			sb.WriteString(" mapper(s)]")
		}
		sb.WriteByte('\n')
	}

	if d.undefined != nil {
		sb.WriteString(indent)
		sb.WriteString("  undefined:\n")
		d.undefined.format(sb, depth+2)
	}

	if d.any != nil {
		sb.WriteString(indent)
		sb.WriteString("  any:\n")
		d.any.format(sb, depth+2)
	}

	if d.scalars.Len() > 0 {
		scalars := make([]Value, 0, d.scalars.Len())
		d.scalars.Iter(func(key Value, _ *trieNode) bool {
			scalars = append(scalars, key)
			return false
		})
		slices.SortFunc(scalars, Value.Compare)
		for _, k := range scalars {
			child, _ := d.scalars.Get(k)
			sb.WriteString(indent)
			sb.WriteString("  ")
			sb.WriteString(k.String())
			sb.WriteString(":\n")
			child.format(sb, depth+2)
		}
	}

	// Several values reaching one node, so the node is printed once and the
	// values that reach it are named together (see alternativeChildren).
	if d.alternatives != nil {
		for i, conv := range d.alternatives.converged {
			var keys []Value
			d.alternatives.members.Iter(func(k Value, nodes []*trieNode) bool {
				if slices.Contains(nodes, conv) {
					keys = append(keys, k)
				}
				return false
			})
			slices.SortFunc(keys, Value.Compare)

			sb.WriteString(indent)
			sb.WriteString("  any of ")
			for j, k := range keys {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(k.String())
			}
			fmt.Fprintf(sb, " -> #%d:\n", i)
			conv.format(sb, depth+2)
		}
	}

	if d.array != nil {
		sb.WriteString(indent)
		sb.WriteString("  array:\n")
		d.array.format(sb, depth+2)
	}

	for _, p := range d.prefixes.walk() {
		sb.WriteString(indent)
		sb.WriteString(`  prefix "`)
		sb.WriteString(p.prefix)
		sb.WriteString("\":\n")
		p.node.format(sb, depth+2)
	}

	for _, p := range d.suffixes.walk() {
		sb.WriteString(indent)
		sb.WriteString(`  suffix "`)
		sb.WriteString(reverseString(p.prefix))
		sb.WriteString("\":\n")
		p.node.format(sb, depth+2)
	}
}

func (a *arrayTrie) format(sb *strings.Builder, depth int) {
	if a == nil {
		return
	}

	indent := strings.Repeat("  ", depth)

	a.end.format(sb, depth)

	if a.any != nil {
		sb.WriteString(indent)
		sb.WriteString("  any:\n")
		a.any.format(sb, depth+2)
	}

	keys := make([]Value, 0, a.scalars.Len())
	a.scalars.Iter(func(k Value, _ *arrayTrie) bool {
		keys = append(keys, k)
		return false
	})
	slices.SortFunc(keys, Value.Compare)
	for _, k := range keys {
		child, _ := a.scalars.Get(k)
		sb.WriteString(indent)
		sb.WriteString("  ")
		sb.WriteString(k.String())
		sb.WriteString(":\n")
		child.format(sb, depth+2)
	}
}

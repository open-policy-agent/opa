// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"sort"
	"strings"
)

func (node *trieNode) mermaid() string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")
	nodeCounter := 0
	nodeIDs := make(map[*trieNode]string)
	node.mermaidFormat(&sb, &nodeCounter, nodeIDs, "")
	return sb.String()
}

func (node *trieNode) mermaidFormat(sb *strings.Builder, counter *int, nodeIDs map[*trieNode]string, parentID string) {
	currentID, exists := nodeIDs[node]
	if !exists {
		currentID = fmt.Sprintf("n%d", *counter)
		*counter++
		nodeIDs[node] = currentID
		label := node.mermaidLabel()
		fmt.Fprintf(sb, "  %s[\"%s\"]\n", currentID, label)
	}

	if parentID != "" {
		fmt.Fprintf(sb, "  %s --> %s\n", parentID, currentID)
	}

	if exists {
		return
	}

	if node.undefined != nil {
		if childID, childExists := nodeIDs[node.undefined]; childExists {
			fmt.Fprintf(sb, "  %s -->|undefined| %s\n", currentID, childID)
		} else {
			node.undefined.mermaidFormat(sb, counter, nodeIDs, "")
			fmt.Fprintf(sb, "  %s -->|undefined| %s\n", currentID, nodeIDs[node.undefined])
		}
	}

	if node.any != nil {
		if childID, childExists := nodeIDs[node.any]; childExists {
			fmt.Fprintf(sb, "  %s -->|any| %s\n", currentID, childID)
		} else {
			node.any.mermaidFormat(sb, counter, nodeIDs, "")
			fmt.Fprintf(sb, "  %s -->|any| %s\n", currentID, nodeIDs[node.any])
		}
	}

	if node.scalars.Len() > 0 {
		type scalarPair struct {
			key  Value
			node *trieNode
		}
		pairs := make([]scalarPair, 0, node.scalars.Len())
		node.scalars.Iter(func(key Value, val *trieNode) bool {
			pairs = append(pairs, scalarPair{key, val})
			return false
		})
		sort.Slice(pairs, func(a, b int) bool {
			return pairs[a].key.Compare(pairs[b].key) < 0
		})
		for _, pair := range pairs {
			var scalarLabel string
			if s, ok := pair.key.(String); ok {
				scalarLabel = string(s)
			} else {
				scalarLabel = pair.key.String()
			}
			if len(scalarLabel) > 20 {
				scalarLabel = scalarLabel[:20] + "..."
			}
			scalarLabel = mermaidEscape(scalarLabel)
			if childID, childExists := nodeIDs[pair.node]; childExists {
				fmt.Fprintf(sb, "  %s -->|\"%s\"| %s\n", currentID, scalarLabel, childID)
			} else {
				pair.node.mermaidFormat(sb, counter, nodeIDs, "")
				fmt.Fprintf(sb, "  %s -->|\"%s\"| %s\n", currentID, scalarLabel, nodeIDs[pair.node])
			}
		}
	}

	if node.array != nil {
		if childID, childExists := nodeIDs[node.array]; childExists {
			fmt.Fprintf(sb, "  %s -->|array| %s\n", currentID, childID)
		} else {
			node.array.mermaidFormat(sb, counter, nodeIDs, "")
			fmt.Fprintf(sb, "  %s -->|array| %s\n", currentID, nodeIDs[node.array])
		}
	}

	if node.next != nil {
		node.next.mermaidFormat(sb, counter, nodeIDs, currentID)
	}
}

func (node *trieNode) mermaidLabel() string {
	var parts []string

	if len(node.ref) > 0 {
		parts = append(parts, node.ref.String())
	}

	if len(node.rules) > 0 {
		for _, rn := range node.rules {
			bodyStr := ""
			if rn.rule.Body != nil {
				bodyStr = rn.rule.Body.String()
				if len(bodyStr) > 50 {
					bodyStr = bodyStr[:50] + "..."
				}
			}
			bodyStr = mermaidEscape(bodyStr)
			parts = append(parts, bodyStr)
		}
	}

	if len(node.mappers) > 0 {
		parts = append(parts, fmt.Sprintf("%d mapper(s)", len(node.mappers)))
	}
	if node.multiple {
		parts = append(parts, "multiple")
	}

	if len(parts) == 0 {
		return "·"
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
	indent := strings.Repeat("  ", depth)

	if len(node.ref) > 0 {
		sb.WriteString(indent)
		sb.WriteString(node.ref.String())
	} else if depth == 0 {
		sb.WriteString("root")
	}

	if len(node.rules) > 0 {
		fmt.Fprintf(sb, " [%d rule(s)]", len(node.rules))
	}
	if len(node.mappers) > 0 {
		fmt.Fprintf(sb, " [%d mapper(s)]", len(node.mappers))
	}
	if node.value != nil {
		fmt.Fprintf(sb, " value=%v", node.value)
	}
	if node.multiple {
		sb.WriteString(" [multiple]")
	}
	sb.WriteString("\n")

	if node.undefined != nil {
		sb.WriteString(indent)
		sb.WriteString("  undefined:\n")
		node.undefined.format(sb, depth+2)
	}

	if node.any != nil {
		sb.WriteString(indent)
		sb.WriteString("  any:\n")
		node.any.format(sb, depth+2)
	}

	if node.scalars.Len() > 0 {
		scalars := make([]Value, 0, node.scalars.Len())
		nodes := make([]*trieNode, 0, node.scalars.Len())
		node.scalars.Iter(func(key Value, val *trieNode) bool {
			scalars = append(scalars, key)
			nodes = append(nodes, val)
			return false
		})
		sort.Slice(scalars, func(a, b int) bool {
			return scalars[a].Compare(scalars[b]) < 0
		})
		for i := range scalars {
			sb.WriteString(indent)
			fmt.Fprintf(sb, "  %v:\n", scalars[i])
			for j := range nodes {
				if ValueEqual(scalars[i], scalars[j]) {
					nodes[j].format(sb, depth+2)
					break
				}
			}
		}
	}

	if node.array != nil {
		sb.WriteString(indent)
		sb.WriteString("  array:\n")
		node.array.format(sb, depth+2)
	}

	if node.next != nil {
		node.next.format(sb, depth)
	}
}

package ast

import (
	"fmt"
	"sort"
	"strings"
)

// Dump returns a string representation of the tree structure rooted at this node.
func (n *TreeNode) Dump() string {
	var sb strings.Builder
	n.dumpRecursive(&sb, "", "")
	return sb.String()
}

func (n *TreeNode) dumpRecursive(sb *strings.Builder, prefix, childPrefix string) {
	sb.WriteString(prefix)
	fmt.Fprintf(sb, "%v", n.Key)

	if n.Hide {
		sb.WriteString(" [hidden]")
	}
	if n.External != nil {
		fmt.Fprintf(sb, " ext:%v", n.External.Ref)
	}
	if len(n.Values) > 0 {
		fmt.Fprintf(sb, " rules:%d", len(n.Values))
	}
	sb.WriteString("\n")

	if len(n.Children) == 0 {
		return
	}

	keys := make([]Value, 0, len(n.Children))
	for k := range n.Children {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return Compare(keys[i], keys[j]) < 0
	})

	for i, key := range keys {
		child := n.Children[key]
		isLast := i == len(keys)-1
		var newPrefix, newChildPrefix string
		if isLast {
			newPrefix = childPrefix + "└── "
			newChildPrefix = childPrefix + "    "
		} else {
			newPrefix = childPrefix + "├── "
			newChildPrefix = childPrefix + "│   "
		}
		child.dumpRecursive(sb, newPrefix, newChildPrefix)
	}
}

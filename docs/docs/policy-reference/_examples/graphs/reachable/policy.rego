package examples.graph.reachable

# METADATA
# description: |
#   graph.reachable returns the set of all nodes reachable from a starting set of nodes
#   in a directed graph. The graph is represented as an object where keys are nodes
#   and values are arrays of neighboring nodes.
test_simple_directed_graph if {
    g := {
        "a": ["b", "c"],
        "b": ["d"],
        "c": ["d"],
        "d": []
    }

    reachable_from_a := graph.reachable(g, {"a"})
    reachable_from_a == {"a", "b", "c", "d"}

    reachable_from_d := graph.reachable(g, {"d"})
    reachable_from_d == {"d"}
}

# METADATA
# description: |
#   Practical example: Role/permission hierarchy (common in RBAC systems).
#   Shows how permissions/roles can be inherited.
test_role_hierarchy if {
    roles := {
        "admin": ["editor", "viewer"],
        "editor": ["viewer"],
        "viewer": []
    }

    admin_permissions := graph.reachable(roles, {"admin"})
    admin_permissions == {"admin", "editor", "viewer"}

    viewer_permissions := graph.reachable(roles, {"viewer"})
    viewer_permissions == {"viewer"}
}

# METADATA
# description: |
#   Important gotcha: Leaf nodes must be present in the graph (even with empty array).
test_leaf_nodes_required if {
    g := {
        "start": ["middle"],
        "middle": ["end"],
        "end": []          # required for "end" to be reachable
    }

    result := graph.reachable(g, {"start"})
    result == {"start", "middle", "end"}
}

# Used by the documentation site to show output
result := graph.reachable({
    "a": ["b", "c"],
    "b": ["d"],
    "c": ["d"],
    "d": []
}, {"a"})
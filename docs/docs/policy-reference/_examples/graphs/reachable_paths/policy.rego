package examples.graph.reachable_paths

# METADATA
# description: |
#   graph.reachable_paths returns all paths from the starting nodes to reachable nodes.
#   Useful for audit logs, debugging, or tracing dependency/approval chains.
test_simple_paths if {
    g := {
        "a": ["b", "c"],
        "b": ["d"],
        "c": ["d"],
        "d": []
    }

    paths := graph.reachable_paths(g, {"a"})
    count(paths) >= 2
}

# METADATA
# description: |
#   Example: Finding all possible approval chains in an organization.
test_approval_chains if {
    approval_graph := {
        "manager": ["director"],
        "director": ["vp"],
        "vp": []
    }

    paths := graph.reachable_paths(approval_graph, {"manager"})
    count(paths) > 0
}

# Used by the documentation site to show output
result := graph.reachable_paths({
    "aTop": [],
    "cMiddle": ["aTop"],
    "bBottom": ["cMiddle"],
    "dIgnored": []
}, {"bBottom"})
package graph_reachable_paths_example

path_data := {
        "aTop": [],
        "cMiddle": ["aTop"],
        "bBottom": ["cMiddle"],
        "dIgnored": [],
}

all_paths[root] := paths if {
        path_data[root]
        paths := graph.reachable_paths(path_data, {root})
}

result contains all_paths[entity_name]

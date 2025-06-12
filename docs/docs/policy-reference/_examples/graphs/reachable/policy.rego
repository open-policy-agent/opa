package graph_reachable_example

org_chart_data := {
        "ceo": {},
        "human_resources": {"owner": "ceo", "access": ["salaries", "complaints"]},
        "staffing": {"owner": "human_resources", "access": ["interviews"]},
        "internships": {"owner": "staffing", "access": ["blog"]},
}

org_chart_graph[entity_name] := edges if {
        org_chart_data[entity_name]
        edges := {neighbor | org_chart_data[neighbor].owner == entity_name}
}

org_chart_permissions[entity_name] := access if {
        org_chart_data[entity_name]
        reachable := graph.reachable(org_chart_graph, {entity_name})
        access := {item | reachable[k]; item := org_chart_data[k].access[_]}
}

result contains org_chart_permissions[entity_name]

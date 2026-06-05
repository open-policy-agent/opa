package play

# Baseline settings the policy applies to every workload.
defaults := {
	"replicas": 1,
	"tier": "standard",
}

# The request only overrides the fields it cares about.
# Keys in the second object take precedence on conflict.
merged := object.union(defaults, input.overrides)

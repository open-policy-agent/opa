package play

defaults := {
	"replicas": 1,
	"tier": "standard",
}

overrides := {"replicas": 3}

# Keys in the second object take precedence on conflict.
merged := object.union(defaults, overrides)

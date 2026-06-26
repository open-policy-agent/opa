package play

# Merge base required labels with environment-specific ones.
base_required_labels := ["app", "owner"]

env_required_labels := ["environment", "cost-center"]

required_labels := array.concat(base_required_labels, env_required_labels)

# Deny a resource whose metadata is missing any required label.
deny contains msg if {
	some label in required_labels
	not input.metadata.labels[label]
	msg := sprintf("missing required label: %q", [label])
}

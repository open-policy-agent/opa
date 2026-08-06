package play

# object.get(object, key, default) — key may also be a path array, which
# still returns the default when an intermediate field (like labels) is missing.
env_of(workload) := object.get(workload, ["labels", "env"], "dev")

# Only production workloads need a team label.
deny contains msg if {
	some w in input.workloads
	env_of(w) == "prod"
	not w.labels.team
	msg := sprintf("production workload %q is missing labels.team", [w.name])
}

envs := {w.name: env_of(w) | some w in input.workloads}

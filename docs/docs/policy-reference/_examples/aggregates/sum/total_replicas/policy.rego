package play

import rego.v1

# Replicas requested by every Deployment in the input.
replicas := [d.spec.replicas | some d in input.deployments]

total_replicas := sum(replicas)

deny contains msg if {
	total_replicas > input.budget
	msg := sprintf(
		"total replicas %d exceeds namespace budget of %d",
		[total_replicas, input.budget],
	)
}

package play

import rego.v1

# Validate Kubernetes container memory request is reasonable
allow if {
	mem_request := input.container.resources.requests.memory
	mem_limit := input.container.resources.limits.memory

	# Parse memory strings to numeric values
	request_value := units.parse(mem_request)
	limit_value := units.parse(mem_limit)

	# Ensure request is less than limit
	request_value < limit_value

	# Ensure limit is not excessive (less than 8Gi)
	limit_value < units.parse("8Gi")
}

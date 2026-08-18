package play

# Cap for the whole namespace, normalized to a number.
max_memory := units.parse(input.namespace_memory_limit)

# Containers whose limit is above the namespace cap.
deny contains msg if {
	some c in input.containers
	units.parse(c.memory_limit) > max_memory
	msg := sprintf(
		"container %q memory limit %s exceeds namespace cap %s",
		[c.name, c.memory_limit, input.namespace_memory_limit],
	)
}

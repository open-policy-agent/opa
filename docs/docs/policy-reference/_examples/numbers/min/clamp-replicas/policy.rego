package play

min_replicas := 2
max_replicas := 10

# Clamp requested replicas within organizational bounds [2, 10].
effective_replicas := min([
	max_replicas,
	max([min_replicas, input.requested_replicas]),
])

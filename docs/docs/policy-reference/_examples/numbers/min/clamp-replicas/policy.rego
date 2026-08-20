package play

# Clamp requested replicas between minimum and maximum bounds.
clamped := min([
	input.max_allowed,
	max([input.min_allowed, input.requested]),
])

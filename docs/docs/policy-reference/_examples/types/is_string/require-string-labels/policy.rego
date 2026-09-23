package play

# Reject resources with annotations whose values are not strings.
deny contains $"annotation \"{key}\" must be a string value" if {
	some key, val in input.metadata.annotations
	not is_string(val)
}

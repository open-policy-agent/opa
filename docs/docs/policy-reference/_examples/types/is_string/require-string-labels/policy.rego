package play

# Reject resources with annotations whose values are not strings.
deny contains msg if {
	some key, val in input.metadata.annotations
	not is_string(val)
	msg := sprintf("annotation %q must be a string value", [key])
}

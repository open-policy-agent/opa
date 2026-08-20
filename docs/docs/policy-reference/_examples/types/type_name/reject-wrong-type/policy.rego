package play

deny contains msg if {
	type_name(input.replicas) != "number"
	got := type_name(input.replicas)
	msg := sprintf("replicas must be a number, got %s", [got])
}

package play

deny contains msg if {
	got := type_name(input.replicas)
	got != "number"
	msg := sprintf("replicas must be a number, got %s", [got])
}

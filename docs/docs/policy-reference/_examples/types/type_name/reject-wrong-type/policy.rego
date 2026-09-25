package play

deny contains $"replicas must be a number, got {got}" if {
	got := type_name(input.replicas)
	got != "number"
}

package kubernetes.validating.existence

deny contains msg if {
	value := input.request.object.metadata.labels.costcenter
	not startswith(value, "cccode-")
	msg := sprintf("Costcenter must start `cccode-`; found `%v`", [value])
}

deny contains msg if {
	not input.request.object.metadata.labels.costcenter
	msg := "Every resource must have a costcenter label"
}

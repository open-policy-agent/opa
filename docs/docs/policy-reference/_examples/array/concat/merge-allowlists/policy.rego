package play

allowed_registries := array.concat(data.base_registries, input.extra_registries)

default allow := false

allow if {
	some registry in allowed_registries
	startswith(input.image, sprintf("%s/", [registry]))
}

deny contains msg if {
	not allow
	msg := sprintf(
		"image %q is not from an allowed registry: %v",
		[input.image, allowed_registries],
	)
}

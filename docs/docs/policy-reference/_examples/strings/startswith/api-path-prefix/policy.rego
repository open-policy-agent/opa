package play

default allow := false

allow if startswith(input.path, "/api/v1/")

deny contains msg if {
	not allow
	msg := sprintf("path %q is outside /api/v1/", [input.path])
}

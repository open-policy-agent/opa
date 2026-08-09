package play

default allow := false

allow if endswith(input.filename, ".json")
allow if endswith(input.filename, ".yaml")

deny contains msg if {
	not allow
	msg := sprintf("filename %q must end with .json or .yaml", [input.filename])
}

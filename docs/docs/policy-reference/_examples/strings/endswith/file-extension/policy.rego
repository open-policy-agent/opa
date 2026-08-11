package play

deny contains $"disallowed ext: {input.filename}" if {
	not _valid_file_ext
}

_valid_file_ext if endswith(input.filename, ".json")
_valid_file_ext if endswith(input.filename, ".yaml")

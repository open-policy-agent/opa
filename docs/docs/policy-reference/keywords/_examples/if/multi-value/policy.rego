package play

import rego.v1

# missing_paths creates a set missing input paths
missing_paths contains path if {
	some path in data.required_paths

	parts := split(path, ".")

	object.get(input, parts, "") == ""
}

# validations is a mapping of input paths to error messages
validations[path] contains "path must be set" if {
	some path, _ in missing_paths
}

# role and email have additional validation rules
validations.role contains "role cannot be blank" if {
	input.role == ""
}

validations.email contains message if {
	not endswith(input.email, "@example.com")

	message := sprintf("email %s must end with @example.com", [input.email])
}

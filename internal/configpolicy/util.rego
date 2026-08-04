# METADATA
# description: |
#   Helpers shared by the embedded configuration validation policies. Compiled
#   into every configpolicy.Policy, so a policy can import data.opa.config.util
#   instead of carrying its own copy of these rules.
#
#   The helpers read the raw configuration from input.config, the part of the
#   input document every validation policy is given.
package opa.config.util

# METADATA
# description: the configured value at path, or null when the option is absent.
value(path) := object.get(input.config, path, null)

# METADATA
# description: |
#   true when the option at path is missing or explicitly null, the cases where a
#   default is injected, matching the pre-Rego behavior where a nil pointer was
#   replaced with a default.
absent(path) if value(path) == null

# METADATA
# description: |
#   true when the option at path is present but not an object. The shape of an
#   option holding an object has to be checked in the policy rather than left to
#   the Go unmarshal: the default patches would otherwise be merged over the bad
#   value, silently replacing it with a well-formed object.
not_object(path) if {
	v := value(path)
	v != null
	not is_object(v)
}

# METADATA
# description: true when the option at path is present but not a number above zero.
not_positive_number(path) if {
	v := value(path)
	v != null
	not _positive_number(v)
}

_positive_number(v) if {
	is_number(v)
	v > 0
}

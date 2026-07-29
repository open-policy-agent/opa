# METADATA
# description: |
#   Injects defaults and validates the server.decoding configuration (request
#   size limits and gzip decompression). Evaluated by the decoding config
#   builder. The policy rejects options of the wrong shape; the remaining field
#   type validation is handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw server.decoding config>}
#   Rules read by the Go layer: processed (config + defaults), errors (fatal).
package opa.config.server.decoding

# Defaults mirror decoding/config.go.
_default_max_length := 268435456 # 256 MB

_default_gzip_max_length := 536870912 # 512 MB

# METADATA
# description: the config with decoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"max_length": _default_max_length} if _absent(["max_length"])

_patches contains {"gzip": {"max_length": _default_gzip_max_length}} if _absent(["gzip", "max_length"])

errors contains msg if {
	_not_positive_number(["max_length"])
	msg := "invalid value for server.decoding.max_length field, should be a positive number"
}

errors contains msg if {
	_not_object(["gzip"])
	msg := "invalid value for server.decoding.gzip field, should be an object"
}

errors contains msg if {
	_not_positive_number(["gzip", "max_length"])
	msg := "invalid value for server.decoding.gzip.max_length field, should be a positive number"
}

# _value is the configured value at path, or null when the option is absent.
_value(path) := object.get(input.config, path, null)

# _absent is true when the option at path is missing or explicitly null, matching
# the pre-Rego behavior where a nil pointer was replaced with defaults.
_absent(path) if _value(path) == null

# _not_positive_number and _not_object reject present-but-wrong-shaped options.
# The shape of an object option has to be checked here rather than left to the Go
# unmarshal: _patches would otherwise merge the defaults over the bad value and
# silently replace it with a well-formed object.
_not_positive_number(path) if {
	value := _value(path)
	value != null
	not _positive_number(value)
}

_positive_number(value) if {
	is_number(value)
	value > 0
}

_not_object(path) if {
	value := _value(path)
	value != null
	not is_object(value)
}

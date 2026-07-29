# METADATA
# description: |
#   Injects defaults and validates the server.encoding configuration (gzip
#   response compression). Evaluated by the encoding config builder. The policy
#   rejects options of the wrong shape; the remaining field type validation is
#   handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw server.encoding config>}
#   Rules read by the Go layer: processed (config + defaults), errors (fatal).
package opa.config.server.encoding

# Defaults mirror encoding/config.go.
_default_min_length := 1024

_default_compression_level := 9

# _accepted_compression_levels mirrors gzip.NoCompression, gzip.BestSpeed and
# gzip.BestCompression.
_accepted_compression_levels := {0, 1, 9}

# METADATA
# description: the config with gzip encoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"gzip": {"min_length": _default_min_length}} if _absent(["gzip", "min_length"])

_patches contains {"gzip": {"compression_level": _default_compression_level}} if {
	_absent(["gzip", "compression_level"])
}

errors contains msg if {
	_not_object(["gzip"])
	msg := "invalid value for server.encoding.gzip field, should be an object"
}

errors contains msg if {
	_not_positive_number(["gzip", "min_length"])
	msg := "invalid value for server.encoding.gzip.min_length field, should be a positive number"
}

errors contains msg if {
	value := _value(["gzip", "compression_level"])
	value != null
	not value in _accepted_compression_levels
	msg := "invalid value for server.encoding.gzip.compression_level field, accepted values are 0, 1 or 9"
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

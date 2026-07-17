# METADATA
# description: |
#   Injects defaults and validates the server.encoding configuration (gzip
#   response compression). Evaluated by the encoding config builder. Field type
#   validation is handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw server.encoding config>}
#   Entrypoints: processed (config + defaults), errors (fatal).
package opa.config.server.encoding

# Defaults mirror encoding/config.go.
_default_min_length := 1024

_default_compression_level := 9

# _accepted_compression_levels mirrors gzip.NoCompression, gzip.BestSpeed and
# gzip.BestCompression.
_accepted_compression_levels := {0, 1, 9}

# METADATA
# entrypoint: true
# description: the config with gzip encoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"gzip": {"min_length": _default_min_length}} if _absent(["gzip", "min_length"])

_patches contains {"gzip": {"compression_level": _default_compression_level}} if {
	_absent(["gzip", "compression_level"])
}

errors contains msg if {
	processed.gzip.min_length <= 0
	msg := "invalid value for server.encoding.gzip.min_length field, should be a positive number"
}

errors contains msg if {
	not processed.gzip.compression_level in _accepted_compression_levels
	msg := "invalid value for server.encoding.gzip.compression_level field, accepted values are 0, 1 or 9"
}

# _absent is true when the option at path is missing or explicitly null, matching
# the pre-Rego behavior where a nil pointer was replaced with defaults.
_absent(path) if object.get(input.config, path, null) == null

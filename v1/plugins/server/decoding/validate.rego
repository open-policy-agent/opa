# METADATA
# description: |
#   Injects defaults and validates the server.decoding configuration (request
#   size limits and gzip decompression). Evaluated by the decoding config
#   builder. Field type validation is handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw server.decoding config>}
#   Entrypoints: processed (config + defaults), errors (fatal).
package opa.config.server.decoding

# Defaults mirror decoding/config.go.
_default_max_length := 268435456 # 256 MB

_default_gzip_max_length := 536870912 # 512 MB

# METADATA
# entrypoint: true
# description: the config with decoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"max_length": _default_max_length} if _absent(["max_length"])

_patches contains {"gzip": {"max_length": _default_gzip_max_length}} if _absent(["gzip", "max_length"])

errors contains msg if {
	processed.max_length <= 0
	msg := "invalid value for server.decoding.max_length field, should be a positive number"
}

errors contains msg if {
	processed.gzip.max_length <= 0
	msg := "invalid value for server.decoding.gzip.max_length field, should be a positive number"
}

# _absent is true when the option at path is missing or explicitly null, matching
# the pre-Rego behavior where a nil pointer was replaced with defaults.
_absent(path) if object.get(input.config, path, null) == null

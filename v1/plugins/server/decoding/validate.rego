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

import data.opa.config.util

# Defaults mirror decoding/config.go.
_default_max_length := 268435456 # 256 MB

_default_gzip_max_length := 536870912 # 512 MB

# METADATA
# description: the config with decoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"max_length": _default_max_length} if util.absent(["max_length"])

_patches contains {"gzip": {"max_length": _default_gzip_max_length}} if util.absent(["gzip", "max_length"])

errors contains "invalid value for server.decoding.max_length field, should be a positive number" if {
	util.not_positive_number(["max_length"])
}

errors contains "invalid value for server.decoding.gzip field, should be an object" if {
	util.not_object(["gzip"])
}

errors contains "invalid value for server.decoding.gzip.max_length field, should be a positive number" if {
	util.not_positive_number(["gzip", "max_length"])
}

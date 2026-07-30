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

import data.opa.config.util

# Defaults mirror encoding/config.go.
_default_min_length := 1024

_default_compression_level := 9

# _accepted_compression_levels mirrors gzip.NoCompression, gzip.BestSpeed and
# gzip.BestCompression.
_accepted_compression_levels := {0, 1, 9}

# METADATA
# description: the config with gzip encoding defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"gzip": {"min_length": _default_min_length}} if util.absent(["gzip", "min_length"])

_patches contains {"gzip": {"compression_level": _default_compression_level}} if {
	util.absent(["gzip", "compression_level"])
}

errors contains "invalid value for server.encoding.gzip field, should be an object" if {
	util.not_object(["gzip"])
}

errors contains "invalid value for server.encoding.gzip.min_length field, should be a positive number" if {
	util.not_positive_number(["gzip", "min_length"])
}

errors contains "invalid value for server.encoding.gzip.compression_level field, accepted values are 0, 1 or 9" if {
	value := util.value(["gzip", "compression_level"])
	value != null
	not value in _accepted_compression_levels
}

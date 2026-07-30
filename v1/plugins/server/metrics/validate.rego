# METADATA
# description: |
#   Injects defaults and validates the server.metrics configuration (Prometheus
#   HTTP request duration histogram buckets). Evaluated by the metrics config
#   builder. The policy rejects options of the wrong shape; the remaining field
#   type validation is handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw server.metrics config>}
#   Rules read by the Go layer: processed (config + defaults), errors (fatal).
package opa.config.server.metrics

import data.opa.config.util

# _default_buckets mirrors defaultHTTPRequestBuckets in config.go.
_default_buckets := [1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1]

# METADATA
# description: the config with the default histogram buckets injected when absent.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"prom": {"http_request_duration_seconds": {"buckets": _default_buckets}}} if {
	util.absent(["prom", "http_request_duration_seconds", "buckets"])
}

errors contains "invalid value for server.metrics.prom field, should be an object" if {
	util.not_object(["prom"])
}

errors contains "invalid value for server.metrics.prom.http_request_duration_seconds field, should be an object" if {
	util.not_object(["prom", "http_request_duration_seconds"])
}

# The message is a rule of its own because naming the option in the rule head would
# put the line over the length limit.
errors contains _buckets_msg if {
	_not_number_array(["prom", "http_request_duration_seconds", "buckets"])
}

_buckets_field := "server.metrics.prom.http_request_duration_seconds.buckets"

_buckets_msg := $`invalid value for {_buckets_field} field, should be an array of numbers`

# _not_number_array rejects a present-but-wrong-shaped buckets option here rather
# than leaving it to the Go unmarshal, so the message names the config option
# instead of the Go struct field.
_not_number_array(path) if {
	value := util.value(path)
	value != null
	not _number_array(value)
}

_number_array(value) if {
	is_array(value)
	every item in value {
		is_number(item)
	}
}

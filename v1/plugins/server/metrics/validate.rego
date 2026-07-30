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

# _default_buckets mirrors defaultHTTPRequestBuckets in config.go.
_default_buckets := [1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1]

# METADATA
# description: the config with the default histogram buckets injected when absent.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"prom": {"http_request_duration_seconds": {"buckets": _default_buckets}}} if {
	_absent(["prom", "http_request_duration_seconds", "buckets"])
}

errors contains msg if {
	_not_object(["prom"])
	msg := "invalid value for server.metrics.prom field, should be an object"
}

errors contains msg if {
	_not_object(["prom", "http_request_duration_seconds"])
	msg := "invalid value for server.metrics.prom.http_request_duration_seconds field, should be an object"
}

errors contains msg if {
	_not_number_array(["prom", "http_request_duration_seconds", "buckets"])
	msg := sprintf("invalid value for %s field, should be an array of numbers", [_buckets_field])
}

_buckets_field := "server.metrics.prom.http_request_duration_seconds.buckets"

# _value is the configured value at path, or null when the option is absent.
_value(path) := object.get(input.config, path, null)

# _absent is true when the option at path is missing or explicitly null, matching
# the pre-Rego behavior where a nil slice was replaced with defaults.
_absent(path) if _value(path) == null

# _not_object and _not_number_array reject present-but-wrong-shaped options. The
# shape of an object option has to be checked here rather than left to the Go
# unmarshal: _patches would otherwise merge the defaults over the bad value and
# silently replace it with a well-formed object.
_not_object(path) if {
	value := _value(path)
	value != null
	not is_object(value)
}

_not_number_array(path) if {
	value := _value(path)
	value != null
	not _number_array(value)
}

_number_array(value) if {
	is_array(value)
	every item in value {
		is_number(item)
	}
}

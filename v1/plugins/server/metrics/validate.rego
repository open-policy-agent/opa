# METADATA
# description: |
#   Injects defaults for the server.metrics configuration (Prometheus HTTP
#   request duration histogram buckets). Evaluated by the metrics config
#   builder. Value/type validation of the buckets is handled by unmarshaling
#   into the Go struct.
#
#   Input: {"config": <raw server.metrics config>}
#   Entrypoint: processed (config + defaults).
package opa.config.server.metrics

# _default_buckets mirrors defaultHTTPRequestBuckets in config.go.
_default_buckets := [1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1]

# METADATA
# entrypoint: true
# description: the config with the default histogram buckets injected when absent.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"prom": {"http_request_duration_seconds": {"buckets": _default_buckets}}} if {
	_absent(["prom", "http_request_duration_seconds", "buckets"])
}

# _absent is true when the option at path is missing or explicitly null, matching
# the pre-Rego behavior where a nil slice was replaced with defaults.
_absent(path) if object.get(input.config, path, null) == null

# METADATA
# description: |
#   Injects defaults and validates the metrics_export configuration (OTLP metrics
#   export). Evaluated by parseMetricsExportConfig. Field type validation is
#   handled by unmarshaling into the Go struct.
#
#   Input: {"config": <raw metrics_export config>}
#   Entrypoints: processed (config + defaults), errors (fatal).
package opa.config.metrics_export

import future.keywords.not

_default_grpc_address := "localhost:4317"

_default_http_address := "localhost:4318"

_default_export_interval_ms := 60000

_default_service_name := "opa"

_default_encryption_scheme := "off"

_supported_types := {"", "otlp/grpc", "otlp/http"}

_supported_encryption := {"off", "tls", "mtls"}

# The address defaults from the (case-insensitive) type, but only when the type
# selects an exporter.
_default_address := _default_grpc_address if lower(input.config.type) == "otlp/grpc"

_default_address := _default_http_address if lower(input.config.type) == "otlp/http"

# METADATA
# entrypoint: true
# description: the config with metrics_export defaults injected for absent options.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"address": _default_address} if _empty(["address"])

_patches contains {"export_interval_ms": _default_export_interval_ms} if _absent(["export_interval_ms"])

_patches contains {"service_name": _default_service_name} if _empty(["service_name"])

_patches contains {"encryption": _default_encryption_scheme} if _empty(["encryption"])

_patches contains {"allow_insecure_tls": false} if _absent(["allow_insecure_tls"])

errors contains msg if {
	input.config.type
	not lower(input.config.type) in _supported_types
	msg := $`unknown metrics_export.type "{input.config.type}", must be "otlp/grpc", "otlp/http" or "" (unset)`
}

errors contains msg if {
	processed.export_interval_ms <= 0
	msg := $`metrics_export.export_interval_ms must be a positive value, got {processed.export_interval_ms}`
}

errors contains msg if {
	not processed.encryption in _supported_encryption
	msg := $`unsupported metrics_export.encryption "{processed.encryption}"`
}

# _absent is true when the option at path is missing or explicitly null (a nil
# pointer in Go).
_absent(path) if object.get(input.config, path, null) == null

# _empty is true when a string option is missing, null, or the empty string,
# matching the pre-Rego behavior that defaulted on "".
_empty(path) if not _nonempty(path)

_nonempty(path) if {
	v := object.get(input.config, path, null)
	v != null
	v != ""
}

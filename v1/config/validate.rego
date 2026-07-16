# METADATA
# description: |
#   Structural validation of OPA's own configuration, evaluated by
#   config.ParseConfig. It injects the top-level defaults (default_decision,
#   default_authorization_decision, labels) and warns on unrecognized option keys
#   (checked against a schema of known keys, both top-level and within known
#   sections). It does not validate option values or semantics, and does not
#   replace each plugin's own config validation.
#
#   Input: {"config": <raw config>, "runtime": {"id", "version"}}
#   Entrypoints: processed (config + defaults), errors (fatal), warnings.
package opa.config

_default_decision := "/system/main"

_default_authorization_decision := "/system/authz/allow"

# METADATA
# entrypoint: true
# description: |
#   The user config with every _patches fragment merged over it. A fragment
#   wins where it overlaps, so enforced values (labels id/version) always apply,
#   while defaults are contributed only when the option is absent.
processed := object.union_n(array.concat([input.config], [patch | some patch in _patches]))

_patches contains {"default_decision": _default_decision} if _absent_or_null("default_decision")

_patches contains {"default_authorization_decision": _default_authorization_decision} if {
	_absent_or_null("default_authorization_decision")
}

_patches contains {"labels": {"id": input.runtime.id, "version": input.runtime.version}}

# _absent_or_null is true when a config field is missing or explicitly null. In
# both cases the default is injected, matching the pre-Rego behavior where a nil
# pointer was treated as unset.
_absent_or_null(field) if not input.config[field]

_absent_or_null(field) if input.config[field] == null

errors contains msg if {
	some field in {"default_decision", "default_authorization_decision"}
	value := input.config[field]
	value != null
	not is_string(value)
	msg := sprintf("%s must be a string", [field])
}

# warnings reports unrecognized options at any depth. _specs enumerates the known
# keys of each "closed" object; objects without a spec are "open" (any key
# allowed), avoiding false positives for user-extensible sections.
warnings contains msg if {
	walk(input.config, [path, _])
	count(path) > 0

	key := path[count(path) - 1]
	is_string(key) # only object keys are validated, not array indices

	parent := array.slice(path, 0, count(path) - 1)

	some spec in _specs
	_matches(parent, spec.pattern)
	not key in spec.keys

	msg := sprintf("unknown configuration option %q encountered", [_dotted(path)])
}

# _matches tests a config path against a spec pattern; "*" matches any segment.
_matches(path, pattern) if {
	count(path) == count(pattern)
	not _mismatch(path, pattern)
}

_mismatch(path, pattern) if {
	some i, segment in pattern
	segment != "*"
	segment != path[i]
}

_dotted(path) := concat(".", [sprintf("%v", [segment]) | some segment in path])

# _specs is the core specs combined with any specs registered by plugins or
# subsystems (supplied as input.specs by the Go layer). Sections whose specs
# have moved closer to their owning package (e.g. metrics_export,
# distributed_tracing, server subsections) arrive via input.specs.
_specs := array.concat(_core_specs, _input_specs)

default _input_specs := []

_input_specs := input.specs

# _core_specs enumerates the known keys of each "closed" object owned by the
# core config package, derived from the authoritative Go structs.
_core_specs := [
	{"pattern": [], "keys": {
		"services", "labels", "discovery", "bundle", "bundles",
		"decision_logs", "status", "plugins", "keys", "default_decision",
		"default_authorization_decision", "caching", "nd_builtin_cache",
		"persistence_directory", "distributed_tracing", "metrics_export",
		"server", "storage",
	}},
	{"pattern": ["decision_logs"], "keys": {
		"plugin", "service", "partition_name", "reporting", "request_context",
		"mask_decision", "drop_decision", "console", "resource", "nd_builtin_cache",
	}},
	{"pattern": ["decision_logs", "reporting"], "keys": {
		"buffer_type", "buffer_size_limit_bytes", "buffer_size_limit_events",
		"upload_size_limit_bytes", "min_delay_seconds", "max_delay_seconds",
		"max_decisions_per_second", "trigger",
	}},
	{"pattern": ["decision_logs", "request_context"], "keys": {"http"}},
	{"pattern": ["decision_logs", "request_context", "http"], "keys": {"headers"}},
	{"pattern": ["status"], "keys": {
		"plugin", "service", "partition_name", "console", "prometheus",
		"prometheus_config", "trigger",
	}},
	{"pattern": ["status", "prometheus_config"], "keys": {"collectors"}},
	{"pattern": ["status", "prometheus_config", "collectors"], "keys": {"bundle_loading_duration_ns"}},
	{
		"pattern": ["status", "prometheus_config", "collectors", "bundle_loading_duration_ns"],
		"keys": {"buckets"},
	},
	{"pattern": ["discovery"], "keys": {
		"name", "prefix", "decision", "service", "resource", "signing",
		"persist", "trigger", "polling",
	}},
	{"pattern": ["discovery", "polling"], "keys": _polling_keys},
	{"pattern": ["bundle"], "keys": {
		"name", "prefix", "service", "resource", "signing", "persist",
		"size_limit_bytes", "trigger", "polling",
	}},
	{"pattern": ["bundle", "polling"], "keys": _polling_keys},
	{"pattern": ["bundles", "*"], "keys": {
		"service", "resource", "signing", "persist", "size_limit_bytes",
		"trigger", "polling",
	}},
	{"pattern": ["bundles", "*", "polling"], "keys": _polling_keys},
	{"pattern": ["server"], "keys": {"metrics", "encoding", "decoding", "logger_plugin"}},
	{"pattern": ["storage"], "keys": {"disk"}},
	{"pattern": ["storage", "disk"], "keys": {"directory", "auto_create", "partitions", "badger"}},
	{"pattern": ["caching"], "keys": {"inter_query_builtin_cache", "inter_query_builtin_value_cache"}},
	{"pattern": ["caching", "inter_query_builtin_cache"], "keys": {
		"max_size_bytes", "forced_eviction_threshold_percentage",
		"stale_entry_eviction_period_seconds",
	}},
	{"pattern": ["caching", "inter_query_builtin_value_cache"], "keys": {"max_num_entries", "named"}},
	{
		"pattern": ["caching", "inter_query_builtin_value_cache", "named", "*"],
		"keys": {"max_num_entries", "disabled"},
	},
	{"pattern": ["distributed_tracing"], "keys": {
		"type", "address", "service_name", "sample_percentage", "encryption",
		"allow_insecure_tls", "tls_cert_file", "tls_private_key_file",
		"tls_ca_cert_file", "resource", "batch_span_processor_options",
	}},
	{"pattern": ["distributed_tracing", "resource"], "keys": {
		"service_version", "service_instance_id", "service_namespace",
		"deployment_environment",
	}},
	{"pattern": ["distributed_tracing", "batch_span_processor_options"], "keys": {
		"blocking", "batch_timeout_ms", "export_timeout_ms",
		"max_export_batch_size", "max_queue_size",
	}},
	{"pattern": ["services", "*"], "keys": {
		"name", "url", "headers", "allow_insecure_tls",
		"response_header_timeout_seconds", "tls", "credentials", "type",
	}},
	{"pattern": ["keys", "*"], "keys": {"key", "private_key", "algorithm", "scope"}},
]

_polling_keys := {"min_delay_seconds", "max_delay_seconds", "long_polling_timeout_seconds"}

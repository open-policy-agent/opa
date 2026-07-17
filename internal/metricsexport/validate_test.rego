package opa.config.metrics_export_test

import data.opa.config.metrics_export

test_empty_config_defaults if {
	result := metrics_export.processed with input as {"config": {}}
	result.export_interval_ms == 60000
	result.service_name == "opa"
	result.encryption == "off"
	result.allow_insecure_tls == false
	not result.address # no address without a type
}

test_address_default_from_type[tc.note] if {
	some tc in [
		{"note": "grpc", "type": "otlp/grpc", "want": "localhost:4317"},
		{"note": "http", "type": "otlp/http", "want": "localhost:4318"},
		{"note": "case-insensitive", "type": "OTLP/GRPC", "want": "localhost:4317"},
	]

	result := metrics_export.processed with input as {"config": {"type": tc.type}}
	result.address == tc.want
}

test_preserves_configured_values[tc.note] if {
	some tc in [
		{"note": "interval", "config": {"export_interval_ms": 30000}, "key": "export_interval_ms", "want": 30000},
		{"note": "allow_insecure_tls", "config": {"allow_insecure_tls": true}, "key": "allow_insecure_tls", "want": true},
	]

	result := metrics_export.processed with input as {"config": tc.config}
	result[tc.key] == tc.want
}

test_valid_configs_have_no_errors[tc.note] if {
	some tc in [
		{"note": "empty", "config": {}},
		{"note": "grpc", "config": {"type": "otlp/grpc"}},
		{"note": "encryption tls", "config": {"encryption": "tls"}},
	]

	metrics_export.errors == set() with input as {"config": tc.config}
}

test_invalid_configs_report_one_error[tc.note] if {
	some tc in [
		{"note": "invalid type", "config": {"type": "unknown"}},
		{"note": "zero interval", "config": {"export_interval_ms": 0}},
		{"note": "negative interval", "config": {"export_interval_ms": -1}},
		{"note": "unsupported encryption", "config": {"encryption": "bad"}},
	]

	errs := metrics_export.errors with input as {"config": tc.config}
	count(errs) == 1
}

package opa.config.server.decoding_test

import data.opa.config.server.decoding

test_injects_defaults[tc.note] if {
	some tc in [
		{"note": "empty config", "config": {}},
		{"note": "gzip present", "config": {"gzip": {}}},
		{"note": "max_length null", "config": {"max_length": null}},
		{"note": "gzip.max_length null", "config": {"gzip": {"max_length": null}}},
	]

	result := decoding.processed with input as {"config": tc.config}
	result.max_length == 268435456
	result.gzip.max_length == 536870912
}

test_preserves_configured_values if {
	raw := {"max_length": 5, "gzip": {"max_length": 42}}
	result := decoding.processed with input as {"config": raw}
	result.max_length == 5
	result.gzip.max_length == 42
}

test_preserves_unknown_keys if {
	result := decoding.processed with input as {"config": {"gzip": {"random_key": 0}}}
	result.gzip.random_key == 0
}

test_rejects_non_positive_max_length[tc.note] if {
	some tc in [
		{"note": "zero", "config": {"gzip": {"max_length": 0}}},
		{"note": "negative", "config": {"gzip": {"max_length": -10}}},
	]

	result := decoding.errors with input as {"config": tc.config}
	"invalid value for server.decoding.gzip.max_length field, should be a positive number" in result
}

test_rejects_non_positive_top_level_max_length if {
	result := decoding.errors with input as {"config": {"max_length": -10}}
	"invalid value for server.decoding.max_length field, should be a positive number" in result
}

test_valid_config_has_no_errors if {
	result := decoding.errors with input as {"config": {"max_length": 42, "gzip": {"max_length": 42}}}
	count(result) == 0
}

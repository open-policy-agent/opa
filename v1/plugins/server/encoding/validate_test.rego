package opa.config.server.encoding_test

import data.opa.config.server.encoding

test_injects_defaults[tc.note] if {
	some tc in [
		{"note": "empty config", "config": {}},
		{"note": "gzip present", "config": {"gzip": {}}},
		{"note": "min_length null", "config": {"gzip": {"min_length": null}}},
		{"note": "compression_level null", "config": {"gzip": {"compression_level": null}}},
	]

	result := encoding.processed with input as {"config": tc.config}
	result.gzip.min_length == 1024
	result.gzip.compression_level == 9
}

test_preserves_configured_values if {
	raw := {"gzip": {"min_length": 42, "compression_level": 1}}
	result := encoding.processed with input as {"config": raw}
	result.gzip.min_length == 42
	result.gzip.compression_level == 1
}

test_preserves_unknown_keys if {
	result := encoding.processed with input as {"config": {"gzip": {"random_key": 0}}}
	result.gzip.random_key == 0
}

test_rejects_non_positive_min_length[tc.note] if {
	some tc in [
		{"note": "zero", "min_length": 0},
		{"note": "negative", "min_length": -10},
	]

	result := encoding.errors with input as {"config": {"gzip": {"min_length": tc.min_length}}}
	"invalid value for server.encoding.gzip.min_length field, should be a positive number" in result
}

# A non-number min_length is rejected here rather than left to the Go unmarshal,
# so the message names the option instead of the Go field.
test_rejects_non_number_min_length[tc.note] if {
	some tc in [
		{"note": "string", "min_length": "foobar"},
		{"note": "boolean", "min_length": true},
		{"note": "array", "min_length": [1]},
	]

	result := encoding.errors with input as {"config": {"gzip": {"min_length": tc.min_length}}}
	"invalid value for server.encoding.gzip.min_length field, should be a positive number" in result
}

# Without this check the gzip defaults would be merged over the bad value,
# silently replacing it with a well-formed object.
test_rejects_non_object_gzip[tc.note] if {
	some tc in [
		{"note": "array", "config": {"gzip": [1, 2, 3]}},
		{"note": "string", "config": {"gzip": "nope"}},
		{"note": "number", "config": {"gzip": 7}},
	]

	result := encoding.errors with input as {"config": tc.config}
	"invalid value for server.encoding.gzip field, should be an object" in result
}

test_rejects_unaccepted_compression_level[tc.note] if {
	some tc in [
		{"note": "out of range", "compression_level": 13},
		{"note": "negative", "compression_level": -1},
		{"note": "string", "compression_level": "9"},
	]

	result := encoding.errors with input as {"config": {"gzip": {"compression_level": tc.compression_level}}}
	"invalid value for server.encoding.gzip.compression_level field, accepted values are 0, 1 or 9" in result
}

test_accepts_valid_compression_levels[tc.note] if {
	some tc in [
		{"note": "none", "compression_level": 0},
		{"note": "best speed", "compression_level": 1},
		{"note": "best compression", "compression_level": 9},
	]

	result := encoding.errors with input as {"config": {"gzip": {"compression_level": tc.compression_level}}}
	count(result) == 0
}

test_empty_config_has_no_errors if {
	result := encoding.errors with input as {"config": {}}
	count(result) == 0
}

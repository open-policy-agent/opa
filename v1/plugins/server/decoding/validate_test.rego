package opa.config.server.decoding_test

import data.opa.config.server.decoding

# _non_numbers are values a max_length option must be rejected for, shared by the
# top-level and gzip cases.
_non_numbers := [
	{"note": "string", "value": "foobar"},
	{"note": "boolean", "value": true},
	{"note": "array", "value": [1]},
]

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

test_rejects_non_positive_gzip_max_length[tc.note] if {
	some tc in [
		{"note": "zero", "config": {"gzip": {"max_length": 0}}},
		{"note": "negative", "config": {"gzip": {"max_length": -10}}},
	]

	result := decoding.errors with input as {"config": tc.config}
	"invalid value for server.decoding.gzip.max_length field, should be a positive number" in result
}

test_rejects_non_positive_top_level_max_length[tc.note] if {
	some tc in [
		{"note": "zero", "config": {"max_length": 0}},
		{"note": "negative", "config": {"max_length": -10}},
	]

	result := decoding.errors with input as {"config": tc.config}
	"invalid value for server.decoding.max_length field, should be a positive number" in result
}

# A non-number max_length is rejected here rather than left to the Go unmarshal,
# so the message names the option instead of the Go field.
test_rejects_non_number_top_level_max_length[tc.note] if {
	some tc in _non_numbers

	result := decoding.errors with input as {"config": {"max_length": tc.value}}
	"invalid value for server.decoding.max_length field, should be a positive number" in result
}

test_rejects_non_number_gzip_max_length[tc.note] if {
	some tc in _non_numbers

	result := decoding.errors with input as {"config": {"gzip": {"max_length": tc.value}}}
	"invalid value for server.decoding.gzip.max_length field, should be a positive number" in result
}

# Without this check the gzip defaults would be merged over the bad value,
# silently replacing it with a well-formed object.
test_rejects_non_object_gzip[tc.note] if {
	some tc in [
		{"note": "array", "config": {"gzip": [1, 2, 3]}},
		{"note": "string", "config": {"gzip": "nope"}},
		{"note": "number", "config": {"gzip": 7}},
	]

	result := decoding.errors with input as {"config": tc.config}
	"invalid value for server.decoding.gzip field, should be an object" in result
}

test_valid_config_has_no_errors if {
	result := decoding.errors with input as {"config": {"max_length": 42, "gzip": {"max_length": 42}}}
	count(result) == 0
}

test_empty_config_has_no_errors if {
	result := decoding.errors with input as {"config": {}}
	count(result) == 0
}

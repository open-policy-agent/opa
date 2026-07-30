package opa.config.server.metrics_test

import data.opa.config.server.metrics

_default_buckets := [1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1]

_buckets_field := "server.metrics.prom.http_request_duration_seconds.buckets"

_buckets_msg := $`invalid value for {_buckets_field} field, should be an array of numbers`

test_injects_default_buckets[tc.note] if {
	some tc in [
		{"note": "empty config", "config": {}},
		{"note": "prom present", "config": {"prom": {}}},
		{"note": "section present", "config": {"prom": {"http_request_duration_seconds": {}}}},
		{"note": "buckets null", "config": {"prom": {"http_request_duration_seconds": {"buckets": null}}}},
	]

	result := metrics.processed with input as {"config": tc.config}
	result.prom.http_request_duration_seconds.buckets == _default_buckets
}

test_preserves_configured_buckets[tc.note] if {
	some tc in [
		{"note": "custom buckets", "buckets": [0.1, 0.2, 0.3, 4]},
		{"note": "explicit empty", "buckets": []},
	]

	raw := {"prom": {"http_request_duration_seconds": {"buckets": tc.buckets}}}
	result := metrics.processed with input as {"config": raw}
	result.prom.http_request_duration_seconds.buckets == tc.buckets
}

test_preserves_unknown_keys if {
	result := metrics.processed with input as {"config": {"prom": {"random_key": 0}}}
	result.prom.random_key == 0
}

# Without these checks the bucket defaults would be merged over the bad value,
# silently replacing it with a well-formed object.
test_rejects_non_object_prom[tc.note] if {
	some tc in [
		{"note": "array", "config": {"prom": [1, 2, 3]}},
		{"note": "string", "config": {"prom": "nope"}},
		{"note": "number", "config": {"prom": 7}},
	]

	result := metrics.errors with input as {"config": tc.config}
	"invalid value for server.metrics.prom field, should be an object" in result
}

test_rejects_non_object_http_request_duration_seconds[tc.note] if {
	some tc in [
		{"note": "array", "value": [1]},
		{"note": "string", "value": "x"},
	]

	result := metrics.errors with input as {"config": {"prom": {"http_request_duration_seconds": tc.value}}}
	"invalid value for server.metrics.prom.http_request_duration_seconds field, should be an object" in result
}

# A non-numeric buckets value is rejected here rather than left to the Go
# unmarshal, so the message names the option instead of the Go field.
test_rejects_non_number_array_buckets[tc.note] if {
	some tc in [
		{"note": "string", "buckets": "x"},
		{"note": "object", "buckets": {}},
		{"note": "array of strings", "buckets": ["a"]},
		{"note": "mixed array", "buckets": [1, "a"]},
	]

	config := {"prom": {"http_request_duration_seconds": {"buckets": tc.buckets}}}
	result := metrics.errors with input as {"config": config}
	_buckets_msg in result
}

test_valid_config_has_no_errors[tc.note] if {
	some tc in [
		{"note": "empty config", "config": {}},
		{"note": "prom present", "config": {"prom": {}}},
		{"note": "section present", "config": {"prom": {"http_request_duration_seconds": {}}}},
		{"note": "custom buckets", "config": {"prom": {"http_request_duration_seconds": {"buckets": [0.1, 4]}}}},
		{"note": "empty buckets", "config": {"prom": {"http_request_duration_seconds": {"buckets": []}}}},
		{"note": "buckets null", "config": {"prom": {"http_request_duration_seconds": {"buckets": null}}}},
	]

	result := metrics.errors with input as {"config": tc.config}
	count(result) == 0
}

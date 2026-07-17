package opa.config.server.metrics_test

import data.opa.config.server.metrics

_default_buckets := [1e-6, 5e-6, 1e-5, 5e-5, 1e-4, 5e-4, 1e-3, 0.01, 0.1, 1]

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

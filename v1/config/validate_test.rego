package opa.config_test

import data.opa.config

# _runtime is the runtime document supplied by the Go layer.
_runtime := {"id": "test-id", "version": "test-version"}

# _input builds a policy input document from a raw configuration object.
_input(raw) := {"config": raw, "runtime": _runtime}

test_processed_injects_default_decisions if {
	result := config.processed with input as _input({})
	result.default_decision == "/system/main"
	result.default_authorization_decision == "/system/authz/allow"
}

test_processed_preserves_configured_decisions if {
	raw := {"default_decision": "/foo/bar", "default_authorization_decision": "/baz/qux"}
	result := config.processed with input as _input(raw)
	result.default_decision == "/foo/bar"
	result.default_authorization_decision == "/baz/qux"
}

test_processed_defaults_when_decision_is_null if {
	result := config.processed with input as _input({"default_decision": null})
	result.default_decision == "/system/main"
}

test_no_error_when_decision_is_null if {
	config.errors == set() with input as _input({"default_decision": null})
}

test_processed_injects_labels if {
	result := config.processed with input as _input({"labels": {"region": "eu"}})
	result.labels == {"region": "eu", "id": "test-id", "version": "test-version"}
}

# Each case is a configuration whose typo'd option should be reported by exactly
# one warning naming its dotted path.
test_warns_on_unknown_option[tc.note] if {
	some tc in [
		{
			# The motivating example from issue #2745: "decision_log" vs "decision_logs".
			"note": "top-level typo",
			"config": {"decision_log": {"console": true}},
			"want": "decision_log",
		},
		{
			"note": "nested option typo",
			"config": {"decision_logs": {"consoel": true}},
			"want": "decision_logs.consoel",
		},
		{
			"note": "typo in a named map entry",
			"config": {"bundles": {"authz": {"servcie": "s1"}}},
			"want": "bundles.authz.servcie",
		},
		{
			"note": "typo in a service (array) entry",
			"config": {"services": [{"name": "s1", "urll": "https://example.com"}]},
			"want": "services.0.urll",
		},
	]

	msgs := config.warnings with input as _input(tc.config)
	msgs == {sprintf("unknown configuration option %q encountered", [tc.want])}
}

# Each case is a valid configuration that must produce no warnings.
test_no_warnings[tc.note] if {
	some tc in [
		{
			"note": "valid config across sections",
			"config": {
				"decision_logs": {"console": true},
				"bundles": {"authz": {"service": "s1", "resource": "bundle.tar.gz"}},
				"services": [{"name": "s1", "url": "https://example.com"}],
				"labels": {"anything": "goes"},
				"plugins": {"custom_plugin": {"whatever": true}},
			},
		},
		{
			"note": "open sections with arbitrary keys",
			"config": {
				"labels": {"team": "x", "custom": "y"},
				"plugins": {"my_plugin": {"arbitrary": {"nested": true}}},
				"keys": {"my_key": {"key": "abc", "algorithm": "HS256"}},
			},
		},
	]

	config.warnings == set() with input as _input(tc.config)
}

test_errors_on_non_string_decision if {
	msgs := config.errors with input as _input({"default_decision": 42})
	msgs == {"default_decision must be a string"}
}

test_no_errors_for_valid_config if {
	config.errors == set() with input as _input({"decision_logs": {"console": true}})
}

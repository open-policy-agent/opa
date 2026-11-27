package test

test_unknown_rules if {
	report := prepared.prepare.validate.unknown
		with data.eval.params.enable as ["foo"]
		with data.eval.params.disable as ["bar"]
		with data.regal.config.rules as {"odd": {"baz": {}}}
		# qux is the only known rule
		with data.internal.rules as {"bugs": {"qux": {}}}

	report == {"unknown rule: foo", "unknown rule: bar", "unknown rule: baz"}
}

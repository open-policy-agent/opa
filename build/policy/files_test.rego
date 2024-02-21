package files_test

import future.keywords.in

import data.files.deny

test_deny_invalid_yaml_file {
	expected := "invalid.yaml is an invalid YAML file: {null{}}"
	expected in deny with data.files.yaml_file_contents as {"invalid.yaml": "{null{}}"}
		with data.files.changes as {"invalid.yaml": {"status": "modified"}}
}

test_allow_valid_yaml_file {
	count(deny) == 0 with data.files.yaml_file_contents as {"valid.yaml": "foo: bar"}
		with data.files.changes as {"valid.yaml": {"status": "modified"}}
}

test_deny_invalid_json_file {
	expected := "invalid.json is an invalid JSON file: }}}"
	expected in deny with data.files.json_file_contents as {"invalid.json": "}}}"}
		with data.files.changes as {"invalid.json": {"status": "modified"}}
}

test_allow_valid_json_file {
	count(deny) == 0 with data.files.json_file_contents as {"valid.json": "{\"foo\": \"bar\"}"}
		with data.files.changes as {"valid.json": {"status": "modified"}}
}

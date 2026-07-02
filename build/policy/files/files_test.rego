package files_test

import data.files

test_deny_invalid_yaml_file if {
	expected := "invalid.yaml is an invalid YAML file: {null{}}"
	expected in files.deny with files.yaml_file_contents as {"invalid.yaml": "{null{}}"}
		with files.changes as {"invalid.yaml": {"status": "modified"}}
}

test_allow_valid_yaml_file if {
	count(files.deny) == 0 with files.yaml_file_contents as {"valid.yaml": "foo: bar"}
		with files.changes as {"valid.yaml": {"status": "modified"}}
}

test_deny_invalid_json_file if {
	expected := "invalid.json is an invalid JSON file: }}}"
	expected in files.deny with files.json_file_contents as {"invalid.json": "}}}"}
		with files.changes as {"invalid.json": {"status": "modified"}}
}

test_allow_valid_json_file if {
	count(files.deny) == 0 with files.json_file_contents as {"valid.json": "{\"foo\": \"bar\"}"}
		with files.changes as {"valid.json": {"status": "modified"}}
}

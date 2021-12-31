package files_test

import data.files.deny

test_deny_logo_if_added_in_wrong_directory {
	expected := "Logo must be placed in docs/website/static/img/logos/integrations"
	deny[expected] with input as [
		{
			"filename": "docs/website/data/integrations.yaml",
			"status": "modified",
		},
		{
			"filename": "docs/website/static/img/logos/example.png",
			"status": "added",
		},
	]
}

test_allow_logo_if_added_in_correct_directory {
	count(deny) == 0 with input as [
		{
			"filename": "docs/website/data/integrations.yaml",
			"status": "modified",
		},
		{
			"filename": "docs/website/static/img/logos/integrations/example.png",
			"status": "added",
		},
	]
}

test_deny_logo_if_not_png_file {
	expected := "Logo must be a .png file"
	deny[expected] with input as [
		{
			"filename": "docs/website/data/integrations.yaml",
			"status": "modified",
		},
		{
			"filename": "docs/website/static/img/logos/integrations/example.jpg",
			"status": "added",
		},
	]
}

test_deny_invalid_yaml_file {
	expected := "invalid.yaml is an invalid YAML file"
	deny[expected] with data.files.yaml_file_contents as {"invalid.yaml": "{null{}}"}
		 with data.files.changes as {"invalid.yaml": {"status": "modified"}}
}

test_allow_valid_yaml_file {
	count(deny) == 0 with data.files.yaml_file_contents as {"valid.yaml": "foo: bar"}
		 with data.files.changes as {"valid.yaml": {"status": "modified"}}
}

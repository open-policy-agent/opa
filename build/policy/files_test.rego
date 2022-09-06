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
	integrations := yaml.marshal({"integrations": {"example": {
		"title": "My test integration",
		"description": "Testing",
	}}})

	count(deny) == 0 with data.files.integrations_file as integrations with input as [
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
	expected := "Logo must be a .png or .svg file"
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

test_deny_logo_if_no_matching_integration {
	integrations := yaml.marshal({"integrations": {"my-integration": {
		"title": "My test integration",
		"description": "Testing",
	}}})

	files := [
		{
			"filename": "docs/website/data/integrations.yaml",
			"status": "modified",
		},
		{
			"filename": "docs/website/static/img/logos/integrations/example.png",
			"status": "added",
		},
	]

	expected := "Logo name must match integration"

	deny[expected] with data.files.integrations_file as integrations with input as files
}

test_allow_logo_if_no_matching_integration {
	integrations := yaml.marshal({"integrations": {"my-integration": {
		"title": "My test integration",
		"description": "Testing",
	}}})

	files := [
		{
			"filename": "docs/website/data/integrations.yaml",
			"status": "modified",
		},
		{
			"filename": "docs/website/static/img/logos/integrations/my-integration.png",
			"status": "added",
		},
	]

	count(deny) == 0 with data.files.integrations_file as integrations with input as files
}

test_deny_integration_if_missing_required_attribute {
	expected := "Integration 'my-integration' missing required attribute 'description'"
	files := [{"filename": "docs/website/data/integrations.yaml"}]
	integrations := yaml.marshal({"integrations": {"my-integration": {
		"title": "My test integration",
		"inventors": ["acmecorp"],
	}}})

	deny[expected] with data.files.integrations_file as integrations with input as files
}

test_deny_integration_allowed_with_required_attributes {
	files := [{"filename": "docs/website/data/integrations.yaml"}]
	integrations := yaml.marshal({"integrations": {"my-integration": {
		"title": "My test integration",
		"description": "This is a test integration",
		"inventors": ["acmecorp"],
	}}})

	count(deny) == 0 with data.files.integrations_file as integrations with input as files
}

test_deny_unlisted_software {
	files := [{"filename": "docs/website/data/integrations.yaml"}]
	integrations := yaml.marshal({
		"integrations": {"my-integration": {
			"title": "My test integration",
			"description": "This is a test integration",
			"software": ["bitcoin-miner"],
		}},
		"software": {"kubernetes": {"name": "Kubernetes"}},
	})

	expected := "Integration 'my-integration' references unknown software 'bitcoin-miner' (i.e. not in 'software' object)"

	deny[expected] with data.files.integrations_file as integrations with input as files
}

test_allow_listed_software {
	files := [{"filename": "docs/website/data/integrations.yaml"}]
	integrations := yaml.marshal({
		"integrations": {"my-integration": {
			"title": "My test integration",
			"description": "This is a test integration",
			"software": ["kubernetes"],
		}},
		"software": {"kubernetes": {"name": "Kubernetes"}},
	})

	count(deny) == 0 with data.files.integrations_file as integrations with input as files
}

test_deny_invalid_yaml_file {
	expected := "invalid.yaml is an invalid YAML file: {null{}}"
	deny[expected] with data.files.yaml_file_contents as {"invalid.yaml": "{null{}}"}
		with data.files.changes as {"invalid.yaml": {"status": "modified"}}
}

test_allow_valid_yaml_file {
	count(deny) == 0 with data.files.yaml_file_contents as {"valid.yaml": "foo: bar"}
		with data.files.changes as {"valid.yaml": {"status": "modified"}}
}

test_deny_invalid_json_file {
	expected := "invalid.json is an invalid JSON file: }}}"
	deny[expected] with data.files.json_file_contents as {"invalid.json": "}}}"}
		with data.files.changes as {"invalid.json": {"status": "modified"}}
}

test_allow_valid_json_file {
	count(deny) == 0 with data.files.json_file_contents as {"valid.json": "{\"foo\": \"bar\"}"}
		with data.files.changes as {"valid.json": {"status": "modified"}}
}

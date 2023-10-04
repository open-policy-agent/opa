package integrations_test

import future.keywords.in

messages_for_key(key, output) = messages {
	messages := {m |
		some e
		output[e]

		key in e

		m := e.message
	}
}

print_if(true, _, _, _) = true

print_if(false, key, false, output) := false {
	print("Exp:", {})
	print("Got: ", messages_for_key(key, output))
}

print_if(false, key, expected, output) := false {
	is_string(expected)
	print("Exp:", expected)
	print("Got:", messages_for_key(key, output))
}

test_integration_has_required_fields_missing {
	output := data.integrations.deny with input as {"integrations": {"/integrations/regal/": {}}}

	key := "fields"
	message := "/integrations/regal/ missing required fields: title"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_integration_has_required_fields_present {
	output := data.integrations.deny with input as {"integrations": {"/integrations/regal/": {"title": "Regal"}}}

	key := "fields"
	message := "/integrations/regal/ missing required fields: title"

	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_integration_has_content_missing {
	output := data.integrations.deny with input as {"integrations": {"/integrations/regal/": {}}}

	key := "content"
	message := "/integrations/regal/ has no content"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_integration_has_content_blank {
	output := data.integrations.deny with input as {"integrations": {"/integrations/regal/": {"content": "\t\t\n   "}}}

	key := "content"
	message := "/integrations/regal/ has no content"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_integration_has_content_present {
	output := data.integrations.deny with input as {"integrations": {"/integrations/regal/": {"content": "foobar"}}}

	key := "content"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_every_integration_has_image_missing {
	output := data.integrations.deny with input as {
		"images": ["reegal.png"],
		"integrations": {"/integrations/regal/": {}},
	}

	key := "integration_image"
	message := "/integrations/regal/ missing image in 'static/img/logos/integrations' with extension of: png,svg"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_every_integration_has_image_present {
	output := data.integrations.deny with input as {
		"images": ["regal.png"],
		"integrations": {"regal": {}},
	}

	key := "integration_image"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_every_integration_has_image_missing_but_permitted {
	output := data.integrations.deny with input as {
		"images": ["reegal.png"],
		"integrations": {"regal": {"allow_missing_image": true}},
	}

	key := "integration_image"

	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_every_image_has_integration_missing {
	output := data.integrations.deny with input as {
		"images": ["regal.png"],
		"integrations": {"foobar": {}},
	}

	key := "image_integration"
	message := "image regal.png is not used by any integration page"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_every_image_has_integration_present {
	output := data.integrations.deny with input as {
		"images": ["regal.png"],
		"integrations": {"/integrations/regal/": {}},
	}

	key := "image_integration"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_integration_organizations_missing {
	output := data.integrations.deny with input as {
		"organizations": {"/organizations/stira/": {}},
		"integrations": {"/integrations/regal/": {"inventors": ["styra"]}},
	}

	key := "inventors"
	message := "/integrations/regal/ references organization styra which does not exist"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_integration_organizations_present {
	output := data.integrations.deny with input as {
		"organizations": {"/organizations/styra/": {}},
		"integrations": {"/integrations/regal/": {"inventors": ["styra"]}},
	}

	key := "inventors"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_integration_softwares_missing {
	output := data.integrations.deny with input as {
		"softwares": {"/softwares/mars/": {}},
		"integrations": {"/integrations/regal/": {"software": ["terraform"]}},
	}

	key := "software"
	message := "/integrations/regal/ references software terraform which does not exist"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_integration_softwares_present {
	output := data.integrations.deny with input as {
		"softwares": {"/softwares/terraform/": {}},
		"integrations": {"/integrations/regal/": {"software": ["terraform"]}},
	}

	key := "software"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_software_has_required_fields_missing {
	output := data.integrations.deny with input as {"softwares": {"/softwares/terraform/": {}}}

	key := "fields"
	message := "/softwares/terraform/ missing required fields: link, title"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_software_has_required_fields_present {
	output := data.integrations.deny with input as {"softwares": {"terraform": {"link": "https://www.terraform.io/", "title": "Terraform"}}}

	key := "fields"

	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_organization_has_required_labels {
	output := data.integrations.deny with input as {"organizations": {"/organizations/styra/": {}}}

	key := "fields"
	message := "/organizations/styra/ missing required fields: link, title"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_organization_has_required_fields_present {
	output := data.integrations.deny with input as {"organizations": {"styra": {"link": "https://styra.com/", "title": "Styra"}}}

	key := "fields"

	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_organization_has_one_or_more_integrations_none {
	output := data.integrations.deny with input as {"organizations": {"/organizations/foobar/": {}}, "integrations": {}}

	key := "orphaned_org"
	message := "/organizations/foobar/ has no integrations"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_organization_has_one_or_more_integrations_one {
	output := data.integrations.deny with input as {"organizations": {"/organizations/foobaz/": {}}, "integrations": {"/integrations/foobar/": {"inventors": ["foobaz"]}}}

	key := "orphaned_org"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_organization_has_one_or_more_integrations_speaker {
	output := data.integrations.deny with input as {"organizations": {"foobaz": {}}, "integrations": {"foobar": {"videos": [{"speakers": [{"organization": "foobaz"}]}]}}}

	key := "orphaned_org"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

test_software_has_one_or_more_integrations_none {
	output := data.integrations.deny with input as {"softwares": {"/softwares/foobar/": {}}, "integrations": {}}

	key := "orphaned_software"
	message := "/softwares/foobar/ has no integrations"

	got := messages_for_key(key, output)

	result := message in got

	print_if(result, key, message, output)
}

test_software_has_one_or_more_integrations_one {
	output := data.integrations.deny with input as {"softwares": {"foobaz": {}}, "integrations": {"foobar": {"software": ["foobaz"]}}}

	key := "orphaned_software"
	got := messages_for_key(key, output)

	result := got == set()

	print_if(result, key, false, output)
}

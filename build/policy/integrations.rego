package integrations

import future.keywords.contains
import future.keywords.if
import future.keywords.in

allowed_image_extensions := ["png", "svg"]

# check that all integrations have an image
deny contains result if {
	some path, integration in input.integrations

	id := split(path, "/")[2]

	# some integrations are allowed to have a missing image as no suitable image is available
	not integration.allow_missing_image == true

	some _, ext in allowed_image_extensions

	possible_filenames := {e |
		some i
		ext := allowed_image_extensions[i]

		e := sprintf("%s.%s", [id, ext])
	}

	possible_filenames - {i | i := input.images[_]} == possible_filenames

	result := {
		"key": "integration_image",
		"message": sprintf("%s missing image in 'static/img/logos/integrations' with extension of: %v", [path, concat(",", allowed_image_extensions)]),
	}
}

# check that all images have an integration
deny contains result if {
	some _, image in input.images

	id := split(image, ".")[0]

	not sprintf("/integrations/%s/", [id]) in object.keys(input.integrations)

	result := {
		"key": "image_integration",
		"message": sprintf("image %s is not used by any integration page", [image]),
	}
}

# check that all integrations have the required fields
deny contains result if {
	some id, integration in input.integrations

	missing_fields := {"title"} - object.keys(integration)

	count(missing_fields) > 0

	result := {
		"key": "fields",
		"message": sprintf("%s missing required fields: %v", [id, concat(", ", sort(missing_fields))]),
	}
}

# check that all integrations have content
deny contains result if {
	some id, integration in input.integrations

	content := trim_space(object.get(integration, "content", ""))

	content == ""

	result := {
		"key": "content",
		"message": sprintf("%s has no content", [id]),
	}
}

# check that all integrations reference an existing organization
deny contains result if {
	some id, integration in input.integrations

	inventors := object.get(integration, "inventors", [])

	some _, inventor in inventors

	not sprintf("/organizations/%s/", [inventor]) in object.keys(input.organizations)

	result := {
		"key": "inventors",
		"message": sprintf("%s references organization %s which does not exist", [id, inventor]),
	}
}

# check that all integrations reference existing software
deny contains result if {
	some id, integration in input.integrations

	softwares := object.get(integration, "software", [])

	some _, software in softwares

	not sprintf("/softwares/%s/", [software]) in object.keys(input.softwares)

	result := {
		"key": "software",
		"message": sprintf("%s references software %s which does not exist", [id, software]),
	}
}

# check that softwares have required fields
deny contains result if {
	some id, software in input.softwares

	missing_fields := {"title", "link"} - object.keys(software)

	count(missing_fields) > 0

	result := {
		"key": "fields",
		"message": sprintf("%s missing required fields: %v", [id, concat(", ", sort(missing_fields))]),
	}
}

# check that organizations have required fields
deny contains result if {
	some path, organization in input.organizations

	missing_fields := {"title", "link"} - object.keys(organization)

	count(missing_fields) > 0

	result := {
		"key": "fields",
		"message": sprintf("%s missing required fields: %v", [path, concat(", ", sort(missing_fields))]),
	}
}

# check that each organization has at least one integration
deny contains result if {
	some path, organization in input.organizations

	id := split(path, "/")[2]

	inventor_integrations := {i |
		some i, integration in input.integrations
		id in integration.inventors
	}
	speaker_integrations := {i |
		some i, integration in input.integrations
		some _, video in integration.videos

		some _, speaker in video.speakers

		speaker.organization == id
	}

	count(inventor_integrations) + count(speaker_integrations) == 0

	result := {
		"key": "orphaned_org",
		"message": sprintf("%s has no integrations", [path]),
	}
}

# check that each software has at least one integration
deny contains result if {
	some path, software in input.softwares

	id := split(path, "/")[2]

	integrations := {i |
		some i, integration in input.integrations
		id in integration.software
	}

	count(integrations) == 0

	result := {
		"key": "orphaned_software",
		"message": sprintf("%s has no integrations", [path]),
	}
}

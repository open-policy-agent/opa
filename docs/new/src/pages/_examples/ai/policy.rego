package ai.chat

deny contains message if {
	every pattern in all_accessible_models {
		not regex.match(pattern, input.parsed_body.model)
	}

	message := sprintf(
		"Model '%s' is not in your accessible models: %s",
		[input.parsed_body.model, concat(", ", all_accessible_models)],
	)
}

# model_access is a mapping of role to patterns which match models
# that users might be accessing.
model_access := {
	"interns": {"model-1"},
	"testers": {"model-1", `^model-\d+-stage$`},
	"data-analysts": {"model-1", `^model-\d+-internal$`},
}

all_accessible_models contains m if {
	some group in input.groups
	some m in model_access[group]
}

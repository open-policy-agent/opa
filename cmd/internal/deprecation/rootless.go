package deprecation

// TODO: these warnings can be removed when the rootless images are no longer published.

const rootlessWarningMessage = `OPA appears to be running in a deprecated -rootless image.
Since v0.50.0, the default OPA images have been configured to use a non-root
user.

This image will soon cease to be updated. The following images should now be
used instead:

* openpolicyagent/opa:latest and NOT (openpolicyagent/opa:latest-rootless)
* openpolicyagent/opa:edge and NOT (openpolicyagent/opa:edge-rootless)
* openpolicyagent/opa:X.Y.Z and NOT (openpolicyagent/opa:X.Y.Z-rootless)

You can choose to acknowledge and ignore this message by unsetting:
OPA_DOCKER_IMAGE_TAG=rootless
`

// warningRootless is a fatal warning is triggered when the user is running OPA
// in a deprecated rootless image.
var warningRootless = warning{
	MatchEnv: func(env []string) bool {
		for _, e := range env {
			if e == "OPA_DOCKER_IMAGE_TAG=rootless" {
				return true
			}
		}
		return false
	},
	MatchCommand: func(name string) bool {
		return name != "run"
	},
	Fatal:   true,
	Message: rootlessWarningMessage,
}

// warningRootlessRun is a non-fatal version of the warning reserved for opa run.
// The warning for run is non-fatal to avoid production disruption
var warningRootlessRun = warning{
	MatchEnv: func(env []string) bool {
		for _, e := range env {
			if e == "OPA_DOCKER_IMAGE_TAG=rootless" {
				return true
			}
		}
		return false
	},
	MatchCommand: func(name string) bool {
		return name == "run"
	},
	Fatal:   false,
	Message: rootlessWarningMessage,
}

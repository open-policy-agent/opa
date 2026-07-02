# regal ignore:directory-package-mismatch
package docker

import rego.v1

# Deny base images that are not from the approved chainguard namespace.
deny_msgs contains msg if {
	input.image
	not startswith(input.image.fullRepo, "docker.io/chainguard/")
	msg := sprintf("base image %q is not allowed; only docker.io/chainguard images are permitted", [input.image.ref])
}

decision := {
	"allow": count(deny_msgs) == 0,
	"deny_msg": [msg | some msg in deny_msgs],
}

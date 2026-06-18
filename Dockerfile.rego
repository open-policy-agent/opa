# regal ignore:directory-package-mismatch
package docker

# Deny base images that are not from the approved chainguard namespace.
deny_msgs[msg] {
	input.image
	not startswith(input.image.fullRepo, "docker.io/chainguard/")
	msg := sprintf("base image %q is not allowed; only docker.io/chainguard images are permitted", [input.image.ref])
}

decision := {
	"allow": count(deny_msgs) == 0,
	"deny_msg": [msg | msg := deny_msgs[_]],
}

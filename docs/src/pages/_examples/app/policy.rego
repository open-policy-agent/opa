package application.authz

# Only owner can update the pet's information. Ownership
# information is provided as part of the request data from
# the application.
default allow := false

allow if {
	input.method == "PUT"
	some petid
	input.path = ["pets", petid]
	input.user == input.owner
}

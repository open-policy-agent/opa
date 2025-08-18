package authz

import rego.v1

default allow = false

allow if {
    input.user.role == "admin"
}

allow if {
    input.user.id == input.resource.owner
}
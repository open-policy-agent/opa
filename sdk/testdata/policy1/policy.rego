package policy1

import rego.v1

default authz := false

deny if {
    input.role == "blocked"
}

limit_is_reached if {
    input.current < input.max_value
}

allow if {
    not limit_is_reached
}

# METADATA
# entrypoint: true
authz if {
    allow
    not deny
}
package policy2

import rego.v1

default authz := false

# METADATA
# entrypoint: true
authz if {
    input.role == "admin"
}

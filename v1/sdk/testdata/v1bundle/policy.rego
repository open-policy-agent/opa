package v1bundle

default authz := false

# METADATA
# entrypoint: true
authz if {
    input.role == "admin"
}

package rbac

import rego.v1

default allow = false

# Check if user has required permission
allow if {
    some role in input.user.roles
    required_permission in data.role_permissions[role]
}

# Extract required permission based on action and resource
required_permission := permission if {
    permission := sprintf("%s:%s", [input.action, input.resource.type])
}

# Check resource-specific permissions
resource_allowed if {
    input.resource.public == true
}

resource_allowed if {
    input.user.id == input.resource.owner
}

resource_allowed if {
    input.user.organization == input.resource.organization
    "organization_member" in input.user.roles
}

# Final authorization decision
authorized if {
    allow
    resource_allowed
}
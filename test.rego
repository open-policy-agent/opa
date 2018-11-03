package rbac

# Example input request.
input = {
    "subject": "bob",
    "resource": "foo123",
    "action": "write",
}

# Example RBAC configuration.
bindings = [
    {
        "user": "alice",
        "roles": ["dev", "test"],
    },
    {
        "user": "bob",
        "roles": ["test"],
    },
]

roles = [
    {
        "name": "dev",
        "permissions": [
            {"resource": "foo123", "action": "write"},
            {"resource": "foo123", "action": "read"},
        ],
    },
    {
        "name": "test",
        "permissions": [{"resource": "foo123", "action": "read"}],
    },
]

# Example RBAC policy implementation.

default allow = false

allow {
    user_has_role[role_name]
    role_has_permission[role_name]
}

user_has_role[role_name] {
    binding := bindings[_]
    binding.user = input.subject
    role_name := binding.roles[_]
}

role_has_permission[role_name] {
    role := roles[_]
    role_name := role.name
    perm := role.permissions[_]
    perm.resource = input.resource
    perm.action = input.action
}

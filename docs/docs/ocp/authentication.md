---
sidebar_position: 6
title: Authentication & Permissions
---

OCP includes Role Based Access Control (RBAC) to govern access to the API, several roles are predefined and can be assigned to a user, these are;

- `administrator` All operations allowed on all resources
- `viewer` Read operations allowed on all resources
- `owner` All operations for all resources they own
- `stack_owner` All operations for stacks they own

Authorized users are identified to the API through bearer tokens, the tokens are
opaque and can be generated using any acceptable methodology:

```shell
cat /dev/urandom | head -c 32 | base64
```

Tokens are tied to a principal and assigned a role from the above list or roles
using YAML configuration, see the example below:

```yaml
tokens:
  admin:
    api_key: 7lPLBKKpmiljMa0J9GwyYWLDJKEVFXEO6ZGAjmDf5eQ=
    scopes:
    - role: administrator
```

package play

# In an RBAC system a user belongs to several groups, each with its own list of
# permissions. Collect the permission list for every group the user is in.
groups_permissions := [input.groups[g].permissions | some g in input.user_groups]
# e.g. [["read", "write"], ["deploy", "read"]]

# Flatten the list of lists into a single array so the policy can check for a
# specific permission.
all_permissions := array.flatten(groups_permissions)

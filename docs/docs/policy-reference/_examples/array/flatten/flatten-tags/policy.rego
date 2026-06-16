package play

# groups_permissions is a list of lists: one list per group the user belongs to.
user_groups := ["eng", "devops"]

groups_permissions := [data.groups[g].permissions | some g in user_groups]
# e.g. [["read","write"], ["deploy","read"]]

# Flatten into a single list so we can check for a specific permission.
all_permissions := array.flatten(groups_permissions)

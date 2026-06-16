<!-- markdownlint-disable MD041 -->

In an RBAC system, users belong to groups and each group carries its own list of
permissions. When you need every permission a user holds, the per-group lists
must be merged into one. This example collects each group's permissions into a
list of lists, then flattens it into a single array so the policy can check
whether a specific permission is present.

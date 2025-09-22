<!-- markdownlint-disable MD041 -->
This example demonstrates using `regex.globs_match` in Rego to ensure actions are
allowed only if the user's permissions overlap with the required permissions for
the action. The user's permissions are defined by patterns, as are the
permissions required by any given action.

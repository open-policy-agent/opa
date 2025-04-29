# Run your first Rego policy!

# This simple example checks if a user is
# admin and only allows them if they are.
package play

# edit Alice's role to test it out!
user := {"email": "alice@example.com", "role": "admin"}

# by default, no one is allowed
default allow := false

# those with the role "admin" are allowed
allow if user.role == "admin"

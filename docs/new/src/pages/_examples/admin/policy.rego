# Run your first Rego policy!
package rbac

deny contains sprintf("%s cannot delete users", [input.role]) if {
	input.method == "DELETE"
	startswith(input.path, "/users/")
	input.role != "admin"
}

# Open in the Rego Playground to see the full example.

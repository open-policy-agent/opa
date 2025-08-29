# METADATA
# scope: package
# custom:
#   unknowns:
#     - input.tickets
#     - input.users
#   mask_rule: data.filters.mask_from_annotation
package filters

tenancy if input.tickets.tenant == input.tenant.id # tenancy check

include if {
	tenancy
	resolver_include
}

include if {
	tenancy
	not user_is_resolver(input.user, input.tenant.name)
}

resolver_include if {
	user_is_resolver(input.user, input.tenant.name)

	# ticket is assigned to user
	input.users.name == input.user
}

resolver_include if {
	user_is_resolver(input.user, input.tenant.name)

	# ticket is unassigned and unresolved
	input.tickets.assignee == null
	input.tickets.resolved == false
}

user_is_resolver(user, tenant) if "resolver" in data.roles[tenant][user] # regal ignore:external-reference

# Default-deny mask.
default masks.tickets.description := {"replace": {"value": "***"}}

# Allow viewing the field if user is an admin or a resolver.
masks.tickets.description := {} if {
	"admin" in data.roles[input.tenant][input.user]
}

masks.tickets.description := {} if {
	"resolver" in data.roles[input.tenant][input.user]
}

default mask_from_annotation.tickets.id := {"replace": {"value": "***"}}

# Allow viewing the field if user is an admin or a resolver.
mask_from_annotation.tickets.id := {} if {
	"admin" in data.roles[input.tenant][input.user]
}

mask_from_annotation.tickets.id := {} if {
	"resolver" in data.roles[input.tenant][input.user]
}

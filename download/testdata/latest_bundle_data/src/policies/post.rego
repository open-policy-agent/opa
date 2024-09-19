package peoplefinder.POST.api.users

import rego.v1
import input.user.attributes.properties as user_props

default allowed = false

default visible = false

default enabled = false

allowed if {
	user_props.department == "Operations"
	user_props.title == "IT Manager"
}

visible if {
	allowed
}

enabled if {
	allowed
}

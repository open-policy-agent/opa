package peoplefinder.POST.api.users.__id

import rego.v1
import input.user.attributes.properties as user_props

default allowed = false

default visible = true

default enabled = false

allowed if {
	user_props.department == "Operations"
}

enabled if {
	allowed
}

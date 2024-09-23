package peoplefinder.PUT.api.users.__id

import rego.v1
import input.user.attributes.properties as user_props

default allowed = false

default visible = true

default enabled = true

allowed if {
	user_props.department == "Operations"
}

allowed if {
	input.user.id == input.resource.id
}

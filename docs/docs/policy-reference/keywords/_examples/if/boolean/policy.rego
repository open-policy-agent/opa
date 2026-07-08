package play

default allow := false

allow if input.role == "admin"

allow if {
	input.path[0] == "users"
	input.path[1] == input.user_id
}

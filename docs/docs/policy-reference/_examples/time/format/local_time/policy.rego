package play

import rego.v1

request_time := time.parse_ns(
	"RFC822Z",
	input.request_utc_time,
)

local_time := time.format([
	request_time,
	input.tz,
	"15:04:05",
])

default allow := false

allow if count(reasons) == 0

reasons contains message if {
	input.role != "admin"
	message := sprintf("E123 %s", [local_time])
}

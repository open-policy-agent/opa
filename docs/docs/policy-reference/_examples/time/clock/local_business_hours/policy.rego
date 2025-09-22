package play

import rego.v1

request_time := time.parse_ns("RFC822Z", input.request_time)

local_hours := data.business_hours[input.tz]

default allow := false

allow if {
	[hour, _, _] := time.clock([request_time, input.tz])
	hour > local_hours.start
	hour < local_hours.end
}

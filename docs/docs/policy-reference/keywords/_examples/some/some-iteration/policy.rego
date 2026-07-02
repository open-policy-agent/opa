package play

import rego.v1

limit_period := 3 # days

limit_count := 4 # requests

requests_in_period contains request if {
	some i, j

	i <= limit_period

	request := data.last_weeks_requests[i][j]

	request.user_id == input.user_id
}

default allow := false

allow if count(requests_in_period) <= limit_count

message := sprintf(
	"user_id %d made %d requests in %d days, limit is %d",
	[input.user_id, count(requests_in_period), limit_period, limit_count],
)

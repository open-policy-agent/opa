package play

import rego.v1

# 2006-01-02 is the time format string for yyyy-mm-dd
start_date := time.parse_ns("2006-01-02", "1999-01-01")

end_date := time.parse_ns("2006-01-02", "2000-01-01")

parsed_time := time.parse_rfc3339_ns(input.time)

default allow := false

allow if {
	parsed_time > start_date
	parsed_time < end_date
}

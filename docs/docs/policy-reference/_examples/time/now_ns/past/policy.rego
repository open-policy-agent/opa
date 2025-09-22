package play

import rego.v1

parsed_time := time.parse_rfc3339_ns(input.time)

in_past if parsed_time < time.now_ns()

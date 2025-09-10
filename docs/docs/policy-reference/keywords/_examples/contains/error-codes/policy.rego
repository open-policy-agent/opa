package play

import rego.v1

errors contains "E0001" if _token_expired

errors contains "E1004" if object.get(input, "email", "") == ""

errors contains "E1209" if object.get(input, "claims", []) == []

default _token_expired := true

_token_expired if time.parse_ns("2006-01-02", input.expired_at) < time.now_ns()

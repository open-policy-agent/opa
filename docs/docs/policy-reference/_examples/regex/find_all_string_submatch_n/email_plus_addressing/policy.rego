package play

import rego.v1

internal_domain := "example.com"

allow if count(deny) == 0

deny contains "plus addressing not allowed unless internal" if {
	email_matches[1] != ""
	email_matches[2] != internal_domain
}

email_matches := regex.find_all_string_submatch_n(`^[^+@]+(\+[^@]*)?@([^@]+)$`, input.email, 1)[0]

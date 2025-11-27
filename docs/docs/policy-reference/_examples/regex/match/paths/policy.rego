package play

import rego.v1

news_pattern := `^/news/.*`

admin_pattern := `^/admin/.*`

path_patterns := {
	"intern": {news_pattern},
	"admin": {news_pattern, admin_pattern},
}

default allow := false

allow if {
	some pattern in path_patterns[input.role]
	regex.match(pattern, input.path)
}

package play

import rego.v1

example_email_1 := "foo [at] example.com"

example_email_2 := "foo@example.com"

match_1 := regex.match(`^[^@]+@[^@]+\.[^@]+$`, example_email_1)

match_2 := regex.match(`^[^@]+@[^@]+\.[^@]+$`, example_email_2)

match_3 := regex.match(`^[^@]+@[^@]+\.[^@]+$`, input.email)

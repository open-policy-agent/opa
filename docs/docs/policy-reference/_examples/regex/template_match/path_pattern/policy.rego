package play

import rego.v1

uuid_v4_pattern := `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`

aws_arn_pattern := `arn:(aws[a-zA-Z-]*):([a-zA-Z0-9-]+):([a-zA-Z0-9-]*):([0-9]*):([a-zA-Z0-9-:/]+)`

path := "/projects/10ceef56-2b18-4cf7-895f-14d2dc45cc66/arn:aws:ec2:us-west-2:123456789012:instance/i-1234567890abcdef0"

path_pattern_template := sprintf("/projects/{%s}/{%s}", [
	uuid_v4_pattern,
	aws_arn_pattern,
])

matches := regex.template_match(path_pattern_template, path, "{", "}")

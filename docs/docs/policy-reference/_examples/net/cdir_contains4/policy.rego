package netcidrcontainsmatches

result := net.cidr_contains_matches(
	{["1.1.0.0/16", "foo"], "1.1.2.0/24"}, 
	{"x": "1.1.1.128", "y": ["1.1.254.254", "bar"]}
)

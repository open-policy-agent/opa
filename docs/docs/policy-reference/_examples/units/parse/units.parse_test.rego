package examples.builtins.units.parse

test_parse_mb {
    units.parse("10MB") == 10485760
}

test_parse_gb {
    units.parse("1GB") == 1073741824
}

test_parse_seconds {
    units.parse("5s") == 5000000000
}

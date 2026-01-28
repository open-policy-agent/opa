package examples.builtins.units.parse



# Basic example: parsing byte units
example_bytes {
    units.parse("10MB") == 10000000
}

# Parsing binary units
example_binary_bytes {
    units.parse("10MiB") == 10485760
}

# Parsing time units
example_time {
    units.parse("5s") == 5000000000
}

# Parsing compound values
example_compound {
    units.parse("1h") == 3600000000000
}

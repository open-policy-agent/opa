package play

# input.port arrives as a string and is converted to a number.
port := to_number(input.port)

# Ports below 1024 are privileged; the comparison is numeric, not textual.
privileged_port if port < 1024

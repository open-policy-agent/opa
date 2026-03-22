package play

# Split a literal comma-separated string.
fields := split("alice,30,engineer", ",")

# Split an input CSV row.
input_fields := split(input.csv_row, ",")

# Extract the name (first field) from the input row.
name := input_fields[0]

# Allow if the role (third field) is "engineer".
allow if input_fields[2] == "engineer"

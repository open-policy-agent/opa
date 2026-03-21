package play

import rego.v1

# Trim surrounding spaces from a literal string.
trimmed_literal := strings.trim("  hello world  ", " ")

# Trim surrounding spaces from an input field.
trimmed_input := strings.trim(input.value, " ")

# Allow only if the trimmed input is non-empty.
allow if strings.trim(input.value, " ") != ""

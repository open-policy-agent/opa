package play

# Trim surrounding spaces from a literal string.
trimmed_literal := trim("  hello world  ", " ")

# Trim surrounding spaces from an input field.
trimmed_input := trim(input.value, " ")

# Allow only if the trimmed input is non-empty.
allow if trim(input.value, " ") != ""

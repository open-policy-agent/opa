package play

# Allowed deviation threshold defined in policy.
tolerance := 5

delta := input.observed - input.expected

# True when the observation is within tolerance of the expected value.
within_tolerance if abs(delta) <= tolerance

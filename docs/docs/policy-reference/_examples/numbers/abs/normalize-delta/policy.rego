package play

delta := input.observed - input.expected

# True when the observation is within tolerance of the expected value.
within_tolerance if abs(delta) <= input.tolerance

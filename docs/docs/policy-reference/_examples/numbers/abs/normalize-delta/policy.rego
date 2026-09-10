package play

# Baseline server timestamp and allowed skew (in seconds).
server_time := 1700000000
max_skew_seconds := 30

# Calculate timestamp drift regardless of whether request is ahead or behind.
drift := input.request_time - server_time

# Allow request if time drift is within acceptable bounds.
allow if abs(drift) <= max_skew_seconds

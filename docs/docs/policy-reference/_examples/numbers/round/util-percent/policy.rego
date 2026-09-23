package play

# Nearest whole percent for a utilization gauge.
util_pct := round((input.used / input.total) * 100)

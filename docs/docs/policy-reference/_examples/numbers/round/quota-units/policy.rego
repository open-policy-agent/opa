package play

# Whole CPU millicores after ceiling (never under-provision).
cpu_units := ceil(input.cpu_cores * 1000)

# Whole GiB after flooring (never over-count free disk).
free_gib := floor(input.free_bytes / (1024 * 1024 * 1024))

# Nearest whole percent for a utilization gauge.
util_pct := round(input.used / input.total * 100)

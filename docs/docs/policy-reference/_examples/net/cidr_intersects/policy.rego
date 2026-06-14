package netcidrintersects

# Check if two CIDR ranges overlap
overlapping := net.cidr_intersects("192.168.1.0/25", "192.168.1.64/26")

# These ranges do not overlap
non_overlapping := net.cidr_intersects("10.0.0.0/24", "10.0.1.0/24")

package netcidrcontains

# Check if an IP address falls within a CIDR range
allow := net.cidr_contains("10.0.0.0/8", "10.1.2.3")

# This IP is outside the range
deny := net.cidr_contains("192.168.1.0/24", "192.168.2.1")

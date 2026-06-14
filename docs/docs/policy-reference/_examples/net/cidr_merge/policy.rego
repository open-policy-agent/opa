package netcidrmerge

# Merge adjacent or overlapping CIDR ranges into the smallest set
merged := net.cidr_merge(["192.168.1.0/25", "192.168.1.128/25"])

# Merge multiple subnets that collapse into a single /16
collapsed := net.cidr_merge(["10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/23"])

package play

import rego.v1

# Check if storage request is within quota
within_quota if {
	requested := units.parse_bytes(input.storage_request)
	max_quota := units.parse_bytes("100GB")

	requested <= max_quota
}

# Calculate remaining quota
remaining_bytes := quota - used if {
	quota := units.parse_bytes("100GB") # 100GB = 100000000000 bytes
	used := units.parse_bytes(input.storage_used)
}

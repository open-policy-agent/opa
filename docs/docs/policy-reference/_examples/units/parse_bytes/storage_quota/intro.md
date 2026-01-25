<!-- markdownlint-disable MD041 -->
The `units.parse_bytes` function converts storage size strings into byte values as **integers**.

Examples:
- `"100KB"` = 100000 bytes
- `"100KiB"` = 102400 bytes (binary)
- `"50GB"` = 50000000000 bytes
- `"2GiB"` = 2147483648 bytes (binary)

This example demonstrates:
1. Validating storage requests against quotas
2. Calculating remaining storage

Unlike `units.parse`, this function:
- Returns **integers** (no floating point)
- Specifically handles byte units (KB, MB, GB, etc.)

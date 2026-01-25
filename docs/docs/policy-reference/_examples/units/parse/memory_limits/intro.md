<!-- markdownlint-disable MD041 -->
This example shows how to validate Kubernetes container memory requests and limits using `units.parse`.

The policy ensures:
1. Memory request is less than memory limit
2. Memory limit doesn't exceed 8Gi

**Note:** Kubernetes uses binary units (Mi, Gi) which are base-1024:
- `"512Mi"` = 536870912 (512 × 1024²)
- `"2Gi"` = 2147483648 (2 × 1024³)
- `"8Gi"` = 8589934592 (8 × 1024³)

This is useful for:
- Preventing resource over-allocation
- Enforcing organizational policies
- Validating infrastructure-as-code

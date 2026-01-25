<!-- markdownlint-disable MD041 -->
The `units.parse` function converts string representations of units into their numeric values.

**Decimal (SI) units** (base 1000):
- K/k = 1000, M = 1000000, G/g = 1000000000

**Binary units** (base 1024):
- Ki = 1024, Mi = 1048576, Gi = 1073741824

**Milli-units**:
- m = 0.001 (e.g., "100m" = 0.1)

Common use cases:
- Parsing Kubernetes resource requests/limits
- Converting rate limits  
- Standardizing unit representations

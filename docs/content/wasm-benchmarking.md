---
title: WASM Performance Benchmarking
kind: documentation
weight: 10
---

# WASM Performance Benchmarking

OPA includes a comprehensive benchmarking infrastructure for measuring and tracking WASM performance over time. This helps prevent performance regressions and identify optimization opportunities.

## Quick Start

Run WASM benchmarks locally:

```bash
make bench-wasm
```

This runs the standard benchmark suite with 100 iterations per test.

## Benchmark Infrastructure

### Components

1. **Benchmark Library** (`internal/wasm/benchmark/`)
   - Core benchmarking framework
   - Performance comparison utilities
   - Result serialization/deserialization

2. **CLI Tool** (`cmd/bench-wasm/`)
   - Command-line interface for running benchmarks
   - Baseline comparison
   - Result reporting

3. **Test Data** (`internal/wasm/benchmark/testdata/`)
   - Standard benchmark policies
   - Corresponding input and data files

### Makefile Targets

- `make bench-wasm` - Run standard benchmarks
- `make bench-wasm-ci` - Run CI benchmarks with regression detection
- `make bench-wasm-baseline` - Generate new baseline results

## Writing Benchmarks

Create benchmark policies in `internal/wasm/benchmark/testdata/`:

1. **Policy file** (`example.rego`):
```rego
package example

import rego.v1

default allow = false

allow if {
    input.user.role == "admin"
}
```

2. **Input file** (`example_input.json`):
```json
{
    "user": {
        "role": "admin"
    }
}
```

3. **Data file** (`example_data.json`) - optional:
```json
{
    "roles": ["admin", "user", "guest"]
}
```

## CI Integration

The benchmark suite runs automatically on pull requests:

1. Executes benchmark suite
2. Compares results against baseline
3. Fails if regression >5% detected
4. Uploads results as artifacts

## Performance Analysis

### Using the CLI Tool

```bash
# Run with custom iterations
./bench-wasm -iterations 1000 policies/*.rego

# Compare against baseline
./bench-wasm -baseline baseline.json policies/*.rego

# Save results
./bench-wasm -output results.json policies/*.rego

# Verbose output
./bench-wasm -verbose policies/*.rego
```

### Interpreting Results

Results show:
- **Avg Time**: Average execution time per iteration
- **Min/Max**: Range of execution times
- **Throughput**: Operations per second

Example output:
```
Benchmark Summary
=================
Test                           Avg Time        Ops/sec
--------------------------------------------------------------
simple_authz                   450.0ms         2.2
complex_rbac                   1.2s            0.8
```

### Regression Detection

The CI system automatically detects regressions:
- ✓ Changes within 5% are acceptable
- ✗ Changes >5% fail the build

## Best Practices

1. **Consistent Environment**: Run benchmarks on consistent hardware
2. **Multiple Iterations**: Use sufficient iterations (100+) for stability
3. **Real-World Policies**: Benchmark realistic authorization scenarios
4. **Regular Baselines**: Update baselines after intentional optimizations

## Troubleshooting

### High Variance

If results show high variance:
- Increase iteration count
- Check for background processes
- Ensure consistent system state

### CI Failures

If benchmarks fail in CI:
1. Check the uploaded artifacts for detailed results
2. Run benchmarks locally to reproduce
3. Compare with baseline to identify regression
4. Optimize or update baseline if regression is acceptable

## Future Improvements

Planned enhancements:
- Memory usage tracking
- Compilation time metrics
- Automatic performance reports
- Historical trend analysis
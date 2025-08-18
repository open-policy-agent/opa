# WASM Benchmark Package

This package provides performance benchmarking infrastructure for OPA's WebAssembly (WASM) compilation and execution.

## Purpose

The benchmark package helps:
- Detect performance regressions in WASM target
- Compare WASM vs regular Rego performance
- Track performance improvements over time
- Ensure consistent performance across releases

## Architecture

```
benchmark/
├── benchmark.go          # Core benchmarking logic
├── benchmark_test.go     # Unit tests
├── baseline.json        # Performance baseline for CI
├── testdata/           # Benchmark test policies
│   ├── simple_authz.rego
│   ├── simple_authz_input.json
│   ├── complex_rbac.rego
│   ├── complex_rbac_input.json
│   └── complex_rbac_data.json
└── README.md
```

## Usage

### In Tests

```go
suite := benchmark.NewSuite()

suite.Add(benchmark.Test{
    Name:   "my_policy",
    Policy: policyString,
    Input:  inputData,
    Data:   contextData,
})

results, err := suite.Run(ctx, iterations, "wasm")
```

### Comparing Results

```go
baseline := loadBaselineResults()
current := runBenchmarks()

report := benchmark.Compare(baseline, current)
// report shows performance changes
```

## Adding New Benchmarks

1. Create policy file in `testdata/`
2. Add corresponding `_input.json` file
3. Optionally add `_data.json` for context data
4. Run `make bench-wasm-baseline` to update baseline

## Performance Targets

- Simple policies: <500ms avg execution time
- Complex policies: <2s avg execution time
- Regression threshold: 5% (configurable)

## Maintenance

- Update baseline after intentional optimizations
- Review benchmark suite quarterly
- Add benchmarks for new feature areas
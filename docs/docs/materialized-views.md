---
title: Materialized Views
sidebar_position: 15
---

Materialized views allow you to pre-compute expensive data transformations and persist the results in OPA's data store. This feature enables significant performance improvements by computing transformations once during bundle activation rather than on every policy evaluation.

## Overview

Similar to materialized views in databases, OPA's materialized views evaluate policies in the reserved `system.store` namespace when data is updated and store their results back in the data document. Subsequent policy evaluations can access these pre-computed results directly, avoiding redundant computation.

### When to Use Materialized Views

Materialized views are ideal when:

- **Repeated transformations**: Your policies apply the same data transformations (filtering, mapping, aggregating) across multiple evaluations
- **Large datasets**: You work with thousands of records where transformation cost is significant
- **Read-heavy workloads**: Views are queried many times between data updates (10+ queries per bundle activation)
- **Expensive computations**: Transformations involve complex logic, loops, or aggregations

### When NOT to Use Materialized Views

Avoid materialized views when:

- **Input-dependent transformations**: Your transformations depend on request-specific `input` data
- **Low query frequency**: Views are queried only 1-5 times between bundle activations
- **Fast activation required**: Your use case requires bundle activation to complete in <100ms
- **Small datasets**: The transformation cost is already negligible (<1ms)

## Architecture

### Design

Materialized views in OPA follow a simple, explicit design:

1. **Reserved Namespace**: Policies defined in `package system.store` are treated as view definitions
2. **Eager Evaluation**: Views are evaluated during bundle activation after data is written to the store
3. **Store Persistence**: Results are written back to the data document at `data.system.store.*`
4. **Read Access**: Normal policies access pre-computed results via standard data references

```
┌─────────────────────────────────────────────────────────────┐
│                     Bundle Activation                        │
├─────────────────────────────────────────────────────────────┤
│ 1. Write data to store                                       │
│ 2. Compile policies                                          │
│ 3. ┌──────────────────────────────────────────────┐         │
│    │ Evaluate system.store policies (views)        │         │
│    │   - Query results from data                   │         │
│    │   - Apply transformations                     │         │
│    │   - Write results to data.system.store.*      │         │
│    └──────────────────────────────────────────────┘         │
│ 4. Transaction commit                                        │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Policy Evaluation                         │
├─────────────────────────────────────────────────────────────┤
│ Policies read pre-computed results:                          │
│   allow if {                                                 │
│     user := data.system.store.active_users[_]                │
│     # No transformation cost - already computed!             │
│   }                                                          │
└─────────────────────────────────────────────────────────────┘
```

### Recursion Prevention

The implementation includes context-based recursion guards to prevent infinite loops if `system.store` policies attempt to read from `data.system.store.*`:

```go
// Recursive reads are silently skipped
if isEvaluatingSystemStore(ctx) {
    return nil
}
ctx = markEvaluatingSystemStore(ctx)
```

## Usage

### Basic Example

Define a view that filters active users:

```rego
package system.store

# Partial rule - produces a set of users
active_users contains user if {
    some user in data.users
    user.active == true
}
```

After bundle activation, access the pre-computed result:

```rego
package authz

# Use the materialized view
allow if {
    input.user in data.system.store.active_users
}
```

### Complete Rule Example

Views can also use complete rules for single computed values:

```rego
package system.store

# Complete rule - produces a single value
max_requests_per_minute := data.config.limits.max_requests_per_minute
```

Access the value:

```rego
package ratelimit

default allow := false

allow if {
    count(requests_this_minute) < data.system.store.max_requests_per_minute
}
```

### Multiple Views

A single package can define multiple views:

```rego
package system.store

# View 1: Filter by role
admins contains user if {
    some user in data.users
    user.role == "admin"
}

# View 2: Filter by department
engineers contains user if {
    some user in data.users
    user.department == "engineering"
}

# View 3: Aggregate counts
user_counts[dept] := count(users) if {
    dept := ["engineering", "sales", "hr"][_]
    users := [u | u := data.users[_]; u.department == dept]
}
```

### Nested Transformations

Views can perform complex transformations:

```rego
package system.store

# Extract and flatten nested data
user_permissions contains permission if {
    some user in data.users
    some role in user.roles
    some permission in data.role_permissions[role]
}

# Aggregate and compute
department_budgets[dept] := sum(salaries) if {
    dept := data.departments[_].name
    salaries := [u.salary |
        u := data.users[_]
        u.department == dept
    ]
}
```

## Performance Analysis

### Query Performance Improvement

Materialized views provide dramatic query performance improvements:

```
Benchmark: 10,000 user records, filtering active admins

Without materialization:
  Time:        2.08ms per query
  Memory:      1.13 MB per query
  Allocations: 25,563 per query

With materialization:
  Time:        0.88ms per query (2.4× faster)
  Memory:      0.67 MB per query (40% reduction)
  Allocations: 13,404 per query (47% reduction)
```

### Materialization Cost

The trade-off is an upfront computation cost during bundle activation:

```
Dataset: 10,000 records
Materialization time: 291ms
Materialization memory: 137.9 MB
Materialization allocations: 2,743,198
```

### Break-Even Analysis

For a typical scenario with 10,000 records and 100 queries per hour:

```
Without materialization:
  Activation: 10ms (fast)
  100 queries: 2.08ms × 100 = 208ms
  Total: 218ms/hour

With materialization:
  Activation: 291ms (slower, ONE-TIME)
  100 queries: 0.88ms × 100 = 88ms
  Total: 291ms (first hour) + 88ms/hour

Break-even: After ~2 queries
After 100 queries: 203ms saved per hour
```

**Key insight**: Even with slower activation, materialized views break even after just 2 queries and provide exponential benefits for read-heavy workloads.

### Optimization Details

The implementation includes several performance optimizations:

1. **Object Pooling**: Reuses temporary allocations via `sync.Pool` (-30 allocs/view)
2. **String Interning**: Caches common strings like "system", "store" (-10 allocs/view)
3. **Direct Native Build**: Bypasses intermediate AST structures (-50 allocs/view, +10% speedup)
4. **Zero-Copy for lazyObj**: Uses `JSONWithOpt{CopyMaps: false}` for data from store

**Total optimization impact**: 11-23% faster materialization, ~130 fewer allocations per view.

## Programming API

### Manager Creation

Create a materialized view manager with a compiled policy set and store:

```go
import (
    "github.com/open-policy-agent/opa/v1/ast"
    "github.com/open-policy-agent/opa/v1/storage"
    "github.com/open-policy-agent/opa/v1/storage/materialized"
)

// After compiling policies and creating a transaction
compiler := ast.MustCompileModules(modules)
store := inmem.New()
txn, _ := store.NewTransaction(ctx, storage.WriteParams)

// Create manager
mgr := materialized.NewManager(compiler, store)
```

### Evaluating Views

Call `EvaluateSystemStore` to compute all materialized views:

```go
// Evaluate all system.store policies and write results
err := mgr.EvaluateSystemStore(ctx, txn)
if err != nil {
    // Handle error - evaluation failed
    return err
}

// Commit transaction to persist materialized views
store.Commit(ctx, txn)
```

### ⚠️ Important: Views Don't Auto-Refresh

**CRITICAL**: Materialized views do NOT automatically update when data changes via `storage.Write()`.

```go
// ❌ WRONG - Views become stale!
store.Write(ctx, txn, storage.AddOp, path, newData)
store.Commit(ctx, txn)
// Views still contain OLD data!

// ✅ CORRECT - Refresh views before commit
store.Write(ctx, txn, storage.AddOp, path, newData)
mgr.EvaluateSystemStore(ctx, txn)  // Refresh views
store.Commit(ctx, txn)              // Atomic update
```

**Why not automatic?**
- Storage triggers fire AFTER commit (too late)
- Creating new transaction for refresh creates race window
- Explicit refresh ensures atomic updates

### Data Update Patterns

#### Pattern 1: Manual Refresh (Recommended)

Explicitly refresh views in the same transaction:

```go
txn, _ := store.NewTransaction(ctx, storage.WriteParams)
defer store.Abort(ctx, txn)

// Update data
store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), newUsers)

// Refresh materialized views (same transaction)
mgr := materialized.NewManager(compiler, store)
if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
    return err
}

// Commit atomically (data + views)
store.Commit(ctx, txn)
```

#### Pattern 2: Transaction Wrapper (Convenience)

Use the wrapper for automatic refresh:

```go
wrapper, _ := materialized.NewTransaction(ctx, store, compiler, storage.WriteParams)
defer wrapper.Abort()

// Write data - automatically tracked
wrapper.Write(storage.AddOp, storage.MustParsePath("/users"), newUsers)

// Commit with automatic view refresh
if err := wrapper.CommitWithRefresh(); err != nil {
    return err
}
```

#### Pattern 3: Conditional Refresh

Refresh only when specific data changes:

```go
txn, _ := store.NewTransaction(ctx, storage.WriteParams)
defer store.Abort(ctx, txn)

// Update data
store.Write(ctx, txn, storage.AddOp, path, data)

// Refresh only if path affects materialized views
if pathAffectsViews(path) {
    mgr.EvaluateSystemStore(ctx, txn)
}

store.Commit(ctx, txn)
```

#### Pattern 4: Skip Refresh for Temporary Data

Disable refresh for data that doesn't affect views:

```go
wrapper, _ := materialized.NewTransaction(ctx, store, compiler, storage.WriteParams)
defer wrapper.Abort()

// Disable auto-refresh for temporary data
wrapper.SetAutoRefresh(false)

wrapper.Write(storage.AddOp, storage.MustParsePath("/temp/cache"), tempData)

// Commit without refresh (faster)
wrapper.Commit()
```

### Bundle Activation Integration

Integrate materialization into your bundle activation flow:

```go
func activateBundle(ctx context.Context, store storage.Store, bundle *bundle.Bundle) error {
    txn, err := store.NewTransaction(ctx, storage.WriteParams)
    if err != nil {
        return err
    }
    defer store.Abort(ctx, txn)

    // 1. Write data
    if err := writeData(ctx, store, txn, bundle.Data); err != nil {
        return err
    }

    // 2. Compile policies
    compiler, err := compileModules(bundle.Modules)
    if err != nil {
        return err
    }

    // 3. Evaluate materialized views
    mgr := materialized.NewManager(compiler, store)
    if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
        return fmt.Errorf("materialized view evaluation failed: %w", err)
    }

    // 4. Commit transaction
    return store.Commit(ctx, txn)
}
```

### Error Handling

The implementation uses fail-fast error handling. Any error during view evaluation causes the entire bundle activation to fail:

```go
err := mgr.EvaluateSystemStore(ctx, txn)
if err != nil {
    // Possible errors:
    // - Query execution failed (syntax, type errors)
    // - Store write failed (storage errors)
    // - Invalid system.store module structure
    log.Errorf("Failed to evaluate system.store policies: %v", err)
    return err
}
```

This conservative approach ensures that policies never work with incomplete or stale materialized views.

## Best Practices

### View Design

**DO**: Keep views focused and specific

```rego
package system.store

# Good: Clear, single-purpose view
active_premium_users contains user if {
    some user in data.users
    user.active == true
    user.subscription == "premium"
}
```

**DON'T**: Create overly broad views that aren't used

```rego
package system.store

# Bad: Computes all possible combinations
all_user_attributes[user.id] := attrs if {
    some user in data.users
    attrs := {k: v | user[k] = v}  # Expensive, rarely needed
}
```

### Avoiding Input Dependencies

**DO**: Transform static data only

```rego
package system.store

# Good: Uses only data (no input)
high_risk_ips contains ip if {
    some entry in data.threat_intelligence
    entry.risk_score > 80
    ip := entry.ip_address
}
```

**DON'T**: Reference input in views (won't work as expected)

```rego
package system.store

# Bad: Input isn't available during materialization!
user_specific_view contains item if {
    input.user == "admin"  # Always undefined!
    some item in data.items
}
```

### Naming Conventions

Use descriptive names that indicate the view's purpose:

```rego
package system.store

# Good naming
active_users         # Clear: users where active=true
admin_emails         # Clear: email addresses of admins
dept_headcounts      # Clear: count of users per department

# Avoid generic names
users                # Ambiguous: all users? filtered users?
data                 # Too generic
temp                 # Unclear purpose
```

### Testing Materialized Views

Test views separately from policies that consume them:

```rego
package system.store_test

import rego.v1

test_active_users if {
    # Setup test data
    data_doc := {
        "users": [
            {"id": 1, "name": "alice", "active": true},
            {"id": 2, "name": "bob", "active": false},
        ]
    }

    # Test the view logic
    result := data.system.store.active_users with data as data_doc

    # Verify
    count(result) == 1
    result[_].name == "alice"
}
```

## Troubleshooting

### View Not Available

**Problem**: `data.system.store.my_view` is undefined

**Solutions**:
1. Verify the policy is in `package system.store`
2. Check bundle activation logs for errors
3. Ensure `EvaluateSystemStore()` is called after data writes
4. Confirm the view rule actually produces results (check with `opa eval`)

### Slow Bundle Activation

**Problem**: Bundle activation takes too long

**Solutions**:
1. Profile which views are slow (add logging/metrics to `evaluateRule`)
2. Optimize expensive views (avoid nested loops, use indexing)
3. Consider lazy evaluation for rarely-accessed views
4. Split large datasets across multiple bundles

### Stale Results

**Problem**: Materialized view doesn't reflect latest data after `storage.Write()`

⚠️ **This is the #1 pitfall!** Views do NOT auto-refresh when data changes.

**Example of the problem**:
```go
// Update user data
store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), newUsers)
store.Commit(ctx, txn)

// ❌ BUG: data.system.store.active_users still contains OLD users!
```

**Root cause**: Views are evaluated explicitly, not automatically on data changes.

**Solutions**:

**Solution 1: Explicit refresh (Recommended)**
```go
txn, _ := store.NewTransaction(ctx, storage.WriteParams)
store.Write(ctx, txn, storage.AddOp, path, newData)

// ✅ Refresh views before commit
mgr.EvaluateSystemStore(ctx, txn)

store.Commit(ctx, txn)  // Atomic update
```

**Solution 2: Transaction wrapper**
```go
wrapper, _ := materialized.NewTransaction(ctx, store, compiler, storage.WriteParams)
wrapper.Write(storage.AddOp, path, newData)
wrapper.CommitWithRefresh()  // Automatic refresh
```

**Solution 3: Bundle activation hook**

If using bundle activation flow, ensure views are evaluated:
```go
// In bundle activation
writeData(ctx, store, txn, bundle.Data)

// Refresh views before commit
mgr := materialized.NewManager(compiler, store)
mgr.EvaluateSystemStore(ctx, txn)

store.Commit(ctx, txn)
```

**Prevention**:
- Document that views need explicit refresh
- Use transaction wrapper for automatic tracking
- Add tests that verify view freshness after data updates

### Memory Usage

**Problem**: Materialized views consume too much memory

**Solutions**:
1. Reduce view scope (filter more aggressively)
2. Use aggregations instead of full result sets
3. Consider view expiration/cleanup policies
4. Monitor memory usage and set appropriate limits

## Limitations

### Current Limitations

1. **Full Recomputation**: Views are fully recomputed on every bundle activation (no incremental updates)
2. **No Dependency Tracking**: Independent views are computed sequentially (no parallelization)
3. **Fail-Fast Errors**: Any view error fails entire bundle activation
4. **No TTL/Expiration**: Views persist until next bundle activation
5. **Synchronous Evaluation**: Views are computed synchronously during activation (blocks transaction)

### Future Enhancements

Potential improvements being considered:

- **Incremental Updates**: Only recompute views affected by data changes
- **Parallel Evaluation**: Evaluate independent views concurrently
- **Dependency Tracking**: Compute views in topological order based on inter-view dependencies
- **Lazy Evaluation**: Defer view computation until first access
- **View Invalidation**: Explicit APIs to invalidate and recompute specific views

## Real-World Use Cases

### User Authorization

Pre-compute user permission sets from role assignments:

```rego
package system.store

# Materialize flattened permissions
user_permissions[user_id] := permissions if {
    some user in data.users
    user_id := user.id
    permissions := {p |
        some role in user.roles
        some permission in data.role_permissions[role]
        p := permission
    }
}
```

Policy can now check permissions efficiently:

```rego
package authz

allow if {
    required_permission := sprintf("%s:%s", [input.resource, input.action])
    required_permission in data.system.store.user_permissions[input.user_id]
}
```

### Threat Intelligence

Pre-filter high-risk indicators from large threat feeds:

```rego
package system.store

# Materialize only high-risk IPs (reduces 100K entries to ~1K)
high_risk_ips contains ip if {
    some indicator in data.threat_feed.indicators
    indicator.risk_score > 80
    indicator.type == "ip"
    ip := indicator.value
}
```

### Resource Catalogs

Pre-compute resource indexes for fast lookups:

```rego
package system.store

# Materialize resource ownership index
resources_by_owner[owner] := resources if {
    owner := data.resources[_].owner
    resources := [r |
        r := data.resources[_]
        r.owner == owner
    ]
}
```

### Compliance Reporting

Pre-aggregate compliance data:

```rego
package system.store

# Materialize compliance stats
compliance_summary := {
    "total_resources": count(data.resources),
    "compliant": count([r | r := data.resources[_]; r.compliant]),
    "non_compliant": count([r | r := data.resources[_]; not r.compliant]),
    "by_severity": {
        s: count([r | r := data.resources[_]; not r.compliant; r.severity == s])
        | s := ["critical", "high", "medium", "low"][_]
    }
}
```

## References

- [GitHub Issue #7934](https://github.com/open-policy-agent/opa/issues/7934) - Original feature proposal
- [Integration Guide](integration.md) - Embedding OPA in your application
- [Configuration Reference](configuration.md) - Bundle activation configuration
- API Documentation: `github.com/open-policy-agent/opa/v1/storage/materialized`

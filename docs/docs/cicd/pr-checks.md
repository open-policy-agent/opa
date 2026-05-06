---
sidebar_label: PR Check Policies
sidebar_position: 2
---

# Pull Request Check Policies

One of the most effective uses of OPA in CI/CD is using Rego policies to control
which checks run on a pull request based on the files that changed. This pattern
replaces brittle `paths-filter` YAML configurations or shell-scripted `if`
conditions with testable, readable policy logic.

While the Rego policy itself is platform-agnostic, the examples on this page
use **GitHub Actions** to demonstrate the workflow integration. The same
approach can be adapted to other CI platforms by replacing the
platform-specific parts (fetching changed files, setting job outputs).

## Why Use Rego for PR Checks?

Traditional approaches to conditional CI checks have significant drawbacks:

| Approach                         | Drawback                                                              |
| -------------------------------- | --------------------------------------------------------------------- |
| `paths-filter` actions           | Limited to glob patterns, no complex logic, hard to test in isolation |
| Shell scripts with `grep`/`find` | Fragile, hard to read, impossible to unit test                        |
| Hardcoded `if:` conditions       | Duplicated across jobs, no single source of truth                     |

Using Rego provides:

- **Testability** - Unit test your check logic with `opa test` before it hits CI
- **Readability** - Declarative rules are easier to review than imperative scripts
- **Reusability** - Share policies across repositories as bundles
- **Auditability** - Policy changes are versioned and reviewed like any other code

## The Pattern

The pattern has three components:

1. **A Rego policy** that takes the list of changed files as input and outputs
   which checks should run
2. **A "check-changes" job** that evaluates the policy and sets workflow outputs
3. **Conditional jobs** that only run when the policy says they should

Optionally, a **summary job** can use OPA to validate that all required jobs
passed, providing a single required status check for branch protection.

## Example: Monorepo Test Selection (GitHub Actions)

Consider a monorepo with a frontend, backend, and shared documentation:

```
my-project/
├── frontend/
├── backend/
├── docs/
├── .github/
│   └── workflows/
│       └── pull-request.yaml
└── policy/
    └── pr-check/
        ├── pr_check.rego
        └── pr_check_test.rego
```

### The Policy

```rego title="policy/pr-check/pr_check.rego"
package pr_check

# Shared config files that affect all packages
shared_build_files := [
    "package.json",
    "tsconfig.base.json",
    "Makefile",
]

changes["frontend"] if {
    some file in input
    startswith(file.filename, "frontend/")
} else if {
    some file in input
    file.filename in shared_build_files
}

changes["backend"] if {
    some file in input
    startswith(file.filename, "backend/")
} else if {
    some file in input
    file.filename in shared_build_files
}

changes["docs"] if {
    some file in input
    startswith(file.filename, "docs/")
}
```

Each rule in `changes` evaluates to `true` if any changed file matches the
conditions. The `else if` clauses ensure that modifications to shared build
files trigger all relevant checks.

### Testing the Policy

```rego title="policy/pr-check/pr_check_test.rego"
package pr_check_test

import data.pr_check

test_frontend_change_triggers_frontend if {
    pr_check.changes.frontend with input as [
        {"filename": "frontend/src/App.tsx"},
    ]
}

test_frontend_change_does_not_trigger_backend if {
    not pr_check.changes.backend with input as [
        {"filename": "frontend/src/App.tsx"},
    ]
}

test_shared_file_triggers_all_packages if {
    pr_check.changes.frontend with input as [
        {"filename": "package.json"},
    ]
    pr_check.changes.backend with input as [
        {"filename": "package.json"},
    ]
}

test_docs_change_only_triggers_docs if {
    pr_check.changes.docs with input as [
        {"filename": "docs/getting-started.md"},
    ]
    not pr_check.changes.frontend with input as [
        {"filename": "docs/getting-started.md"},
    ]
    not pr_check.changes.backend with input as [
        {"filename": "docs/getting-started.md"},
    ]
}
```

Run the tests locally with:

```bash
opa test policy/pr-check/
```

### The Workflow (GitHub Actions)

The workflow uses the GitHub API to fetch changed files and passes them to OPA
for evaluation. Job outputs propagate the policy decision to downstream jobs.

```yaml title=".github/workflows/pull-request.yaml"
name: Pull Request Checks
on: [pull_request]

jobs:
  check-changes:
    runs-on: ubuntu-latest
    outputs:
      frontend: ${{ steps.check.outputs.frontend }}
      backend: ${{ steps.check.outputs.backend }}
      docs: ${{ steps.check.outputs.docs }}
    steps:
      - uses: actions/checkout@v4

      - name: Download OPA
        uses: open-policy-agent/setup-opa@v2
        with:
          version: latest

      - name: Test PR check policies
        run: opa test policy/pr-check/

      - name: Get changed files
        id: changes
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh api repos/${{ github.repository }}/pulls/${{ github.event.number }}/files \
            --paginate > changed_files.json

      - name: Evaluate policy
        id: check
        run: |
          # Default all checks to true (safe fallback)
          frontend=true
          backend=true
          docs=true

          # Override with policy evaluation results
          for check in frontend backend docs; do
            result=$(opa eval \
              --input changed_files.json \
              --data policy/pr-check/pr_check.rego \
              --format raw \
              "data.pr_check.changes.$check" 2>/dev/null || echo "")
            if [ "$result" = "true" ]; then
              declare "$check=true"
            elif [ -z "$result" ]; then
              declare "$check=false"
            fi
          done

          echo "frontend=$frontend" >> "$GITHUB_OUTPUT"
          echo "backend=$backend" >> "$GITHUB_OUTPUT"
          echo "docs=$docs" >> "$GITHUB_OUTPUT"

  test-frontend:
    needs: check-changes
    if: needs.check-changes.outputs.frontend == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm test --workspace=frontend

  test-backend:
    needs: check-changes
    if: needs.check-changes.outputs.backend == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm test --workspace=backend

  build-docs:
    needs: check-changes
    if: needs.check-changes.outputs.docs == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm run build --workspace=docs

  pr-check-summary:
    runs-on: ubuntu-latest
    if: always()
    needs: [check-changes, test-frontend, test-backend, build-docs]
    steps:
      - name: Download OPA
        uses: open-policy-agent/setup-opa@v2
        with:
          version: latest

      - name: Check job results
        env:
          NEEDS: ${{ toJson(needs) }}
        run: |
          echo "$NEEDS" | opa eval --fail \
            --stdin-input \
            --format raw \
            'every value in input { value.result != "failure" }'
```

## The Summary Job Pattern

The `pr-check-summary` job deserves special attention. It runs `if: always()`,
meaning it executes regardless of whether other jobs were skipped or failed. It
then uses OPA to check whether any required job reported a failure.

This approach lets you use a **single required status check**
(`pr-check-summary`) in your branch protection rules, rather than listing every
individual job. When you add or remove conditional jobs, you only need to update
the `needs` list — the branch protection rules remain unchanged.

## Adapting the Pattern

To adapt this pattern to your repository:

1. **Identify your check categories** - What logical groupings of files exist?
   (packages, services, languages, etc.)
2. **Define the file-to-check mapping** in a Rego policy - Which file patterns
   trigger which checks?
3. **Write tests** - Cover edge cases like shared files, root config changes,
   and files that should not trigger any check.
4. **Wire it into your workflow** - Use the `check-changes` job pattern to set
   outputs, and `if:` conditions on downstream jobs.

## Real-World Examples

This pattern is used in production by the OPA project itself:

- [OPA's pull-request workflow](https://github.com/open-policy-agent/opa/blob/main/.github/workflows/pull-request.yaml) -
  Selectively runs Go, Wasm, docs, Rego, and YAML checks based on changed files
- [OPA's PR check policy](https://github.com/open-policy-agent/opa/blob/main/build/policy/pr-check/pr_check.rego) -
  The Rego policy that drives the check selection
- [Java OPA SDK PR checks](https://github.com/open-policy-agent/java-opa-sdk/pull/13) -
  Applies the same pattern to a Gradle monorepo with multiple subprojects

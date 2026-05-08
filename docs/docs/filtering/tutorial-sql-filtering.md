---
title: "Tutorial: SQL Data Filtering"
sidebar_position: 6
---

This tutorial demonstrates end-to-end data filtering with OPA around a concrete question: **whose salaries can a Director see?**

You will write an authorization policy, use OPA's partial evaluation to derive a SQL `WHERE` clause, and apply that filter to a real database query.

## Prerequisites

- [OPA installed](../#1-download-opa)
- [sqlite3](https://sqlite.org/index.html) (pre-installed on macOS and most Linux distributions)
- `curl` and `jq`

## Steps

### 1. Create and populate the database

We'll work with the following dataset:

| name  | department  | role     | salary |
| ----- | ----------- | -------- | ------ |
| Alice | engineering | director | 130000 |
| Bob   | engineering | engineer | 90000  |
| Carol | engineering | engineer | 85000  |
| Dave  | marketing   | director | 120000 |
| Eve   | marketing   | manager  | 95000  |

Save the following SQL to a file named `employees.sql`:

```sql title="employees.sql"
CREATE TABLE employees (name TEXT, department TEXT, role TEXT, salary INTEGER);
INSERT INTO employees VALUES ('Alice', 'engineering', 'director', 130000);
INSERT INTO employees VALUES ('Bob',   'engineering', 'engineer',  90000);
INSERT INTO employees VALUES ('Carol', 'engineering', 'engineer',  85000);
INSERT INTO employees VALUES ('Dave',  'marketing',   'director', 120000);
INSERT INTO employees VALUES ('Eve',   'marketing',   'manager',   95000);
```

Then create the database by loading that file:

```shell
sqlite3 company.db < employees.sql
```

### 2. Write the policy

The rule is: Directors may see the salaries of employees in their own department.

`input.employees` is declared as _unknown_ — it represents database rows that OPA has not seen yet. `input.user` is _known_ at query time and its values will be substituted during partial evaluation.

Save the following Rego code to a file named `policy.rego`:

```rego title="policy.rego"
package authz

# METADATA
# scope: document
# compile:
#   unknowns: [input.employees]

include if {
    input.user.role == "director"
    input.employees.department == input.user.department
}
```

### 3. Start OPA

```shell
opa run --server policy.rego
```

OPA is now listening on `http://localhost:8181`.

### 4. Ask OPA for a SQL filter

In another terminal, call the compile endpoint with the logged-in user as input. Alice is a Director in Engineering:

```shell
curl -s -X POST http://localhost:8181/v1/compile/authz/include \
  -H "Content-Type: application/json" \
  -H "Accept: application/vnd.opa.sql.sqlite+json" \
  -d '{"input": {"user": {"name": "alice", "role": "director", "department": "engineering"}}}'
```

OPA partially evaluates the policy:

- `input.user.role == "director"` — both sides are known; the condition is true, so it is consumed.
- `input.employees.department == input.user.department` — the left hand side is unknown; the known right hand side (`"engineering"`) is substituted, yielding the SQL condition.

The response:

```json
{
  "result": {
    "query": "WHERE employees.department = 'engineering'"
  }
}
```

### 5. Query the database

Extract the filter and use it in a SQL query:

```shell
FILTER=$(curl -s -X POST http://localhost:8181/v1/compile/authz/include \
  -H "Content-Type: application/json" \
  -H "Accept: application/vnd.opa.sql.sqlite+json" \
  -d '{"input": {"user": {"name": "alice", "role": "director", "department": "engineering"}}}' \
  | jq -r '.result.query')

sqlite3 company.db "SELECT name, salary FROM employees $FILTER;"
```

Output — Alice sees all Engineering salaries:

| name  | salary |
| ----- | ------ |
| Alice | 130000 |
| Bob   | 90000  |
| Carol | 85000  |

Dave is a Director in Marketing, so he gets a different filter from the same policy:

```shell
FILTER=$(curl -s -X POST http://localhost:8181/v1/compile/authz/include \
  -H "Content-Type: application/json" \
  -H "Accept: application/vnd.opa.sql.sqlite+json" \
  -d '{"input": {"user": {"name": "dave", "role": "director", "department": "marketing"}}}' \
  | jq -r '.result.query')

sqlite3 company.db "SELECT name, salary FROM employees $FILTER;"
```

Output — Dave sees all Marketing salaries:

| name | salary |
| ---- | ------ |
| Dave | 120000 |
| Eve  | 95000  |

### 6. Non-Directors are denied

Bob is an Engineer, not a Director. The `input.user.role == "director"` condition is known and false, so no rule body can ever be satisfied — the policy unconditionally denies:

```shell
curl -s -X POST http://localhost:8181/v1/compile/authz/include \
  -H "Content-Type: application/json" \
  -H "Accept: application/vnd.opa.sql.sqlite+json" \
  -d '{"input": {"user": {"name": "bob", "role": "engineer", "department": "engineering"}}}'
```

Response — the `query` key is absent:

```json
{}
```

An absent `query` means unconditional deny. The application should return zero rows without issuing a database query.

:::warning Ensure safe defaults
OPA returns the filter — it does not enforce it. The application is responsible to use it as intended.

In this example, if the user is not a Director, no rule body can be satisfied and OPA returns an unconditional deny — represented as a missing `query` key in the result — meaning the application should safely return zero rows.
:::

## What partial evaluation did

OPA evaluated the policy with `input.user` fully known. The expressions that involved only known values (`input.user.role == "director"`) were fully evaluated and consumed — they do not appear in the output. Only expressions involving the unknown `input.employees` survived as residual conditions, which OPA then translated into SQL.

The application never needs to know _how_ the policy decides which salaries are visible. It sends user context and receives a SQL filter (or a deny) to act on.

## Handling unconditional results

| OPA response               | Meaning             | Application action           |
| -------------------------- | ------------------- | ---------------------------- |
| `{ "query": "WHERE ..." }` | Conditional allow   | Append filter to SQL query   |
| `{ "query": "" }`          | Unconditional allow | Run query with no `WHERE`    |
| `{}`                       | Unconditional deny  | Return zero rows, skip query |

## Clean up

Stop the OPA server with `Ctrl+C` in the terminal where it is running, then remove the files created during this tutorial:

```shell
rm employees.sql policy.rego company.db
```

## Next steps

- [Evaluating a Data Filter Policy](./partial-evaluation) — a step-by-step walkthrough of partial evaluation
- [Writing valid Data Filtering Policies](./fragment) — which Rego constructs are supported as filter conditions
- [Language SDKs](/ecosystem#languages) — in a production setup, using a language SDK is recommended over raw `curl` calls. The ecosystem page lists SDKs for Go, Java, Python, JavaScript, and more, all of which provide typed clients for the compile API used in this tutorial.

---
sidebar_position: 12
sidebar_label: Go Integration
---

<head>
  <title>Go Integration | Regal</title>
</head>

# Go Integration

:warning: Using Regal from Go is currently experimental and subject to change. Please swing by
[Slack](https://slack.openpolicyagent.org) if you're keen to use Regal in a production system.

If you'd like to integrate Regal into a Go application, this guide contains some pointers.

## Using Regal from Go

Regal can be used from a Go application by importing the `linter` package:

```go
import "github.com/open-policy-agent/regal/pkg/linter"
```

### Input

Input to `Lint` can be provided in a number of ways:

* Using the `InputFromPaths` helper to load Rego files from the filesystem,
* Using the `InputFromText` helper to parse a single Rego module from a string,

#### Using `InputFromPaths`

```go
paths := []string{"foo.rego", "bar.rego"}

input, err := rules.InputFromPaths(paths)
if err != nil {
    // handle error
}
```

#### Using `InputFromText`

```go

regoText := `package foo...`

input, err := rules.InputFromText("policy.rego", regoText)
if err != nil {
    // handle error
}
```

### Linting

To get a Regal report back for the provided input, create a Regal instance and call `Lint`:

```go
regalInstance := linter.NewLinter().WithInputModules(&input)

lintingReport, err := regalInstance.Lint(r.Context())
if err != nil {
    response.ErrorMessage = err.Error()
    writeJSON(w, http.StatusOK, response)
    return
}
```

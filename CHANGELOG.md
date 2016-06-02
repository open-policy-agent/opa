# Change Log

All notable changes to this project will be documented in this file. This
project adheres to [Semantic Versioning](http://semver.org/).

## 0.1.0

### Language

- Basic value types: null, boolean, number, string, object, and array
- Reference and variables types
- Incremental and complete rule definitions
- Negation of expressions
- Packages and imports

### Compiler

- Reference resolver to support packages
- Safety check to prevent recursive rules
- Safety checks to ensure successful evaluation will bind all variables in head
  and body of rules

### Evaluation

- Initial top down query evaluation algorithm

### Storage

- Basic in-memory storage that exposes JSON Patch style API

### Tooling

- Interactive shell that can be run to experiment with ad-hoc queries

### APIs

- Server mode supports HTTP APIs that allow callers to push in data and
  execute policy queries.

### Infrastructure

- Basic build infrastructure to produce cross-platform builds, run
  style/lint/format checks, execute tests, static HTML site.

### Documentation

- Architectural overview of OPA and design philosophy
- Language reference that serves as guide for new users

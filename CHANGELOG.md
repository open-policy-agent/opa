# Change Log

All notable changes to this project will be documented in this file. This
project adheres to [Semantic Versioning](http://semver.org/).

## 0.13.2

This release updates OPA to use the latest stable Golang release
(1.12.8) that includes important fixes in the net/http package. See
this
[golang-nuts](https://groups.google.com/forum/#!topic/golang-nuts/fCQWxqxP8aA)
group message for details.

## 0.13.0

### Multiple Bundles

This release adds support for downloading multiple bundles to OPA
using the new `bundles` key in the configuration. APIs that include
bundle information have been updated to support multiple bundles:

* Status API messages include the status and revision of each bundle.
* Decision Log API messages include the revision of each bundle.
* Data API responses include the revision of each bundle in the
  provenance field if requested.
* Health API waits for all bundles to activate if requested.

These changes are **backwards compatible**. If you are using the
existing `bundle` key in the configuration, you will not see any
changes in the APIs listed above.

We recommend that you switch to the new `bundles` key and update
consumers of the above APIs to support multiple bundles.

For more information on bundles see the [this
page](https://www.openpolicyagent.org/docs/latest/bundles/) in the OPA
documentation.

### Console Decision Logger

This release adds support for emitting decision logs to stdout. This
is useful for shipping decision logs directly to existing logging
backends.

You can enable console decision logging on the command line:

```
opa run --server --set decision_logs.console=true
```

Console decision logging can be enabled alongside normal and custom
decision logging.

### Fixes

- ast: Report safety errors on line where expression starts ([#1497](https://github.com/open-policy-agent/opa/issues/1497))
- ast: Update rule index to support glob.match ([#1496](https://github.com/open-policy-agent/opa/issues/1496))
- bundle: Add support for loading YAML files from bundles ([#1471](https://github.com/open-policy-agent/opa/issues/1471))
- bundle: Cache compiler on storage context ([#1515](https://github.com/open-policy-agent/opa/issues/1515))
- cmd: Fix double print of rego errors ([#1518](https://github.com/open-policy-agent/opa/issues/1518))
- docs: Add section on how to express "FOR ALL" in Rego ([#1307](https://github.com/open-policy-agent/opa/issues/1307))
- docs: Fix mention of reference head var ([#1477](https://github.com/open-policy-agent/opa/issues/1477))
- docs: Remove cast_xyz functions from docs ([#1405](https://github.com/open-policy-agent/opa/issues/1405))
- server: Pass transaction in decision log event ([#1543](https://github.com/open-policy-agent/opa/issues/1543))
- storage: Add safety checks to in-memory store ([#1594](https://github.com/open-policy-agent/opa/issues/1594))
- topdown: Fix corrupt object panic caused by copy propagation ([#1177](https://github.com/open-policy-agent/opa/issues/1177))
- topdown: Fix virtual cache to allow composite key terms ([#1197](https://github.com/open-policy-agent/opa/issues/1197))

### Miscellaneous

- OPA sets the User-Agent header in requests made to services.
- `openpolicyagent/opa:edge` Docker images are available now. The
  `edge` tag refers to the tip of master.
- OPA supports signing and encoding of JWTs. See [Token
  Signing](https://www.openpolicyagent.org/docs/latest/language-reference/#token-signing)
  for details.
- Prometheus metrics include cancelled HTTP requests.
- Compiler exposes optional unsafe built-in function check.
- Discovery query can be configured now. See [Discovery
  Configuration](https://www.openpolicyagent.org/docs/latest/configuration/#discovery)
  for details.
- Optimized rewriteDynamics stage in compiler to reduce allocations.
- OPA subcommands support "fails" explanation now. The "fails"
  explanation is similar to the "notes" explanation except that it
  prints Fail events instead of Note events. This is useful for among
  other things, debugging test failures.
- Partial evaluation can disable inlining on specific virtual
  documents. If set correctly this can improve partial evaluation
  performance significantly because OPA can avoid computing
  cross-products.
- `rego.Rego#PrepareForEVal` now times partial evaluation properly.
- The diagnostics feature deprecated in v0.10.1 has been removed.

## 0.12.2

### Fixes

- Fix performance impact of bundle activation on policy queries ([#1516](https://github.com/open-policy-agent/opa/issues/1516))
- Fix log masking to use correct transaction ([#1551](https://github.com/open-policy-agent/opa/pull/1551))

## 0.12.1

### Fixes

- Fix deadlock caused by log masking decision evaluation ([#1543](https://github.com/open-policy-agent/opa/issues/1543))

### Miscellaneous

- Add decision log event for undefined decision on `POST /` endpoint

## 0.12.0

This release includes two new features and an important bug fix.

### Decision Log Masking

This release includes an important feature for protecting sensitive
information in decision logs: masking. With the new decision log
masking feature you can configure OPA to remove sensitive information
from the `input` and `result` fields of decision log events. See the
[Decision Log](https://www.openpolicyagent.org/docs/edge/decision-logs/#masking-sensitive-data) documentation for details.

### AWS Signing for Bundle Downloads

This release adds support for signing bundle download requests using
an AWS signing scheme. This feature allows you to configure OPA to
download bundles directly from S3. See the [Configuration](https://www.openpolicyagent.org/docs/edge/configuration/#aws-signature)
documentation for details.

### Fixes

* server: Fix deadlock caused by leaked write transaction ([#1478](https://github.com/open-policy-agent/opa/issues/1478))

### Miscellaneous

- server: Add request headers to authorization input ([#1456](https://github.com/open-policy-agent/opa/issues/1456))
- rego: Add time zone support to time/date built-in functions
- eval: Add --instrument flag for profiling evaluation via command line

## 0.11.0

### Compatibility Notes

This release includes a few small but backward incompatible
changes:

* The compiler will reject functions that redeclare arguments. A
  search of public .rego files on GitHub only returned one result
  which was contained in the OPA documentation. For example:

    ```
    f(x) {
        x := 1  # bad: redeclaration of 'x'
        x == 1  # ok
    }
    ```

* Errors returned by built-in calls are no longer coded as
  `eval_internal_error`. Instead they are returned as
  `eval_builtin_error`. This change is made so callers can
  differentiate between actual internal errors and built-in errors
  that are result of bad inputs from the policy.

* The `ast.QueryCompiler#WithInput` function and
  `ast.QueryContext#Input` field have been removed because they were
  unused and had no affect.

* The `ast.Compiler` and `ast.QueryCompiler` functions to register
  extra changes now require a stage and metric name.

### Major Features

This release includes a few notable features and improvements:

* The `some` keyword allows you to declare local variables to avoid
  namespacing issues. See the [Some
  Keyword](https://www.openpolicyagent.org/docs/edge/how-do-i-write-policies/#some-keyword)
  section in the documentation for more detail.

* The `opa test`, `eval`, REPL, and HTTP API have been extended with a
  new explanation mode for filtering tracing notes. This makes it
  easier to see the output of `trace(msg)` calls from your policy.

* The WebAssembly (Wasm) compiler has been extended to include support for
  compiling rules into Wasm. Previously the compiler relied on partial
  evaluation to inline all rules. In some cases this is not possible
  due to limitations on Rego queries. In coming releases, the Wasm
  support will be extended to cover the entire language.

* The `rego` package has been extended to support prepared
  queries. Prepared queries cache the parsed and compiled query ASTs
  for re-use across multiple `Eval` calls. For small policies the
  speedup can be significant. See the [GoDoc](https://godoc.org/github.com/open-policy-agent/opa/rego#example-Rego-PrepareForEval) for details.

### Fixes

- Add Kubernetes admission control debugging tips ([#1039](https://github.com/open-policy-agent/opa/issues/1039))
- Add docs on health check API endpoint ([#1086](https://github.com/open-policy-agent/opa/issues/1086))
- Add hardened configuration example to security page ([#1172](https://github.com/open-policy-agent/opa/issues/1172))
- Add support for with keyword stacking ([#802](https://github.com/open-policy-agent/opa/issues/802))
- Fix type inferencing on object keys ([#1361](https://github.com/open-policy-agent/opa/issues/1361))
- Fix simple Kubernetes deployment example ([#874](https://github.com/open-policy-agent/opa/issues/874))
- Fix bug in data mocking that resulted in wrong iteration behavior ([#1261](https://github.com/open-policy-agent/opa/issues/1261))
- Fix bug in set deep copy that caused panic ([#1406](https://github.com/open-policy-agent/opa/issues/1406))
- Fix bug in REPL that prevented rules from being declared ([#1104](https://github.com/open-policy-agent/opa/issues/1104))

### Miscellaneous

- docs: Better documentation for providing the `input` document over HTTP ([#1293](https://github.com/open-policy-agent/opa/issues/1293))
- docs: Add note about HTTP_PROXY  friends ([#1410](https://github.com/open-policy-agent/opa/issues/1410))
- Add CLI config overrides and ENV injection
- Add additional compiler metrics for each stage
- Add an “edge” release to the docs
- Add param to include bundle activation in /health response
- Add provenance query output
- Add support for graceful shutdown of OPA server
- Improve discovery feature documentation
- Make `json` logs the default and add `json-pretty`
- Raise error when loading empty module in bundle
- Return eval_builtin_error instead of eval_internal_error
- Rewrite == to = in queries passed to the compile API
- docs: Update bundle docs with caching info
- Update logrus to 1.4.0
- server: Add early exit on PUT /v1/policies
- topdown: Fix set unification partial eval bug
- topdown: Omit rule body from enter/redo events

## 0.10.7

This release publishes the Hugo-based documentation to GitHub Pages :tada:

### Fixes

- Add `array.slice` built-in function ([#1243](https://github.com/open-policy-agent/opa/issues/1243))
- Add `net.cidr_contains` and `net.cidr_intersects` built-ins
  ([#1289](https://github.com/open-policy-agent/opa/issues/1289)). This
  change deprecates the old `net.cidr_overlap` built-in function. The
  latter will be supported for backwards compatibility but new
  policies should refer to `net.cidr_contains`.

### Miscellaneous

- Bump kube-mgmt container version to 0.8 in tutorial
- Remove unnecessary resizing allocs from AST set and object
- Add Kubernetes Admission Control guide

## 0.10.6

This release migrates the OPA documentation over to Hugo (from
GitBook). Going forward the OPA documentation will be generated using
Hugo and hosted on Netlify (instead of GitHub Pages). The Hugo/Netlify
stack brings us inline with the goal for other CNCF projects and
provides nice features like "preview before merge".

This release includes a small but backwards incompatible change to the
`http.send` built-in. Previously, `http.send` would _always_ decode
responses as JSON even if the Content-Type was unset or explicitly not
JSON. If you were previously relying on HTTP responses that did not
set the Content-Type correctly, you will need to update your policy to
pass `"force_json_decode": true` as in the `http.send` parameters.

### Fixes

- Fix panic in mod operation ([#1245](https://github.com/open-policy-agent/opa/issues/1245))
- Fix eval tree enumeration to return errors ([#1272](https://github.com/open-policy-agent/opa/issues/1272))
- Fix http.send to handle non-JSON responses ([#1258](https://github.com/open-policy-agent/opa/issues/1258))
- Fix backticks in SSH example that were causing problems ([#1260](https://github.com/open-policy-agent/opa/issues/1260))
- Fix IAM examples to use regex instead of glob syntax ([#1282](https://github.com/open-policy-agent/opa/issues/1282))

### Miscellaneous

- Add support to register custom stages in the compiler
- Add rootless Docker image stream
- Improve hash distribution on objects
- Reduce number of allocs in set membership implementation
- docs: Add homebrew install instruction to the Getting Started tutorial
- docs: Many improvements around := vs ==, best practices, cheatsheet, etc.
- cmd: Add --fail-defined flag to eval subcommand
- server: Fix patch path escaping

## 0.10.5

* These release contians a small but backwards incompatible change to
  the custom decision logger API. Custom decision loggers can now
  return an error which will cause the OPA to fail-closed.

### Fixes

- Fix substring built-in bounds checking ([#1235](https://github.com/open-policy-agent/opa/issues/1235))
- Add trailing newlines when pretty printing API responses
- Add default Go metrics to Prometheus
- Add pprof endpoint to HTTP server

## 0.10.4

* This release adds support for scoping bundles to specific roots
  under `data`. This allows bundles to be used in conjunction with
  sidecars like `kube-mgmt` that load local data and policy into
  OPA. See the [Bundles](https://www.openpolicyagent.org/docs/bundles.html)
  page for more details.

* This release includes a small but backwards incompatible change to
  the Decision Log event format. Instead of including the OPA version
  as a top-level field, the OPA version is included in the labels. The
  OPA version field was only added in v0.10.3 so this should not
  impact many consumers.

### Fixes

- Add coverage support to `opa eval` sub-command
- Fix path checking in server to prevent overlapping base and virtual docs ([#1207](https://github.com/open-policy-agent/opa/issues/1207))
- Fix cmd integration tests to cleanup plugin directory ([#1185](https://github.com/open-policy-agent/opa/issues/1185))
- Improve TLS support in `http.send` ([#1067](https://github.com/open-policy-agent/opa/issues/

## 0.10.3

* This release includes support for authentication via client
  certificates (thanks @srenatus!) For improvements to authentication
  see [#1163](https://github.com/open-policy-agent/opa/issues/1163).

* This release includes a backwards incompatible change to the
  plugin interface. Specifically, when plugins are registered, callers
  must provide a factory that can _validate_ configuration before
  instantiating the plugin. This allows OPA to ensure that all
  configuration is valid before activating changes. Since plugins were
  undocumented prior to this release, this change should be low
  impact. For details on plugin development see the new Plugins page
  on the website.

* This release includes a backwards incompatible change to the HTTP
  decision logger event type. Specifically, "null" inputs are now
  handled correctly and decision logs for ad-hoc queries now populate
  the "query" field in the event instead of the "path" field. If you
  are using consuming decision log events in Go, please switch to the
  decision logger framework documented here: https://github.com/open-policy-agent/opa/blob/master/docs/book/plugins.md.

### Fixes

- Add OPA version to decision logs ([#1089](https://github.com/open-policy-agent/opa/issues/1089))
- Add query metrics to decision logs ([#1033](https://github.com/open-policy-agent/opa/issues/1033))
- Add health endpoint to HTTP server ([#1086](https://github.com/open-policy-agent/opa/issues/1086))
- Add line of failure in `opa test` ([#961](https://github.com/open-policy-agent/opa/issues/961))
- Fix panic caused by assignment rewriting ([#1125](https://github.com/open-policy-agent/opa/issues/1125))
- Fix parser to avoid duplicate comments in AST ([#426](https://github.com/open-policy-agent/opa/issues/426))
- Fix semantic check for function references ([#1132](https://github.com/open-policy-agent/opa/issues/1132))
- Fix query API to return 4xx on bad request ([#1081](https://github.com/open-policy-agent/opa/issues/1081))
- Fix incorrect early exit from ref resolver ([#1110](https://github.com/open-policy-agent/opa/issues/1110))
- Fix rewriting of assignment values ([#1154](https://github.com/open-policy-agent/opa/issues/1154))
- Fix resolution inside references ([#1155](https://github.com/open-policy-agent/opa/issues/1155))
- Fix '^' location of lines starting with tabs ([#1129](https://github.com/open-policy-agent/opa/issues/1129))
- docs: Update count function doc to mention strings (#1126) ([#1122](https://github.com/open-policy-agent/opa/issues/1122))

### Miscellaneous

- Add tutorial for OPA/Ceph integration using Rook
- Add metrics timer for server handler
- Add support for custom backends in decision logger
- Fix find operation on sets for non-empty refs
- Fix bug in local declaration rewriting
- Fix discovery docs to show a realistic example
- Update decision log event to include error
- Update decision log events to model paths and queries
- Update server and decision logger to represent input properly
- Update server to include decision ID in error events
- Avoid zero values in http.Transport{} in REST client

### WebAssembly

- wasm: Add support for composite terms ([#1113](https://github.com/open-policy-agent/opa/issues/1113))
- wasm: Add support for not keyword ([#1112](https://github.com/open-policy-agent/opa/issues/1112))
- wasm: Add == operator
- wasm: Add checks on single term and dot stmts
- wasm: Add support for boolean and null literals
- wasm: Add support for pattern matching on composites
- wasm: Fix planner for chained iteration
- wasm: Fix pretty printer writer usage
- wasm: Output filenames in testgen errors
- wasm: Refactor assignment for better typing
- wasm: Remove module dumping from build command
- wasm: Rename ir.LoopStmt to ir.ScanStmt
- wasm: Update tester to allow for missing cases

## 0.10.2

### Fixes

- Add manifest metadata to bundle data (#1079) ([#1062](https://github.com/open-policy-agent/opa/issues/1062))
- Add profile command to REPL ([#838](https://github.com/open-policy-agent/opa/issues/838))
- Add decision ID note in API docs ([#1061](https://github.com/open-policy-agent/opa/issues/1061))
- Fix formatting of trailing comments in composites ([#1060](https://github.com/open-policy-agent/opa/issues/1060))
- Fix panic caused by input being set incorrectly ([#1083](https://github.com/open-policy-agent/opa/issues/1083))
- Fix partial eval to apply saved terms ([#1074](https://github.com/open-policy-agent/opa/issues/1074))

### Miscellaneous

- Add Stringer implementation for expr values
- Add Stringer implementation on metrics object
- Add helper function to compile strings
- Add note to configuration reference about -c flag
- Add support for configuration discovery
- Add support for multiple tracers
- Add trace helper to rego package
- Add code coverage percentage
- Fix REPL to check number of assignment operands
- Fix bug in test runner rule name dedup
- Fix security link in REST API reference
- Fix formatting of empty sets
- Fix incorrect reporting of module parse time
- Fix out of range errors for eq/assign in compiler
- Fix parser to limit size of exponents
- Update compiler to iterate over modules in sort order
- Update OPA front page
- Mark diagnostics feature as deprecated

## 0.10.1

### Fixes

- Add show debug command to REPL ([#750](https://github.com/open-policy-agent/opa/issues/750))

### Miscellaneous

- Add `glob` built-ins for easier path matching (thanks @aeneasr)
- Add support for specifying services as object

## 0.10.0

### Major Features

- **Wasm compiler**. This release adds initial/experimental support for
  compiling Rego policies into Wasm executables. Wasm executables can be loaded
  and executed in compatible Wasm runtimes like V8 (nodejs). You can try this
  out by running `opa build`.

- **Data mocking**. This release adds support for replacing/mocking the `data`
  document using the `with` keyword. In the past, `with` only supported the
  `input` document. This made it tricky to test context-dependent policies. With
  the new `with` keyword support, it's easier to write tests against contextual
  policies.

- **Negation Optimization**. This release includes an optimization in partial
  evaluation for dealing with negated statements (`not` keyword). In the past,
  OPA would generate a support rule for negated statements. This is harder for
  clients to consume and not readily optimized. The optimization computes the
  necessary cross-product of the negated query and inlines it into the caller.
  This leads to simpler partial evaluation results that are readily optimized,
  translated into other query languages (e.g., [SQL and Elasticsearch](https://blog.openpolicyagent.org/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4)),
  or compiled into Wasm.

### Fixes

- Add builtin to verify and decode JWT ([#884](https://github.com/open-policy-agent/opa/issues/884))
- Add GoDoc sample for using rego.Tracer ([#1002](https://github.com/open-policy-agent/opa/issues/1002))
- Add built-in function to get runtime info ([#420](https://github.com/open-policy-agent/opa/issues/420))
- Add support for YAML encoded input values ([#290](https://github.com/open-policy-agent/opa/issues/290))
- Add support for client certificates ([#684](https://github.com/open-policy-agent/opa/issues/684))
- Add support for non-zero exit code in eval subcommand ([#981](https://github.com/open-policy-agent/opa/issues/981))
- Fix == rewriting on embedded terms ([#995](https://github.com/open-policy-agent/opa/issues/995))
- Fix copy propagation panic in comprehensions ([#1012](https://github.com/open-policy-agent/opa/issues/1012))
- Implement regex.find_n (#1001) ([#747](https://github.com/open-policy-agent/opa/issues/747))
- Improve with modifier target error ([#343](https://github.com/open-policy-agent/opa/issues/343))
- Iterate over smaller set when intersecting ([#531](https://github.com/open-policy-agent/opa/issues/531))
- Only write one trailing newline at end of file ([#1032](https://github.com/open-policy-agent/opa/issues/1032))
- Redirect HTTP requests with trailing slashes ([#972](https://github.com/open-policy-agent/opa/issues/972))
- Update bundle reader to allow relative data.json ([#1019](https://github.com/open-policy-agent/opa/issues/1019))
- Expose version information via REST API ([#277](https://github.com/open-policy-agent/opa/issues/277))

### Miscellaneous

- Add default decision configuration
- Add extra helpers to loader result
- Add indentation to trace in failure output
- Add router option to the HTTP server
- Add support for headers in http.send (thanks @repenno)
- Deprecating --insecure-addr flag (thanks @repenno)
- Add POST v1/query API for large inputs (thanks @rite2nikhil)
- Remove heap allocations from AST set with open addressing
- Replace siphash with xxhash in AST
- Output traces on failures in verbose mode (thanks @srenatus)
- Rewrite duplicate test rule names (thanks @srenatus)

## 0.9.2

### Miscellaneous Fixes

- Add option to enable http redirects ([#921](https://github.com/open-policy-agent/opa/issues/921))
- Add copy propagation to support rules ([#911](https://github.com/open-policy-agent/opa/issues/911))
- Add support for inlining negated expressions in partial evaluation
- Add deps subcommand to analyze base and virtual document dependencies
- Add partial evaluation support to eval subcommand
- Add `net.cidr_overlap` built-in function (thanks @aeneasr)
- Add `regex.template_match` built-in function (thanks @aeneasr)
- Add external security audit information (thanks @caniszczyk)
- Add initial support for plugin loading (thanks @vrnmthr)
- Fix copy propagator type assertion panic ([#912](https://github.com/open-policy-agent/opa/issues/912))
- Fix panic in parser error detail construction ([#948](https://github.com/open-policy-agent/opa/issues/948))
- Fix with value rewriting for call terms ([#916](https://github.com/open-policy-agent/opa/issues/916))
- Fix coverage flag for test command (thanks @johscheuer)
- Fix compile operation timing in REPL
- Fix to indent 4 spaces instead of a tab (thanks @superbrothers)
- Fix REPL output in policy guide (thanks @ttripp)
- Multiple fixes in the Kubernetes admission controller tutorial (thanks @johscheuer)
- Improve formatting of empty ast.Body ([#909](https://github.com/open-policy-agent/opa/issues/909))
- Improve Kubernetes admission control policy loading explanation (thanks @rite2nihkil)
- Update http.send test to work without internet access ([#945](https://github.com/open-policy-agent/opa/issues/945))
- Update test runner to set Fail to true ([#954](https://github.com/open-policy-agent/opa/issues/954))

### Security Audit Fixes

- Improve token authentication docs and handler ([#901](https://github.com/open-policy-agent/opa/issues/901))
- Link to security docs in tutorials ([#917](https://github.com/open-policy-agent/opa/issues/917))
- Update bundle reader to cap buffer size ([#920](https://github.com/open-policy-agent/opa/issues/920))
- Validate queries by checking unsafe builtins ([#919](https://github.com/open-policy-agent/opa/issues/919))
- Fix XSS in debug page ([#918](https://github.com/open-policy-agent/opa/issues/918))

### Miscellaneous

## 0.9.1

### Fixes

- Add io.jwt.verify_es256 and io.jwt.verify_ps256 built-in functions (@optnfast)
- Add array.concat built-in function ([#851](https://github.com/open-policy-agent/opa/issues/851))
- Add support for command line bundle loading ([#870](https://github.com/open-policy-agent/opa/issues/870))
- Add regex split built-in function
- Fix incorrect AST node in Index events ([#859](https://github.com/open-policy-agent/opa/issues/859))
- Fix terraform tutorial type check errors ([#888](https://github.com/open-policy-agent/opa/issues/888))
- Fix CONTRIBUTING.md to include sign-off step (@optnfast)
- Improve save set performance ([#860](https://github.com/open-policy-agent/opa/issues/860))

## 0.9.0

### Major Features

This release adds two major features to OPA itself.

- Query Profiler: the `opa eval` subcommand now supports a `--profiler` option
  to help policy authors understand the performance profile of their policies.
  Give it a shot and let us know if you find it helpful or if you find cases
  that could be improved!

- Compile API: OPA now exposes Partial Evaluation with first-class interfaces.
  In prior releases, Partial Evaluation was only used for optimizations
  purposes. As of v0.9, callers can use Partial Evaluation via HTTP or Golang to
  obtain conditional decisions that can be evaluated on the client-side.

### Fixes

- Add ADOPTERS.md file ([#691](https://github.com/open-policy-agent/opa/issues/691))
- Add time.weekday builtin ([#789](https://github.com/open-policy-agent/opa/issues/789))
- Fix REPL output for multiple bool exprs ([#850](https://github.com/open-policy-agent/opa/issues/850))
- Remove support rule if default value is not needed ([#820](https://github.com/open-policy-agent/opa/issues/820))

### Miscellaneous

Here is a short list of notable miscellaneous improvements.

- Add any/all built-in functions (thanks @vrnmthr)
- Add built-in function to parse Rego modules
- Add copy propagation optimization to partial evaluation output
- Add docs for exercising policies with test framework
- Add extra output formats to eval subcommand
- Add support for providing input to eval via stdin
- Improve parser error readability
- Improve rule index to support unknown values
- Rewrite == with = in compiler
- Update build to enable CGO

...along with 30+ other fixes and improvements.

## 0.8.2

### Fixes

- Fix virtual document cache invalidation ([#736](https://github.com/open-policy-agent/opa/issues/736))
- Fix partial cache invalidation for data changes ([#589](https://github.com/open-policy-agent/opa/issues/589))
- Fix query to path conversion in decision logger ([#783](https://github.com/open-policy-agent/opa/issues/783))
- Fix handling of pointers to structs ([#722](https://github.com/open-policy-agent/opa/issues/722), thanks @srenatus)
- Improve sprintf number handling ([#748](https://github.com/open-policy-agent/opa/issues/748))
- Reduce memory overhead of decision logs ([#705](https://github.com/open-policy-agent/opa/issues/705))
- Set bundle status in case of HTTP 304 ([#794](https://github.com/open-policy-agent/opa/issues/794))

### Miscellaneous

- Add docs on best practices around identity
- Add built-in function to verify JWTs signed with HS246 (thanks @hbouvier)
- Add built-in function to URL encode objects (thanks @vrnmthr)
- Add query parameters to authorization policy input ([#786](https://github.com/open-policy-agent/opa/pull/786))
- Add support for listening on a UNIX domain socket ([#692](https://github.com/open-policy-agent/opa/issues/692), thanks @JAORMX)
- Add trace event for rule index lookups ([#716](https://github.com/open-policy-agent/opa/issues/716))
- Add support for multiple listeners in server (thanks @JAORMX)
- Remove decision log buffer size limit by default
- Update codebase with various go-fmt/ineffassign/mispell fixes (thanks @srenatus)
- Update REPL command to set unknowns
- Update subcommands to support loader filter ([#782](https://github.com/open-policy-agent/opa/issues/782))
- Update evaluator to cache storage reads
- Update object to keep track of groundness

## 0.8.1

### Fixes

- Handle escaped paths in data writes ([#695](https://github.com/open-policy-agent/opa/issues/695))
- Rewrite with modifiers to allow refs as values ([#701](https://github.com/open-policy-agent/opa/issues/701))

### Miscellaneous

- Add Kafka authorization tutorial
- Add URL query encoding built-ins
- Add runtime API to register plugins
- Update eval subcommand to support multiple files or directories (thanks @devenney)
- Update Terraform tutorial for OPA v0.8
- Fix bug in topdown query ID generation

## 0.8.0

### Major Features

This release includes a few major features that improve OPA's management
capabilities.

- Bundles: OPA can be configured to download bundles of policy and data from
  remote HTTP servers. This allows administrators to configure OPA to pull down
  all of the policy and data required at the enforcement point. When OPA boots
  it will download the bundle and active it. OPA will periodically check in with
  the server to download new revisions of the bundle.

- Status: OPA can be configured to report its status to remote HTTP servers. The
  status includes a description of the active bundle. This allows administrators
  to monitor the status of OPA in a central place.

- Decision Logs: OPA can be configured to report _decision logs_ to remote HTTP
  servers. This allows administrators to audit and debug decisions in a central
  place.

### File Loading Convention

The command line file loading convention has been changed slightly. If you were
previously loading files with `opa run *` you should use `opa run .` now. OPA
will not namespace data under top-level directory names anymore. The problem
with the old approach was that data layout was dependent on the root directory
name. For example `opa run /some/path1` and `opa run /some/path2` would yield
different results even if both paths contained identical data.

### Tracing Improvements

Thanks to @jyoverna for adding a `trace` built-in function that allows policy
authors to include notes in the trace. For example, authors can now embed
`trace` calls in their policies. When OPA encounters a `trace` call it will
include a "note" in the trace. Callers can filter the trace results to show only
notes. This helps diagnose incorrect decisions in large policies. For example:

```
package example

allow {
  input.method = allows_methods[_]
  trace(sprintf("input method is %v", [input.method]))
}

allowed_methods = ["GET", "HEAD"]
```

### Fixes

- Add RS256 JWT signature verification built-in function ([#421](https://github.com/open-policy-agent/opa/issues/421))
- Add X.509 certificate parsing built-in function ([#635](https://github.com/open-policy-agent/opa/issues/635))
- Fix substring built-in bounds checking ([#465](https://github.com/open-policy-agent/opa/issues/465))
- Generate support rules for negated expressions ([#623](https://github.com/open-policy-agent/opa/issues/623))
- Ignore some built-in calls during partial eval ([#622](https://github.com/open-policy-agent/opa/issues/622))
- Plug comprehensions in partial eval results ([#656](https://github.com/open-policy-agent/opa/issues/656))
- Report safety errors for generated vars ([#661](https://github.com/open-policy-agent/opa/issues/661))
- Update partial eval to check call args recursively ([#621](https://github.com/open-policy-agent/opa/issues/621))

### Other Notable Changes

- Add base64 encoding built-in functions
- Add JSON format to test and check subcommands
- Add coverage package and update test subcommand to report coverage
- Add eval subcommand to run queries from the command line (deprecates opa run --eval)
- Add parse subcommand to parse Rego modules and print AST
- Add reminder/reminder (%) operator
- Update rule index to support ==
- Update to Go 1.10
- Various fixes to fmt subcommand and format package
- Fix input and data loading to roundtrip values. Allows loading of []string, []int, etc.

As well as many other smaller improvements, refactoring, and fixes.

## 0.7.1

### Fixes

- Use rego.ParsedInput to provide input from form ([#571](https://github.com/open-policy-agent/opa/issues/571))

### Miscellaneous

- Add omitempty tag for ad-hoc query result field
- Fix rego package to check capture vars
- Fix root document assignment in REPL
- Update query compiler to deep copy parsed query

## 0.7.0

### Major Features

- Nested expressions: now you can write expressions like `(temp_f - 32)*5/9`!

- Assignment/comparison operators: now you can write `x := <expression>` to
  declare local variables and `x == y` when you strictly want to compare two
  values (and not bind any variables like with `=`).

- Prometheus support: now you can hook up Prometheus to OPA and collect
  performance metrics on the different APIs. (thanks @rlguarino)

### New Built-in Functions

This release adds and improves a bunch of new built-in functions. See the [Language Reference](http://www.openpolicyagent.org/docs/language-reference.html) for details.

- Add globs_match built-in function (thanks @yashtewari)
- Add HTTP request built-in function
- Add time.clock and time.date built-in functions
- Add n-way set union and intersection built-in functions
- Improve walk built-in function performance for partially ground paths

### Fixes

- Fix REPL assignment support ([#615](https://github.com/open-policy-agent/opa/issues/615))
- Fix panic due to nil term value ([#601](https://github.com/open-policy-agent/opa/issues/601))
- Fix safety check bug for call args ([#625](https://github.com/open-policy-agent/opa/issues/625))
- Update Kubernetes Admission Control tutorial ([#567](https://github.com/open-policy-agent/opa/issues/567))
- Update release script to build for Windows ([#573](https://github.com/open-policy-agent/opa/issues/573))

### Miscellaneous

- Add support for DELETE method in Data API ([#609](https://github.com/open-policy-agent/opa/issues/609)) (thanks @repenno)
- Add basic query performance instrumentation
- Add documentation covering how OPA compares to other systems
- Remove use of unsafe.Pointer for string hashing

## 0.6.0

This release adds initial support for partial evaluation. Partial evaluation
allows callers to mark certain inputs as unknown and then evaluate queries to
produce *new* queries which can be evaluated once inputs become known.

### Features

- Add initial implementation of partial evaluation
- Add sort built-in function ([#465](https://github.com/open-policy-agent/opa/issues/465))
- Add built-in function to check value types

### Fixes

- Fix rule arg type inferencing ([#542](https://github.com/open-policy-agent/opa/issues/542))
- Fix documentation on "else" keyword ([#475](https://github.com/open-policy-agent/opa/issues/475))
- Fix REPL to deduplicate auto-complete paths ([#432](https://github.com/open-policy-agent/opa/pull/432)
- Improve getting started example ([#532](https://github.com/open-policy-agent/opa/issues/532))
- Improve handling of forbidden methods in HTTP server ([#445](https://github.com/open-policy-agent/opa/issues/445))

### Miscellaneous

- Refactor sets and objects for constant time lookup

## 0.5.13

### Fixes

- Improve InterfaceToValue to handle other Go types ([#473](https://github.com/open-policy-agent/opa/issues/473))
- Fix bug in conflict detection ([#518](https://github.com/open-policy-agent/opa/issues/518))

## 0.5.12

### Fixes

- Fix eval of objects/sets containing vars ([#505](https://github.com/open-policy-agent/opa/issues/505))
- Fix REPL printing of generated vars

## 0.5.11

### Fixes

- Refactor topdown evaluation/unification ([#131](https://github.com/open-policy-agent/opa/issues/131))
- Rewrite refs in rule args ([#497](https://github.com/open-policy-agent/opa/issues/497))

### Miscellaneous

- Fix bug in expression formatting
- Fix dynamic rewriting to copy with modifiers
- Fix off-by-one bug in array helper

## 0.5.10

### Fixes

- Fix index usage for virtual docs ([#490](https://github.com/open-policy-agent/opa/issues/490))
- Fix match error panic ([#494](https://github.com/open-policy-agent/opa/issues/494))
- Fix wildcard mangling in rule head ([#480](https://github.com/open-policy-agent/opa/issues/480))

### Miscellaneous

- Add parse_duration_ns to generate nanos based on duration string
- Add product to calculate the product of array or set

## 0.5.9

### Fixes

- Fix unsafe var errors on functions ([#471](https://github.com/open-policy-agent/opa/issues/471), [#467](https://github.com/open-policy-agent/opa/issues/467))

### Miscellaneous

- Fix docs example of set union
- Fix file watch bug causing panic in server mode
- Modify AST to represent function names as refs
- Refactor runtime to separate init and start
- Refactor test runner to accept Store argument

## 0.5.8

### Fixes

- Substitute comprehension terms requring eval ([#453](https://github.com/open-policy-agent/opa/issues/453))

### Miscellaneous

- Add alpine-based Docker image
- Add stdin mode to opa fmt
- Fix syntax error in comprehension example
- Improve input parsing performance in V0 API
- Refactor loader to read inputs once (allows use of process substitution)
- Remove backup creation from fmt subcommand
- Remove use of sprintf in formatter

## 0.5.7

This release adds a new `test` subcommand to OPA. The `test` subcommand enables policy unit testing. The unit tests are expressed as rules containing assertions over test data. The `test` subcommand provides a test runner that automatically discovers and executes these test rules. See `opa test --help` for examples.

### Fixes

- Fix type error marshalling bug ([#391](https://github.com/open-policy-agent/opa/issues/391))
- Fix type inference bug ([#381](https://github.com/open-policy-agent/opa/issues/381))
- Fix unification bug ([#436](https://github.com/open-policy-agent/opa/issues/436))
- Fix type inferecen bug for partial objects with non-string keys ([#440](https://github.com/open-policy-agent/opa/issues/440))
- Suppress match errors if closures contained errors ([#438](https://github.com/open-policy-agent/opa/issues/438))

## 0.5.6

As part of this release, logrus was revendored to deal with the naming issue. If you use logrus, or one of your other dependencies does (such as Docker), be sure to check out https://github.com/sirupsen/logrus/issues/570#issuecomment-313933276.

### Fixes

- Fix incorrect REPL interpretation of some exprs ([#433](https://github.com/open-policy-agent/opa/issues/433))
- Fix inaccurate location information in some parser errors ([#214](https://github.com/open-policy-agent/opa/issues/214))

### Miscellaneous

- Add Terraform Testing tutorial to documentation
- Add shorthand for defining partial documents (e.g., `p[1]` instead of `p[1] { true }`)
- Add walk built-in function to recursively process nested documents
- Refactor Policy API response representations based on usage

## 0.5.5

This release adds [Diagnostics](http://www.openpolicyagent.org/docs/rest-api.html#diagnostics) support to the server. This greatly improves OPA's debuggability when deployed as a daemon.

### Miscellaneous

- Fix data race in the parser extensions
- Fix index.html GET requests returning error on empty input
- Fix race condition in watch test
- Fix image version in HTTP API tutorial
- Add metrics command to the REPL
- Limit length of pretty printed values in the REPL
- Simplify input query parameters in data GET requests
- Update server to support pretty explanations

## 0.5.4

### Miscellaneous

- Properly remove temporary files when running `opa fmt -d`
- Add support for refs with composite operands (e.g,. `p[[x,y]]`)

## 0.5.3

### Fixes

- Add support for raw strings ([#265](https://github.com/open-policy-agent/opa/issues/265))
- Add support to cancel compilation after some number of errors ([#249](https://github.com/open-policy-agent/opa/issues/249))

### Miscellaneous

- Add Kubernetes admission control tutorial
- Add tracing support to rego package
- Add watch package for watching changes to queries
- Add dependencies package to perform dependency analysis on ASTs

## 0.5.2

### Fixes

- Fix mobile view navigation bug
- Fix panic in compiler from concurrent map writes ([#379](https://github.com/open-policy-agent/opa/pull/379)
- Fix ambiguous syntax around body and set comprehensions ([#377](https://github.com/open-policy-agent/opa/issues/377))

### Miscellaneous

- Add support for set and object comprehensions
- Add support for system.main policy in server
- Add transaction support in rego package
- Improve type checking error messages
- Format REPL modules before printing them

## 0.5.1

### Fixes

- Correct `opa fmt` panic on missing files
- Fix minor site issues

### Miscellaneous

- Add rego examples with input and compiler
- Add support for query cancellation

## 0.5.0

### User Functions

OPA now supports user-defined functions that have the same semantics as built-in
functions. This allows policy authors to quickly define reusable pieces of logic
in Rego without overloading the input document or thinking about variable
safety.

### Storage Improvements

The storage layer has been improved to support single-writer/multiple-reader
concurrency. The storage interfaces have been simplified in the process. Users
can rely on https://godoc.org/github.com/open-policy-agent/opa/storage/inmem in
place of the old storage package.

### Website Refresh

The website has been redesigned and the documentation has been ported over to
GitBook.

### `opa check` and `opa fmt`

OPA supports two new commands that check and format policies. Check out `opa
help` for more information.

### Miscellaneous

- Add YAML serialization built-ins
- Add time built-ins

### Fixes

- Fixed incorrect source locations on refs and manually constructed terms. All
  term locations should be set correctly now.
- Fixed evaluation bug that caused partial sets and partial objects to be
  undefined in some cases.

## 0.4.10

This release includes a bunch of new built-in functions to help with string manipulation, JWTs, and more.

The JSON marshalling built-ins have been renamed. Policies that used `json_unmarshal` and `json_marshal` before will need to be updated to use `json.marshal` and `json.unmarshal` respectively.

### Misc

- Add `else` keyword
- Improved undefined built-in error message
- Fixed error message in `-` built-in
- Fixed exit instructions in REPL tutorial
- Relax safety check for built-in outputs

## 0.4.9

This release includes a bunch of cool stuff!

- Basic type checking for queries and virtual docs ([#312](https://github.com/open-policy-agent/opa/pull/312))
- Optimizations for HTTP API authorization policies ([#319](https://github.com/open-policy-agent/opa/pull/319))
- New /v0 API to support webhook integrations ([docs](http://www.openpolicyagent.org/documentation/references/rest-v0))

### Fixes

- Add support for namespaced built-ins ([#314](https://github.com/open-policy-agent/opa/issues/314))
- Improve logging to include request/response bodies ([#328](https://github.com/open-policy-agent/opa/pull/328))
- Add basic performance metrics ([#320](https://github.com/open-policy-agent/opa/pull/320))

### Miscellaneous

- Add built-ins to un/marshal JSON
- Add input form to diagnostic page

## 0.4.8

### Miscellaneous

- Fix top-level navigation links
- Improve file loader error handling

## 0.4.7

### Fixes

- Fix recursive binding by short-circuiting ref eval ([#298](https://github.com/open-policy-agent/opa/issues/298))
- Fix reordering for unsafe ref heads ([#297](https://github.com/open-policy-agent/opa/issues/297))
- Fix rewriting of single term exprs ([#299](https://github.com/open-policy-agent/opa/issues/299))

## 0.4.6

This release changes the `run` command options:

- Removed glog in favour of Sirupsen/logrus. This means the command line arguments to control logging have changed. See `run --help` for details.
- Removed `--policy-dir` option. For now, if policy persistence is required, users can treat policies as config files and manage them outside of OPA. Once OPA supports persistence of data (e.g., with file-based storage) then policy persistence will be added back in.

### Fixes

- Add support for additional HTTP listener ([#289](https://github.com/open-policy-agent/opa/issues/289))
- Allow slash in policy id/path ([#292](https://github.com/open-policy-agent/opa/issues/292))
- Improve request logging ([#281](https://github.com/open-policy-agent/opa/issues/281))

### Miscellaneous

- Add deployment documentation
- Remove erroneous flag.Parse() call
- Remove persist/--policy-dir option
- Replace glog with logrus

Also, updated Docker tagging so that latest points to most recent release (instead of most recent development build). The most recent development build can still be obtained with the {version}-dev tag.

## 0.4.5

### API security

This release adds support for TLS, token-based authentication, and authorization in the OPA APIs!

For details on how to secure the OPA API, go to http://openpolicyagent.org/documentation/references/security.

### Fixes

- Fix stray built-in error messages ([#275](https://github.com/open-policy-agent/opa/issues/275))
- Update error codes and messages throughout ([#237](https://github.com/open-policy-agent/opa/issues/237))
- [Fix evaluation bug with nested value refs](https://github.com/open-policy-agent/opa/pull/276/commits/e3336cce130eedda08f224ce4f28e19212447dcb)
- [Fix rego.Eval to close transactions](https://github.com/open-policy-agent/opa/pull/276/commits/745bd235127fae6bc22ff870bf62922c9358ccc0)
- [Fix buggy usage of errors.Cause](https://github.com/open-policy-agent/opa/pull/276/commits/bdf43b6a52de639e4f66810306311641ed7eea85)

### Miscellaneous

- Updated to support Go 1.8

## 0.4.4

### Fixes

- Fix issue in high-level Go API ([#261](https://github.com/open-policy-agent/opa/issues/261))

## 0.4.3

### Fixes

- Fix parsing of inline comments ([#258](https://github.com/open-policy-agent/opa/issues/258))
- Fix unset of input/data in REPL ([#259](https://github.com/open-policy-agent/opa/issues/259))
- Handle non-string/var values in rule tree lookup ([#257](https://github.com/open-policy-agent/opa/issues/257))

## 0.4.2

### Rego

This release changes the Rego syntax. The two main changes are:

- Expressions are now separated by semicolons (instead of commas). When writing rules, the semicolons are optional.
- Rules are no longer written in the form: `head :- body`. Instead they are written as `head { body }`.

Also:

- Set, array, and object literals now support trailing commas.
- To declare a set literal with one element, you must include a trailing comma, e.g., `{ foo, }`.
- Arithmetic and set operations can now be performed with infix notation, e.g., `x = 2 + 1`.
- Sets can be referred to like objects and arrays now ([#243](https://github.com/open-policy-agent/opa/issues/243)). E.g., `p[_].foo = 1 # check if element in has attr foo equal to 1`.

### Evaluation

This release changes the evaluation behaviour for packages. Previously, if a package contained no rules OR all of the rules were undefined when evaluated, a query against the package path would return undefined. As of v0.4.2 the query will return an empty object.

### REST API

This release changes the Data API to return an HTTP 200 response if the document is undefined. The message body will contain an object without the `result` property.

### Fixes

- Allow sets to be treated like objects/arrays

### Miscellaneous

- Added high level API for Go users. See `github.com/open-policy-agent/opa/rego` package.
- Improved expression String() function to handle infix operators better.
- Added support for set intersection and union built-ins. See language docs.

## 0.4.1

### Rego

For more details on these changes see sections in [How Do I Write Policies](http://www.openpolicyagent.org/documentation/how-do-i-write-policies/).

- Added new **default** keyword. The default keyword can be used to provide a default value for rules with complete definitions.
- Added new **with** keyword. The with keyword can be used to programmatically set the value of the input document inside policies.

### Fixes

- Fix input document definition in REPL ([#231](https://github.com/open-policy-agent/opa/issues/231))
- Fix reference evaluation bug ([#238](https://github.com/open-policy-agent/opa/issues/238))

### Miscellaneous

- Add basic REST API authorization benchmark

## 0.4.0

### REST API changes

This release contains a few non-backwards compatible changes to the REST API:

- The `request` document has been renamed to `input`. If you were calling the
  GET /data[/path]?request=value you should update to use [POST
  requests](http://www.openpolicyagent.org/documentation/references/rest#get-a-document-with-input)
  (see below).

- The API responses have been updated to return results embedded inside a
  wrapper object: `{"result": value}`. This will allow OPA to return unambiguous
  metadata in future (e.g., pagination, analysis, etc.) If you were previously
  consuming Data API GET responses, you should update your code to access the
  value under the `"result"` key of the response object.

- The API models have been updated to use snake_case
  ([#222](https://github.com/open-policy-agent/opa/issues/222)). This would only
  affect you if you were previously consuming error responses or policy ASTs.

The Data API has been updated to support the [POST
requests](http://www.openpolicyagent.org/documentation/references/rest#get-a-document-with-input).
This is the recommended way of supplying query inputs.

### Built-in Function changes

The built-in framework has been extended to support simplified built-in
implementations:

- Refactor topdown built-in functions
  ([#205](https://github.com/open-policy-agent/opa/issues/205))

### Fixes

- Add cURL note to REST API docs ([#211](https://github.com/open-policy-agent/opa/issues/211))
- Fix empty request parameter parsing ([#212](https://github.com/open-policy-agent/opa/issues/212))
- Fix handling of missing input document ([#227](https://github.com/open-policy-agent/opa/issues/227))
- Improve floating point literal support ([#215](https://github.com/open-policy-agent/opa/issues/215))
- Improve module parsing errors ([#213](https://github.com/open-policy-agent/opa/issues/213))
- Fix ast.Number hash and equality
- Fix parsing of escaped strings

### Miscellaneous

- Improve evaluation error messages

## 0.3.1

### Fixes

- Fixed unsafe vars with built-in operator names bug ([#206](https://github.com/open-policy-agent/opa/issues/206))
- Fixed body to rule conversion bug ([#202](https://github.com/open-policy-agent/opa/issues/202))
- Improved request parameter handling ([#201](https://github.com/open-policy-agent/opa/issues/201))

### Miscellaneous

- Improved release infrastructure

## 0.3.0

The last major/minor release of 2016! Woohoo! This release contains a few
non-backwards compatible changes to the APIs.

### Storage API changes

These changes simplify and clean up the storage.Store interface. This should
make it easier to implement custom stores in the future.

- Update storage to support context.Context ([#155](https://github.com/open-policy-agent/opa/issues/155))
- Update underlying number representation ([#154](https://github.com/open-policy-agent/opa/issues/154))
- Updates to use new storage.Path type ([#159](https://github.com/open-policy-agent/opa/issues/159))

### The request Document

These changes update the language to align query arguments with state stored in
OPA. With these changes, OPA can readily analyze policies and determine
references that refer to state stored in OPA versus query arguments versus local
variables.

These changes also update how query arguments are provided via the REST API.

- Updates to how query arguments are handled [#197](https://github.com/open-policy-agent/opa/pull/197)

### topdown API changes

- topdown.Context has been renamed to topdown.Topdown to avoid confusion with Golang's context.

### Fixes

- Add help topics to REPL ([#172](https://github.com/open-policy-agent/opa/issues/172))
- Fix error handling bug in Query API ([#183](https://github.com/open-policy-agent/opa/issues/183))
- Fix handling of prefixed paths with -w flag ([#193](https://github.com/open-policy-agent/opa/issues/193))
- Improve exit handling in REPL ([#175](https://github.com/open-policy-agent/opa/issues/175))
- Update parser support for <var> = <term> rules ([#192](https://github.com/open-policy-agent/opa/issues/192))

### Miscellaneous

- Add Visual Studio and Atom plugins
- Add lazy loading of modules during compilation
- Fix bug in serialization of empty objects/arrays

## 0.2.2

### Fixes

- Add YAML loading and refactor into separate file ([#135](https://github.com/open-policy-agent/opa/issues/135))
- Add command line flag to eval, print, and exit ([#152](https://github.com/open-policy-agent/opa/issues/152))
- Add compiler check for consistent rule types ([#147](https://github.com/open-policy-agent/opa/issues/147))
- Add set_diff built-in ([#133](https://github.com/open-policy-agent/opa/issues/133))
- Add simple 'show' command to print current module ([#108](https://github.com/open-policy-agent/opa/issues/108))
- Added examples to 'help' output in REPL ([#151](https://github.com/open-policy-agent/opa/issues/151))
- Check package declarations for conflicts ([#137](https://github.com/open-policy-agent/opa/issues/137))
- Deep copy modules in compiler ([#158](https://github.com/open-policy-agent/opa/issues/158))
- Fix evaluation of refs to set literals ([#149](https://github.com/open-policy-agent/opa/issues/149))
- Fix indexing usage for refs with intersecting vars ([#153](https://github.com/open-policy-agent/opa/issues/153))
- Fix output for references iterating sets ([#148](https://github.com/open-policy-agent/opa/issues/148))
- Fix parser handling of keywords in variable names ([#178](https://github.com/open-policy-agent/opa/issues/178))
- Improve file loading support ([#163](https://github.com/open-policy-agent/opa/issues/163))
- Remove conflict error for same key/value pairs ([#165](https://github.com/open-policy-agent/opa/issues/165))
- Support "data" query in REPL ([#150](https://github.com/open-policy-agent/opa/issues/150))

### Miscellaneous

- Add new compiler harness for ad-hoc queries
- Add tab completion of imports

## 0.2.1

### Improvements

- Added support for non-ground global values
- Added several new string manipulation built-ins
- Added TextMate grammar file
- Setup Docker image build and push to Docker Hub as part of CI

## 0.2.0

### Language

- Set literals
- Composite and reference values in Rule head
- Complete Rule definitions containing variables
- Builtins for regular expressions, string concatenation, casting to numbers

### Compiler

- Improved error reporting in parser and compiler

### Evaluation

- Iteration over base and virtual documents (in the same reference)
- Query tracing and explanation support

### Storage

- Pluggable data storage support

### Documentation

- GoDoc strings with examples
- REST API specification
- Concise language reference

### Performance

- Per-query cache of virtual documents in topdown

And many other small improvements and fixes.

## 0.1.0

### Language

- Basic value types: null, boolean, number, string, object, and array
- Reference and variables types
- Array comprehensions
- Built-in functions for basic arithmetic and aggregation
- Incremental and complete rule definitions
- Negation and disjunction
- Module system

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

- REPL that can be run to experiment with ad-hoc queries

### APIs

- Server mode supports HTTP APIs to manage policies, push and query documents, and execute ad-hoc queries.

### Infrastructure

- Basic build infrastructure to produce cross-platform builds, run
  style/lint/format checks, execute tests, static HTML site

### Documentation

- Introductions to policy, policy-enabling, and how OPA works
- Language reference that serves as guide for new users
- Tutorial that introduces users to the REPL
- Tutorial that introduces users to policy-enabling with a Docker Authorization plugin

# Change Log

All notable changes to this project will be documented in this file. This
project adheres to [Semantic Versioning](http://semver.org/).

## Unreleased

## 0.50.1

This is a bug fix release addressing the following issues:

### Fixes

- ast/compile: Guard recursive module equality check. ([#5756](https://github.com/open-policy-agent/opa/issues/5756)) authored by @philipaconrad. 
  Resolves a performance regression when using large bundles.
- ast: Relaxing strict-mode check for unused args in else-branching functions ([#5758](https://github.com/open-policy-agent/opa/issues/5758)) authored by @johanfylling reported by @ethanjli.

### Miscellaneous

- Use normalized policy paths as compiler module keys and store IDs (authored by @ashutosh-narkar). 
  Resolves an issue with bundle loading on Windows.

## 0.50.0

This release contains a mix of new features, bugfixes, security fixes, optimizations and build updates related to
OPA's published images.

### New Built-in Functions: JSON Schema Verification and Validation

These new built-in functions add functionality to verify and validate JSON Schema ([#5486](https://github.com/open-policy-agent/opa/pull/5486)) (co-authored by @jkulvich and @johanfylling).

- `json.verify_schema`: Checks that the input is a valid JSON schema object
- `json.match_schema`: Checks that the document matches the JSON schema

See the [documentation](https://www.openpolicyagent.org/docs/v0.50.0/policy-reference/#object) for all details.

### Annotations scoped to `package` carries across modules

`package` scoped schema annotations are now applied across modules instead of only local to the module where
it's declared  ([#5251](https://github.com/open-policy-agent/opa/issues/5251)) (authored by @johanfylling). This change may cause compile-time errors and behavioural changes to
type checking when the `schemas` annotation is used, and to rules calling the `rego.metadata.chain()` built-in function:

  - Existing projects with the same package declared in multiple files will trigger a `rego_type_error: package annotation redeclared`
error _if_ two or more of these are annotated with the `package` scope.
  - If using the `package` scope, the `schemas` annotation will be applied to type checking also for rules declared in
another file than the annotation declaration, as long as the package is the same.
  - The chain of metadata returned by the `rego.metadata.chain()` built-in function will now contain an entry for the
package even if the annotations are declared in another file, if the scope is `package`.

### Remote bundle URL shorthand for `run` command

To load a remote bundle using `opa run`, the `set` directive can be provided multiple times as shown below:
```
 $ opa run -s --set "services.default.url=https://example.com" \
              --set "bundles.example.service=default" \
              --set "bundles.example.resource=/bundles/bundle.tar.gz" \
              --set "bundles.example.persist=true"
```

The following command can be used as a shorthand to easily start OPA with a remote bundle ([#5674](https://github.com/open-policy-agent/opa/issues/5674)) (authored by @anderseknert):
```
$ opa run -s https://example.com/bundles/bundle.tar.gz
```

### Performance Improvements for `json.patch` Built-in Function

Performance improvements in `json.patch` were achieved with the introduction of a new `EditTree` data structure,
which is built for applying in-place modifications to an `ast.Term`, and can render the final result of all edits efficiently
by applying all patches in a JSON-Patch sequence rapidly, and then collapsing all edits at the end with minimal wasted `ast.Term` copying (authored by @philipaconrad).
For more details and benchmarks refer [#5494](https://github.com/open-policy-agent/opa/pull/5494) and [#5390](https://github.com/open-policy-agent/opa/pull/5390).

### Surface decision log errors via status API

Errors encountered during decision log uploads will now be surfaced via the Status API in addition to being logged. This
functionality should give users greater visibility into any issues OPA may face while processing, uploading logs etc ([#5637](https://github.com/open-policy-agent/opa/issues/5637)) (authored by @ashutosh-narkar).

See the [documentation](https://www.openpolicyagent.org/docs/v0.50.0/management-status/#status-service-api) for more details.

### OPA Published Images Update

All published OPA images now run with a non-root uid/gid. The `uid:gid` is set to `1000:1000` for all images. As a result
there is no longer a need for the `-rootless` image variant and hence it will be not be published as part of future releases.
This change is in line with container security best practices. OPA can still be run with root privileges by explicitly setting the user,
either with the `--user` argument for `docker run`, or by specifying the `securityContext` in the Kubernetes Pod specification.


### Runtime, Tooling, SDK

- server: Support compression of response payloads if HTTP client supports it ([#5310](https://github.com/open-policy-agent/opa/issues/5310)) authored by @AdrianArnautu
- bundle: Ensure the bundle resulting from merging a set of bundles does not contain `nil` data ([#5703](https://github.com/open-policy-agent/opa/issues/5703))  authored by @anderseknert
- repl: Use lowercase for repl commands only and keep any provided arguments as-is ([#5229](https://github.com/open-policy-agent/opa/issues/5229)) authored by @Trolloldem
- metrics: New endpoint `/metrics/alloc_bytes` to show OPA's memory utilization ([#5715](https://github.com/open-policy-agent/opa/pull/5715)) authored by @anderseknert
- server: When using OPA TLS authorization, authz policy authors will now have access to the client certificates
presented as part of the TLS connection. This new data will be available under the key `client_certificates` ([#5538](https://github.com/open-policy-agent/opa/issues/5538))  authored by @charlieegan3
- server: Use streaming implementation of json.Decode rather than using an intermediate buffer for the incoming request ([#5661](https://github.com/open-policy-agent/opa/pull/5661)) authored by @anderseknert

### Topdown and Rego

- ast: Extend compiler `strict` mode check to include unused arguments ([#5602](https://github.com/open-policy-agent/opa/issues/5602)) authored by @boranx. This change may cause
compile-time errors for policies that have unused arguments in the scope when the `strict` mode is enabled. These
variables could be replaced with `_` (wildcard) or get cleaned up if they are not intended to be used in the body of the functions.
- ast: Respect inlined `schemas` annotations even if `--schema` flag isn't used ([#5506](https://github.com/open-policy-agent/opa/issues/5506)) authored by @johanfylling
- ast: Force type-checker to respect `allow_net` capability when fetching remote schemas ([#5670](https://github.com/open-policy-agent/opa/issues/5670)) authored by @johanfylling
- ast/parse: Provide custom parsing options that allow location information of AST nodes to be included in their JSON
representation. This location information can be used by tools that work with the  OPA AST ([#3143](https://github.com/open-policy-agent/opa/issues/3143))  authored by @charlieegan3

### Docs

- docs/policy-reference: Fix typo in policy reference doc ([#5654](https://github.com/open-policy-agent/opa/pull/5654)) authored by @alvarogomez93
- docs/extensions: Fix sample code provided in the custom built-in implementation example ([#5666](https://github.com/open-policy-agent/opa/pull/5666)) authored by @Ronnie-personal
- docs/bundles: Clarify delta bundle behavior when it contains an empty list of patch operations ([#5629](https://github.com/open-policy-agent/opa/issues/5629))  authored by @charlieegan3
- docs/http-api-authz: Update the HTTP API authz tutorial with steps related to proper bundle creation ([#5682](https://github.com/open-policy-agent/opa/pull/5682)) authored by @lamoboos223
- Fix broken 'future keywords' url link ([#5686](https://github.com/open-policy-agent/opa/pull/5686)) authored by @neelanjan00


### Website + Ecosystem

- Ecosystem:
  - Styra Load ([#5659](https://github.com/open-policy-agent/opa/pull/5659)) authored by @charlieegan3

- Website:
  - Update OPA documentation search to use Algolia v3 ([#5706](https://github.com/open-policy-agent/opa/pull/5706)) authored by @Parsifal-M
  - Drop Google Universal Analytics (UA) code as part of Google Analytics 4 migration (authored by @chalin)

### Miscellaneous

- Dependency bumps, notably:
  - golang from 1.20.1 to 1.20.2
  - github.com/containerd/containerd from 1.6.16 to 1.6.19
  - github.com/golang/protobuf from 1.5.2 to 1.5.3
  - golang.org/x/net from 0.5.0 to 0.8.0
  - google.golang.org/grpc from 1.52.3 to 1.53.0
  - OpenTelemetry-related dependencies (#5701)


## 0.49.2

This release migrates the [ORAS Go library](oras.land/oras-go/v2) from v1.2.2 to v2.
The earlier version of the library had a dependency on the [docker](github.com/docker/docker)
package. That version of the docker package had some reported vulnerabilities such as
CVE-2022-41716, CVE-2022-41720. The ORAS Go library v2 removes the dependency on the docker package.

## 0.49.1

This is a bug fix release addressing the following Golang security issues:

### Golang security fix CVE-2022-41723

> A maliciously crafted HTTP/2 stream could cause excessive CPU consumption in the HPACK decoder, sufficient to cause a
> denial of service from a small number of small requests.

### Golang security fix CVE-2022-41724

> Large handshake records may cause panics in crypto/tls. Both clients and servers may send large TLS handshake records
> which cause servers and clients, respectively, to panic when attempting to construct responses.

### Golang security fix CVE-2022-41722

> A path traversal vulnerability exists in filepath.Clean on Windows. On Windows, the filepath.Clean function could
> transform an invalid path such as "a/../c:/b" into the valid path "c:\b". This transformation of a relative
> (if invalid) path into an absolute path could enable a directory traversal attack.
> After fix, the filepath.Clean function transforms this path into the relative (but still invalid) path ".\c:\b".

## 0.49.0

This release focuses on bugfixes and documentation improvements, as well as a few small performance improvements.

### Runtime, Tooling, SDK

- runtime: Update rule index's trie node scalar handling so that numerics compare correctly ([#5585](https://github.com/open-policy-agent/opa/issues/5585)) authored by @ashutosh-narkar reported by @alvarogomez93
- ast: Improve error information when metadata yaml fails to compile ([#4475](https://github.com/open-policy-agent/opa/issues/4475)) authored and reported by @johanfylling
- bundle: Retain metadata annotations for Wasm entrypoints during inspection ([#5588](https://github.com/open-policy-agent/opa/issues/5588)) authored and reported by @johanfylling
- compile: Allow object generating rules to be annotated as entrypoints ([#5577](https://github.com/open-policy-agent/opa/issues/5577)) authored and reported by @johanfylling
- plugins/discovery: Support for persisting and loading discovery bundle from disk ([#2886](https://github.com/open-policy-agent/opa/issues/2886)) authored by @ashutosh-narkar reported by @anderseknert
- perf: Use `json.Encode` to avoid extra allocation (authored by @anderseknert)
- `opa inspect`: Fix prefix error when inspecting bundle from root ([#5503](https://github.com/open-policy-agent/opa/issues/5503)) authored by @harikannan512 reported by @HarshPathakhp
- topdown: `http.send` to cache responses based on status code ([#5617](https://github.com/open-policy-agent/opa/issues/5617)) authored by @ashutosh-narkar
- types: Add GoDoc about named types (authored by @wata727)
- deps: Remove `github.com/pkg/errors` dependency (authored by @Iceber)


### Docs

- Update entrypoint documentation ([#5565](https://github.com/open-policy-agent/opa/issues/5565)) authored by @johanfylling reported by @robertgartman
- Add missing folder argument in bundle build example (authored by @charlieegan3)
- Clarify `crypto.x509.parse_certificates` docs (authored by @charlieegan3)
- Added AWS S3 Web Identity Credentials info to tutorial (authored by @vishrana)
- docs/graphql: non-nullable id argument and typo fix (authored by @philipaconrad)

### Website + Ecosystem

- Ecosystem:
  - ccbr (authored by @niuzhi)

- Website:
  - Show prominent warning when viewing old docs (authored by @charlieegan3)
  - Prevent navbar clipping on narrow screens + sticky nav  (authored by @charlieegan3)

### Miscellaneous

Dependency bumps:
- build: bump golang 1.19.4 -> 1.19.5 (authored by @yanggangtony)
- ci: aquasecurity/trivy-action from 0.8.0 to 0.9.0
- github.com/containerd/containerd from 1.6.15 to 1.6.16
- google.golang.org/grpc from 1.51.0 to 1.52.3

## 0.48.0

This release rolls in security fixes from recent patch releases, along with
a number of bugfixes, and a new builtin function.

### Improved error reporting available in `opa eval`

A common frustration when writing policies in OPA is when an error happens,
causing a rule to unexpectedly return `undefined`. Using
`--strict-builtin-errors` would allow finding the first error encountered
during evaluation, but terminates execution immediately.

To improve the debugging experience, it is now possible to display *all* of
the errors encountered during normal evaluation of a policy, via the new
`--show-builtin-errors` option.

Consider the following error-filled policy, `multi-error.rego`:

```rego
package play

this_errors(number) := result {
        result := number / 0
}

this_errors_too(number) := result {
        result := number / 0
}

res1 := this_errors(1)

res2 := this_errors_too(1)
```

Using `--strict-builtin-errors`, we would only see the first divide by zero
error:

    opa eval --strict-builtin-errors -d multi-error.rego data.play

```
1 error occurred: multi-error.rego:4: eval_builtin_error: div: divide by zero
```

Using `--show-builtin-errors` shows both divide by zero issues though:

    opa eval --show-builtin-errors -d multi-error.rego data.play -f pretty

```
2 errors occurred:
multi-error.rego:4: eval_builtin_error: div: divide by zero
multi-error.rego:8: eval_builtin_error: div: divide by zero
```

By showing more errors up front, we hope this will improve the overall
policy writing experience.

### New Built-in Function: `time.format`

It is now possible to format a time value from nanoseconds to a formatted
timestamp string via a built-in function. The builtin accepts 3 argument
formats, each allowing for different options:

 1. A number representing the nanoseconds since the epoch (UTC).
 2. A two-element array of the nanoseconds, and a timezone string.
 3. A three-element array of nanoseconds, timezone string, and a layout
  string (same format as for `time.parse_ns`).

See [the documentation](https://www.openpolicyagent.org/docs/v0.48.0/policy-reference/#builtin-time-timeformat)
for all details.

Implemented by @burnerlee.

### Optimization in rule indexing

Previously, every time the evaluator looked up a rule in the index, OPA
performed checks for grounded refs over the entire index *before* looking
up the rule.

Now, OPA performs all groundedness checks once at index construction time,
which keeps index lookup times much more consistent as the number of
indexed rules scales up.

Policies with large numbers of index-ready rules can expect a small
performance lift, proportional to the number of indexed rules.

### Bundle fetching with AWS Signing Version 4A

AWS has recently developed an extension to SigV4 called Signature Version
4A (SigV4A) which enables signatures that are valid in more than one AWS
Region. This new signature method is required for signing multi-region API
requests, such as Amazon S3 Multi-Region Access Points (MRAP).

OPA now supports this new request signing method for bundle fetching, which
means that you can use an S3 MRAP as a bundle source. This is configured
via the new `services[<your_service_name>].credentials.s3_signing.signature_version`
field.

See the [the documentation](https://www.openpolicyagent.org/docs/v0.48.0/configuration/#aws-signature)
for more details.

Implemented by @jwineinger

### Runtime

- rego: Check store modules before skipping parsing (authored by @charlieegan3)
- topdown/rego: Add BuiltinErrorList support to rego package, add to eval command (authored by @charlieegan3)
- topdown: Fix evaluator's re-wrapping of `NDBCache` errors (authored by @srenatus)
- Fix potential memory leak from `http.send` in interquery cache (authored by @asleire)
- ast/parser: Detect function rule head + `contains` keyword ([#5525](https://github.com/open-policy-agent/opa/issues/5525)) authored and reported by @philipaconrad
- ast/visit: Add `SomeDecl` to visitor walks ([#5480](https://github.com/open-policy-agent/opa/issues/5480)) authored by @srenatus
- ast/visit: Include `LazyObject` in visitor walks ([#5479](https://github.com/open-policy-agent/opa/issues/5479)) authored by @srenatus reported by @benweint

### Tooling, SDK

- topdown: cache undefined rule evaluations ([#593](https://github.com/open-policy-agent/opa/issues/593)) authored by @edpaget reported by @tsdandall
- topdown: Specify host verification policy for http redirects ([#5388](https://github.com/open-policy-agent/opa/issues/5388)) authored and reported by @ashutosh-narkar
- providers/aws: Refactor + Fix 2x Authorization header append issue ([#5472](https://github.com/open-policy-agent/opa/issues/5472)) authored by @philipaconrad reported by @Hiieu
- Add support to enable ND builtin cache via discovery ([#5457](https://github.com/open-policy-agent/opa/issues/5457)) authored by @ashutosh-narkar reported by @asadali
- format: Only use ref heads for all rule heads if necessary ([#5449](https://github.com/open-policy-agent/opa/issues/5449)) authored and reported by @srenatus
- `opa inspect`: Fix path of data namespaces on windows (authored by @shm12)
- ast+cmd: Only enforcing `schemas` annotations if `--schema` flag is used (authored by @johanfylling)
- sdk: Allow use of a query tracer (authored by @charlieegan3)
- sdk: Allow use of metrics, profilers, and instrumentation (authored by @charlieegan3)
- sdk: Return provenance information in Result types (authored by @charlieegan3)
- sdk: Allow use of StrictBuiltinErrors (authored by @charlieegan3)
- Allow print calls in IR (authored by @anderseknert)
- tester/runner: Fix panic'ing case in utility function ([#5496](https://github.com/open-policy-agent/opa/issues/5496)) authored and reported by @philipaconrad

### Docs

- Community page updates (authored by @anderseknert)
- Update Hugo version, update deprecated Page fields (authored by @charlieegan3)
- docs: Update TLS-based Authentication Example ([#5521](https://github.com/open-policy-agent/opa/issues/5521)) authored by @charlieegan3 reported by @jjthom87
- docs: Update opa eval flags to link to bundle docs (authored by @charlieegan3)
- docs: Make SDK first option for Go integraton (authored by @anderseknert)
- docs: Fix typo on Policy Language page.  (authored by @mcdonagj)
- docs/integrations: Update kubescape repo links (authored by @dwertent)
- docs/oci: Corrected config section (authored by @ogazitt)
- website/frontpage: Update Learn More links (authored by @pauly4it)

- integrations.yaml: Ensure inventors listed in organizations (authored by @anderseknert)
- integrations: Fix malformed inventors item (authored by @anderseknert)
- Add Digraph to ADOPTERS.md (authored by @jamesphlewis)

### Miscellaneous

- Remove changelog maintainer mention filter (authored by @anderseknert)
- Chore: Fix len check in the `ast/visit_test` error message (authored by @boranx)
- `opa inspect`: Fix wrong windows bundle tar files path separator (authored by @shm12)
- Add CHANGELOG.md to website build triggers (authored by @srenatus)

Dependency bumps:
- Golang 1.19.3 -> 1.19.4
- github.com/containerd/containerd from 1.6.10 -> 1.6.15
- github.com/dgraph-io/badger/v3
- golang.org/x/net to 0.5.0
- json5 and postcss-modules
- oras.land/oras-go from 1.2.1 -> 1.2.2

CI/Distribution fixes:
- Update base images for non debug builds (authored by @charlieegan3)
- Remove deprecated linters in golangci config (authored by @yanggangtony)

## 0.47.4

This is a bug fix release addressing a panic in `opa test`.

 - tester/runner: Fix panic'ing case in utility function. ([#5496](https://github.com/open-policy-agent/opa/issues/5496)) authored by @philipaconrad

## 0.47.3

This is a bug fix release addressing an issue that prevented OPA from fetching bundles stored in S3 buckets.

 - providers/aws: Refactor + fix 2x Authorization header append issue. ([#5472](https://github.com/open-policy-agent/opa/issues/5472)) authored by @philipaconrad, reported by @Hiieu

## 0.47.2 and 0.46.3

This is a second security fix to address CVE-2022-41717/GO-2022-1144.

We previously believed that upgrading the Golang version and its stdlib would be sufficient
to address the problem. It turns out we also need to bump the x/net dependency to v0.4.0.,
a version that hadn't existed when v0.46.2 was released.

This release bumps the golang.org/x/net dependency to v0.4.0, and contains no other
changes over v0.46.2.

Note that the affected code is OPA's HTTP server. So if you're using OPA as a Golang library,
or if your confident that your OPA's HTTP interface is protected by other means (as it should
be -- not exposed to the public internet), you're OK.

## 0.47.1 and 0.46.2

This is a bug fix release addressing two issues: one security issue, and one bug
related to formatting backwards-compatibility.

### Golang security fix CVE-2022-41717

> An attacker can cause excessive memory growth in a Go server accepting HTTP/2 requests.

Since we advise against running an OPA service exposed to the general public of the
internet, potential attackers would be limited to people that are already capable of
sending direct requests to the OPA service.

### `opa fmt` and backwards compatibility ([#5449](https://github.com/open-policy-agent/opa/issues/5449))

In v0.46.1, it was possible that `opa fmt` would format a rule in such a way that:

1. Before formatting, it was working fine with older OPA versions, and
2. after formatting, it would only work with OPA version >= 0.46.1.

This backwards incompatibility wasn't intended, and has now been fixed.

## 0.47.0

This release contains a mix of bugfixes, optimizations, and new features.

### New Built-in Function: `object.keys`

It is now possible to conveniently retrieve an object's keys via a built-in function.

Before, you had to resort to constructs like

```rego
import future.keywords.in

keys[k] {
    _ = input[k]
}

allow if "my_key" in keys
```

Now, you can simply do

```rego
import future.keywords.in

allow if "my_key" in object.keys(input)
```

See [the documentation](https://www.openpolicyagent.org/docs/v0.47.0/policy-reference/#builtin-object-objectkeys)
for all details.

Implemented by @kevinswiber.

### New Built-in Function: AWS Signature v4 Request Signing

It is now possible to use a built-in function to prepare a request with a signature, so that
it can be used with AWS endpoints that use request signing for authentication.

See this example:

```rego
req := {"method": "get", "url": "https://examplebucket.s3.amazonaws.com/data"}
aws_config := {
    "aws_access_key": "MYAWSACCESSKEYGOESHERE",
    "aws_secret_access_key": "MYAWSSECRETACCESSKEYGOESHERE",
    "aws_service": "s3",
    "aws_region": "us-east-1",
}
example_verify_resource {
    resp := http.send(providers.aws.sign_req(req, aws_config, time.now_ns()))
    # process response from AWS ...
}
```

See [the documentation on the new built-in](https://www.openpolicyagent.org/docs/v0.47.0/policy-reference/#providers.aws)
for all details.

Reported by @jicowan and implemented by @philipaconrad.

### Performance improvements for `object.get` and `in` operator

Before, using `object.get` and `in` had come with a performance penalty that wasn't
to be expected just from the look of the calls: Since they have been implemented using
built-in functions (obvious for `object.get`, not obvious for `"admin" in input.user.roles`),
all of their operands had to be read from the store (if applicable) and converted into
AST types.

Now, we use shallow references ("lazy objects") for store reads in the evaluator.
In these two cases, this can bring huge performance improvements, when the object
argument of these two calls is a ref into the base document (like `data.users`):

```rego
object.get(data.roles, input.role, [])
{ "id": 12 } in data.users
```

### Tooling, SDK, and Runtime

- `opa eval`: Added `--strict` to enable strict code checking in evaluation ([#5182](https://github.com/open-policy-agent/opa/issues/5182)) authored by @Parsifal-M
- `opa fmt`: Remove `{ true }` block following `else` head
- `opa fmt`: Generate new wildcards for else and chained function heads in the parser ([#5347](https://github.com/open-policy-agent/opa/issues/5347)). This fixes superfluous
  introductions of `_1` instead of `_` in when formatting functions that use wildcard arguments, like `f(_) := true`.
- `opa fmt`: Fix assignment rewrite in else formatting ([#5348](https://github.com/open-policy-agent/opa/issues/5348))
- OCI Download: Set auth credentials only if needed ([#5212](https://github.com/open-policy-agent/opa/issues/5212)) authored by @carabasdaniel
- Server: Differentiate between "missing" and "undefined doc" in default decision ([#5344](https://github.com/open-policy-agent/opa/issues/5344))

### Topdown and Rego

- `http.send`: Fix interquery cache size calculation with concurrent requests ([#5359](https://github.com/open-policy-agent/opa/issues/5359)) reported and authored by @asleire
- `http.send`: Remove socket query param for unix sockets ([#5313](https://github.com/open-policy-agent/opa/issues/5313)) reported and authored by @michivi
- Annotations: Add type coercion guards to avoid panics ([#5368](https://github.com/open-policy-agent/opa/issues/5368))
- Compiler: Provide more accurate error locations for `some` with unused vars ([#4238](https://github.com/open-policy-agent/opa/issues/4238))
- Optimization: Read lazy objects from the store ([#5325](https://github.com/open-policy-agent/opa/issues/5325)). This improves the performance of `x in data.foo` and `object.get(data.bar, ...)` calls significantly.
- Partial Evaluation: Skip comprehensions when checking eqs in copy propagation ([#5367](https://github.com/open-policy-agent/opa/issues/5367)). This fixes a bug when optimization on bundles would change the outcome of the subsequent evaluation.
- Parser: Fix else error handling with ref heads -- errors had occurred at a later stage then desired, because an edge case slipped through the earlier check.
- Planner/IR: Fix ref heads processing -- the CallDynamic optimization wasn't planned properly; a bug introduced with ref heads.

### Documentation

- Builtins: Mention base64 URL encoding specifically ([#5406](https://github.com/open-policy-agent/opa/issues/5406)) reported by @phi1010
- Builtins: Include behavior with sets in `json.patch` ([#5328](https://github.com/open-policy-agent/opa/issues/5328))
- Comparison: small fix to table to match sample code and other tables (authored by @anlandu)
- Builtins: Document reference timestamp behavior for `time.parse_ns`
- Typo fixes, authored by @deining
- Golang integration: update example code, move SDK above low-level packages

### Website + Ecosystem

- Ecosystem:
  - Add Easegress (authored by @localvar)
  - Add Terraform Cloud
- Website: Updated Footer Color ([#5254](https://github.com/open-policy-agent/opa/issues/5254)), reported and authored by @UtkarshMishra12
- Website: Add "canonical" link to latest to help with SEO and ancient pages being returned by search engines.
- Website: Add experimental "OPA version" badge. (Still needs to be tested more thorougly before advertisting it.)

### Miscellaneous

- Dependency bumps: Notably, we're now using wasmtime-go v3
- CI fixes:
  - Move performance tests to nightly tests
  - CLI: add simple bundle build tests
  - Nightly: Revamp how we're doing fuzz testing

## 0.46.1

This is  bugfix release to resolve an issue in the release pipeline. Everything else is
the same as 0.46.0.

## 0.46.0

This release contains a mix of bugfixes, optimizations, and new features.

### New language feature: refs in rule heads

With this version of OPA, we can use a shorthand for defining deeply-nested structures
in Rego:

Before, we had to use multiple packages, and hence multiple files to define a structure
like this:
```json
{
  "method": {
    "get": {
      "allowed": true
    }
    "post": {
      "allowed": true
    }
  }
}
```

```rego
package method.get
default allowed := false
allowed { ... }
```


```rego
package method.post
default allowed := false
allowed { ... }
```

Now, we can define those rules in single package (and file):

```rego
package method
import future.keywords.if
default get.allowed := false
get.allowed if { ... }

default post.allowed := false
post.allowed if { ... }
```

Note that in this example, the use of the future keyword `if` is mandatory
for backwards-compatibility: without it, `get.allowed` would be interpreted
as `get["allowed"]`, a definition of a partial set rule.

Currently, variables may only appear in the last part of the rule head:

```rego
package method
import future.keywords.if

endpoints[ep].allowed if ep := "/v1/data" # invalid
repos.get.endpoint[x] if x := "/v1/data" # valid
```

The valid rule defines this structure:
```json
{
  "method": {
    "repos": {
      "get": {
        "endpoint": {
          "/v1/data": true
        }
      }
    }
  }
}
```

To define a nested key-value pair, we would use

```rego
package method
import future.keywords.if

repos.get.endpoint[x] = y if {
  x := "/v1/data"
  y := "example"
}
```

Multi-value rules (previously referred to as "partial set rules") that are
nested like this need to use `contains` future keyword, to differentiate them
from the "last part is a variable" case mentioned just above:

```rego
package method
import future.keywords.contains

repos.get.endpoint contains x if x := "/v1/data"
```

This rule defines the same structure, but with multiple values instead of a key:
```json
{
  "method": {
    "repos": {
      "get": {
        "endpoint": ["/v1/data"]
      }
    }
  }
}
```

To ensure that it's safe to build OPA policies for older OPA versions, a new
capabilities field was introduced: "features". It's a free-form string array:

```json
{
  "features": [
    "rule_head_ref_string_prefixes"
  ]
}
```

If this key is not present, the compiler will reject ref-heads. This could be
case when building bundles for older OPA version using their capabilities.


### Entrypoint annotations in rule metadata

It is now possible to annotate a rule with `entrypoint: true`, and it will
automatically be picked up by the tooling that expected `--entrypoint` (`-e`)
parameters before.

For example, to build this rego policy into a wasm module, you had to pass
an entrypoint:

```rego
package test
allow {
    input.x
}
```
- `opa build --target wasm --entrypoint test/allow policy.rego`

With the annotation:
```rego
package test

# METADATA
# entrypoint: true
allow {
    input.x
}
```
- `opa build --target wasm policy.rego`

The places where entrypoints are taken from metadata are:

1. Building optimized bundles
2. Building Wasm bundles
3. Building Plan bundles
4. Using optimization with `opa eval`

Knowing a module's entrypoints can also help in different analysis tasks.

### New Built-in Functon: `graphql.schema_is_valid`

The new built-in allows checking schemas:

```rego
schema := `
  extend type User {
      id: ID!
  }
  extend type Product {
      upc: String!
  }
  union _Entity = Product | User
  extend type Query {
    entity: _Entity
  }
`
valid_schema_example {
    graphql.schema_is_valid(schema)
}
```

Requested by @olegroom.

### New Built-in Functon: `net.cidr_is_valid`

The new built-in function allows checking if a string is a valid CIDR.

```rego
valid_cidr_example {
	net.cidr_is_valid("192.168.0.0/24")
}
```

Authored by @ricardomaraschini.

### Tooling, SDK, and Runtime

- `opa build`: exit with failure on empty signing key ([#4972](https://github.com/open-policy-agent/opa/issues/4972)) authored by @Joffref reported by @caldwecr
- `opa exec`: add `--fail` and `--fail-defined` flags ([#5007](https://github.com/open-policy-agent/opa/issues/5007)) authored by @byronic reported by @phantlantis
- `opa exec`: convert slashes of explicit bundles (Windows) ([#5134](https://github.com/open-policy-agent/opa/issues/5134)) reported by @peterchenadded
- `opa test`: check coverage limit range `[0, 100]` ([#5284](https://github.com/open-policy-agent/opa/issues/5284)) authored by @hzliangbin reported by @aholmis
- `opa build`+`opa check`: respect capabilities for parsing, i.e. future keywords ([#5323](https://github.com/open-policy-agent/opa/issues/5323)) reported by @TheLunaticScripter
- `opa bench --e2e`: support providing OPA config ([#4899](https://github.com/open-policy-agent/opa/issues/4899))
- `opa eval`: new explain mode, `--explain=debug`, that includes unifcations in traces (authored by @jaspervdj)

- Decision logs: Allow rule-based dropping of decision log entries ([#3945](https://github.com/open-policy-agent/opa/issues/3945)) authored by @mariusblarsen and @iamatwork
- Decision Logs: Include the `req_id` attribute in the decision logs ([#5006](https://github.com/open-policy-agent/opa/issues/5006)) reported and authored by @humbertoc-silva
- Plugins: export OpenTelemetry TracerProvider for use in plugins (authored by @vinhph0906)


### Compiler + Topdown

- `graph.reachable_path`: fix issue with missing subpaths ([#4666](https://github.com/open-policy-agent/opa/issues/4666)) authored by @fredallen-wk
- `http.send`: Ensure `force_cache` attribute ignores `Date` header ([#4960](https://github.com/open-policy-agent/opa/issues/4960)) reported by @bartandacc
- `with`: Allow replacing functions with rules ([#5299](https://github.com/open-policy-agent/opa/issues/5299))
- Evaluation: Skip default functions in full extent ([#5202](https://github.com/open-policy-agent/opa/issues/5202)) reported by @ericjkao
- Evaluation: capture more cases of conflicts in function evaluation ([#5272](https://github.com/open-policy-agent/opa/issues/5272))
- Rule Indexing: fix incorrect results from indexing `glob.match` even if output is captured ([#5283](https://github.com/open-policy-agent/opa/issues/5283))

- Planner: various correctness fixes: [#5271](https://github.com/open-policy-agent/opa/issues/5271), [#5265](https://github.com/open-policy-agent/opa/issues/5265), [#5252](https://github.com/open-policy-agent/opa/issues/5252)

- Builtins: Refactor registration functions and signatures (authored by @philipaconrad)
- Compiler: Speed up typechecker when working with Refs (authored by @philipaconrad)
- Trace: add `UnifyOp` to tracer events (authored by @jaspervdj)

### Documentation

- Envoy Tutorial: use latest proxy_init (v8)
- Envoy Plugin: Add note about new config param to skip body parsing
- Policy Reference: Add `semver` examples
- Contributing Code: Provide some tips for style fixes

### Website + Ecosystem

- Website: Make "outdated version" banner red if looked-at version is ancient
- Ecosystem: Add CircleCI and Topaz

### Miscellaneous

- Code Cleanup:
  - Don't use the deprecated `ioutil` functions
  - Use `t.Setenv` in tests
  - Use `t.TempDir` to create temporary test directory (authored by @Juneezee)
  - Linters: add `unconvert` and `tenv`
- internal/strvals: port helm strvals fix (CLI --set arguments), reported by @pjbgf, helm fix authored by @mattfarina
- Wasm: Update README

- Dependency bumps, notably:
  - Golang: 1.19.2 -> 1.19.3
  - golang.org/x/text 0.3.7 -> 0.4.0
  - oras.land/oras-go 1.2.0 -> 1.2.1

## 0.45.0

This release contains a mix of bugfixes, optimizations, and new features.

### Improved Decision Logging with `nd_builtin_cache`

OPA has several non-deterministic built-ins, such as `rand.intn` and
`http.send` that can make debugging policies from decision log results
a surprisingly tricky and involved process. To improve the situation
around debugging policies that use those built-ins, OPA now provides
an opt-in system for caching the inputs and outputs of these built-ins
during policy evaluation, and can include this information in decision
log entries.

A new top-level config key is used to enable the non-deterministic
builtin caching feature, as shown below:

    nd_builtin_cache: true

This data is exposed to OPA's [decision log masking system](https://www.openpolicyagent.org/docs/v0.45.0/management-decision-logs/#masking-sensitive-data)
under the `/nd_builtin_cache` path, which allows masking or dropping
sensitive values from decision logs selectively. This can be useful
in situations where only some information about a non-deterministic
built-in was needed, or the arguments to the built-in involved
sensitive data.

To prevent unexpected decision log size growth from non-deterministic
built-ins like `http.send`, the new cache information is included in
decision logs on a best-effort basis. If a decision log event exceeds
the `decision_logs.reporting.upload_size_limit_bytes` limit for an OPA
instance, OPA will reattempt uploading it, after dropping the non-
deterministic builtin cache information from the event. This behavior
will trigger a log error when it happens, and will increment the
`decision_logs_nd_builtin_cache_dropped` metrics counter, so that it
will be possible to debug cases where the cache information is unexpectedly
missing from a decision log entry.

#### Decision Logging Example

To observe the change in decision logging we can run OPA in server mode
with `nd_builtin_cache` enabled:

```bash
opa run -s --set=decision_logs.console=true,nd_builtin_cache=true
```

After sending it the query `x := rand.intn("a", 15)` we should see
something like the following in the decision logs:

```
{..., "msg":"Decision Log", "nd_builtin_cache":{"rand.intn":{"[\"a\",15]":3}}, "query":"assign(x, rand.intn(\"a\", 15))", ..., "result":[{"x":3}], ..., "type":"openpolicyagent.org/decision_logs"}
```

The new information is included under the optional `nd_builtin_cache`
JSON key, and shows what arguments were provided for each unique
invocation of `rand.intn`, as well as what the output of that builtin
call was (in this case, `3`).

If we sent the query `x := rand.intn("a", 15); y := rand.intn("b", 150)"`
we can see how unique input arguments get recorded in the cache:

```
{..., "msg":"Decision Log", "nd_builtin_cache":{"rand.intn":{"[\"a\",15]":12,"[\"b\",150]":149}}, "query":"assign(x, rand.intn(\"a\", 15)); assign(y, rand.intn(\"b\", 150))", ..., "result":[{"x":12,"y":149}], ..., "type":"openpolicyagent.org/decision_logs"}
```

With this information, it's now easier to debug exactly why a particular
rule is used or why a rule fails when non-deterministic builtins are used in
a policy.

### New Built-in Function: `regex.replace`

This release introduces a new builtin for regex-based search/replace on
strings: `regex.replace`.

See [the built-in functions docs for all the details](https://www.openpolicyagent.org/docs/v0.45.0/policy-reference/#builtin-regex-regexreplace)

This implementation fixes [#5162](https://github.com/open-policy-agent/opa/issues/5162) and was authored by @boranx.

### `object.union_n` Optimization

The `object.union_n` builtin allows easily merging together an array of Objects.

Unfortunately, as noted in [#4985](https://github.com/open-policy-agent/opa/issues/4985)
its implementation generated unnecessary intermediate copies from doing
pairwise, recursive Object merges. These pairwise merges resulted in poor
performance for large inputs; in many cases worse than writing the
equivalent operation in pure Rego.

This release changes the `object.union_n` builtin's implementation to use
a more efficient merge algorithm that respects the original implementation's
sequential, left-to-right merging semantics. The `object.union_n` builtin
now provides a 2-3x improvement in speed and memory efficiency over the pure
Rego equivalent.

### Tooling, SDK, and Runtime

- cli: Fix doubled CLI hints/errors. ([#5115](https://github.com/open-policy-agent/opa/issues/5115)) authored by @ivanphdz
- cli/test: Add capabilities flag to test command. (authored by @ivanphdz)
- fmt: Fix blank lines after multiline expressions. (authored by @jaspervdj)
- internal/report: Include heap usage in the telemetry report.
- plugins/logs: Improve error message when decision log chunk size is greater than the upload limit. ([#5155](https://github.com/open-policy-agent/opa/issues/5155))
- ir: Make the `internal/ir` package public as `ir`.

### Rego

- ast/parser+formatter: Allow 'if' in rule 'else' statements.
- ast/schema: Add support for recursive json schema elements. ([#5166](https://github.com/open-policy-agent/opa/issues/5166)) authored and reported by @liamg
- ast/schema: Fix race condition in parsing with reused references.(authored by @liamg)
- internal/gojsonschema: Fix race condition in `SetAllowNet`. ([#5187](https://github.com/open-policy-agent/opa/issues/5187)) authored and reported by @liamg
- ast/compiler: Rewrite declared variables in function calls and recursively rewrite local variables in `with` clauses. ([#5148](https://github.com/open-policy-agent/opa/issues/5148)) authored and reported by @liu-du
- ast: Skip rules when parsing a body (or query) to help improve ambiguous parsing cases.

### Topdown

- topdown/object: Rework `object.union_n` to use in-place merge algorithm. (reported by @charlesdaniels)
- topdown/jwt_decode_verify: Ensure `exp` and `nbf` fields are numbers when present. ([#5165](https://github.com/open-policy-agent/opa/issues/5165)) authored and reported by @charlieflowers
- topdown: Fix `InterQueryCache` only dropping one entry when over the size limit. (authored by @vinhph0906)
- topdown+builtins: Block all ND builtins from partial evaluation.
- topdown/builtins: Add Rego Object support for GraphQL builtins to improve composability.
- topdown/json: Fix panic in `json.filter` on empty JSON paths.
- topdown/sets_bench_test: Add `intersection` builtin tests.
- topdown/tokens: Protect against nistec panics. ([#5128](https://github.com/open-policy-agent/opa/issues/5218))

### Documentation

- Add IR to integration docs.
- Added Gloo Edge Tutorial with examples. (authored by @Parsifal-M)
- Updated examples for CLI commands.
- Updated section on performance metrics (authored by @hutchins)
- docs/annotations: Add policy example and a link to the policy reference. ([#4937](https://github.com/open-policy-agent/opa/issues/4937)) authored by @Parsifal-M
- docs/policy-language: Be more explicit about future keywords.
- docs/security: Fix token authz example. (authored by @pigletfly)
- docs: Update generated CLI docs. (authored by @charlieflowers)
- docs: Update mentions of `#development` to `#contributors`. (authored by @charlieflowers)

### Website + Ecosystem

- website/security: Style improvements. (authored by @orweis)

### Miscellaneous

- ci: Add `prealloc` linter check and linter fixes.
- ci: Add govulncheck to Nightly CI.
- build/wasm: Use golang1.16 `go:embed` mechanism.
- util/backoff: Seed from math/rand source.
- version: Use `runtime/debug.BuildInfo`.

- Dependency bumps, notably:
  - build: bump golang 1.19.1 -> 1.19.2
  - build(deps): bump golang.org/x/net
  - build(deps): bump internal/gqlparser to v2.5.1
  - build(deps): bump tj-actions/changed-files from 29.0.3 -> 32.0.0
  - deps(build): bump wasmtime-go 0.36.0 -> 1.0.0 (authored by @Parsifal-M)

## 0.44.0

This release contains a number of fixes, two new builtins, a few new features,
and several performance improvements.

### Security Fixes

This release includes the security fixes present in the recent v0.43.1 release,
which mitigate CVE-2022-36085 in OPA itself, and CVE-2022-27664 and
CVE-2022-32190 in our Go build tooling.

See the Release Notes for v0.43.1 for more details.

### Set Element Addition Optimization

Rego Set element addition operations did not scale linearly ([#4999](https://github.com/open-policy-agent/opa/pull/4999))
in the past, and like the Object type before v0.43.0, experienced noticeable
reallocation/memory movement overheads once the Set grew past 120k-150k elements
in size.

This release introduces different handling of Set internals during element
addition operations to avoid pathological reallocation behavior, and allows
linear performance scaling up into the 500k key range and beyond.

### Set `union` Built-in Optimization

The Set `union` builtin allows applying the union operation to a set of sets.

However, as discovered in [#4979](https://github.com/open-policy-agent/opa/issues/4979),
its implementation generated unnecessary intermediate copies, which resulted in
poor performance; in many cases, worse than writing the equivalent operation in
pure Rego.

This release improves the `union` builtin's implementation, such that only the
final result set is ever modified, reducing memory allocations and GC pressure.
The `union` builtin is now about 15-30% faster than the equivalent operation in
pure Rego.

### New Built-in Functions: `strings.any_prefix_match` and `strings.any_suffix_match`

This release introduces two new builtins, optimized for bulk matching of string
prefixes and suffixes: `strings.any_prefix_match`, and
`strings.any_suffix_match`.
It works with sets and arrays of strings, allowing efficient matching of
collections of prefixes or suffixes against a target string.

See [the built-in functions docs for all the details](https://www.openpolicyagent.org/docs/v0.42.0/policy-reference/#builtin-strings-stringsany_prefix_match)

This implementation fixes [#4994](https://github.com/open-policy-agent/opa/issues/4994) and was authored by @cube2222.

### Tooling, SDK, and Runtime

- Logger: Allow configuration of the timestamp format ([#2413](https://github.com/open-policy-agent/opa/issues/2413))
- loader: Add support for fs.FS (authored by @ear7h)

#### Bundles

This release includes several bugfixes and improvements around bundle building:

- cmd: Add optimize flag to OPA eval command to allow building optimized bundles
- cmd/build+compile: Allow opt-out of dependents gathering to allow compilation of more bundles into WASM ([#5035](https://github.com/open-policy-agent/opa/issues/5035))
- opa build -t wasm|plan: Fail on unmatched entrypoints ([#3957](https://github.com/open-policy-agent/opa/issues/3957))
- opa build: Fix bundle mode to work with ignore flag
- bundle/status: Include bundle size in status information
- bundle: Remove raw bytes check for lazy bundle loading mode

#### Storage Fixes

This release has performance improvements and bugfixes for the disk storage system:

- storage/disk: Improve handling of in-flight transactions during truncate operations ([#4900](https://github.com/open-policy-agent/opa/issues/4900))
- storage/inmem: Allow disabling `util.Roundtrip` on Write for improved performance ([#4708](https://github.com/open-policy-agent/opa/issues/4708))
- storage: Improve multi-bundle data with overlapping roots is handled ([#4998](https://github.com/open-policy-agent/opa/issues/4998)) reported by @sirpi
- storage: Fix issue with policyID in Truncate calls ([#4958](https://github.com/open-policy-agent/opa/issues/4958)) authored by @martinjoha reported by @martinjoha

#### Rego

- eval+rego: Support caching output of non-deterministic builtins. ([#1514](https://github.com/open-policy-agent/opa/issues/1514))

#### AST and Topdown

The AST and Topdown module received a number of important bugfixes in this release:

- ast/term: Fix multiple-reader race condition for Sets/Objects
- ast/compile: Respect unsafeBuiltinMap for 'with' replacements
- ast: Add capacity to array initialization when size is known (authored by @mstrYoda)
- topdown/object: Fix unchecked error case in `object.union_n` builtin ([#5073](https://github.com/open-policy-agent/opa/issues/5073))
- topdown/reachable: Fix missing operand type checks. ([#4951](https://github.com/open-policy-agent/opa/issues/4951))
- topdown/units_parse: Avoid extra decimal places for integers
- topdown/type+wasm: Fix inconsistent `is_type` return values. ([#4943](https://github.com/open-policy-agent/opa/issues/4943))
- builtins: Fix inconsistent error messages in `units.parse*`
- Add query parameter in canonical request of AWS Sigv4 signature to avoid 403 errors from AWS (authored by @sinhaaks)

#### Test Suite

- Add error type to `units.*` builtin test assertions
- test/e2e/certrefresh: Add `file.Sync()` to eliminate test failures due to slow disk writes
- topdown/exported_tests: Remove Golang 1.16 x509 exception
- cmd/bench: Fix port collision in utility function used for E2E testing

### Documentation

- SECURITY: Migrate policy to web site, update content ([#4272](https://github.com/open-policy-agent/opa/issues/4272)) reported by @adoliver
- Add deprecated flag to all deprecated builtins ([#5072](https://github.com/open-policy-agent/opa/issues/5072))
- builtins: Update description of `format_int` to say it rounds down
- docs/policy-reference: Update Rego EBNF grammar (authored by @shaded-enmity)
- docs/builtins: Fix typo in `semver.compare` ([#5012](https://github.com/open-policy-agent/opa/issues/5012)) reported by @tetsuya28
- docs: Fix AWS Signature section in Configuration (authored by @pauly4it)
- docs: Update port and bundle folder for GraphQL tutorial
- docs: Document that function overloading is unsupported
- docs: Fixing related_resources annotations example ([#4982](https://github.com/open-policy-agent/opa/issues/4982)) reported by @humbertoc-silva
- docs: Fixing typo in metadata ([#5018](https://github.com/open-policy-agent/opa/issues/5018)) authored by @cimin0 reported by @cimin0

### Website + Ecosystem

- Update links to opa-kafka-plugin
- Add OCI documentation (authored by @carabasdaniel)
- Add article on using OPA for data filtering in Kafka
- Ecosystem: Add some links to Rönd (authored by @ugho16)
- Add community integration for Fiber (authored by @mstrYoda)
- Add Spacelift Integration (authored by @theseanodell)
- Fix broken link for Minio OPA integration  (authored by @unautre)

- Ecosystem Additions:
  - cosign (#5040) (authored by @Dentrax)

### Miscellaneous

- Dockerfile: Append root "/" to $PATH ([#5003](https://github.com/open-policy-agent/opa/issues/5003)) authored by @matusf reported by @matusf
- Add VNG Cloud to adopters (authored by @vinhph0906)

- Dependency bumps, notably:
  - build: bump golang: 1.19 -> 1.19.1
  - build: use go 1.19, drop go 1.16
  - build(deps): bump aquasecurity/trivy-action from 0.6.1 -> 0.7.1
  - build(deps): bump github.com/agnivade/levenshtein from 1.0.1 -> 1.1.1
  - build(deps): bump github.com/containerd/containerd from 1.6.6 -> 1.6.8
  - build(deps): bump github.com/go-ini/ini from 1.66.6 -> 1.67.0
  - build(deps): bump github.com/prometheus/client_golang
  - build(deps): bump google.golang.org/grpc from 1.48.0 -> 1.49.0
  - build(deps): bump tj-actions/changed-files from 28.0.0 -> 29.0.3

- Dependency removals:
  - internal: Vendor gqlparser library ([#5065](https://github.com/open-policy-agent/opa/issues/5065)) reported by @vikstrous2

## 0.43.1

This is a security release fixing the following vulnerabilities:

- CVE-2022-36085: Respect unsafeBuiltinMap for 'with' replacements in the compiler

  See https://github.com/open-policy-agent/opa/security/advisories/GHSA-f524-rf33-2jjr for all details.

- CVE-2022-27664 and CVE-2022-32190.

  Fixed by updating the Go version used in our builds to 1.18.6,
  see https://groups.google.com/g/golang-announce/c/x49AQzIVX-s.
  Note that CVE-2022-32190 is most likely not relevant for OPA's usage of net/url.
  But since these CVEs tend to come up in security assessment tooling regardless,
  it's better to get it out of the way.
## 0.43.0

This release contains a number of fixes, enhancements, and performance improvements.

### Object Insertion Optimization

Rego Object insertion operations did not scale linearly ([#4625](https://github.com/open-policy-agent/opa/issues/4625))
in the past, and experienced noticeable reallocation/memory movement
overheads once the Object grew past 120k-150k keys in size.

This release introduces different handling of Object internals during insert
operations to avoid pathological reallocation behavior, and allows linear
performance scaling up into the 500k key range and beyond.

### Tooling, SDK, and Runtime

- Add lines covered/not covered counts to test coverage report (authored by @FarisR99)
- Plugins: Status and logs plugins now accept any HTTP 2xx status code (authored by @lvisterin)
- Runtime: Generalize OS check for MacOS to other Unix-likes (authored by @iamleot)

#### Bundles Fixes

The Bundles system received several bugfixes and performance improvements in this release:

  - Bundle: `opa bundle` command now supports `.yml` files ([#4859](https://github.com/open-policy-agent/opa/issues/4859)) authored by @Joffref reported by @rdrgmnzsakt
  - Plugins/Bundle: Use unique temporary files for persisting activated bundles to disk ([#4782](https://github.com/open-policy-agent/opa/issues/4782)) authored by @FredrikAppelros reported by @FredrikAppelros
  - Server: Old policy path is now checked for bundle ownership before update ([#4846](https://github.com/open-policy-agent/opa/issues/4846))
  - Storage+Bundle: Old bundle data is now cleaned before new bundle activation ([#4940](https://github.com/open-policy-agent/opa/issues/4940))
  - Bundle: Paths are now normalized before bundle root check occurs to ensure checks are os-independent

#### Storage Fixes

The Storage system received mostly bugfixes, with a notable performance improvement for large bundles in this release:

  - storage/inmem: Speed up bundle activation by avoiding unnecessary read operations ([#4898](https://github.com/open-policy-agent/opa/issues/4898))
  - storage/inmem: Paths are now created during truncate operations if they did not exist before
  - storage/disk: Symlinks work with relative paths now ([#4869](https://github.com/open-policy-agent/opa/issues/4869))

### Rego and Topdown

The Rego compiler and runtime environment received a number of bugfixes, and a few new features this release, as well as a notable performance improvement for large Objects
(covered above).

- AST/Compiler: New method for obtaining parsed, but otherwise unprocessed modules is now available ([#4910](https://github.com/open-policy-agent/opa/issues/4910))
- `object.subset`: Support array + set combination ([#4858](https://github.com/open-policy-agent/opa/issues/4858)) authored by @x-color
- Compiler: Prevent erasure of `print()` statements in the compiler via a `WithEnablePrintStatements` option to `compiler.Compiler` and `compiler.optimizer` (authored by @kevinstyra)
- Topdown fixes:
  - AST/Builtins: `type_name` builtin now has more precise type metadata and improved docs
  - Topdown/copypropagation: Ref-based tautologies like `input.a == input.a` are no longer eliminated during the copy-propagation pass ([#4848](https://github.com/open-policy-agent/opa/issues/4848)) reported by @johanneskra
  - Topdown/parse_units: Use big.Rat for units parsing to avoid floating-point rounding issues on fractional units. ([#4856](https://github.com/open-policy-agent/opa/issues/4856)) reported by @tmos22
  - Topdown: `is_valid` builtins no longer error, and should always return booleans ([#4760](https://github.com/open-policy-agent/opa/issues/4760))
  - Topdown: `glob.match` now can be used without delimiters ([#4923](https://github.com/open-policy-agent/opa/issues/4923)) authored by @vinhph0906 reported by @vinhph0906

### Documentation

 - Docs: Add GraphQL API authorization tutorial
 - Docs/bundles: Add bundle CLI command documentation  ([#3831](https://github.com/open-policy-agent/opa/issues/3831)) authored by @Joffref
 - Docs/policy-reference: Remove extra quote in Grammar to fix formatting ([#4915](https://github.com/open-policy-agent/opa/issues/4915)) authored by @friedrichsenm reported by @friedrichsenm
 - Docs/policy-testing: Add missing future.keywords imports ([#4849](https://github.com/open-policy-agent/opa/issues/4849)) reported by @robert-elles
 - Docs: Add note about counter_server_query_cache_hit metric ([#4389](https://github.com/open-policy-agent/opa/issues/4389))
 - Docs: Kube tutorial includes updated cert install procedure ([#4902](https://github.com/open-policy-agent/opa/issues/4902)) reported by @Imp
 - Docs: GraphQL builtins section now includes a note about framework-specific `@directive` definitions in GraphQL schemas
 - Docs: Add warning about name collisions in older policies from importing 'future.keywords'

### Website + Ecosystem

- Website: Show navbar on smaller devices ([#3353](https://github.com/open-policy-agent/opa/issues/3353)) authored by @Parsifal-M reported by @OBrienCommaJosh
- Website/frontpage: Update front page examples to use the future.keywords imports
- Website/live-blocks: Only pass 'import future.keywords' when needed and supported
- Website/live-blocks: Update codemirror-rego to 1.3.0
- Website: Fix community page layout/scrolling issues (authored by @mstade)

- Ecosystem Additions:
  - Rond (authored by @ugho16)
  - walt.id

### Miscellaneous

- Dependency bumps, notably:
  - aquasecurity/trivy-action from 0.5.1 to 0.6.1
  - github.com/sirupsen/logrus from 1.8.1 to 1.9.0
  - github.com/vektah/gqlparser/v2 from 2.4.5 to 2.4.6
  - google.golang.org/grpc from 1.47.0 to 1.48.0
  - terser in /docs/website/scripts/live-blocks
  - glob-parent in /docs/website/scripts/live-blocks
- Added GKE Policy Automation to ADOPTERS.md (authored by @mikouaj)
- Fix minor code unreachability error (authored by @Abirdcfly)

## 0.42.2

This is a bug fix release that addresses the following:

- storage/disk: make symlinks work with relative paths ([#4869](https://github.com/open-policy-agent/opa/issues/4869))
- bundle: Normalize paths before bundle root check

## 0.42.1

This is a bug fix release that addresses the following:

1. An issue while writing data to the in-memory store at a non-root nonexistent path ([#4855](https://github.com/open-policy-agent/opa/issues/4855)), reported by @wermerb and others.
2. Policies owned by a bundle could be replaced via the REST API because of a missing bundle scope check ([#4846](https://github.com/open-policy-agent/opa/issues/4846)).
3. Adds missing `future.keywords` import for the examples in the policy testing section of the docs ([#4849](https://github.com/open-policy-agent/opa/issues/4849)), reported by @robert-elles.

## 0.42.0

This release contains a number of fixes and enhancements.

### New built-in function: `object.subset`

This function checks if a collection is a subset of another collection.
It works on objects, sets, and arrays.

If both arguments are objects, then the operation is recursive, e.g. `{"c": {"x": {10, 15, 20}}`
is considered a subset of `{"a": "b", "c": {"x": {10, 15, 20, 25}, "y": "z"}`.

See [the built-in functions docs for all the details](https://www.openpolicyagent.org/docs/v0.42.0/policy-reference/#builtin-object-objectsubset)

This implementation fixes [#4358](https://github.com/open-policy-agent/opa/issues/4358) and was authored by @charlesdaniels.

### New keywords: "contains" and "if"

These new keywords let you increase the expressiveness of your policy code:

Before

```rego
package authz
allow { not denied } # `denied` left out for presentation purposes

deny[msg] {
    count(violations) > 0
    msg := sprintf("there are %d violations", [count(violations)])
}
```

After

```rego
package authz
import future.keywords

allow if not denied # one expression only => no { ... } needed!

deny contains msg if {
    count(violations) > 0
    msg := sprintf("there are %d violations", [count(violations)])
}
```

Note that rule bodies containing only one expression can be abbreviated when using `if`.

To use the new keywords, use `import future.keywords.contains` and `import future.keywords.if`; or
import all of them at once via `import future.keywords`. When these future imports are present, the
pretty printer (`opa fmt`) will introduce `contains` and `if` where applicable.

`if` is allowed in all places to separate the rule head from the body, like
```rego
response[key] = value if { key := "open", y := "sesame" }
```
_but_ not for partial set rules, unless also using `contains`:
```rego
deny[msg]         if msg := "forbidden" # INVALID
deny contains msg if msg := "forbidden" # VALID
```

### Tooling, SDK, and Runtime

- Plugins:
  - S3 Plugin: Allow multiple AWS credential providers at once, chained together ([#4791](https://github.com/open-policy-agent/opa/issues/4791)), reported and authored by @abhisek
  - Discovery Plugin: Check for empty key config ([#4656](https://github.com/open-policy-agent/opa/issues/4656)) reported by @humbertoc-silva
  - Logs Plugin: Update mechanism to escape field paths ([#4717](https://github.com/open-policy-agent/opa/issues/4717)) reported by @pauly4it
  - Status Plugin: fix `bundle_failed_load_counter` metric for bundles without revisions ([#4822](https://github.com/open-policy-agent/opa/issues/4822)) reported and authored by @jkbschmid
- Server: The `system.authz` policy now properly supports the interquery caching of `http.send` calls ([#4829](https://github.com/open-policy-agent/opa/issues/4829)), reported by @HarshPathakhp
- `opa bench`: Passing `--e2e` makes the benchmark measure the performance of a query including the server's HTTP handlers and their processing.
- `opa fmt`: Output list _and_ diff changes with `--fail` flag (#4710) (authored by @davidkuridza)
- Disk Storage: Bundles are now streamed into the disk store, and not extracted completely in-memory ([#4539](https://github.com/open-policy-agent/opa/issues/4539))
- Golang package `repl`: Add a `WithCapabilities` function (authored by @jaspervdj)
- SDK: Allow configurable ID (authored by @rakshasa-1729)
- Windows: User lookups in various code paths have been avoided. They had no use, but are costly, and removing them should increase
  the performance of any CLI calls (even `opa version`) on Windows. Fixes [#4646](https://github.com/open-policy-agent/opa/issues/4646).
- Server: Open read storage transaction in Query API handler (not write)

### Rego and Topdown

- Runtime Errors: Fix type error message in `count`, `object.filter`, and `object.remove` built-in functions ([#4767](https://github.com/open-policy-agent/opa/issues/4767))
- Parser: Remove early MHS return in infix parsing, fixing confusing error messages ([#4672](https://github.com/open-policy-agent/opa/issues/4672)) authored by @philipaconrad
- AST: Disallow shadowing of called functions in comprehension heads ([#4762](https://github.com/open-policy-agent/opa/issues/4762))
- Planner/IR: shadow rule funcs if mocking functions ([#4746](https://github.com/open-policy-agent/opa/issues/4746))
- Compiler: Fix "every" handling in partial eval: by reordering body for safety differently, and correctly plugging its terms on safe ([#4801](https://github.com/open-policy-agent/opa/pull/4801)), reported by @jguenther-va
- Compiler: fix util.HashMap eq comparison ([#4759](https://github.com/open-policy-agent/opa/pull/4759))
- Built-ins: use strings.Builder in glob.match() (authored by @charlesdaniels)

### Documentation

- Builtins: Fix documentation of `startswith` and `endswith` (authored by @whme)
- Kubenetes Tutorial: Remove unused assignement in example ([#4778](https://github.com/open-policy-agent/opa/issues/4778)) authored by @Joffref
- OCI: Update configuration docs for private images in OCI registries (authored by @carabasdaniel)
- AWS S3 Signing: Fix profile_credentials docs (authored by @wangli1030)

### Website + Ecosystem

- Add "Edit on GitHub" button to docs ([#3784](https://github.com/open-policy-agent/opa/issues/3784)) authored by @avinashdesireddy
- Wasm: fix function table markup ([#4664](https://github.com/open-policy-agent/opa/issues/4664))
- Ecosystem: use location.hash to track open modal ([#4667](https://github.com/open-policy-agent/opa/issues/4667))

Note that website changes like these become effective immediately and are not tied to a release.
We still use our release notes to record the nice fixes contributed by our community.

- Ecosystem Additions:
  - Alfred, the self-hosted playground (authored by @dolevf)
  - Java Spring tutorial (authored by @psevestre)
  - Pulumi

### Miscellaneous

- Add Terminus to ADOPTERS.md (#4734) ([#4713](https://github.com/open-policy-agent/opa/issues/4713)) reported by @charlieflowers
- Remove any data attributes not used in the "YAML tests" ([#4813](https://github.com/open-policy-agent/opa/issues/4813))
- Dependency bumps, notably:
  - github.com/prometheus/client_golang 1.12.2 ([#4697](https://github.com/open-policy-agent/opa/issues/4697))
  - github.com/vektah/gqlparser/v2 2.4.5
- Build process and CI:
  - Use Trivy for vulnerability scans in code and container images (authored by @JAORMX)
  - Bump golangci-lint to v1.46.2, fix some issues ([#4765](https://github.com/open-policy-agent/opa/issues/4765))
  - Remove npm-opa-wasm test
  - Skip flaky darwin tests on PR runs
  - Fix flaky oci e2e test ([#4748](https://github.com/open-policy-agent/opa/issues/4748)) authored by @carabasdaniel
  - Integrate builtin_metadata.json handling in release process ([#4754](https://github.com/open-policy-agent/opa/issues/4754))


## 0.41.0

This release contains a number of fixes and enhancements.

### GraphQL Built-in Functions

A new set of built-in functions are now available to validate, parse and verify GraphQL query and schema! Following are
the new built-ins:

    graphql.is_valid: Checks that a GraphQL query is valid against a given schema
    graphql.parse: Returns AST objects for a given GraphQL query and schema
    graphql.parse_and_verify: Returns a boolean indicating success or failure alongside the parsed ASTs for a given GraphQL query and schema
    graphql.parse_query: Returns an AST object for a GraphQL query
    graphql.parse_schema: Returns an AST object for a GraphQL schema

### Built-in Function Metadata

Built-in function declarations now support additional metadata to specify name and description for function arguments
and return values. The metadata can be programmatically consumed by external tools such as IDE plugins. The built-in
function documentation is created using the new built-in function metadata.
Check out the new look of the [Built-In Reference](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions)
page!

Under the hood, a new file called `builtins_metadata.json` is generated via `make generate` which can be consumed by
external tools.

### Tooling, SDK, and Runtime

- OCI Downloader: Add logic to skip bundle reloading based on the digest of the OCI artifact ([#4637](https://github.com/open-policy-agent/opa/issues/4637)) authored by @carabasdaniel
- Bundles: Exclude empty manifest from bundle signature ([#4712](https://github.com/open-policy-agent/opa/issues/4712)) authored by @friedrichsenm reported by @friedrichsenm

### Rego and Topdown

- units.parse: New built-in for parsing standard metric decimal and binary SI units (e.g., K, Ki, M, Mi, G, Gi)
- format: Fix `opa fmt` location for non-key rules  (#4695) (authored by @jaspervdj)
- token: Ignore keys of unknown alg when verifying JWTs with JWKS ([#4699](https://github.com/open-policy-agent/opa/issues/4699)) reported by @lenalebt

### Documentation

- Adding Built-in Functions: Add note about `capabilities.json` while creating a new built-in function
- Policy Reference: Add example for `rego.metadata.rule()` built-in function
- Policy Reference: Fix grammar for `import` keyword ([#4689](https://github.com/open-policy-agent/opa/issues/4689)) authored by @mmzeeman reported by @mmzeeman
- Security: Fix command line flag name for file containing the TLS certificate ([#4678](https://github.com/open-policy-agent/opa/issues/4678)) authored by @pramodak reported by @pramodak

### Website + Ecosystem

- Update Kubernetes policy examples on the website to use latest kubernetes schema (`apiVersion`: `admission.k8s.io/v1`) (authored by @vicmarbev)
- Ecosystem:
  - Add Sansshell (authored by @sfc-gh-jchacon)
  - Add Nginx

### Miscellaneous

- Various dependency bumps, notably:
  - OpenTelemetry-go: 1.6.3 -> 1.7.0
  - go.uber.org/automaxprocs: 1.4.0 -> 1.5.1
  - github.com/containerd/containerd: 1.6.2 -> 1.6.4
  - google.golang.org/grpc: 1.46.0 -> 1.47.0
  - github.com/bytecodealliance/wasmtime-go: 0.35.0 -> 0.36.0
  - github.com/vektah/gqlparser/v2: 2.4.3 -> 2.4.4
- `make test`: Fix "too many open files" issue on Mac OS
- Remove usage of github.com/pkg/errors package (authored by @imjasonh)

## 0.40.0

This release contains a number of fixes and enhancements.

### Metadata introspection

The _rich metadata_ added in the v0.38.0 release can now be introspected
from the policies themselves!

    package example

    # METADATA
    # title: Edits by owner only
    # description: |
    #   Only the owner is allowed to edit their data.
    deny[{"allowed": false, "message": rego.metadata.rule().description}] {
        input.user != input.owner
    }

This snippet will evaluate to

    [{
      "allowed": false,
      "message": "Only the owner is allowed to edit their data.\n"
    }]

Both the rule's metadata can be accessed, via `rego.metadata.rule()`, and the
entire chain of metadata attached to the rule via the various scopes that different
metadata annotations can have, via `rego.metadata.chain()`.

All the details can be found in the documentation of [these new built-in functions](https://www.openpolicyagent.org/docs/v0.40.0/policy-reference/#rego).

### Function mocking

It is now possible to **mock functions** in tests! Both built-in and non-built-in
functions can be mocked:

    package authz
    import data.jwks.cert
    import data.helpers.extract_token

    allow {
        [true, _, _] = io.jwt.decode_verify(extract_token(input.headers), {"cert": cert, "iss": "corp.issuer.com"})
    }

    test_allow {
        allow
          with input.headers as []
          with data.jwks.cert as "mock-cert"
          with io.jwt.decode_verify as [true, {}, {}] # mocked built-in
          with extract_token as "my-jwt"              # mocked non-built-in
    }

For further information about policy testing with data and function mock, see [the Policy Testing docs](https://www.openpolicyagent.org/docs/v0.40.0/policy-testing/#data-and-function-mocking)
All details about `with` can be found in its [Policy Language section](https://www.openpolicyagent.org/docs/v0.40.0/policy-language/#with-keyword).

### Assignments with `:=`

Remaining restrictions around the use of `:=` in rules and functions have been lifted ([#4555](https://github.com/open-policy-agent/opa/issues/4555)).
These constructs are now valid:

    check_images(imgs) := x { # function
      # ...
    }

    allow := x { # rule
      # ...
    }

    response[key] := object { # partial object rule
      # ...
    }

In the wake of this, rules may now be "redeclared", i.e. you can use `:=` for more than one rule body:

    deny := x {
      # body 1
    }
    deny := x {
      # body 2
    }

This was forbidden before, but didn't serve a real purpose: it would catch trivial-to-catch errors
like

    p := 1
    p := 2 # redeclared

But it would do no good in more difficult to debug "multiple assignment" problems like

    p := x {
      some x in [1, 2, 3]
    }

### Tooling, SDK, and Runtime

- Status Plugin: Remove activeRevision label on all but one Prometheus metric ([#4584](https://github.com/open-policy-agent/opa/issues/4584)) reported and authored by @costimuraru
- Status: Include bundle type ("snapshot" or "delta") in status information
- `opa capabilities`: Expose capabilities through CLI, and allow using versions when passing `--capabilities v0.39.0` to the various commands ([#4236](https://github.com/open-policy-agent/opa/issues/4236)) authored by @IoannisMatzaris <!-- FC -->
- Logging: Log warnings at WARN level not ERROR, authored by @damienjburks
- Runtime: Persist activated bundle Etag to store ([#4544](https://github.com/open-policy-agent/opa/issues/4544))
- `opa eval`: Don't use source locations when formatting partially evaluated output ([#4609](https://github.com/open-policy-agent/opa/issues/4609))
- `opa inspect`: Fixing an issue where some errors encountered by the inspect command aren't properly reported
- `opa fmt`: Fix a bug with missing whitespace when formatting multiple `with` statements on one indented line ([#4634](https://github.com/open-policy-agent/opa/issues/4634))

#### Experimental OCI support

When configured to do so, OPA's bundle and discovery plugins will retrieve bundles from **any OCI registry**.
Please see [the Services Configuration section](https://www.openpolicyagent.org/docs/v0.40.0/configuration/#services)
for details.

Note that at this point, it's best considered a "feature preview". Be aware of this:
- Bundles are not cached, but re-retrieved and activated periodically.
- The persistence directory used for storing retrieved OCI artifacts is not yet managed by OPA,
  so its content may accumulate. By default, the OCI downloader will use a temporary file location.
- The documentation on how to push bundles to an OCI repository currently only exists in the development
  docs, see [OCI.md](https://github.com/open-policy-agent/opa/blob/v0.40.0/docs/devel/OCI.md).

Thanks to @carabasdaniel for starting the work on this!

### Rego and Topdown

- Builtins: Require prefix length for IPv6 in `net.cidr_merge` ([#4596](https://github.com/open-policy-agent/opa/issues/4596)), reported by @alexhu20
- Builtins: `http.send` can now parse and cache YAML responses, analogous to JSON responses
- Parser: Guard against invalid domains for "some" and "every", reported by @doyensec
- Formatting: Don't add 'in' keyword import when 'every' is there ([#4606](https://github.com/open-policy-agent/opa/issues/4606))

### Documentation

- Policy Language: Reorder Universal Quantification content, stress `every` over other constructions ([#4603](https://github.com/open-policy-agent/opa/issues/4603))
- Language pages: Use assignment operator where it's allowed.
- SSH Tutorial: Use bundle API
- Annotations: Update "Custom" annotation section
- Cloudformation: Fix markup and add warning related to booleans
- Blogs: mention OAuth2 and OIDC blog posts

### Website + Ecosystem

- Redirect previous patch releases to latest patch release ([#4225](https://github.com/open-policy-agent/opa/issues/4225))
- Add playground button to navbar
- Add SRI to static html files
- Remove right margin on sidebar (#4529) (authored by @orweis)
- Show yellow banner for old version (#4533)
- Remove unused variables to avoid error in strict mode(#4534) (authored by @panpan0000)
- Ecosystem:
  - Add AWS CloudFormation Hook
  - Add GKE policy automation
  - Add permit.io (authored by @ozradi)
  - Add Magda (authored by @t83714) 

### Miscellaneous

- Workflow: no content permissions for GitHub action 'post-release', authored by @naveensrinivasan
- Various dependency bumps, notably:
  - OpenTelemetry-go: 1.6.1 -> 1.6.3
  - go.uber.org/automaxprocs: 1.4.0 -> 1.5.1
- Binaries and Docker images are now built using Go 1.18.1.
- Dockerfile: add source annotation (#4626)

## 0.39.0

This release contains a number of fixes and enhancements.

### Disk Storage

The on-disk storage backend has been fully integrated with the OPA server, and
can now be enabled via configuration:

```yaml
storage:
  disk:
    directory: /var/opa # put data here
    auto_create: true   # create directory if it doesn't exist
    partitions:         # partitioning is important for data storage,
    - /users/*          # please see the documentation
```

It is intended to enable the use of OPA in scenarios where the data needed for
policy evaluation exceeds the available memory.

The on-disk contents will persist among restarts, but should not be used as a
single source of truth: there are no backup mechanisms, and certain data partitioning
changes will require a start-over. These are things that may get improved in the
future.

For all the details, please refer to the [configuration](https://www.openpolicyagent.org/docs/v0.39.0/configuration/#disk-storage)
and [detailled Disk Storage section](https://www.openpolicyagent.org/docs/v0.39.0/misc-disk/)
of the documentations.

### Tooling, SDK, and Runtime

- Server: Add warning when `input` attribute is missing in `POST /v1/data` API ([#4386](https://github.com/open-policy-agent/opa/issues/4386)) authored by @aflmp
- SDK: Support partial evaluation ([#4240](https://github.com/open-policy-agent/opa/pull/4240)), authored by @kroekle; with a fix to avoid using different state (authored by @Iceber)
- Runtime: Suppress payloads in debug logs for handlers that compress responses (`/metrics` and `/debug/pprof`) (authored by @christian1607)
- `opa test`: Add file path to failing tests to make debugging failing tests easier ([#4457](https://github.com/open-policy-agent/opa/issues/4457)), authored by @liamg
- `opa fmt`: avoid whitespace mixed with tabs on `with` statements ([#4376](https://github.com/open-policy-agent/opa/issues/4376)) reported by @tiwood
- Coverage reporting: Remove duplicates from coverage report ([#4393](https://github.com/open-policy-agent/opa/issues/4393)) reported by @gianna7wu
- Plugins: Fix broken retry logic in decision logs plugin ([#4486](https://github.com/open-policy-agent/opa/issues/4486)) reported by @iamatwork
- Plugins: Update regular polling fallback mechanism for downloader
- Plugins: Support for adding custom parameters and headers for OAuth2 Client Credentials Token request (authored by @srlk)
- Plugins: Log message on unexpected bundle content type ([#4278](https://github.com/open-policy-agent/opa/issues/4278))
- Plugins: Mask Authorization header value in debug logs ([#4495](https://github.com/open-policy-agent/opa/issues/4495))
- Docker images: Use GID 1000 in `-rootless` images ([#4380](https://github.com/open-policy-agent/opa/issues/4380)); also warn when using UID/GID 0.
- Runtime: change processed file event log level to info

### Rego and Topdown

- Type checker: Skip pattern JSON Schema attribute compilation ([#4426](https://github.com/open-policy-agent/opa/issues/4426)): These are not supported, but could have caused the parsing of a JSON Schema document to fail.
- Topdown: Copy without modifying expr, fixing a bug that could occur when running multiple partial evaluation requests concurrently.
- Compiler strict mode: Raise error on unused imports ([#4354](https://github.com/open-policy-agent/opa/issues/4354)) authored by @damienjburks
- AST: Fix print call rewriting in else rules ([#4489](https://github.com/open-policy-agent/opa/issues/4489))
- Compiler: Improve error message on missing `with` target ([#4431](https://github.com/open-policy-agent/opa/issues/4431)) reported by @gabrielfern
- Parser: hint about 'every' future keyword import

### Documentation and Website

- AWS CloudFormation Hook: New tutorial
- Community: Stretch background so it covers on larger screens ([#4402](https://github.com/open-policy-agent/opa/issues/4402)) authored by @msorens
- Build: Make local dev and PR preview not build everything ([#4379](https://github.com/open-policy-agent/opa/issues/4379))
- Philosophy: Grammar fixes (authored by @ajonesiii)
- README: Add note about Hugo version mismatch errors (authored by @ogazitt)
- Integrations: Add GraphQL-Graphene (authored by @dolevf), Emissary-Ingress (authored by @tayyabjamadar), rekor-sidekick,
- Integrations CI: ensure referenced software is listed, and logo file names match; allow SVG logos
- Envoy: Update policy primer with new control headers
- Envoy: Update bob_token and alice_token in tutorial (authored by @rokkiter)
- Envoy: Include new configurable gRPC msg sizes (authored by @emaincourt)
- Annotations: add missing title to index (authored by @itaysk)

### Miscellaneous

- Various dependency bumps, notably:
  - OpenTelemetry-go: 1.4.1 -> 1.6.1
  - Wasmtime-go: 0.34.0 -> 0.35.0
- Binaries and Docker images are now built using Go 1.18; CI runs build/test for Ubuntu and macos with Go 1.16 and 1.17.
- CI: remove go-fuzz, use native go 1.18 fuzzer

## 0.38.1

This is a bug fix release that addresses one issue when using `opa test` with the
`--bundle` (`-b`) flag, and a policy that uses the `every` keyword.

There are no other code changes in this release.

### Fixes

- Compiler: don't raise an error with unused declared+generated vars
  (every) ([#4420](https://github.com/open-policy-agent/opa/issues/4420)),
  reported by @kristiansvalland

## 0.38.0

This release contains a number of fixes and enhancements.

It contains one **backwards-incompatible change** to the JSON representation
of metrics in **Status API** payloads, please see the section below.

### Rich Metadata

It is now possible to annotate Rego policies in a way that can be
processed programmatically, using _Rich Metadata_.

    # METADATA
    # title: My rule
    # description: A rule that determines if x is allowed.
    # authors:
    # - Jane Austin <jane@example.com>
    allow {
      ...
    }

The available keys are:

- title
- description
- authors
- organizations
- related_resources
- schemas
- scope
- custom

Custom annotations can be used to annotate rules, packages, and
documents with whatever you specifically need, beyond the generic
keywords.

Annotations can be retrieved using the [Golang library](https://www.openpolicyagent.org/docs/v0.38.0/annotations/#go-api)
or via the CLI, `opa inspect -a`.

All the details can be found in the documentation on [Annotations](https://www.openpolicyagent.org/docs/v0.38.0/annotations/).

### Every Keyword

A new keyword for explicit iteration is added to Rego: `every`.

It comes in two forms, iterating values, or keys and values, of a
collection, and asserting that the body evaluates successfully for
each binding of key and value to the collection's elements:

    every k, v in {"foo": "FOO", "bar": "BAR" } {
      upper(k) == v
    }

To use it, `import future.keywords.every` or `future.keywords`.

For further information, please refer to the [Every Keyword docs](https://www.openpolicyagent.org/docs/v0.38.0/policy-language/#every-keyword)
and the new section on [_FOR SOME and FOR ALL_ in the Intro docs](https://www.openpolicyagent.org/docs/v0.38.0/#for-some-and-for-all).

### Tooling, SDK, and Runtime

- Compile API: add `disableInlining` option ([#4357](https://github.com/open-policy-agent/opa/issues/4357)) reported and fixed by @srlk
- Status API: add `http_code` to response ([#4259](https://github.com/open-policy-agent/opa/issues/4259)) reported and fixed by @jkbschmid
- Status plugin: publish experimental bundle-related metrics via prometheus endpoint (authored by @rafaelreinert) -- See [Status Metrics](https://www.openpolicyagent.org/docs/v0.38.0/monitoring/#status-metrics) for details.
- SDK: don't panic without config ([#4303](https://github.com/open-policy-agent/opa/issues/4303)) authored by @damienjburks
- Storage: Support index for array appends (for JSON Patch compatibility)
- `opa deps`: Fix pretty printed output to show virtual documents ([#4342](https://github.com/open-policy-agent/opa/issues/4342))

### Rego and Topdown

- Parser: parse 'with' on 'some x in xs' expression ([#4226](https://github.com/open-policy-agent/opa/issues/4226))
- AST: hash containers on insert/update ([#4345](https://github.com/open-policy-agent/opa/issues/4345)), fixing a data race reported by @skillcoder
- Planner: Fix bug related to undefined results in dynamic lookups

### Documentation and Website

- Policy Reference: update EBNF to include "every" and "some x in ..." ([#4216](https://github.com/open-policy-agent/opa/issues/4216))
- REST API: Update docs on 400 response
- README: Include Google Analytic Instructions
- Envoy primer: use variables instead of objects
- Istio tutorial: expose application to outside traffic
- New "Community" Webpage (authored by @msorens)

### WebAssembly

- OPA now uses Wasmtime 0.34.0 to evaluate its Wasm modules.

### Miscellaneous

- Build: `make build` now builds without errors (by disabling Wasm) on darwin/arm64 (M1)
- Various dependency bumps.
  - OpenTelemetry SDK: 1.4.1
  - github.com/prometheus/client_golang: 1.12.1

### Backwards incompatible changes

The JSON representation of the Status API's payloads -- both for `GET /v1/status`
responses and the metrics sent to a remote Status API endpoint -- have changed:

Previously, they had been serialized into JSON using the standard library "encoding/json"
methods. However, the metrics coming from the Prometheus integration are only available
in Golang structs generated from Protobuf definitions. For serializing these into JSON,
the standard library functions are unsuited:

- enums would be converted into numbers,
- field names would be `snake_case`, not `camelCase`,
- and NaNs would cause the encoder to panic.

Now, we're using the protobuf ecosystem's `jsonpb` package, to serialize the Prometheus
metrics into JSON in a way that is compliant with the Protobuf specification.

Concretely, what would before be
```
  "metrics": {
    "prometheus": {
      "go_gc_duration_seconds": {
        "help": "A summary of the GC invocation durations.",
        "metric": [
          {
            "summary": {
              "quantile": [
                {
                  "quantile": 0,
                  "value": 0.000011799
                },
                {
                  "quantile": 0.25,
                  "value": 0.000011905
                },
                {
                  "quantile": 0.5,
                  "value": 0.000040002
                },
                {
                  "quantile": 0.75,
                  "value": 0.000065238
                },
                {
                  "quantile": 1,
                  "value": 0.000104897
                }
              ],
              "sample_count": 7,
              "sample_sum": 0.000309117
            }
          }
        ],
        "name": "go_gc_duration_seconds",
        "type": 2
      },
```

is *now*:
```
  "metrics": {
    "prometheus": {
      "go_gc_duration_seconds": {
        "name": "go_gc_duration_seconds",
        "help": "A summary of the pause duration of garbage collection cycles.",
        "type": "SUMMARY",
        "metric": [
          {
            "summary": {
              "sampleCount": "1",
              "sampleSum": 4.1765e-05,
              "quantile": [
                {
                  "quantile": 0,
                  "value": 4.1765e-05
                },
                {
                  "quantile": 0.25,
                  "value": 4.1765e-05
                },
                {
                  "quantile": 0.5,
                  "value": 4.1765e-05
                },
                {
                  "quantile": 0.75,
                  "value": 4.1765e-05
                },
                {
                  "quantile": 1,
                  "value": 4.1765e-05
                }
              ]
            }
          }
        ]
      },
```

Note that `sample_count` is now `sampleCount`, and the `type` is using the enum's
string representation, `"SUMMARY"`, not `2`.

Note: For compatibility reasons (the Prometheus golang client doesn't use the V2
protobuf API), this change uses `jsonpb` and not `protojson`.

## 0.37.2

This is a bugfix release addressing two bugs:

1. A regression introduced in the formatter fix for CVE-2022-23628.
2. Support indices for appending to an array, conforming to JSON Patch (RFC6902)
   for patch bundles.

### Miscellaneous

- format: generated vars may have a proper location
- storage: Support index for array appends

## 0.37.1

This is a bug fix release that reverts the github.com/prometheus/client_golang
upgrade in v0.37.0. The upgrade exposed an issue in the serialization of Go
runtime metrics in the Status API
([#4319](https://github.com/open-policy-agent/opa/issues/4319)).

### Miscellaneous

- Revert "build(deps): bump github.com/prometheus/client_golang (#4307)"

## 0.37.0

This release contains a number of fixes and enhancements.

This is the first release that includes a binary and a docker image for
`linux/arm64`, `opa_linux_arm64_static` and `openpolicyagent/opa:0.37.0-static`.
Thanks to @ngraef for contributing the build changes necessary.

### Strict Mode

There have been numerous possible checks in the compiler that fall into this category:

1. They would help avoid common mistakes; **but**
2. Introducing them would potentially break some uncommon, but legitimate use.

We've thus far refrained from introducing them. **Now**, a new "strict mode"
allows you to opt-in to these checks, and we encourage you to do so!

With *OPA 1.0*, they will become the new default behaviour.

For more details, [see the docs on _Compiler Strict Mode_](https://www.openpolicyagent.org/docs/v0.37.0/strict/).

### Delta Bundles

Delta bundles provide a more efficient way to make data changes by containing
*patches to data* instead of snapshots.
Using them together with [HTTP Long Polling](https://www.openpolicyagent.org/docs/v0.37.0/management-bundles/#http-long-polling),
you can propagate small changes to bundles without waiting for polling delays.

See [the documentation](https://www.openpolicyagent.org/docs/v0.37.0/management-bundles/#delta-bundles)
for more details.


### Tooling and Runtime

- Bundles bug fix: Roundtrip manifest before hashing to allow changing the manifest
  and still using signature verification of bundles ([#4233](https://github.com/open-policy-agent/opa/issues/4233)),
  reported by @CristianJena

- The test runner now also supports custom builtins, when invoked through the Golang
  interface (authored by @MIA-Deltat1995)

- The compile package and the `opa build` command support a new output format: "plan".
  It represents a _query plan_, steps needed to take to evaluate a query (with policies).
  The plan format is a JSON encoding of the intermediate representation (IR) used for
  compiling queries and policies into Wasm.

  When calling `opa build -t plan ...`, the plan can be found in `plan.json` at the top-
  level directory of the resulting bundle.tar.gz.
  [See the documentation for details.](https://www.openpolicyagent.org/docs/v0.37.0/ir/).

- Compiler+Bundles: Metadata to be added to a bundle's manifest can now be provided via `WithMetadata`
  ([#4289](https://github.com/open-policy-agent/opa/issues/4289)), authored by @marensws, reported by @johanneslarsson
- Plugins: failures in auth plugin resolution are now output, previously panicked, authored by @jcchavezs
- Plugins: Fix error when initializing empty decision logging or status plugin ([#4291](https://github.com/open-policy-agent/opa/issues/4291))
- Bundles: Persisted bundle activation failures are treated like failures with
  non-persisted bundles ([#3840](https://github.com/open-policy-agent/opa/issues/3840)), reported by @dsoguet
- Server: `http.send` caching now works in system policy `system.authz` ([#3946](https://github.com/open-policy-agent/opa/issues/3946)),
  reported by @amrap030.
- Runtime: Apply credentials masking on `opa.runtime().config` ([#4159](https://github.com/open-policy-agent/opa/issues/4159))
- `opa test`: removing deprecated code for `--show-failure-line` (`-l`), authored by @damienjburks
- `opa eval`: add description to all output formats
- `opa inspect`: unhide command for [bundle inspection](https://www.openpolicyagent.org/docs/v0.37.0/cli/#opa-inspect)

### Rego and Topdown

Built-in function enhancements and fixes:

- `object.union_n`: New built-in for creating the union of more than two objects ([#4012](https://github.com/open-policy-agent/opa/issues/4012)),
  reported by @eliw00d
- `graph.reachable_paths`: New built-in to calculate the set of reachable paths in a graph (authored by @justinlindh-wf)
- `indexof_n`: New built-in function to get all the indexes of a specific substring (or character) from a string (authored by @shuheiktgw)
- `indexof`: Improved performance (authored by @shuheiktgw)
- `object.get`: Support nested key array for deeper lookups with default (authored by @charlieegan3)
- `json.is_valid`: Use Golang's `json.Valid` to avoid unnecessary allocations (authored by @kristiansvalland)

Strict-mode features:

- Add _duplicate imports_ check ([#2698](https://github.com/open-policy-agent/opa/issues/2698)) reported by @mikol
- _Deprecate_ `any()` and `all()` built-in functions ([#2437](https://github.com/open-policy-agent/opa/issues/2437))
- Make `input` and `data` reserved keywords ([#2600](https://github.com/open-policy-agent/opa/issues/2600)) reported by @jpeach
- Add _unused local assignment_ check ([#2514](https://github.com/open-policy-agent/opa/issues/2514))


Miscellaneous fixes and enhancements:

- `format`: don't group iterable when one has defaulted location
- `topdown`: ability to retrieve input and plug bindings in the `Event`, authored by @istalker2
- `print()` built-in: fix bug when used with `with` modifier and a function call value ([#4227](https://github.com/open-policy-agent/opa/issues/4227))
- `ast`: don't error when future keyword import is redundant during parsing

### Documentation

- A [new "CLI" docs section](https://www.openpolicyagent.org/docs/v0.37.0/cli/) describes the various
  OPA CLI commands and their arguments ([#3915](https://github.com/open-policy-agent/opa/issues/3915))
- Policy Testing: Add reference to rule indexing in the context of test code coverage
  ([#4170](https://github.com/open-policy-agent/opa/issues/4170)), reported by @ekcs
- Management: Add hint that S3 regional endpoint should be used with bundles (authored by @danoliver1)
- Many broken links were fixed, thanks to @phelewski
- Fix rendering of details: add detail-tab for collapsable markdown (authored by @bugg123)

### WebAssembly

- Add native support for `json.is_valid` built-in function
  ([#4140](https://github.com/open-policy-agent/opa/issues/4140)), authored by @kristiansvalland
- Dependencies: bump wasmtime-go from 0.32.0 to 0.33.1

### Miscellaneous

- Publish multi-arch image manifest lists including linux/arm64 ([#2233](https://github.com/open-policy-agent/opa/issues/2233)),
  authored by @ngraef, reported by @povilasv
- `logging`: Remove logger `GetFields` function ([#4114](https://github.com/open-policy-agent/opa/issues/4114)),
  authored by @viovanov
- Website: add versioned docs for latest version, so when 0.37.0 is released, both
  https://www.openpolicyagent.org/docs/v0.37.0/ and https://www.openpolicyagent.org/docs/latest
  contain docs, and 0.37.0 can already be used for stable links to versioned docs pages.
- Community: Initial draft of the community badges program
- `make test`: fix "too many open files" issue on Mac OS
- Various dependency bumps

## 0.36.1

This release includes a number of documentation fixes.
It also includes the experimental binary for darwin/arm64.

There are no code changes.

### Documentation

- OpenTelemetry: fix configuration example, authored by @rvalkenaers
- Configuration: fix typo for `tls-cert-refresh-period`, authored by @mattmahn
- SSH and Sudo authorization: Add missing filename
- Integration: fix example policy

### Release

- Build darwin/arm64 in post tag workflow

## 0.36.0

This release contains a number of fixes and enhancements.

### OpenTelemetry and opa exec

This release adds OpenTelemetry support to OPA. This makes it possible to emit spans to an OpenTelemetry collector via
gRPC on both incoming and outgoing (i.e. http.send) calls in the server. See the updated docs on
[monitoring](https://www.openpolicyagent.org/docs/latest/monitoring/) for more information and configuration options
([#1469](https://github.com/open-policy-agent/opa/issues/1469)) authored by @[rvalkenaers](https://github.com/rvalkenaers)

This release also adds a new `opa exec` command for doing one-off evaluations of policy against input similar to
`opa eval`, but using the full capabilities of the server (config file, plugins, etc). This is particularly useful in
contexts such as CI/CD or when enforcing policy for infrastructure as code, where one might want to run OPA with remote
bundles and decision logs but without having a running server. See the updated docs on
[Terraform](https://www.openpolicyagent.org/docs/latest/terraform/) for an example use case.
([#3525](https://github.com/open-policy-agent/opa/issues/3525))

### Built-in Functions

- Four new functions for working with HMAC (`crypto.hmac.md5`, `crypto.hmac.sha1`, `crypto.hmac.sha256`, and `crypto.hmac.sha512`) was added ([#1740](https://github.com/open-policy-agent/opa/issues/1740)) reported by @[jshaw86](https://github.com/jshaw86)
- `array.reverse(array)` and `strings.reverse(string)` was added for reversing arrays and strings ([#3736](https://github.com/open-policy-agent/opa/issues/3736)) authored by @[kristiansvalland](https://github.com/kristiansvalland) and @[olamiko](https://github.com/olamiko)
- The `http.send` built-in function now uses a metric for counting inter-query cache hits ([#4023](https://github.com/open-policy-agent/opa/issues/4023)) authored by @[mirayadav](https://github.com/mirayadav)
- An overflow issue with dates very far in the future has been fixed in the `time.*` built-in functions ([#4098](https://github.com/open-policy-agent/opa/issues/4098)) reported by @[morgante](https://github.com/morgante)

### Tooling

- A problem with future keyword import of `in` was fixed for `opa fmt` ([#4111](https://github.com/open-policy-agent/opa/issues/4111)) reported by @[keshavprasadms](https://github.com/keshavprasadms)
- An issue with `opa fmt` when refs contained operators was fixed (authored by @[jaspervdj-luminal](https://github.com/jaspervdj-luminal))
- Fix file renaming check in optimization using `opa build` (authored by @[davidmarne-wf](https://github.com/davidmarne-wf))
- The `allow_net` capability was added, allowing setting limits on what hosts can be reached in built-ins like `http.send` and `net.lookup_ip_addr` ([#3665](https://github.com/open-policy-agent/opa/issues/3665))

### Server

- A new credential provider for AWS credential files was added ([#2786](https://github.com/open-policy-agent/opa/issues/2786)) reported by @[rgueldem](https://github.com/rgueldem)
- The new `--tls-cert-refresh-period` flag can now be provided to `opa run`. If used with a positive duration, such as "5m" (5 minutes),
  "24h", etc, the server will track the certificate and key files' contents. When their content changes, the certificates will be
  reloaded ([#2500](https://github.com/open-policy-agent/opa/issues/2500)) reported by @[patoarvizu](https://github.com/patoarvizu)
- A new `v1/status` endpoint was added, providing the same data as the status plugin would send to a remote endpoint ([#4089](https://github.com/open-policy-agent/opa/issues/4089))
- The HTTP router of OPA is now exposed to the plugin manager ([#2777](https://github.com/open-policy-agent/opa/issues/2777)) authored by @[bhoriuchi](https://github.com/bhoriuchi) reported by @[mneil](https://github.com/mneil)
- Calling `print` now works in decision masking policies
- An unintended switch between long/regular polling on 304 HTTP status was fixed ([#3923](https://github.com/open-policy-agent/opa/issues/3923)) authored by @[floriangasc](https://github.com/floriangasc)
- The error message about prohibited config in the discovery plugin has been improved
- The discovery plugin no longer panics in Trigger() if downloader is nil
- The bundle plugin now ignores service errors for file:// resources
- The bundle plugin file loader was updated to support directories
- A timer to HTTP request was added to the downloader
- The requested_by field in the logging plugin is now optional

### Rego

- The error message raised when using `-` with a number and a set is now more specific (as opposed to the correct usage with two sets, or two numbers) ([#1643](https://github.com/open-policy-agent/opa/issues/1643))
- Fixed an edge case when using print and arrays in unification ([#4078](https://github.com/open-policy-agent/opa/issues/4078))
- Improved performance of some array operations by caching an array's groundness bit ([#3679](https://github.com/open-policy-agent/opa/issues/3679))
- ⚠️ Stricter check of arity in undefined function stage ([#4054](https://github.com/open-policy-agent/opa/issues/4054)).
  This change will fail evaluation in some unusual cases where it previously would succeed, but these policies should be very uncommon.

  An example policy that previously would succeed but no longer will (wrong arity):

```rego
package policy

default p = false
p {
    x := is_blue()
    input.bar[x]
}

is_blue(fruit) = y { # doesn't use fruit
    y := input.foo
}
```

### SDK

- The `opa.runtime()` built-in is now made available to the SDK ([#4050](https://github.com/open-policy-agent/opa/issues/4050) authored by @[oren-zohar](https://github.com/oren-zohar) and @[cmschuetz](https://github.com/cmschuetz)
- Plugins are now exposed on the SDK object
- The SDK now supports graceful shutdown ([#3980](https://github.com/open-policy-agent/opa/issues/3980)) reported by @[brianchhun-chime](https://github.com/brianchhun-chime)
- `print` output is now sent to the configured logger

### Website and Documentation

- All pages in the docs now have a feedback button ([#3664](https://github.com/open-policy-agent/opa/issues/3664)) authored by @[alan-ma](https://github.com/alan-ma)
- The Kafka docs have been updated to use the new Kafka plugin, and to use the OPA management APIs
- The Terraform tutorial was updated to use `opa exec` ([#3965](https://github.com/open-policy-agent/opa/issues/3965))
- The docs on Contributing as well as the Vendor Guidelines have been updated
- The term "whitelist" has been replaced by "allowlist" across the docs
- A simple destructuring assignment example was added to the docs
- The docs have been reviewed on the use of assignment, equality and comparison operators, to make sure they follow best practice

### CI

- SHA256 checksums of CI builds now published to release directory ([#3448](https://github.com/open-policy-agent/opa/issues/3448)) authored by @[johanneslarsson](https://github.com/johanneslarsson) reported by @[raesene](https://github.com/raesene)
- golangci-lint upgraded to v1.43.0 (authored by @[shuheiktgw](https://github.com/shuheiktgw))
- The build now creates an executable for darwin/arm64. This should work as expected, but is currently tested in the CI pipeline like the other binaries
- PRs targeting the [ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/) page are now checked for mistakes using Rego policies

## 0.35.0

This release contains a number of fixes and enhancements.

### Early Exit Optimization

This release adds an early exit optimization to the evaluator. With this optimization, the evaluator stops evaluating rules when an answer has been found and subsequent evaluation would not yield any new answers. The optimization is automatically applied to complete rules and functions that meet specific requirements. For more information see the [Early Exit in Rule Evaluation](https://www.openpolicyagent.org/docs/latest/policy-performance/#early-exit-in-rule-evaluation) section in the docs. [#2092](https://github.com/open-policy-agent/opa/issues/2092)

### Built-in Functions

- The `net.lookup_ip_addr` function was added to allow policies to resolve hostnames to IPv4/IPv6 addresses ([#3993](https://github.com/open-policy-agent/opa/issues/3993))
- The `http.send` function has been improved to close TCP connections quickly after receiving the HTTP response and avoid creating HTTP clients unnecessarily when a cached response exists ([#4015](https://github.com/open-policy-agent/opa/issues/4015)). This change reduces the number of open file descriptors required in high-throughput environments and prevents OPA from encountering ulimit errors.

### Rego

- `print()` calls in the head of rules no longer cause runtime errors ([#3967](https://github.com/open-policy-agent/opa/issues/3967))
- Type errors for calls to undefined functions no longer contain rewritten variable names ([#4031](https://github.com/open-policy-agent/opa/issues/4031))
- The `rego.SkipPartialNamespace` option now correctly sets the flag on the partial evaluation queries (previously it would always set the value to `true`) ([#3996](https://github.com/open-policy-agent/opa/issues/3996)) authored by @[thomascoquet](https://github.com/thomascoquet)
- The internal set implementation has been updated to insert elements in sorted order rather than lazily sorting during comparisons.
- Fixed `import` alias parsing bug identified by fuzzer ([#3988](https://github.com/open-policy-agent/opa/issues/3988))

### WebAssembly

- The Golang SDK will now issue a `grow()` call if the `input` document exceeds the available memory space.
- The `malloc()` implementation will now call `opa_abort` if the `grow()` call fails.

### Server

- The decision logger adapts upload chunk sizes based on previous outputs. This allows the decision loggger to encode significantly more decisions into each upload chunk, thereby reducing heap usage for buffered decisions. For more information on the adapative chunking behaviour, see the [Decision Logs](https://www.openpolicyagent.org/docs/latest/management-decision-logs/) page in the docs.
- The decision logger can be configured to send records to a custom plugin as well as an HTTP endpoint at the same time ([#4013](https://github.com/open-policy-agent/opa/issues/4013))
- `print()` calls from the `system.authz` policy are now included in the logs ([#4048](https://github.com/open-policy-agent/opa/issues/4048))
- OPA can use an [Azure Managed Identities Token](https://www.openpolicyagent.org/docs/latest/configuration/#azure-managed-identities-token) to authenticate with control plane services ([#3916](https://github.com/open-policy-agent/opa/issues/3916)) authored by @[Scowluga](https://github.com/Scowluga).
- The logging configuration will be correctly applied to service clients so that DEBUG logs are surfaced ([#4071](https://github.com/open-policy-agent/opa/issues/4071))

### Tooling

- The `opa fmt` command will not generate a line-break when there are generated variables in a function call ([#4018](https://github.com/open-policy-agent/opa/issues/4018)) reported by @[torsrex](https://github.com/torsrex)
- The `opa inspect` command no longer prints a blank namespace when a data.json file is included at the root ([#4022](https://github.com/open-policy-agent/opa/issues/4022))
- The `opa build` command will output debug messages if an optimized entrypoint is discarded.

### Website and Documentation

- The website has been updated to build with Hugo 0.88.1 ([#3787](https://github.com/open-policy-agent/opa/issues/3787))
- The version picker in the documentation is now scrollable ([#3955](https://github.com/open-policy-agent/opa/issues/3955)) authored by @[orweis](https://github.com/orweis)
- The description of the `urlquery` built-in functions have been clarified ([#1592](https://github.com/open-policy-agent/opa/issues/1592)) reported by @[klarose](https://github.com/klarose)
- The decision logger documentation has been improved to cover controls for large-scale environments ([#3976](https://github.com/open-policy-agent/opa/issues/3976))
- The "strict built-in errors" mode is now covered in the docs along with built-in function error behaviour ([#3686](https://github.com/open-policy-agent/opa/issues/3686))
- The OAuth2 and OIDC examples around key rotation and caching have been improved

### CI

- Issues and PRs that have not seen activity in 30 days will be automatically marked as "inactive"
- The `Makefile` can now produce Docker images for other architectures. We do not yet publish binaries or images for non-amd64 architectures however if you want to build OPA yourself, the `Makefile` does not prohibit it.

### Backwards Compatibility

- The diagnostics buffer in the OPA server has been completely removed as part of the deprecation and removal of the diagnostic feature ([#1052](https://github.com/open-policy-agent/opa/issues/1052))

## 0.34.2

### Fixes

- ast: Fix print call rewriting for calls in head ([#3967](https://github.com/open-policy-agent/opa/issues/3967))

## 0.34.1

### Fixes

- runtime: Fix logging configuration (#3959) ([#3958](https://github.com/open-policy-agent/opa/issues/3958))

## 0.34.0

This release includes a number of enhancements and fixes. In particular, this
release adds a new keyword for membership and iteration (`in`) and a specialized
built-in function (`print`) for debugging.

### The `in` operator

This release adds a new `in` operator that provides syntactic sugar for
references that perform membership tests or iteration on collections (i.e.,
arrays, sets, and objects.) The following table shows common patterns for arrays
with the old and new syntax:

Pattern | Existing Syntax | New Syntax
--- | --- | ---
Check if 7 exists in array | `7 == arr[_]` | `7 in arr`
Check if 7 does not exist in array | n/a (requires helper rule) | `not 7 in arr`
Iterate over the elements of array | `x := arr[_]` | `some x in arr`

For more information on the `in` operator see [Membership and iteration:
`in`](https://www.openpolicyagent.org/docs/edge/policy-language/#membership-and-iteration-in)
in the docs.

### The `print` function

This release adds a new `print` function for debugging purposes. The `print`
function can be used to output any value inside of the policy. The `print`
function has special handling for _undefined_ values so that execution does not
stop if any of the operands are undefined. Instead, a special marker is emitted
in the output. For example:

```rego
package example

default allow = false

allow {
  print("the subject's username is:", input.subject.username)
  input.subject.username == "admin"
}
```

Given the policy above, we can see the output of the `print` function via STDERR when using `opa eval`:

```bash
echo '{"subject": {"username": "admin"}}' | opa eval -d policy.rego -I -f pretty 'data.example.allow'
```

Output:

```
the subject's username is: admin
true
```

If the username, subject, or entire input document was undefined, the `print` function will still execute:

```bash
echo '{}' | opa eval -d policy.rego -I -f pretty 'data.example.allow'
```

Output:

```
the subject's username is: <undefined>
false
```

The `print` function is integrated into the `opa` subcommands, REPL, server, VS
Code extension, and the playground. Library users must opt-in to `print`
statements. For more information see the
[Debugging](https://www.openpolicyagent.org/docs/edge/policy-reference/#debugging)
section in the docs.

### Enhancements

- SDK: Allow map of plugins to be passed to SDK ([#3826](https://github.com/open-policy-agent/opa/issues/3826)) authored by @[edpaget](https://github.com/edpaget)
- `opa test`: Change exit status when tests are skipped ([#3773](https://github.com/open-policy-agent/opa/issues/3773)) authored by @[kirk-patton](https://github.com/kirk-patton)
- Bundles: Improve loading performance ([#3860](https://github.com/open-policy-agent/opa/issues/3860)) authored by @[0xAP](https://github.com/0xAP)
- `opa fmt`: Keep new lines in between function arguments ([#3836](https://github.com/open-policy-agent/opa/issues/3836)) reported by @[anbrsap](https://github.com/anbrsap)
- `opa inspect`: Add experimental subcommand for bundle inspection ([#3754](https://github.com/open-policy-agent/opa/issues/3754))

### Fixes

- Bundles/API: When deleting a policy, the check determining if it's bundle-owned was using the path prefix, which would yield false positives under certain circumstances.
  It now checks the path properly, piece-by-piece. ([#3863](https://github.com/open-policy-agent/opa/issues/3863) authored by @[edpaget](https://github.com/edpaget)
- CLI: Using `--set` with null value _again_ translates to empty object ([#3846](https://github.com/open-policy-agent/opa/issues/3846))
- Rego: Forbid dynamic recursion with hidden (`system.*`) document ([#3876](https://github.com/open-policy-agent/opa/issues/3876)
- Rego: Raise conflict errors in functions when output not captured ([#3912](https://github.com/open-policy-agent/opa/issues/3912))

  This change has the potential to break policies that previously evaluated successfully!
  See _Backwards Compatibility_ notes below for details.
- Experimental disk storage: React to "txn too big" errors ([#3879](https://github.com/open-policy-agent/opa/issues/3879)), reported and authored by @[floriangasc](https://github.com/floriangasc)

### Documentation

- Kubernetes and Istio: Update tutorials for recent Kubernetes versions ([#3910](https://github.com/open-policy-agent/opa/issues/3910)) authored by @[olamiko](https://github.com/olamiko)
- Deployment: Add section about Capabilities ([#3769](https://github.com/open-policy-agent/opa/issues/3769))
- Built-in functions: Add warning to `http.send` and extension docs about side-effects in other systems (#3922) ([#3893](https://github.com/open-policy-agent/opa/issues/3893))
- Docker Authorization: The tutorial now uses a Bundles API server.
- SDK: An example of SDK use is provided.

### Miscellaneous

- Runtime: Refactor logger usage -- see below for *Backwards Compatibility* notes.
- Wasm: fix an issue with undefined, plain `input` references ([#3891](https://github.com/open-policy-agent/opa/issues/3891))
- test/e2e: Extend TestRuntime to avoid global fixture
- types: Fix Arity function to return zero when type is known (#3932)
- Wasm/builder: bump LLVM to 13.0.0, latest versions of wabt and binaryen (#3908)
- Wasm: deal with importing memory in the compiler (#3763)

### Backwards Compatibility

* Function return values need to be well-defined: for a single input `x`, the function's
  output `f(x)` can only be one value. When evaluating policies, this condition had not
  been ensured for function calls that don't make use of their values, like

  ```rego
  package p
  r {
      f(1)
  }
  f(_) = true
  f(_) = false
  ```

  Before, `data.p.r` evaluated to `true`. Now, it will (correctly) return an error:

      eval_conflict_error: functions must not produce multiple outputs for same inputs

  In more realistic settings, this can be encountered when true/false return values
  are captured and returned where they don't need to be:

  ```rego
  package p
  r {
      f("any", "baz")
  }
  f(path, _) = r {
      r := path == "any"
  }
  f(path, x) = r {
      r := glob.match(path, ["/"], x)
  }
  ```

  In this example, any function input containing `"any"` would make the function yield
  two different results:

  1. The first function body returns `true`, matching the `"any"` argument.
  2. The second function body returns the result of the `glob.match` call -- `false`.

  The fix here would be to _not_ capture the return value in the function bodies:

  ```rego
  f(path, _) {
      path == "any"
  }
  f(path, x) {
      glob.match(path, ["/"], x)
  }
  ```

* The `github.com/open-policy-agent/opa/runtime#NewLoggingHandler` function now
  requires a logger instance. Requiring the logger avoids the need for the
  logging handler to depend on the global logrus logger (which is useful for
  test purposes.) This change is unlikely to affect users.

## 0.33.1

This is a bugfix release addressing an issue in the formatting of rego code that contains
object literals. With the last release, those objects would under some conditions have their
keys re-ordered, with some of them put into a single line.

Thanks to @[iainmcgin](https://github.com/iainmcgin) for reporting.

### Fixes

- format: make groupIterable sort by row ([#3849](https://github.com/open-policy-agent/opa/issues/3849))

## 0.33.0

This release includes a number of improvements and fixes.

### Built-in Functions

This release introduces `crypto.x509.parse_rsa_private_key` so that policy authors can decode RSA private keys and structure them as JWKs ([#3765](https://github.com/open-policy-agent/opa/issues/3765)). Authored by @[cris-he](https://github.com/cris-he).

### Fixes

- Fix object comparison to avoid sorting keys in-place. This prevents the interpreter from generating non-deterministic results when values are inserted into the partial set memoization cache. ([#3819](https://github.com/open-policy-agent/opa/issues/3819))
- Fix data races in `ast` package caused by sorting `types.Any` instances in-place and shallow-copying module comments when a deep-copy should be performed ([#3793](https://github.com/open-policy-agent/opa/issues/3793)). Reported by @[markushinz](https://github.com/markushinz).
- Fix "file name too long" error caused by bundle loader treating PEM encoded private keys as file paths ([#3766](https://github.com/open-policy-agent/opa/issues/3766))
- Fix plugins to support manual triggering mode when discovery is disabled ([#3797](https://github.com/open-policy-agent/opa/issues/3797))

### Server & Tooling

- The server now supports policy-based health checks that can inspect the state of plugins and other internal components ([#3759](https://github.com/open-policy-agent/opa/issues/3759)) authored by @[gshively11](https://github.com/gshively11)
- The bundle reader now loads files lazily to avoid hitting file descriptor limits ([#3777](https://github.com/open-policy-agent/opa/issues/3777)). Authored by @[bhoriuchi](https://github.com/bhoriuchi)
- The `opa eval` sub-command supports a `--timeout` option for limiting how long evaluation can run.

### Rego

- The type checker now supports variadic arguments on void functions. This change paves the way for `print()` support as well as variadic arguments on all functions.
- The parser now memoizes term parsing. This prevents non-linear runtime for large nested objects and sets.

### CI & Dependencies

- Fix spurious build errors in wasm library.
- Update wasmtime dependency to v0.30.0.
- Run PR checks on macOS in addition to Linux ([#3176](https://github.com/open-policy-agent/opa/issues/3176)).

### Documentation

- Update the Kubernetes and Envoy (standalone) tutorials to show how the OPA management APIs can be used to distribute policies.

### Backwards Compatibility

* The `github.com/open-policy-agent/opa/ast#ArgErrDetail` struct has been
  modified to use the new `types.FuncArgs` struct to represent the required
  arguments. Callers that depend on the exact structure of the error details
  must update to use the `types.FuncArgs` struct.

## 0.32.1

This is a bugfix release to address a problem related to mismatching checksums in the official go mod proxy.
As a consequence, users with code depending on the OPA Go module that bypassed the proxy would see an error like

    go get github.com/google/flatbuffers/go: github.com/google/flatbuffers@v1.12.0: verifying module: checksum mismatch
        downloaded: h1:N8EguYFm2wwdpoNcpchQY0tPs85vOJkboFb2dPxmixo=
        sum.golang.org: h1:/PtAHvnBY4Kqnx/xCQ3OIV9uYcSFGScBsWI3Oogeh6w=

**Be aware** that Github's Dependabot feature makes use of that check, and will start to _fail_ for projects using the OPA Go module version 0.32.0.

There workaround applied to OPA is to replace to flatbuffers dependency's version manually.

For more information, see
- https://github.com/google/flatbuffers/issues/6466: The issue has been discussed upstream, and a 1.12.1 release has been published to address it.
- https://github.com/dgraph-io/badger/pull/1746: OPA transitively depends on the flatbuffer package because of badger.

There are *no functional changes* in this bugfix release.
If you use the container images, or the published binaries, of OPA 0.32.0, you are **not affected** by this.

Many thanks to [James Alseth](https://github.com/jalseth) for triaging this, and engaging with upstream to fix this.

## 0.32.0

This release includes a number of improvements and fixes.

### 💾 Disk-based Storage (Experimental)

This release adds a disk-based storage implementation to OPA. The implementation can be found in [github.com/open-policy-agent/storage/disk](https://pkg.go.dev/github.com/open-policy-agent/opa/storage/disk). There is also an example in the [`rego` package](https://pkg.go.dev/github.com/open-policy-agent/opa/rego#pkg-examples) that shows how policies can be evaluated with the disk-based store. The disk-based store is currently only available as a library (i.e., it is not integrated into the rest of OPA yet.) In the next few releases, we are planning to integrate the implementation into the OPA server and provide tooling to help leverage the disk-based store.

### Built-in Functions

This release includes a few improvements to existing built-in functions:

- The `http.send` function now supports UNIX domain sockets ([#3661](https://github.com/open-policy-agent/opa/issues/3661)) authored by @[kirk-patton](https://github.com/kirk-patton)
- The `units.parse_bytes` function now supports E* and P* units ([#2911](https://github.com/open-policy-agent/opa/issues/2911))
- The `io.jwt.encode_sign` function uses the built-in context randomization source (which is helpful for replay purposes)

### Server

This release includes multiple improvements for OPA server deployments in serverless environments:

- Plugins can now be triggered manually within OPA. This feature allows users extending and customizing OPA to control exactly when operations like bundle downloads and decision log uploads occur. The built-in plugins now include a `trigger` configuration that can be set to `manual` or `periodic` (which is the default). When `manual` triggering is enabled, the plugins WILL NOT perform any periodic/background operations. Instead, the plugins will only execute when the [`Trigger`](https://github.com/open-policy-agent/opa/blob/main/plugins/plugins.go#L101) API is invoked.
- Plugins can now wait for server initialization. When runtime initialization is finished, plugins can be notified. This allows plugins to synchronize their behaviour with server startup. [#3701](https://github.com/open-policy-agent/opa/issues/3701) authored by @[gshively11](https://github.com/gshively11).
- The [Health API](https://www.openpolicyagent.org/docs/latest/rest-api/#health-api) now supports an `exclude-plugin` parameter to control which plugins are checked. [#3713](https://github.com/open-policy-agent/opa/issues/3713) authored by @[gshively11](https://github.com/gshively11).

### Tooling

- The compiler no longer fetches remote schemas by default when used as as library. Capabilities have been updated to include an `allow_net` field to control whether network operations can be performed ([#3746](https://github.com/open-policy-agent/opa/issues/3746)). This field is only used to control schema fetching today. In future versions of OPA, the `allow_net` parameter will be used to control other behaviour like `http.send`.
- The `WebAssembly runtime not supported` error message has been improved [#3739](https://github.com/open-policy-agent/opa/pull/3739).

### Rego

- Added support for `anyOf` and `allOf` keywords in JSON schema support in the type checker ([#3592](https://github.com/open-policy-agent/opa/issues/3592)) authored by [@jchen10500](https://github.com/jchen10500) and [@juliafriedman8](https://github.com/juliafriedman8).
- Added support for custom JSON result marshalling in the `rego` package.
- Added a new convenience function (`Allowed() bool`) to the `rego.ResultSet` API.
- Improved string-representation construction performance for arrays, sets, and objects.
- Improved the topdown evaluator to support `ast.Value` results from the store so that unnecessary conversions can be avoided.
- Improved the `rego` package to make the wasmtime-go dependency optional at build-time ([#3545](https://github.com/open-policy-agent/opa/issues/3545)).
- Fixed a bug in the comprehension indexer whereby index keys were not constructed correctly leading to incorrect outputs ([#3579](https://github.com/open-policy-agent/opa/issues/3579)).
- Fixed a stack overflow during partial evaluation due to incorrect term rewriting in the copy propagation implementation ([#3071](https://github.com/open-policy-agent/opa/issues/3071)).
- Fixed a bug in partial evaluation when shallow inlinign is enabled that resulted in built-in functions being invoked instead of saved ([#3681](https://github.com/open-policy-agent/opa/issues/3681)).

### WebAssembly

- The internal Wasm SDK now supports the inter-query built-in cache.
- The pre-compiled runtime is now built with llvm 12.0.1 and the builder image includes clang-format.
- The internal Wasm SDK has been updated to use wasmtime-go v0.29.0.

### Documentation

This release includes a number of documentation improvements:

- The wasm `opa_eval` arguments have been clarified [#3699](https://github.com/open-policy-agent/opa/issues/3696)
- The contributing and development guide have been moved into a dedicated [Contributing](https://www.openpolicyagent.org/docs/latest/contributing/) section on the website [#3751](https://github.com/open-policy-agent/opa/issues/3751)
- The Envoy standalone tutorial includes cleanup steps now (thanks [@princespaghetti](https://github.com/princespaghetti))
- Various typos have been fixed by multiple folks (thanks [@Tej-Singh-Rana](https://github.com/Tej-Singh-Rana) [@gujun4990](https://github.com/gujun4990))
- The Kubernetes ingress validation tutorial has been updated to include new mandatory attributes and newer API versions (thanks [@ereslibre](https://github.com/ereslibre))
- The recommendations around using OPA Gatekeeper have been improved.

### Infrastructure

- OPA is now built with Go v1.17 and CI jobs have been added to ensure OPA builds with older versions of Go.

### Backwards Compatibility

The `rego` package no longer relies on build constraints to enable the Wasm runtime. Instead, library users must opt-in to Wasm runtime support by adding an import statement in the Go code:

```go
import _ "github.com/open-policy-agent/opa/features/wasm"
```

This change ensures that (by default) the wasmtime-go blobs are not vendored in projects that embed OPA as a library. If you are currently relying on the Wasm runtime support in the `rego` package (via the `rego.Target("wasm")` option), please update you code to include the import above. See [#3545](https://github.com/open-policy-agent/opa/issues/3545) for more details.

## 0.31.0

This release contains **performance improvements** for evaluating partial sets and objects,
and introduces a new ABI call to OPA's Wasm modules to speed up Wasm evaluations.

It also comes with an improvement for checking policies -- unsafe declared variables are now caught at compile time.
This means that **some policies** that have been working fine with previous versions, because their unsafe variables
had not ever been queried, will fail to compile with OPA 0.31.0.
See below for details and what to do about that.

### Spotlights

#### Partial Sets and Objects Performance

Resolving an issue ([#822](https://github.com/open-policy-agent/opa/issues/822)) created on July 4th 2018,
OPA can now cache the results of partial sets and partial objects.

A benchmark that accesses a partial set of increasing size _twice_ shows a saving of more than 50%:

    name                             old time/op    new time/op    delta
    PartialRuleSetIteration/10-16       230µs ±10%     101µs ± 3%  -56.10%  (p=0.000 n=10+10)
    PartialRuleSetIteration/100-16     13.4ms ± 9%     5.5ms ± 9%  -58.74%  (p=0.000 n=10+9)
    PartialRuleSetIteration/1000-16     1.31s ±10%     0.51s ± 8%  -61.12%  (p=0.000 n=10+9)

    name                             old alloc/op   new alloc/op   delta
    PartialRuleSetIteration/10-16      77.7kB ± 0%    35.8kB ± 0%  -53.94%  (p=0.000 n=10+10)
    PartialRuleSetIteration/100-16     3.72MB ± 0%    1.29MB ± 0%  -65.26%  (p=0.000 n=10+10)
    PartialRuleSetIteration/1000-16     365MB ± 0%     114MB ± 0%  -68.86%  (p=0.000 n=10+10)

    name                             old allocs/op  new allocs/op  delta
    PartialRuleSetIteration/10-16       1.84k ± 0%     0.69k ± 0%  -62.42%  (p=0.000 n=10+10)
    PartialRuleSetIteration/100-16      99.3k ± 0%     14.5k ± 0%  -85.43%  (p=0.000 n=10+9)
    PartialRuleSetIteration/1000-16     10.0M ± 0%      1.0M ± 0%  -89.58%  (p=0.000 n=10+9)

These numbers were gathered querying `fixture[i]; fixture[j]` with a policy of

```rego
fixture[x] {
	x := numbers.range(1, n)[_]
}
```
where `n` is 10, 100, or 1000.

There are multiple access patterns that are accounted for: if a _ground_ scalar is used to
access a previously not-cached partial rule,

```rego
allow {
	managers[input.user] # here
}

managers[x] {
	# some logic here
}
```

the evaluation algorithm will calculate the set membership of `input.user` _only_, and cache the result.

If there is a query that requires evaluating the entire partial, however, the algorithm will also cache the entire partial:
```rego
allow {
	some person
	managers[person]
	# more expressions
}

managers[x] {
	# some logic here
}
```
thus avoiding extra evaluations later on.
The same is true if `managers` was used as a fully materialized set in an execution.


This also means that the question about whether to write

```rego
q = { x | ... } # set comprehension
```

or

```rego
q[x] { ... } # partial set rule
```

becomes much less important for policy evaluation performance.

#### WebAssembly Performance

OPA-generated Wasm modules have gotten a fast-path evaluation method:
By calling the one-off function

    opa_eval(reserved, entrypoint, data_addr, input_addr, input_len, format)

which returns a pointer to the serialized result set (in JSON if format is 0, "value" format if 1),
the number of VM calls needed for evaluating a policy via Wasm is drastically reduced.

The performance benefit is huge:

    name         old time/op    new time/op    delta
    WasmRego-16    84.3µs ± 6%    15.1µs ± 0%  -82.07%  (p=0.008 n=5+5)

The added `opa_eval` export comes with an ABI bump to version 1.2.
See [#3627](https://github.com/open-policy-agent/opa/pull/3627) for all details.

Along the same line, we've examined the processing of query evaluations that are Wasm-backed _through the `rego` package_.
This allowed us to avoid unneccessary work ([#3666](https://github.com/open-policy-agent/opa/issues/3666)).


#### Unsafe declared variables now cause a compile-time error

Before this release, local variables that had been _declared_, i.e. introduced via the `some` keyword, had been able
to slip through the safety checks unnoticed.

For example, a policy like

```rego
package demo

q {
	input == "open sesame"
}

p[x] {
	some x
}
```

would have _not_ caused any error **if `data.demo.p` wasn't queried**.
Querying `data.demo.p` would return an "var requires evaluation" error.

With this release, the erroneous rule no longer goes unnoticed, but is **caught at compile time**: "var x is unsafe".

The most likely fix is to remove the rule with the unsafe variable, since it cannot have contributed to a successful
evaluation in previous OPA versions.

See [#3580](https://github.com/open-policy-agent/opa/issues/3580) for details.

### Topdown and Rego

- New built-in function: `crypto.x509.parse_and_verify_certificates` ([#3601](https://github.com/open-policy-agent/opa/issues/3601)), authored by @[jalseth](https://github.com/jalseth)

  This function enables you to verify that there is a chain from a leaf certificate back to the trusted root.
- New built-in function: `rand.intn` generates a random number between `0` and `n` ([#3615](https://github.com/open-policy-agent/opa/issues/3615)), authored by @[base698](https://github.com/base698)

  The function takes a string argument to ensure that the same call, within one policy evaluation, returns the same random number.
- `http.send` enhancement: New `caching_mode` parameter to configure if deserialized or serialized response bodies should be cached ([#3599](https://github.com/open-policy-agent/opa/issues/3599))
- Custom built-in function enhancement: let custom builtins halt evaluation ([#3534](https://github.com/open-policy-agent/opa/issues/3534))
- Partial evaluation: Fix stack overflow on certain expressions ([#3559](https://github.com/open-policy-agent/opa/issues/3559))

### Tooling

- Query Profiling: `opa eval --profile` now supports a `--count=#` flag to gather metrics and profiling data over multiple runs, and displays aggregate statistics for the results ([#3651](https://github.com/open-policy-agent/opa/issues/3651)).

  This allows you to gather more robust numbers to assess policy performance.

- Docker images: Publish static image ([#3633](https://github.com/open-policy-agent/opa/issues/3633))

  As of this release, you can use the staticly-built Linux binary from a docker image: `openpolicyagent/opa:0.31.0-static`.
  It contains the same binary that has been published since release v0.29.4, statically linked to musl, with evaluating Wasm disabled.

### Fixes

- Built-in `http.send`: ignore `tls_use_system_certs` setting on Windows. Having this set to _true_ (the default as of v0.29.0) would _always_ return an error on Windows.
- The console decision logger is no longer tied to the general log level ([#3654](https://github.com/open-policy-agent/opa/issues/3654))
- Update query compiler to reject empty queries ([#3625](https://github.com/open-policy-agent/opa/issues/3625))
- Partial Evaluation fix: Don't generate comprehension with unsafe variables ([#3557](https://github.com/open-policy-agent/opa/issues/3557))
- Parser: modules containing _only_ tabs and spaces no longer lead to a runtime panic.
- Wasm: ensure that the desired stack space for the C library calls (64KiB) is not reduced by data segments added in the compiler.
   This is achieved by putting the stack first -- stack overflows now become "out of bounds" memory access traps.
   Before, it would silently corrupt the static data.

### Server and Runtime

- New configuration for Management APIs: using `resource`, the request path for sending decision logs can be configured now ([#3618](https://github.com/open-policy-agent/opa/issues/3618)), authored by @[cbuto](https://github.com/cbuto)

  `/logs` is still the default, but can now be overridden.
  With this change, the `partition_name` config becomes deprecated, since its functionality is subsumed by this new configurable.

### Documentation

- How to debug? Clarify how to access `Note` events for debugging via explanations ([#3628](https://github.com/open-policy-agent/opa/issues/3628)) authored by @[enori](https://github.com/enori)
- Clarify special characters for key, i.e. what `x["y"]` is necessary because `x.y` isn't valid ([#3638](https://github.com/open-policy-agent/opa/issues/3638)) authored by @[Hongbo-Miao](https://github.com/Hongbo-Miao)
- Management APIs: Remove deprecated fields from docs
- Policy Reference: add missing backtick; `type_name` builtin is natively implemented in Wasm

## 0.30.2

This is a bugfix release that modifies the AWS credential provider to use POST
instead of GET for retrieving AWS STS tokens. The GET method can leak
credentials into the debug log if the AWS STS endpoint is unavailable.

## 0.30.1

This is a bugfix release to correct the behaviour of the `indexof` builtin ([#3606](https://github.com/open-policy-agent/opa/issues/3606)).
In v0.30.0, it only checked the first character of the substring to be found: `indexof("foo", "fox")` erroneously returned 0 instead of -1.

### Miscellaneous

- wasm-sdk: Fix typo in non-wasm error message, authored by @[olivierlemasle](https://github.com/olivierlemasle)

## 0.30.0

This release contains a number of enhancements and fixes.

### Server and Runtime

- Support listening on abstract Unix Domain Sockets ([#3533](https://github.com/open-policy-agent/opa/issues/3533)) authored by @[amanymous-net](https://github.com/amanymous-net)
- Support minimum TLS version configuration, default to 1.2 ([#3226](https://github.com/open-policy-agent/opa/issues/3226)) authored by @[kale-amruta](https://github.com/kale-amruta)
- Enhancement in REST Plugin: You can now specify a CA cert for remote services implementing the management APIs (bundles, status, decision logs, discovery) ([#1954](https://github.com/open-policy-agent/opa/issues/1954))
- Bugfix: treat missing/empty roots as owning all paths ([#3521](https://github.com/open-policy-agent/opa/issues/3521))

  Before, it would have been possible to overwrite a policy that was supplied by a bundle (with an empty manifest, or a manifest without declared roots), due to an erroneous check.
  This will now be forbidden, and return a 400 HTTP status, in accordance with the documentation.
- Extend POST v1/query endpoint to accept input, refactor index.html to use fetch()
- Bundle download: In case of download or activation errors, the cached Etag is reset to the last successful activation. Previously OPA would reset the cached Etag entirely, which could trigger unnecessary bundle downloads in edge-case scenarios.

### Tooling

- `opa build`: Do not write manifest if empty ([#3480](https://github.com/open-policy-agent/opa/issues/3480)). Under the hood, the manifest metadata is now included in the Equal() function's checks.
- `opa fmt`: Fix incorrect help text ([#3518](https://github.com/open-policy-agent/opa/issues/3518)) authored by @[andrehaland](https://github.com/andrehaland)
- `opa bench`: Do not print nil errors ([#3530](https://github.com/open-policy-agent/opa/issues/3530))

### Rego

- Expose random seeding in rego package ([#3560](https://github.com/open-policy-agent/opa/issues/3560))
- Enhance `ast.InterfaceToValue` to handle non-native types
- Enhance indexer to understand function args
- Enhance static property lookup of objects: Use binary search
- Fix PE unknown check to avoid saving unnecessarily ([#3552](https://github.com/open-policy-agent/opa/issues/3552))
- Fix inlining controls for functions ([#3463](https://github.com/open-policy-agent/opa/issues/3463))
- Fix (shallow) partial eval of ref to empty collection in presence of `with` statement ([#3420](https://github.com/open-policy-agent/opa/issues/3420))
- Fix cache value size checking during insert operation
- Fix `indexof` when using UTF-8 characters
- Fix `http.send` flaky test

#### Wasm

- SDK: update wasmtime-go to 0.28.0, authored by @[olivierlemasle](https://github.com/olivierlemasle)
- Bugfix: count() now counts invalid UTF-8 runes (previously aborted)
- Compiler: emit unreachable instruction after opa_abort()

### Miscellaneous

- `make check` now uses golangci-lint via docker, authored by @[willbeason](https://github.com/willbeason)
- The statically-built linux binary is properly used in the make targets that need it, and published to edge binaries.
- Built binaries are now smoke tested on Windows, macos, and Linux.
- Fix test failing with Go 1.17 rc in gojsonschema ([#3589](https://github.com/open-policy-agent/opa/issues/3589)) authored by @[olivierlemasle](https://github.com/olivierlemasle)
- Build: Bump Go version to 1.16.3 ([#3555](https://github.com/open-policy-agent/opa/issues/3555))
- CI: enable dependabot for wasmtime-go

#### Documentation

- OAuth2/OIDC: Fixed `concat` arguments in metadata discovery method ([#3543](https://github.com/open-policy-agent/opa/pull/3543), @[iggbom](https://github.com/iggbom))
- Policy Reference: syntax highlighting EBNF grammar (@[PatMyron](https://github.com/PatMyron))
- Extending OPA: fix typo (@[dxps](https://github.com/dxps))
- Extending OPA: marshal the decision log (@[TheLunaticScripter](https://github.com/TheLunaticScripter))
- Kubernetes Introduction: fix typo (@[dbaker-rh](https://github.com/dbaker-rh))
- Envoy: Add guidance for OPA-Envoy benchmarks
- Change default linux download to `opa_linux_amd64_static`

## 0.29.4

This is a bugfix release that re-introduces linux binaries that do not depend on glibc, i.e., run in unmodified Alpine Linux systems.

### Fixes

- build: add static (wasm-disabled) linux build (#3511) ([#3499](https://github.com/open-policy-agent/opa/issues/3499)) authored by @[srenatus](https://github.com/srenatus)

### Miscellaneous

- bundle: Implement a DirectoryLoader for fs.FS (#3493) ([#3489](https://github.com/open-policy-agent/opa/issues/3489)) authored by @[simongottschlag](https://github.com/simongottschlag)

## 0.29.3

This bugfix release addresses another edge case in function evaluation ([#3505](https://github.com/open-policy-agent/opa/pull/3505)).

## 0.29.2

This is a bugfix release to resolve an issue in topdown's function output caching ([#3501](https://github.com/open-policy-agent/opa/issues/3501))

## 0.29.1

This is a bugfix release to resolve an issue in the release pipeline.

## 0.29.0

This release contains a number of enhancements and fixes.

### SDK

- This release includes a new top-level package to support OPA integrations in Go programs: `github.com/open-policy-agent/opa/sdk`. Users that want to integrate OPA as a library in Go and expose features like bundles and decision logging should use this package. The package is controlled by specifying an OPA configuration file. Hot reloading is supported out-of-the-box. See the GoDoc for [the package docs](https://pkg.go.dev/github.com/open-policy-agent/opa@v0.29.0/sdk) for more details.

### Server

- A deadlock in the bundle plugin during shutdown has been resolved ([#3363](https://github.com/open-policy-agent/opa/issues/3363))
- An issue between bundle signing and bundle persistence when multiple data.json files are included in the bundle has been resolved ([#3472](https://github.com/open-policy-agent/opa/issues/3472))
- The `github.com/open-policy-agent/opa/runtime#Params` struct now supports a router parameter to enable custom routes on the HTTP server.
- The bundle manifest can now include an extra `metadata` key where arbitrary key-value pairs can be stored. Authored by @[viovanov](https://github.com/viovanov)
- The bundle plugin now supports file:// urls in the `resource` field for test purposes.
- The decision log plugin emits a clearer message at DEBUG instead of INFO when there is no work to do. Authored by [andrewbanchich](https://github.com/andrewbanchich)
- The discovery plugin now supports a `resource` configuration field like the bundle plugin. Similarly, the `resource` is treated as the canonical setting to identify the discovery bundle.

### Tooling

- The `opa test` timeout as been increased to 30 seconds when benchmarking ([#3107](https://github.com/open-policy-agent/opa/issues/3107))
- The `opa eval --schema` flag has been fixed to correctly set the schema when a _single_ schema file is passed
- The `opa build --debug` flag output has been improved for readability
- The `array.items` JSON schema value is now supported by the type checker
- The `opa fmt` subcommand can now exit with a non-zero status when a diff is detected (by passing `--fail`)
- The `opa test` subcommand no longer emits bogus file paths when fed a file:// url

### Built-in Functions

- The `http.send` built-in function falls back to the system certificate pool when the `tls_ca_cert` or `tls_ca_cert_env_variable` options are not specified ([#2271](https://github.com/open-policy-agent/opa/issues/2271)) authored by @[olamiko](https://github.com/olamiko)

### Evaluation

- The order of support rules emitted by partial evaluation is now deterministic ([#3453](https://github.com/open-policy-agent/opa/issues/3453)) authored by @[andrehaland](https://github.com/andrehaland)
- The big number performance regression caught by the fuzzer has been resolved ([#3262](https://github.com/open-policy-agent/opa/issues/3262))
- The evaluator has been updated to memoize calls to rules with arguments (functions) within a single query. This avoids recomputing function results when the same input is passed multiple times (similar to how complete rules are memoized.)

### WebAssembly

- The `wasm` target no longer panics if the OPA binary does not include a wasm runtime ([#3264](https://github.com/open-policy-agent/opa/issues/3264))
- The interrupt handling mechanism has been rewritten to make safe use of the wasmtime package. The SDK also returns structured errors now that are more aligned with topdown. ([#3225](https://github.com/open-policy-agent/opa/issues/3225))
- The SDK provides the subset of required imports now (which is useful for debugging with opa_println in the runtime library if needed.)
- The opa_number_float type has been removed from the value library (it was unused after moving to libmpdec)
- The runtime library builder has been updated to use llvm-12 and the wasmtime-go package has been updated to v0.27.0

### Documentation

- The HTTP API authorization tutorial has been updated to show how to distribute policies using bundles
- The Envoy tutorial has been tweaked to show better path matching examples

### Infrastructure

- The release-patch script has been improved to deal with _this file_ in bugfix/patch releases ([#2533](https://github.com/open-policy-agent/opa/issues/2533)) authored by @[jjshanks](https://github.com/jjshanks)
- The Makefile check targets now rely on golangci-lint and many linting errors have been resolved (authored by @[willbeason](https://github.com/willbeason))
- Multiple nightly fuzzing and data race issues in test cases have been resolved

## 0.28.0

This release includes a number of features, enhancements, and fixes. The default
branch for the Git repository has also been updated to `main`.

#### Schema Annotations

This release adds support for _annotations_. Annotations allow users to
declare metadata on rules and packages. Currently, OPA supports one form of
metadata: schema declarations. For example:

```rego
package example

# METADATA
# schemas:
# - input: schema.service
deny["service is missing required 'owner' label"] {
  input.kind == "Service"
  not input.metadata.labels.owner
}

# METADATA
# schemas:
# - input: schema.deployment
deny["deployment replica count too low for 'production' namespace"] {
  input.kind == "Deployment"
  input.metadata.namespace == "production"
  object.get(input.spec, "replicas", 1) < 3
}
```

Users can include schema annotations in their policies to tell OPA about the
structure of external data loaded under `input` or `data`. By learning the
schema of base documents, OPA can surface mistakes in the policy at authoring
time (e.g., referring to a non-existent field in a JSON object or calling a
built-in function with an invalid value.) For more information on the
annotations and schema support see the [Type
Checking](https://www.openpolicyagent.org/docs/latest/schemas/) page in the
documentation. In the future, annotations will be expanded to support other
kinds of metadata and additional tooling will be added to leverage them.

### Server

- The server now automatically sets GOMAXPROCS when running inside of a container that has cgroups applied. This helps the Go runtime avoid consuming too many CPU resources and being throttled by the kernel. ([#3328](https://github.com/open-policy-agent/opa/issues/3328))
- The server now logs an error if users enable the `token` authentication mode without a corresponding authorization policy. ([#3380](https://github.com/open-policy-agent/opa/issues/3380)) authored by @[kale-amruta](https://github.com/kale-amruta)
- The server now supports a `GET /v1/config` endpoint that returns OPA's active configuration. This API is useful if you need to debug the running configuration in an OPA configured via Discovery. ([#2020](https://github.com/open-policy-agent/opa/issues/2020))
- The server now respects the `?pretty` option in the v0 API ([#3332](https://github.com/open-policy-agent/opa/issues/3332)) authored by @[clarshad](https://github.com/clarshad)
- The Bundle plugin is more forgiving when it comes to Etag processing on HTTP 304 responses ([#3361](https://github.com/open-policy-agent/opa/issues/3361))
- The Decision Log plugin now supports a "Decision Per Second" rate limit configuration setting.
- The Status plugin can now be configured to use a custom reporter similar to the Decision Log plugin (e.g., so that Status messages can be sent to AWS Kinesis, etc.)
- The Status plugin now reports the number of decision logs that are dropped due to buffer limits.
- The service clients can authenticate with the Azure Identity OAuth2 implementation the client credentials JWT flow is used ([#3372](https://github.com/open-policy-agent/opa/issues/3372))
- Library users can now customize the logger used by the plugins by providing the `plugins.Logger` option when creating the plugin manager.

### Tooling

- The various OPA subcommands that accept schema files now accept a directory tree of schemas instead of only a single schema.
- The `opa refactor move` subcommand was added to support package renaming use cases ([#3290](https://github.com/open-policy-agent/opa/issues/3290))
- The `opa check` subcommand now supports a `-s`/`--schema` flag like the `opa eval` subcommand.

### Documentation

- The [Management API](https://www.openpolicyagent.org/docs/latest/management-introduction/) docs have been restructured so that each API has a dedicated page. In addition, the [Bundle API](https://www.openpolicyagent.org/docs/latest/management-bundles/#implementations) docs now include getting started steps for cloud-provider specific services (e.g., AWS, GCP, Azure, etc.)

### Security

- OPA now supports PKCS8 encoded EC private keys for JWT verification (which includes service authentication, bundle verification, and verification built-in functions) ([#3283](https://github.com/open-policy-agent/opa/issues/3283)). Authored by @[andrehaland](https://github.com/andrehaland).
- The bundle signing and verification APIs have been updated to support custom signers/verififers ([#3336](https://github.com/open-policy-agent/opa/pull/3336)). Authored by @[gshively11](https://github.com/gshively11).

### Evaluation

- The `time.diff` function was added to support calculating differences between date/time values ([#3348](https://github.com/open-policy-agent/opa/issues/3348)) authored by @[andrehaland](https://github.com/andrehaland)
- The `units.parse_bytes` function now supports floating-point values ([#3297](https://github.com/open-policy-agent/opa/issues/3297)) authored by @[andy-paine](https://github.com/andy-paine)
- The evaluator was fixed to use correct bindings when evaluating the full-extent of a partial rule set. This issue was causing unexpected undefined results and evaluation errors in some rare cases. ([#3369](https://github.com/open-policy-agent/opa/issues/3369) [#3376](https://github.com/open-policy-agent/opa/issues/3376))
- The evaluator was fixed to correctly generate package paths when namespacing is disabled partial evaluation. ([#3302](https://github.com/open-policy-agent/opa/issues/3302)).
- The `http.send` function no longer errors out on invalid Expires headers. ([#3284](https://github.com/open-policy-agent/opa/issues/3284))
- The inter-query cache now serializes elements on insertion thereby reducing memory usage significantly (because deserialized elements carry a ~20x cost.) ([#3042](https://github.com/open-policy-agent/opa/issues/3042))
- The rule indexer was fixed to correctly handle mapped and non-mapped values which could occur with `glob.match` usage ([#3293](https://github.com/open-policy-agent/opa/issues/3293))

### WebAssembly

- The `opa eval` subcommand now correctly returns the set of all variable bindings and expression values when the `wasm` target is enabled. Previously it returned only set of variable bindings. ([#3281](https://github.com/open-policy-agent/opa/issues/3281))
- The `glob.match` function now handles the default delimiter correctly. ([#3294](https://github.com/open-policy-agent/opa/issues/3294))
- The `opa build` subcommand no longer requires a capabilities file when the `wasm` target is enabled. If capabilities are not provided, OPA will use the capabilities for its own version. ([#3270](https://github.com/open-policy-agent/opa/issues/3270))
- The `opa build` subcommand now dumps the IR emitted by the planner when `--debug` is specified.
- The `opa eval` subcommand no longer panics when a policy fails to type check and the `wasm` target is enabled.
- The comparison functions can now return `false` instead of either being `true` or `undefined`.  ([#3271](https://github.com/open-policy-agent/opa/issues/3271))
- The internal wasm runtime will now correctly return `CancelErr` to indicate cancellation errors (instead of `BuiltinErr` which it returned previously.)
- The internal wasm runtime now correctly handles non-halt built-in errors ([#3320](https://github.com/open-policy-agent/opa/issues/3320))
- The planner no longer generates unexpected scan statements when negation used over base documents under `data` ([#3279](https://github.com/open-policy-agent/opa/issues/3279)) and ([#3305](https://github.com/open-policy-agent/opa/issues/3305))
- The planner now correctly discards out-of-scope variables when exiting comprehensions ([#3325](https://github.com/open-policy-agent/opa/issues/3325))
- The `rego` package no longer panics when the `wasm` target is enabled and undefined functions are encountered ([#3251](https://github.com/open-policy-agent/opa/issues/3251))
- 🎈 The remaining exceptions in the e2e test framework for the internal wasm runtime have been resolved.

### Build

- The `make image` target now uses the CI image for building the Go binary. This avoids platform-specific build issues by building the Go binary inside of Docker.

## 0.27.1

This release contains a fix for crashes experienced when configuring OPA to use S3 signing as service credentials ([#3255](https://github.com/open-policy-agent/opa/issues/3255)).

In addition to that, we have a small number of enhancements and fixes:

### Tooling

- The `eval` subcommand now allows using `--import` without using `--package`. Authored by @[onelittlenightmusic](https://github.com/onelittlenightmusic), [#3240](https://github.com/open-policy-agent/opa/pull/3240).

## Compiler

- The `ast` package now exports another method for JSON conversion, `ast.JSONWithOpts`, that allows further options to be set ([#3244](https://github.com/open-policy-agent/opa/pull/3244).

### Server

- REST plugins using `s3_signing` as credentials method can now include the specified service in the signature (SigV4). Authored by @[cogwirrel](https://github.com/cogwirrel), [#3210](https://github.com/open-policy-agent/opa/pull/3210).

### Documentation

- Remove soon-to-be deprecated `any` and `all` from the [Policy Reference](https://www.openpolicyagent.org/docs/v0.27.1/policy-reference/#aggregates) ([#3241](https://github.com/open-policy-agent/opa/pull/3241)) -- see also [#2437](https://github.com/open-policy-agent/opa/issues/2437).
- Add missing `discovery.service` field to [Discovery configuration](https://www.openpolicyagent.org/docs/v0.27.1/configuration/#discovery) table ([#3237](https://github.com/open-policy-agent/opa/pull/3237)).
- Fix dead links to the Envoy pages ([#3248](https://github.com/open-policy-agent/opa/pull/3248)).

### WebAssembly

- Executions using the internal Wasm SDK will now be interrupted when the provided context is done (cancelled or deadline reached).
- The generated Wasm modules could become much smaller: unused functions are replaced by `unreachable` stubs, and the heavyweight runtime components related to regular expressions are excluded when none of the regex-related builtins are used: `glob.match`, `regex.is_valid`, `regex.match`, `regex.is_valid`, and `regex.find_all_string_submatch_n`.
- The Wasm runtime now allows passing in the time to be used for evaluation, enabling callers to control the time-of-day observed by Wasm compiled policies.
- Wasmtime runtime has been updated to the latest version (v0.24.0).

## 0.27.0

This release contains a number of enhancements and bug fixes.

### Tooling

- The `eval` subcommand now supports a `-s`/`--schema` flag that accepts a JSON schema for the `input` document. The schema is used when type checking the policy so that invalid references to (or operations on) `input` data are caught at compile time. In the future, the schema support will be expanded to accept multiple schemas and rule-level annotations. See the new [Schemas](https://www.openpolicyagent.org/docs/edge/schemas/) documentation for details. Authored by @[aavarghese](https://github.com/aavarghese) and @[vazirim](https://github.com/vazirim).
- The `eval`, `test`, `bench` and REPL subcommands now supports a `-t`/`--target` flag to set the evaluation engine to use. The default engine is `rego` referring to the standard Rego interpreter in OPA. Users can now select `wasm` to enable Wasm compilation and execution of policies ([#2878](https://github.com/open-policy-agent/opa/issues/2878)).
- The `eval` subcommand now supports a `raw` option for `-f`/`--format` that is useful in bash scripts. Authored by @[jaspervdj-luminal](https://github.com/jaspervdj-luminal).
- The test framework now supports "skippable" tests. Prefix the test name with `todo_` to have the test runner skip the test, e.g., `todo_test_allow { ... }`.
- The `eval` subcommand now correctly supports the `--ignore` flag. Previously the flag was not being applied.

### Server

- The `POST /v1/compile` API now supports a `?metrics` query parameter similar to other APIs. Authored by @[jkbschmid](https://github.com/jkbschmid).
- The directory used for persisting downloaded bundles can now be configured. See the [Configuration](https://www.openpolicyagent.org/docs/latest/configuration/) page for details.
- The HTTP Decision Logger plugin no longer blocks server shutdown for the grace period when there are no logs to upload.
- The Bundle plugin now unregisters listeners correctly. This issue would cause listeners to be invoked when bundle updates were dispatched even if the listener was unregistered ([#3190](https://github.com/open-policy-agent/opa/issues/3190)).
- The server now correctly decodes policy IDs in the HTTP request URL. Authored by @[mattmahn](https://github.com/mattmahn) ([#2116](https://github.com/open-policy-agent/opa/issues/2116)).
- The server now configures the `http_request_duration_seconds` metric (for all of the server endpoitns) with smaller, more granular buckets that better map to actual response latencies from OPA.  Authored by @[luong-komorebi](https://github.com/luong-komorebi) ([#3196](https://github.com/open-policy-agent/opa/issues/3196)).

### Security

- PKCS8 keys are now supported when signing bundles and communicating with control plane services. Previously only PKCS1 keys were supported ([#3116](https://github.com/open-policy-agent/opa/issues/3116)).
- The built-in OPA HTTP API authorizer policy can now return a _reason_ to explain why a request to the OPA API is denied ([#3056](https://github.com/open-policy-agent/opa/issues/3056)). See the [Security](https://www.openpolicyagent.org/docs/edge/security/) documentation for details. Thanks to @[ajanthan](https://github.com/ajanthan) for helping improve this.

### Compiler

- The compiler can be configured to emit debug messages that explain comprehension indexing decisions. Debug messages can be enabled when running `opa build` with `--debug`.
- A panic was fixed in one of the rewriting stages when comprehensions were used as object keys ([#2915](https://github.com/open-policy-agent/opa/issues/2915))

### Evaluation

- A bug in big integer comparison was fixed. This issue was discovered when comparing serial numbers from X.509 certificates. Authored by @[andrehaland](https://github.com/andrehaland) ([#3147](https://github.com/open-policy-agent/opa/issues/3147)).
- The `io.jwt.decode_verify` function now uses the environment supplied time-of-day value instead of calling `time.Now()` ([#3105](https://github.com/open-policy-agent/opa/issues/3105)).

### Documentation

- The documentation now includes a dedicated section the OPA-Envoy integration. See [https://www.openpolicyagent.org/docs/latest/envoy-introduction/](https://www.openpolicyagent.org/docs/latest/envoy-introduction/) for details.
- The ecosystem page now ranks integrations by number of unique domains instead of the sheer number of references.

### WebAssembly

- The `data` document no longer needs to be initialized to an empty object ([#3130](https://github.com/open-policy-agent/opa/issues/3130)).
- The mpd library is now initalized by the module's `Start` function ([#3110](https://github.com/open-policy-agent/opa/issues/3110)).
- The planner now longer re-plans rules blindly when `with` statements are encountered ([#3150](https://github.com/open-policy-agent/opa/issues/3150)).
- The planner and compiler now support dynamic dispatch. Previously the planner would enumerate all functions and invocation was controlled at runtime ([#2936](https://github.com/open-policy-agent/opa/issues/2936)).
- The compiler now inserts memoization instructions into function bodies instead of at callsites. This reduces the number of wasm instructions in the resulting binary ([#3169](https://github.com/open-policy-agent/opa/pull/3169)).
- The wasmtime runtime is now the default runtime used by OPA to execute compiled policies. The new runtime no longer leaks memory when policies are reloaded.
- The planner and compiler now intern strings and booleans and implement a few micro-optimizations to reduce the size of the resulting binary.
- The capabilities support has been updated to include an ABI major and minor version for tracking backwards compatibility on compiled policies ([#3120](https://github.com/open-policy-agent/opa/issues/3120)).

### Backwards Compatibility

- The `opa test` subcommand previously supported a `-t` flag as shorthand for `--timeout`. With this release, the `-t` shorthand has been redefined for `--target`. After searching GitHub for examples of `opa test -t` (and finding nothing) we felt comfortable making this backwards incompatible change.
- The Go version used to build the OPA release has been updated from `1.14.9` to `1.15.8`. Because of this, TLS certificates that rely on Common Name for verification are no longer supported and will not work. For more information see https://github.com/golang/go/issues/39568.

## 0.26.0

This release contains a number of enhancements and bug fixes.

### Built-in Functions

- This release includes a number of built-in function improvements for Wasm compiled policies. The following built-in functions have been implemented natively and no longer need to be supplied by SDKs: `graph.reachable`, `json.filter`, `json.remove`, `object.get`, `object.remove`, and `object.union`.

- This release fixes several bugs in the Wasm implementation of certain `regex` built-in functions ([#2962](https://github.com/open-policy-agent/opa/issues/2962)), `format_int` ([#2923](https://github.com/open-policy-agent/opa/issues/2923)) and `round` ([#2999](https://github.com/open-policy-agent/opa/pull/2999)).

- This release adds `ceil` and `floor` built-in functions. Previously these could be implemented in Rego using `round` however these are more convenient.

### Enhancements

- OPA has been extended support [OAuth2 JWT Bearer Grant Type](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-jwt-bearer-grant-type) and [OAuth2 Client Credential JWT](https://www.openpolicyagent.org/docs/edge/configuration/#oauth2-client-credentials-jwt-authentication) authentication options for communicating with control plane services. This change allows OPA to use services that rely on Ping Identity as well as GCP service accounts for authentication. OPA has also been extended to support [custom authentication plugins](https://www.openpolicyagent.org/docs/edge/configuration/#custom-plugin) (thanks @[gshively11](https://github.com/gshively11)).

- OPA plugins can now enter a "WARN" state to indicate they are operating in a degraded capacity (thanks @[gshively11](https://github.com/gshively11)).

- The `opa bench` command can now benchmark partial evaluation queries. The options to enable partial evaluation are shared with `opa eval`. See `opa bench --help` for details.

- Wasm compiled policies now contain source locations that are included inside of runtime error messages (such as object key conflicts.) In addition, Wasm compiled policies only export the minimal set of APIs described on the [WebAssembly#exports](https://www.openpolicyagent.org/docs/latest/wasm/#exports) page.

### Fixes

- ast: Fix parsing of numbers to reject leading zeroes ([#2947](https://github.com/open-policy-agent/opa/issues/2947)) authored by @[LCartwright](https://github.com/LCartwright).
- bundle: Fix loader to only verify bundle keys if configured to do so ([#3028](https://github.com/open-policy-agent/opa/issues/3028)).
- cmd: Fix build to avoid packaging policy.wasm twice ([#3007](https://github.com/open-policy-agent/opa/issues/3007)).
- cmd: Fix pretty-printed PE output to hide spurious blank lines
- server: Fix false-positive in bundle root check that would prevent data updates in some cases ([#2868](https://github.com/open-policy-agent/opa/issues/2868)).
- server: Fix query cache to respect ?instrument option ([#3000](https://github.com/open-policy-agent/opa/issues/3000)).
- server: Fix server to support discovery on inter-query cache configuration
- topdown: Fix PE to avoid generating expressions that do not type check ([#3012](https://github.com/open-policy-agent/opa/issues/3012)).
- wasm: Fix planner to avoid generating a conflict error in some cases ([#2926](https://github.com/open-policy-agent/opa/issues/2926)).
- wasm: Fix planner to generate correct virtual document iteration instructions ([#3065](https://github.com/open-policy-agent/opa/issues/3065)).
- wasm, topdown: Fix with keyword handle to ensure last statement wins ([#3010]((https://github.com/open-policy-agent/opa/issues/3010))).
- wasm: Fix planner to handle assignment conflicts correctly when else keyword is used ([#3031]((https://github.com/open-policy-agent/opa/issues/3031))).

### Documentation

- Add new section on integrating policies with OAuth2 and OIDC.
- Update Kubernetes admission control tutorial to work as non-root user.
- Fix link to signing documentation ([#3027](https://github.com/open-policy-agent/opa/issues/3027)) authored by @[princespaghetti](https://github.com/princespaghetti).

### Backwards Compatibility

- Previously, OPA deduplicated sets and objects in all cases except when iterating over/referring directly to values generated by partial rules. This inconsistency would only be noticed when running ad-hoc queries or within policies when aggregating the results of array comprehensions (e.g., `count([1 | p[x]])` could observe duplicates in `p`.) This release removes the inconsistency by deduplicating sets and objects in all cases ([#429](https://github.com/open-policy-agent/opa/issues/429)). This was the second oldest open issue on the project.

### Deprecations

- OPA now logs warnings when it receives legacy `bundle` config sections instead of the `bundles` section introduced in v0.13.0.

## 0.25.2

This release extends the HTTP server authorizer (`--authorization=basic`) to supply the HTTP message body in the `input` document. See the [Authentication and Authorization](https://www.openpolicyagent.org/docs/edge/security/#authentication-and-authorization) section in the security documentation for details.

## 0.25.1

This release contains a fix for running OPA under Docker with a non-default working directory ([#2974](https://github.com/open-policy-agent/opa/issues/2974)).

## 0.25.0

This release contains a number of improvements and fixes. Importantly, this release includes a notable change to built-in function error handling. See the section below for details.

### Built-in Function Error Handling

Previously, built-in function errors would cause policy evaluation to halt immediately. Going forward, by default, built-in function errors no longer halt evaluation. Instead, expressions are treated as false/undefined if any of the invoked built-in functions return errors.

This change resolves a common issue people face when passing unsanitized input values to built-in functions. For example, prior to this change the expression `io.jwt.decode("GARBAGE")` would halt evaluation of the entire policy because the string is not a valid encoding of a JSON Web Token (JWT). If the expression was `io.jwt.decode(input.token)` and the user passed an invalid string value for `input.token` the same error would occur. With this change, the same expression is simply undefined, i.e., there is no result. This means policies can use negation to test for invalid values. For example:

```rego
decision := {"allowed": allow, "denial_reason": reason}

default allow = false

allow {
  io.jwt.verify_hs256(input.token, "secret")
  [_, payload, _] := io.jwt.decode(input.token)
  payload.role == "admin"
}

reason["invalid JWT supplied as input"] {
  not io.jwt.decode(input.token)
}
```

If you require the old behaviour, enable "strict" built-in errors on the query:

| Caller | Example |
| --- | --- |
| HTTP | `POST /v1/data/example/allow?strict-builtin-errors` |
| Go (Library) | `rego.New(rego.Query("data.example.allow"), rego.StrictBuiltinErrors(true))` |
| CLI | `opa eval --strict-builtin-errors 'data.example.allow'` |

If you have implemented custom built-in functions and require policy evaluation to halt on error in those built-in functions, modify your built-in functions to return the [topdown.Halt](./topdown/errors.go) error type.

### Built-in Functions

This release includes a few new built-in functions:

- `base64url.encode_no_pad`, `hex.encode`, and `hex.decode` for dealing with encoded data ([#2849](https://github.com/open-policy-agent/opa/issues/2849)) authored by @[johanneslarsson](https://github.com/johanneslarsson)
- `json.patch` for applying JSON patches to values inside of policies ([#2839](https://github.com/open-policy-agent/opa/issues/2839)) authored by @[jaspervdj-luminal](https://github.com/jaspervdj-luminal)
- `json.is_valid` and `yaml.is_valid` for testing validity of encoded values (authored by @[jaspervdj-luminal](https://github.com/jaspervdj-luminal))

There were also a few fixes to existing built-in functions:

- Fix unicode handling in a few string-related functions ([#2799](https://github.com/open-policy-agent/opa/issues/2799)) authored by @[anderseknert](https://github.com/anderseknert)
- Fix `http.send` to override `no-cache` HTTP header when `force_cache` specified ([#2841](https://github.com/open-policy-agent/opa/issues/2841)) authored by @[anderseknert](https://github.com/anderseknert)
- Fix `strings.replace_n` to replace overlapping patterns deterministically ([#2822](https://github.com/open-policy-agent/opa/issues/2822))
- Fix panic in `units.parse_bytes` when passed a zero-length string ([#2901](https://github.com/open-policy-agent/opa/issues/2901))

### Miscellaneous

This release adds new credential providers for management services:

- GCP metadata server ([#2938](https://github.com/open-policy-agent/opa/pull/2938)) authored by @[kelseyhightower](https://github.com/kelseyhightower)
- AWS Web Identity credentials ([#2462](https://github.com/open-policy-agent/opa/pull/2725)) authored by @[RichiCoder1](https://github.com/RichiCoder1)
- OAuth2 ([#1205](https://github.com/open-policy-agent/opa/issues/1205)) authored by @[anderseknert](https://github.com/anderseknert)

In addition the following server features were added:

- Add shutdown wait period flag to `opa run` (`--shutdown-wait-period`) ([#2764](https://github.com/open-policy-agent/opa/issues/2764)) authored by @[bcarlsson](https://github.com/bcarlsson)
- Add bundle file size limit configuration option (`bundles[_].size_limit_bytes`) to override default 1GiB limit ([#2781](https://github.com/open-policy-agent/opa/issues/2781))
- Separate decision log and status message logs from access logs (which useful for running OPA at log level `error` while continuing to report decision and status log to console) ([#2733](https://github.com/open-policy-agent/opa/issues/2733)) authored by @[anderseknert](https://github.com/anderseknert)

### Fixes

- Fix panic caused by race condition in the decision logger ([#2835](https://github.com/open-policy-agent/opa/pull/2948)) authored by @[kubaj](https://github.com/kubaj)
- Fix decision logger to flush on graceful shutdown ([#780](https://github.com/open-policy-agent/opa/issues/780)) authored by @[anderseknert](https://github.com/anderseknert)
- Fix `--verification-key` handling to accept PEM files ([#2796](https://github.com/open-policy-agent/opa/issues/2796))
- Fix `--capabilities` flag in `opa build` command ([#2848](https://github.com/open-policy-agent/opa/issues/2848)) authored by @[srenatus](https://github.com/srenatus)
- Fix loading of **signed** persisted bundles ([#2824](https://github.com/open-policy-agent/opa/issues/2824))
- Fix API response mutation caused by decision log masking ([#2752](https://github.com/open-policy-agent/opa/issues/2752)) authored by @[gshively11](https://github.com/gshively11)
- Fix evaluator to prevent `with` statements from mutating original `input` document ([#2813](https://github.com/open-policy-agent/opa/issues/2813))
- Fix set iteration runtime to be O(n) instead of O(n^2) ([#2966](https://github.com/open-policy-agent/opa/pull/2966))
- Increased OPA version telemetry report timeout from 1 second to 5 seconds to deal with slow networks

### Documentation

- Improve docs to mention built-in function support in WebAssembly compiled policies
- Improve docs around JWT HMAC encoding ([#2870](https://github.com/open-policy-agent/opa/issues/2870)) authored by @[anderseknert](https://github.com/anderseknert)
- Improve HTTP authorization tutorial steps for zsh ([#2917](https://github.com/open-policy-agent/opa/issues/2917) authored by @[ClaudenirFreitas](https://github.com/ClaudenirFreitas))
- Improve docs to describe meaning of Prometheus metrics
- Remove mention of unsafe (and unsupported) "none" signature algorithm from JWT documentation

### WebAssembly

This release also includes a number of improvements to the Wasm support in OPA. Importantly, OPA now integrates a Wasm runtime that can be used to execute Wasm compiled policies. The runtime is integrated into the existing "topdown" evaluator so that specific portions of the policy can be compiled to Wasm as a performance optimization. When the evaluator executes a policy using the Wasm runtime it emits a special `Wasm` trace event. The Wasm runtime support in OPA is currently considered **experimental** and will be iterated on in coming releases.

This release also extends the Wasm compiler in OPA to natively support the following built-in functions (in alphabetical order):

* `base64.encode`, `base64.decode`, `base64url.encode`, and `base64url.decode`
* `glob.match`
* `json.marshal` and `json.unmarshal`
* `net.cidr_contains`, `net.cidr_intersects`, and `net.cidr_overlap`
* `regex.match`, `regex.is_valid`, and `regex.find_all_string_submatch_n`
* `to_number`
* `walk`

### Backwards Compatibility

- The `--insecure-addr` flag (which was deprecated in v0.10.0) has been removed completely ([#763](https://github.com/open-policy-agent/opa/issues/763))

## 0.24.0

This release contains a number of small enhancements and bug fixes.

### Bundle Persistence

This release adds support for persisting bundles for recovery purposes. When persistence is enabled, OPA will save activated bundles to disk. On startup, OPA checks for persisted bundles and activates them immediately. This allows OPA to startup if the bundle server is unavailable ([#2097](https://github.com/open-policy-agent/opa/issues/2097)). For more information see the [Bundle](https://www.openpolicyagent.org/docs/latest/management/#bundles) documentation.

### Built-in Functions

This release includes a few new built-in functions:

- `base64.is_valid` for testing if strings are valid base64 encodings ([#2690](https://github.com/open-policy-agent/opa/issues/2690)) authored by @[carlpett](https://github.com/carlpett)
- `net.cidr_merge function` for merging sets of IPs and CIDRs ([#2692](https://github.com/open-policy-agent/opa/issues/2692))
- `urlquery.decode_object` for parsing URL query parameters into objects ([#2647](https://github.com/open-policy-agent/opa/issues/2647)) authored by @[GBrawl](https://github.com/GBrawl)

In addition, `http.send` has been enhanced to support caching overrides and in-band error handling ([#2666](https://github.com/open-policy-agent/opa/issues/2666) and [#2187](https://github.com/open-policy-agent/opa/issues/2187)).

### Fixes

- Fix `opa build` to support custom built-in functions ([#2738](https://github.com/open-policy-agent/opa/issues/2738)) authored by @[gshively11](https://github.com/gshively11)
- Fix for file watching volume mounted configmaps ([#2588](https://github.com/open-policy-agent/opa/issues/2588)) authored by @[drewwells](https://github.com/drewwells)
- Fix discovery plugin to set last request and last successful request timestamps in status updates ([#2630](https://github.com/open-policy-agent/opa/issues/2630))
- Fix planner crash on virtual document iteration ([#2601](https://github.com/open-policy-agent/opa/issues/2601))
- Fix decision logger to requeue failed chunks ([#2724](https://github.com/open-policy-agent/opa/pull/2724) authored by @[anderseknert](https://github.com/anderseknert))
- Fix object/set implementation in WASM-C library to avoid resizing.
- Fix JSON parser in WASM-C library to copy memory for strings and numbers.
- Improve WASM-C library to recycle object and set element structures while growing.

In addition, this release contains several fixes for panics identified by fuzzing:

- ast: Fix compiler to expand exprs in rule args ([#2649](https://github.com/open-policy-agent/opa/issues/2649))
- ast: Fix output var analysis to accept refs with non-var heads ([#2678](https://github.com/open-policy-agent/opa/issues/2678))
- ast: Fix panic during local var rewriting ([#2720](https://github.com/open-policy-agent/opa/issues/2720))
- ast: Fix panic in local var rewriting caused by object corruption ([#2661](https://github.com/open-policy-agent/opa/issues/2661))
- ast: Fix panic in parser post-processing of expressions ([#2714](https://github.com/open-policy-agent/opa/issues/2714))
- ast: Fix parser to ignore rules with args and key in head ([#2662](https://github.com/open-policy-agent/opa/issues/2662))
- ast: Fix object corruption during safety reordering
- types: Fix panic on reference to object with composite key ([#2648](https://github.com/open-policy-agent/opa/issues/2648))

### Backwards Compatibility

- Renamed `timer_rego_builtin_http.send_ns` to `timer_rego_builtin_http_send_ns` to avoid issues with periods in metric keys.
- Removed deprecated `watch` package ([#2265](https://github.com/open-policy-agent/opa/issues/2265))

### Miscellaneous

- Add support for H2C on HTTP listener ([#2739](https://github.com/open-policy-agent/opa/issues/2739) thanks @[srenatus](http://github.com/srenatus)!).
- Add Go version information to `opa version` output (thanks @[srenatus](http://github.com/srenatus)!)
- The official OPA build has been updated to Go v1.14.9. Previously it was using v1.13.7 which is no longer supported (thanks @[srenatus](http://github.com/srenatus)!)

## 0.23.2

This release contains a fix for a regression in v0.23.1 around bundle downloading. The bug caused OPA to cancel bundle downloads prematurely. Users affected by this issue would see the following error message in the OPA logs:

```
[ERROR] Bundle download failed: bundle read failed: archive read failed: context canceled
  plugin = "bundle"
  name = <bundle name>
```

## 0.23.1

### Fixes

- plugins/discovery: Set the last request and last successful request in discovery status ([#2630](https://github.com/open-policy-agent/opa/issues/2630))

### Miscellaneous

- plugins/rest: Add response header timeout for REST client

## 0.23.0

### `http.send` Caching

The `http.send` built-in function now supports caching across policy queries. The `caching.inter_query_builtin_cache.max_size_bytes` configuration setting places a limit on the amount of memory that will be used for built-in function caching. By default, not limit is set. For `http.send`, cache duration is controlled by HTTP response headers. For more details see the [`http.send`](https://www.openpolicyagent.org/docs/latest/policy-reference/#http) documentation.

### Capabilities

OPA now supports a _capabilities_ check on policies. The check allows callers to restrict the built-in functions that policies may depend on. If the policies passed to OPA require built-ins not listed in the capabilities structure, an error is returned. The capabilities check is currently supported by the `check` and `build` sub-commands and can be accessed programmatically on the `ast.Compiler` structure. The repository also includes a set of capabilities files for previous versions of OPA under the `capabilities/` directory.

For example, given the following policy:

```rego
package example

deny["missing semantic version"] {
  not valid_semantic_version_tag
}

valid_semantic_version_tag {
  semver.is_valid(input.version)
}
```

We can check whether it is compatible with different versions of OPA:

```bash
# OK!
$ opa build ./policies/example.rego --capabilities ./capabilities/v0.22.0.json

# ERROR!
$ opa build ./policies/example.rego --capabilities ./capabilities/v0.21.1.json
```

### Built-in Functions

This release includes a new built-in function to test if a string is a valid regular expression: `regex.is_valid`.

### WebAssembly

* Host environments no longer have to provide the `opa_println` function when instantiating compiled policy modules.
* SDKs no longer have to set the heap top address during initialization.

### Fixes

- Add a new inter-query cache to cache responses across queries ([#1753](https://github.com/open-policy-agent/opa/issues/1753))
- Fix `opa` CLI flags to match documentation ([#2586](https://github.com/open-policy-agent/opa/issues/2586)) authored by @[OmegaVVeapon](https://github.com/OmegaVVeapon)
- Fix rule indexing when multiple glob.match mappers are required ([#2617](https://github.com/open-policy-agent/opa/issues/2617))
- Fix AST to marshal non-string object keys ([#516](https://github.com/open-policy-agent/opa/issues/516))
- Fix signature calculation to include port if necessary ([#2568](https://github.com/open-policy-agent/opa/issues/2568))
- Fix partial evaluation to check function output for false values ([#2573](https://github.com/open-policy-agent/opa/issues/2573))

### Miscellaneous

- Add `http.send` latency to query metrics ([#2034](https://github.com/open-policy-agent/opa/issues/2034))
- Add support for `opa build` unknowns under `data` ([#2581](https://github.com/open-policy-agent/opa/issues/2581))
- Add support to wait for plugin readiness before starting server
- Add parameter to set wall clock time during evaluation for replay purposes
- Fix groundness bit on objects during update
- Fix x509 built-in functions to parse PEM or DER inputs
- Fix bundle signing and verification to use standard JWT key ID header
- Optimize AST collections to cache hash values
- Optimize object iteration to avoid hashing
- Optimize evaluator by removing unnecessary term copying

### Deprecations

* The `watch` query parameter on the Data API has been deprecated. The query watch feature was unused and the lack of incremental evaluation would have introduced scalability issues for users. The feature will be removed in a future release.

* The `partial` query parameter on the Data API has been deprecated. Note, this only applies to the `partial` query parameter that the Data API supports, not Partial Evaluation itself. The `partial` parameter allowed users to lazily trigger Partial Evaluation (for optimization purposes) during a policy query. While this is useful for kicking the tires in a development environment, putting optimization into the policy query path is not recommended. If users want to kick the tires with Partial Evaluation, we recommend running the `opa build` command.

### Backwards Compatibilty

* The `storage.Indexing` interface has been removed. Storage indexing has not been supported since 0.5.12. It was time to remove the interface. Custom store implementations that may have included no-op implementations of the interface can be updated.

* The `ast.Array` type has been redefined a struct. Previously `ast.Array` was a type alias for `[]*ast.Term`. This change is backwards incompatible because slice operations can no longer be performed directly on values of type `ast.Array`. To accomodate, the `ast.Array` type now exports functions for the same operations. This change decouples callers from the underlying array implementation which opens up room for future optimizations.

## 0.22.0

### Bundle Signing

OPA now supports digital signatures for policy bundles. Specifically, a signed bundle is a normal OPA bundle that includes a file named ".signatures.json" that dictates which files should be included in the bundle, what their SHA hashes are, and of course is cryptographically secure. When OPA receives a new bundle, it checks that it has been properly signed using a key that OPA has been configured with out-of-band. Only if that verification succeeds does OPA activate the new bundle; otherwise, OPA continues using its existing bundle and reports an activation failure via the status API and error logging. For more information see https://openpolicyagent.org/docs/latest/management/#signing. Many thanks to @[ashish246](https://github.com/ashish246) who co-designed the feature and provided valuable input to the development process with his proof-of-concept [#1757](https://github.com/open-policy-agent/opa/issues/1757).

### Optimization Levels

`opa build` now supports multiple optimization levels. The first level (`--optimize=1`) enables constant folding (based on partial evaluation) that only inlines values that can be computed entirely at build time. The second level (`--optimize=2`) enables the existing (more aggressive) version of partial evaluation that eagerly inlines as much of the policy as possible. For more information on the optimization levels see the [Optimization Levels](https://www.openpolicyagent.org/docs/latest/policy-performance/#optimization-levels) section in the documentation.

### Built-in Functions

- `numbers.range` ([#2479](https://github.com/open-policy-agent/opa/issues/2479)) was added to support policies that need to generate a range of integers (e.g., a network port range).
- `semver.is_valid` and `semver.compare` ([#2538](https://github.com/open-policy-agent/opa/pull/2538/)) was added to support policies that need to validate semantic version numbers (authored by @[charlieegan3](https://github.com/charlieegan3)).

### WebAssembly

- All [String](https://www.openpolicyagent.org/docs/latest/policy-reference/#strings) built-in functions (except `sprintf`) are now implemented natively inside of Wasm-compiled policies.

### Fixes

- A few small issues in the Go integration and `rego` package examples have been resolved ([#2294](https://github.com/open-policy-agent/opa/issues/2294)) and [#2367](https://github.com/open-policy-agent/opa/issues/2367)) authored by @[gaga5lala](https://github.com/gaga5lala).
- The Kubernetes Admission Controller tutorial as been updated to work with recent versions of Kubernetes ([#2467](https://github.com/open-policy-agent/opa/issues/2467) authored by @[gaga5lala](https://github.com/gaga5lala)).
- A few issues in partial evaluation around negation inlining and partial rules have been resolved (e.g., [#2492](https://github.com/open-policy-agent/opa/issues/2492), [#2491](https://github.com/open-policy-agent/opa/issues/2491)).

### Miscellaneous

- OPA now supports IMDSv2 for the AWS metadata service. This improves the security posture of OPA deployments in AWS ([#2482](https://github.com/open-policy-agent/opa/issues/2482)) authored by @[nhw76](https://github.com/nhw76).
- Several improvements to the project documentation including a policy style discussion, an integration option comparison, and discussion of bootstrapping and fail-open versus fail-closed modes.
- The project's CI/CD infrastructure has been migrated to GitHub Actions. The new CI/CD infrastructure is designed and implemented to be portable and includes a number of quality-of-life improvements.
- End-to-end query latency with decision logging enabled has been improved by 10%-15% in real-world cases.

### Backwards Compatibility

* The `rego.Tracer` and `rego.EvalTracer` API's have been deprecated in favor of
  the newer `rego.QueryTracer` and `rego.EvalQueryTracer` API.
* The `tester.Runner#SetCoverageTracer` API has been deprecated in favor of the
  newer `test.Runner#SetCoverageQueryTracer` API.

## 0.21.1

This release fixes [#2497](https://github.com/open-policy-agent/opa/issues/2497) where the comprehension indexing optimization produced incorrect results for nested comprehensions that close over variables in the outer scope. This issue only affects policies containing nested comprehensions that are recognized by the indexer (which is a relatively small percentage).

This release also backports the GitHub Actions migration and a fix to the Wasm library build step.

## 0.21.0

### Features

* Decision log masks can now mutate decision log events. Previously, the masks could only erase data in the events. With this change, users can implement masks that obfuscate or add information to the decision log events before they are emitted. Thanks to @dkiser for implementing this feature [#2379](https://github.com/open-policy-agent/opa/issues/2379))!

* This release contains a new built-in function for parsing X.509 Certificate Signing Requests (`crypto.x509.parse_certificate_request`). Thanks to @vivekbagade for implementing this feature [#2402](https://github.com/open-policy-agent/opa/issues/2402)!

* This release adds support for aggregation and bit arithmetic operations for WebAssembly compiled policies. These functions no longer have to be provided by the host environment.

### Fixes

- cmd: Fix bug in --disable-inlining option parsing ([#2196](https://github.com/open-policy-agent/opa/issues/2196)) authored by @[Syn3rman](https://github.com/Syn3rman)
- docs: Improve terraform example to incorporate `child_modules` ([#1772](https://github.com/open-policy-agent/opa/issues/1772))
- server: Fix panic caused by compiler misuse with bundles ([#2197](https://github.com/open-policy-agent/opa/issues/2197))
- topdown: Fix incorrect memoization during partial evaluation ([#2455](https://github.com/open-policy-agent/opa/issues/2455))
- topdown: Fix loss of precision in arithmetic and aggregate builtins ([#2469](https://github.com/open-policy-agent/opa/issues/2469))

### Miscellaneous

* Thanks to @Syn3rman for implementing an improvement to our release process to automatically tag external contributors ([#2323](https://github.com/open-policy-agent/opa/issues/2323))!

* The coverage and profiling tracers no longer require variable values from the evaluator. This change improves perfomance significantly when coverage or profiling is enabled and policies inspect large data sets. Benchmarks show anywhere from 0.5x to over 30x speedup depending on the policy.

### Backwards Compatibility

* `topdown.Tracer` has been deprecated in favor of a newer interface
  `topdown.QueryTracer`.
* All tracers (regardless of interface implementation) will now only be checked
  for being enabled at the beginning of query evaluation rather than on a
  per-event basis.
* `topdown.BuiltinContext#Tracers` has been deprecated in favor of
  `topdown.BuiltinContext#QueryTracers`. The older `Tracers` field will be `nil`
  starting this release, and eventually removed.

## 0.20.5

### Fixes

- compile: Change name of result var for wasm binary ([#2441](https://github.com/open-policy-agent/opa/issues/2441))
- format: Deep copy inputs to avoid mutating the caller's copy ([#2439](https://github.com/open-policy-agent/opa/issues/2439))

### Miscellaneous

- docs: Add `opa_println` to wasm required imports

## 0.20.4

### Fixes

- format: Refactor wildcard names to rewrite early ([#2430](https://github.com/open-policy-agent/opa/issues/2430))

## 0.20.3

### Fixes

- docs/content small output correction on terraform page ([#1772](https://github.com/open-policy-agent/opa/issues/1772))
- format: Fix wildcards in nested refs

## 0.20.2

### Fixes

- format: Fix panic with else blocks and comments ([#2420](https://github.com/open-policy-agent/opa/issues/2420))

## 0.20.1

This release fixes an issue in the Docker image build. The
default ca-certificates were not being included becasue the Docker
image is FROM scratch now.

## 0.20.0

### Major Features

This release includes a number of features, optimizations, and bugfixes.

#### Version Reporting

OPA now determines the latest stable release version using
https://telemetry.openpolicyagent.org. The only information provided to the
telemetry service is the version (e.g., `0.20.0`), a UUIDv4 generated on
startup, and the build platform/architecture (e.g., `darwin, amd64`). This
feature is on by default in `opa run` however it can be easily disabled by
specifying `--skip-version-check` on the command-line. If you are inside the
REPL, type `help` to see the latest version information. If you are running OPA
as a server, OPA will log an INFO level message indicating if OPA is out of
date. Version checking is best-effort. Any errors that occur while communicating
with https://telemetry.openpolicyagent.org are only logged at DEBUG level. For
more information see https://openpolicyagent.org/docs/latest/privacy/.

#### New `opa build` command

The `opa build` command can now be used to package OPA policy and data files
into [bundles](https://www.openpolicyagent.org/docs/latest/management-bundles)
that can be easily distributed via HTTP. See `opa build --help` for details.
This change is backwards incompatible. If you were previously relying on `opa
build` to compile policies to wasm, you can still do so:

```bash
# before v0.20.0
opa build -d policy.rego 'data.example.allow'

# v0.20.0 and newer
opa build policy.rego -e example/allow -t wasm
```

### Built-in Functions

This release includes a number of new built-in functions:

* `graph.reachable` for computing the transitive closure from edge sets. This
  function allows users to write policies that traverse organization charts,
  security groups, etc. (thanks to @jaspervdj-luminal!)
* `io.jwt.verify_rs512` and other variants (`rs`/`es`/`hs`/`ps`, `384`/`512`)
  were added (thanks to @GBrawl!)
* `uuid.rfc4122` for generating UUIDv4s (thanks to @reneklootwijk!)

This release also includes a few fixes to existing built-in functions:

* `units.parse_bytes` now supports units without the `B` or `b` suffix (thanks to @GBrawl!)
* `io.jwt.verify_decode` now supports floating-point `nbf` and `exp` claims (thanks to @GBrawl!)
* `array.slice` clamping logic fixed to prevent panic ([#2320](https://github.com/open-policy-agent/opa/issues/2320)).

### Operations

* The `opa run` command now supports a `--diagnostic-addr` flag that causes the
  server to expose the `/health` and `/metric` endpoint on a different address.
  This makes it easier to secure sidecar deployments in Kubernetes because the
  main API endpoints can be served on localhost and the diagnostic endpoints can
  be served on 0.0.0.0 so that the kubelet and other components can access them
  ([#2002](https://github.com/open-policy-agent/opa/issues/2002)). The envoy
  tutorial has been updated to show this in action.

* The AWS credential provided has been updated to support the standard
  `AWS_SESSION_TOKEN` and `AWS_SECURITY_TOKEN` environment variables. These are
  used when signing S3 bundle requests for an AWS IAM assumed role (thanks to
  @kpiotrowski!)

### WebAssembly

This release includes a number of improvements for wasm compiled policies.

* UTF-8 and UTF-16 strings are now fully supported in the internal string
  representation ([#1885](https://github.com/open-policy-agent/opa/issues/1885))
* Numeric values are implemented on top of arbitrary-precision floating point
  numbers to avoid loss-of-precision issues.
* The arithemetic, set, array, and type checking built-in function categories
  are now supported by the wasm library. This means they do not have to be
  implemented by the language-specific opa-wasm SDKs.
* The set and object implementations now use a chained hash set under the hood
  ([#2225](https://github.com/open-policy-agent/opa/issues/2225))

### Performance

* OPA will attempt to index collections generated by comprehensions to ensure
  linear runtime for policies performing "group-by" operations (e.g., inverting
  an objects.) For more information see the [Policy Performance](https://www.openpolicyagent.org/docs/latest/policy-performance/)
  page ([#2276](https://github.com/open-policy-agent/opa/issues/2276)).

### Tooling

* The OPA extension for VS Code now supports `Go To Definition` inside policies.
  This feature uses the new `opa oracle find-definition` command.
* The `opa test` command now includes location information on trace output.
* The `opa fmt` command now preserves `else` block style when possible (thanks to @mikaelcabot!)

### Documentation

This release includes several improvements to the website and documentation.

* Improved terraform tutorial example ([#1772](https://github.com/open-policy-agent/opa/issues/1772)) (thanks to @princespaghetti!)
* Fixed token validation logic in envoy tutorial example ([#2395](https://github.com/open-policy-agent/opa/issues/2395)) (thanks to @princespaghetti!)
* Usability issues on the frontpage have been resolved ([#2205](https://github.com/open-policy-agent/opa/issues/2205), [#2206](https://github.com/open-policy-agent/opa/issues/2206) (thanks to @arunbsar!)
* The [Policy Performance](https://www.openpolicyagent.org/docs/latest/policy-performance/)
  page now includes resource utilization guidelines ([#1601](https://github.com/open-policy-agent/opa/issues/1601))
* By popular demand, the "document model" explanation has been brought back into
  existence. It now lives in the [Philosophy](https://www.openpolicyagent.org/docs/latest/philosophy/#the-opa-document-model)
  section ([#2284](https://github.com/open-policy-agent/opa/issues/2284)).
* The [Ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/) page
  implements a simple sorting algorithm that ranks items by amount of related
  content.
* The policy cheat sheet has been merged into the [Policy Reference](https://www.openpolicyagent.org/docs/latest/policy-reference/) page.

### Fixes

* REPL now correctly displays booleans in tabled output ([#2338](https://github.com/open-policy-agent/opa/issues/2338), thanks to @timakin!)
* Discovery now supports service configuration updates. This makes token refresh easier in distributed environments on AWS. ([#2058](https://github.com/open-policy-agent/opa/issues/2058))
* Fixed compiler panic if body omitted from `else` statement ([#2353](https://github.com/open-policy-agent/opa/issues/2353))
* Fixed panic in /health API with the envoy plugin ([#2396](https://github.com/open-policy-agent/opa/issues/2396))
* Partial Evaluation no longer generates unsafe queries for certain negated expressions ([#2045](https://github.com/open-policy-agent/opa/issues/2045))
* Partial Evaluation no longer saves an incorrect binding list in some cases ([#2368](https://github.com/open-policy-agent/opa/issues/2368))
* Output variable analysis no longer visits closures. This makes the analysis easier to use outside of the safety check.
* Rules parsed from expressions now have location information set correctly.

### Miscellaneous

* If you are building OPA for debian systems, the Makefile now supports a `make
  deb` target. The target requires `dpkg-deb` to be installed. Thanks to @keshto
  for contributing this!
* OPA is now built, by default, with CGO disabled. Also, the default Docker
  image (`openpolicyagent/opa`) is back to using `FROM scratch`.

### Backwards Compatibility

* An internal utility function that unmarshals JSON (`util.UnmarshalJSON`) has
  been fixed to return an error if the input bytes contain garbage following a
  valid JSON value. In the past, the `util.UnmarshalJSON` function would just
  return the valid JSON value and ignore the garbage following it. This change
  is backwards incompatible since clients that were previously transmitting bad
  data will now receive an error, however, we think it's important to surface
  errors rather than hide them ([#2331](https://github.com/open-policy-agent/opa/issues/2331)).

* The Go plugin/shared library loading feature that was deprecated in v0.14.0
  has finally been removed completely. If you are interested in extending OPA,
  see the [Extensions](https://www.openpolicyagent.org/docs/latest/extensions/)
  for how to do so at compile-time ([#2049](https://github.com/open-policy-agent/opa/issues/2049)).

* The `github.com/open-policy-agent/opa/metrics#Counter` interface has been
  extended to require an `Add(uint64)` function. This change only affects users
  that have implemented their own version of the
  `github.com/open-policy-agent/opa/metrics#Metrics` interface (which is the
  factory for counters.)

* As mentioned above, the `opa build` command-line syntax has changed. We think
  this is the right time to refresh the command and we are more confident that
  the new syntax will remain stable going forward.

### Deprecation

* This release deprecates `opa test -l` flag. Since we now display the trace
  with line information, this flag is no longer needed.

* In the next release we plan to deprecate the `?watch` and `?partial` HTTP API
  parameters. The `?watch` feature is unused and introduces significant
  complexity in the server implementation. The `?partial` parameter lazily
  invokes Partial Evaluation _inline_ with policy invocation. This is useful for
  development and debug purposes, however, it's not recommended for enforcement
  points ot use (since PE optimization can introduce significant latency.) Users
  should rely on the new `opa build` command to perform PE on their policies.
  See `opa build --help` for more information.


## 0.19.2

### Fixes

- plugins: Fix race between manager and plugin startup ([#2343](https://github.com/open-policy-agent/opa/issues/2343))

## 0.19.1

### Fixes

- cmd/fmt: Only list files if there were changes ([#2295](https://github.com/open-policy-agent/opa/issues/2295))

## 0.19.0

### New Parser

This release includes a new parser implementation that resolves a number
of existing issues with the old parser. As part of implementing the new parser
a small number of backwards incompatible changes have been made.

#### Backwards Compatibility

The new parser contains a small number of backwards incompatible changes that
correct questionable behaviour from the old parser. These changes affect
a very small number of actual policies and we feel confident in the decision to
break backwards compatibility here.

- Numbers no longer lose-precision [#501](https://github.com/open-policy-agent/opa/issues/501)
- Leading commas do not cause objects to lose values [#2198](https://github.com/open-policy-agent/opa/issues/2198)
- Rules wrapped with braces no longer parse [#2199](https://github.com/open-policy-agent/opa/issues/2199)
- Rule names can no longer contain dots/hyphens [#2200](https://github.com/open-policy-agent/opa/issues/2200)
- Object comprehensions now have priority over logical OR in all cases [#2201](https://github.com/open-policy-agent/opa/issues/2201)

In addition there are a few small changes backwards incompatible changes in APIs:

- The `message` field on `rego_parse_error` objects contains a human-readable description
  of the parse error. The old parser would often report "no match found" to indicate
  the input contained invalid syntax. The new parser has slightly more specific
  errors. If you integrated with OPA and implemented error handling based on the
  content of these human-readable error message strings, your integration may be affected.
- The `github.com/open-policy-agent/opa/format#Bytes` function has been removed (it was unused.)

#### Benchmark Results

The output below shows the Go `benchstat` result for master (5a5d2a42) compared to the new parser.

```
name                                 old time/op    new time/op    delta
ParseModuleRulesBase/1-16               210µs ± 1%       4µs ± 1%  -98.02%  (p=0.008 n=5+5)
ParseModuleRulesBase/10-16             1.39ms ± 1%    0.03ms ± 0%  -97.93%  (p=0.008 n=5+5)
ParseModuleRulesBase/100-16            13.5ms ± 1%     0.3ms ± 1%  -97.93%  (p=0.008 n=5+5)
ParseModuleRulesBase/1000-16            148ms ± 5%       3ms ± 6%  -97.77%  (p=0.008 n=5+5)
ParseStatementBasicCall-16              141µs ± 5%       3µs ± 1%  -97.92%  (p=0.008 n=5+5)
ParseStatementMixedJSON-16             9.06ms ± 2%    0.07ms ± 1%  -99.19%  (p=0.008 n=5+5)
ParseStatementSimpleArray/1-16          131µs ± 6%       2µs ± 1%  -98.10%  (p=0.008 n=5+5)
ParseStatementSimpleArray/10-16         499µs ± 6%       7µs ± 2%  -98.54%  (p=0.008 n=5+5)
ParseStatementSimpleArray/100-16       4.00ms ± 2%    0.06ms ± 4%  -98.58%  (p=0.008 n=5+5)
ParseStatementSimpleArray/1000-16      42.0ms ± 3%     0.5ms ± 4%  -98.70%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x1-16      233µs ± 6%       4µs ± 3%  -98.49%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x1-16      514µs ± 0%       9µs ± 4%  -98.33%  (p=0.008 n=5+5)
ParseStatementNestedObjects/10x1-16     911µs ± 5%      14µs ± 5%  -98.46%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x5-16     4.24ms ± 1%    0.01ms ± 1%  -99.82%  (p=0.016 n=4+5)
ParseStatementNestedObjects/1x10-16     138ms ± 1%       0ms ± 1%  -99.99%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x5-16      714ms ± 0%       5ms ± 5%  -99.26%  (p=0.016 n=4+5)
ParseBasicABACModule-16                3.12ms ± 3%    0.04ms ± 4%  -98.63%  (p=0.008 n=5+5)

name                                 old alloc/op   new alloc/op   delta
ParseModuleRulesBase/1-16              99.2kB ± 0%     5.7kB ± 0%  -94.30%  (p=0.008 n=5+5)
ParseModuleRulesBase/10-16              600kB ± 0%      29kB ± 0%  -95.16%  (p=0.008 n=5+5)
ParseModuleRulesBase/100-16            5.72MB ± 0%    0.27MB ± 0%  -95.34%  (p=0.008 n=5+5)
ParseModuleRulesBase/1000-16           58.0MB ± 0%     2.7MB ± 0%  -95.42%  (p=0.008 n=5+5)
ParseStatementBasicCall-16             70.2kB ± 0%     5.0kB ± 0%  -92.82%  (p=0.008 n=5+5)
ParseStatementMixedJSON-16             3.64MB ± 0%    0.06MB ± 0%  -98.34%  (p=0.008 n=5+5)
ParseStatementSimpleArray/1-16         63.7kB ± 0%     4.8kB ± 0%  -92.42%  (p=0.008 n=5+5)
ParseStatementSimpleArray/10-16         205kB ± 0%       8kB ± 0%  -96.00%  (p=0.008 n=5+5)
ParseStatementSimpleArray/100-16       1.64MB ± 0%    0.05MB ± 0%  -97.19%  (p=0.008 n=5+5)
ParseStatementSimpleArray/1000-16      16.5MB ± 0%     0.4MB ± 0%  -97.50%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x1-16     98.6kB ± 0%     5.7kB ± 0%  -94.22%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x1-16      224kB ± 0%       9kB ± 0%  -96.05%  (p=0.008 n=5+5)
ParseStatementNestedObjects/10x1-16     381kB ± 0%      13kB ± 0%  -96.63%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x5-16     1.76MB ± 0%    0.01MB ± 0%  -99.38%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x10-16    56.2MB ± 0%     0.0MB ± 0%  -99.97%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x5-16      280MB ± 0%       4MB ± 0%  -98.67%  (p=0.008 n=5+5)
ParseBasicABACModule-16                1.27MB ± 0%    0.04MB ± 0%  -97.08%  (p=0.008 n=5+5)

name                                 old allocs/op  new allocs/op  delta
ParseModuleRulesBase/1-16               2.28k ± 0%     0.07k ± 0%  -96.75%  (p=0.008 n=5+5)
ParseModuleRulesBase/10-16              16.1k ± 0%      0.5k ± 0%  -96.59%  (p=0.008 n=5+5)
ParseModuleRulesBase/100-16              159k ± 0%        5k ± 0%  -96.64%  (p=0.008 n=5+5)
ParseModuleRulesBase/1000-16            1.62M ± 0%     0.05M ± 0%  -96.72%  (p=0.008 n=5+5)
ParseStatementBasicCall-16              1.36k ± 0%     0.05k ± 0%  -96.25%  (p=0.008 n=5+5)
ParseStatementMixedJSON-16               105k ± 0%        1k ± 0%     ~     (p=0.079 n=4+5)
ParseStatementSimpleArray/1-16          1.34k ± 0%     0.04k ± 0%  -97.09%  (p=0.008 n=5+5)
ParseStatementSimpleArray/10-16         5.49k ± 0%     0.12k ± 0%  -97.90%  (p=0.008 n=5+5)
ParseStatementSimpleArray/100-16        47.8k ± 0%      0.8k ± 0%     ~     (p=0.079 n=4+5)
ParseStatementSimpleArray/1000-16        481k ± 0%        8k ± 0%  -98.33%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x1-16      2.38k ± 0%     0.05k ± 0%  -97.82%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x1-16      6.02k ± 0%     0.12k ± 0%  -97.94%  (p=0.008 n=5+5)
ParseStatementNestedObjects/10x1-16     10.6k ± 0%      0.2k ± 0%  -98.01%  (p=0.008 n=5+5)
ParseStatementNestedObjects/1x5-16      51.2k ± 0%      0.1k ± 0%     ~     (p=0.079 n=4+5)
ParseStatementNestedObjects/1x10-16     1.66M ± 0%     0.00M ± 0%  -99.99%  (p=0.008 n=5+5)
ParseStatementNestedObjects/5x5-16      8.16M ± 0%     0.07M ± 0%  -99.13%  (p=0.008 n=5+5)
ParseBasicABACModule-16                 36.5k ± 0%      0.7k ± 0%  -98.09%  (p=0.008 n=5+5)
```

### Fixes and Enhancements

- ast: Add rules/functions that contain errors to the type env ([#2155](https://github.com/open-policy-agent/opa/issues/2155))
- ast: Fix panic when rule args contain call expressions ([#2081](https://github.com/open-policy-agent/opa/issues/2081))
- ast: Fix bug in term rewritten when 'input' is passed as an argument ([#2084](https://github.com/open-policy-agent/opa/issues/2084))
- bundle: Remove extra root name in bundle file ids ([#2117](https://github.com/open-policy-agent/opa/issues/2117))
- cmd/fmt: Fix to always write formatted file to stdout ([#2235](https://github.com/open-policy-agent/opa/issues/2235))
- cmd/test: --explain now turns on verbose output ([#2069](https://github.com/open-policy-agent/opa/issues/2069))
- cmd/test: Default `-v` traces show notes and fails ([#2068](https://github.com/open-policy-agent/opa/issues/2068))
- docs/website: Fix mobile docs nav menu ([#2074](https://github.com/open-policy-agent/opa/issues/2074))
- format: Print var if wildcard is used multiple times ([#2053](https://github.com/open-policy-agent/opa/issues/2053))
- plugins/bundle: Update the downloader's e-tag based on bundle activation ([#2220](https://github.com/open-policy-agent/opa/issues/2220))
- plugins: Add support to specify bearer token path (which enables token refresh) ([#2241](https://github.com/open-policy-agent/opa/issues/2241))
- profiler: Fix panic when location is missing by grouping expressions missing a location ([#2134](https://github.com/open-policy-agent/opa/issues/2134))
- rego: Avoid re-using transactions in compiler ([#2197](https://github.com/open-policy-agent/opa/issues/2197))
- repl: Add unset-package command ([#2140](https://github.com/open-policy-agent/opa/issues/2140))
- server: Do not return partial modules /v1/policies output ([#2036](https://github.com/open-policy-agent/opa/issues/2036))
- server: Specify partial evaluation namespace to avoid conflicts ([#2247](https://github.com/open-policy-agent/opa/issues/2247))
- topdown: Add time.add_date builtin ([#1990](https://github.com/open-policy-agent/opa/issues/1990))
- topdown: Fix partial evaluation to save comprehensions correctly ([#2243](https://github.com/open-policy-agent/opa/issues/2243))
- topdown: Improve pretty trace location details ([#2143](https://github.com/open-policy-agent/opa/issues/2143))
- topdown: Include HTTP response headers in `http.send` output ([#2238](https://github.com/open-policy-agent/opa/issues/2238))
- [Multiple](https://github.com/open-policy-agent/opa/commit/3eeb09c3e83749aff31e15bdfff5d82f3224c102) [important](https://github.com/open-policy-agent/opa/commit/29d8fbbef6facc96d03be3e07473d12e38acd843) [improvements](https://github.com/open-policy-agent/opa/commit/c5c85795aaa3701763f98d16308c5944f05f3da4) [to `http.send()`](https://github.com/open-policy-agent/opa/commit/ce92d19f655efffd6bda26006a2f4898cbdb69ed) [thanks to](https://github.com/open-policy-agent/opa/commit/351a7313df35e8de9e9474fd56a1a905cd51e0c1)  @jpeach

### Miscellaneous

- [Added `man` target in the Makefile for `man` page generation!](https://github.com/open-policy-agent/opa/commit/4c81aa75c05e4dd69408b9c879be40f9f4369a2c) (thanks to @olivierlemasle)
- [Added Sublime Text syntax file](https://github.com/open-policy-agent/opa/blob/master/misc/syntax/sublime/rego.sublime-syntax)
- [Added link to Emacs mode for Rego](https://github.com/psibi/rego-mode) (thanks to @psibi)
- [Added net.cidr_contains_matches built-in function](https://github.com/open-policy-agent/opa/pull/2221/commits/6ae4ed9e6578ffb272604a79b1ef9a944cda7782)
- [Improved support for registering custom built-in functions](https://github.com/open-policy-agent/opa/blob/84b61c647a0d76e62043d6f52510411e8b00d2f0/docs/content/extensions.md)

## 0.18.0

### Features

- Add `opa bench` and `opa test --bench` sub commands for benchmarking policy evaluation. ([#1424](https://github.com/open-policy-agent/opa/issues/1424))
- Permit verifying JWT's with a public key
- `http.send` improvements:
  - Allow for skipping TLS verification via `tls_insecure_skip_verify` option
  - Add `Host` header support

### New Built-in Functions

- Bitwise operators ([#1919](https://github.com/open-policy-agent/opa/issues/1919))
  - `bits.or`
  - `bits.and`
  - `bits.negate`
  - `bits.xor`
  - `bits.lsh`
  - `bits.rsh`
- `json.remove` which works similar to `object.remove` but supports a JSON pointer path.

### Fixes
- docs: Render tutorials as list ([#2071](https://github.com/open-policy-agent/opa/issues/2071))
- ast: Fix type check for objects with non-json keys ([#2183](https://github.com/open-policy-agent/opa/issues/2183))
- ast: Return an error when parsing an empty module ([#2054](https://github.com/open-policy-agent/opa/issues/2054))
- docs: Fix broken PAM module link ([#2113](https://github.com/open-policy-agent/opa/issues/2113))
- docs: Fix code fence in kubernetes-primer.md ([#2177](https://github.com/open-policy-agent/opa/issues/2177))
- topdown: Invoke iterator when evaluating negation ([#2142](https://github.com/open-policy-agent/opa/issues/2142))
- Correct checkptr errors found with Go 1.14
- `opa parse`: fix panic when parsing invalid JSON

### Compatibility Notes

- The `ast.ParseModule` helper will now return an error if an empty module is provided.
  Previously it would return a `nil` error and `nil` module. ([#2054](https://github.com/open-policy-agent/opa/issues/2054))
- The `cmd` and `tester` packages in OPA will now require Go 1.13+ to compile. Most library users should be unaffected.

### Miscellaneous

- bundle: Dedicate `policy.wasm` for the compiled policy.

## 0.17.3

### Fixes

- vendor: Update xxhash to workaround checkptr errors with Go 1.14
- cmd/parse: Fix panic when parsing encounters an error

## 0.17.2

### Fixes

- Add location information into pretty printed trace output. ([#2070](https://github.com/open-policy-agent/opa/issues/2070))
- Add timeout for `http.send` builtin ([#2099](https://github.com/open-policy-agent/opa/issues/2099))
- build: Force module mode and using only the vendor directory ([#2063](https://github.com/open-policy-agent/opa/issues/2063))
- cover: Exclude `some` expressions in coverage report ([#1972](https://github.com/open-policy-agent/opa/issues/1972))
- docs: How to say "ray-go" ([#2106](https://github.com/open-policy-agent/opa/issues/2106))
- topdown: Make http.send() caching use full request ([#1980](https://github.com/open-policy-agent/opa/issues/1980))
- topdown: Wrap all builtin functions for errors normalization ([#2101](https://github.com/open-policy-agent/opa/issues/2101))
- topdown: http.send use provided CA without client certs ([#1976](https://github.com/open-policy-agent/opa/issues/1976))

### Miscellaneous

- Add `object` manipulation built-ins
- docs: Add link to Rego Playground in table of contents
- docs: Update tutorial with note about consistency
- topdown: Export builtin implementations outside the package

## 0.17.1

### Fixes

- ast: Fix rewriting vars in rule args ([#2080](https://github.com/open-policy-agent/opa/issues/2080))

## 0.17.0

### Major Features

- This release improves partial evaluation to avoid saving statements when they
  do not depend on unknowns (e.g., comprehensions, references that require
  materializing the full extent of partial sets/objects, etc.) Also, expressions
  containing `with` statements are partially evaluated now.

- This release lets policy authors to de-reference calls (and other terms)
  without assigning the result to an intermediate variable. For example, instead
  of writing `a := f(x); a.foo == 1` users can now write `f(x).a == 1` directly.
  Thanks @jaspervdj-luminal!

### New Built-in Functions

This release includes the following new built-in functions:

- `object.get` built-in function to lookup object keys with a fallback.
- `crypto.md5`, `crypto.sha1`, `crypto.sha256` built-in functions to hash strings.

### Compatibility Notes

- The `glob.match` built-in function was not defaulting the delimiter to "."
  like the documentation described. This was fixed in
  [#2061](https://github.com/open-policy-agent/opa/pull/2061) however the fix
  is not backwards compatible. If you are using the `glob.match` built-in
  function, you should ensure that a delimiter is being supplied. A search of
  .rego files on GitHub only revealed a few instances of the `glob.match` in-use
  so we decided to err towards fixing the broken behaviour rather than
  preserving buggy behaviour going forward.

- Related to the fix for [#2031](https://github.com/open-policy-agent/opa/issues/2031)
  and the changes with OPA v0.16.0 to use `/` separated `path`'s with
  the decision log plugin API. The decision logger will no longer modify
  the `server.Info#Path` field. Older versions would substitute `.` for
  `/` but this was causing incorrect results. As of v0.16.0 the server has
  been updated to provide the correct paths so REST API users are unaffected.
  Golang API users of the `plugins.log.Logger#Log` interface may be impacted
  if passing `ast.Ref` style strings as a path as it will no longer be changed
  to `/` separated. Callers need to do any transformation beforehand.

### Fixes

- docs: Update Kubernetes apiVersions to use `apps/v1` instead of `extensions/v1` ([#1977](https://github.com/open-policy-agent/opa/issues/1977))
- plugins/logs: Leave the path unchanged for decisions ([#2031](https://github.com/open-policy-agent/opa/issues/2031))
- plugins/bundle: Include last successful request timestamp in status ([#2009](https://github.com/open-policy-agent/opa/issues/2009))
- plugins/bundle: Pass copy of status to bulk listeners ([#1962](https://github.com/open-policy-agent/opa/issues/1962))
- rego: Fix panic when partial evaluating with tracers ([#2007](https://github.com/open-policy-agent/opa/issues/2007))
- rego: Propagate custom builtins to `PartialResult` ([#1792](https://github.com/open-policy-agent/opa/issues/1792))
- server: Update health check to use plugin status ([#2010](https://github.com/open-policy-agent/opa/issues/2010))
- topdown: Correct glob default delimeter ([#2039](https://github.com/open-policy-agent/opa/issues/2039))

### Miscellaneous


- ast: Do not index expressions containing with statements
- ast: Fix panic in module parsing
- ast: Improve visitor performance by avoiding heap allocations
- cmd/build: return error if there is more than one positional argument
- docs: Fix live docs button size
- docs: Fix token validation example in Envoy tutorial

## 0.16.2

This release includes an important bugfix for users that enable
tracing and use the "pretty" trace formatter.

- topdown: Fix bug in var rewriting during trace formatting ([#2022](https://github.com/open-policy-agent/opa/issues/2022))

## 0.16.1

### Fixes

- Fix for `*-rootless` Docker images `USER` being set incorrectly ([#1982](https://github.com/open-policy-agent/opa/issues/1982))

## 0.16.0

### New Built-in Functions

- Add `json.filter` to mask/filter nested fields ([#1617](https://github.com/open-policy-agent/opa/issues/1617))
- Add `net.cidr_expand` to generate CIDR hosts

### Fixes

- Reduce server latency for indexed policies by ~30-40% by caching prepared queries across requests ([#1958](https://github.com/open-policy-agent/opa/issues/1567))
- Improve type checker error and trace output readability ([#1430](https://github.com/open-policy-agent/opa/issues/1430) and [#1208](https://github.com/open-policy-agent/opa/issues/1208))
- Re-create service clients to pickup certificate changes ([#1898](https://github.com/open-policy-agent/opa/issues/1898))
- Report full system path for bundle file locations ([#1796](https://github.com/open-policy-agent/opa/issues/1796))
- Add `status.console` option to log Status messages to console ([#1937](https://github.com/open-policy-agent/opa/issues/1937))
- Fix `io.jwt.decode_verify` to support multiple keys in JWKS ([#1901](https://github.com/open-policy-agent/opa/issues/1901))
- Fix `path` decision log field for queries against "/data" ([1532](https://github.com/open-policy-agent/opa/issues/1532))

This release also includes:

- Documentation improvements on how to use the `io.jwt.*` built-in functions for token verification
- Metrics for bundle processing and activation (e.g., read, parse, and compile times)
- Better parse metric reporting in the server

### Compatibility Notes

- The fix for #1532 required a backwards incompatible change for v0 and v1
  queries against "/data". This only affects queries against the exact path
  "/data", not paths prefixed with "/data/", e.g., "/data/example/allow". Since
  queries against "/data" are rare and normally only seen in development
  environments, this is a low-impact change. As part of this change, the
  `server.Info#Path` field has been changed to use slash-separated paths instead
  of the string representation of Rego references (i.e., a dotted path rooted at
  "data"). If you are registering a logging callback directly against the server
  (e.g., by calling `server.Server#WithDecisionLoggerWithErr`) you will have to
  update your logging callback to deal with the new path format. Decision log
  consumers should treat a missing/empty `path` field as a query against
  "/data".

## 0.15.1

In this release we reached a milestone for Wasm: any Rego policy can
be compiled to Wasm now! In the next few weeks we will focus on
expanding on the set of built-ins supported out-of-the-box and inside
the NodeJS SDK.

### Fixes

- bundle: Make the DirectoryLoader public ([#1840](https://github.com/open-policy-agent/opa/issues/1840))
- topdown: Add raw_body parameter to http.send ([#1903](https://github.com/open-policy-agent/opa/issues/1903))
- wasm: Update planner to support with keyword ([#1116](https://github.com/open-policy-agent/opa/issues/1116))

### Miscellaneous

- ast: Fix NoWith helper on exprs
- ast: Fix module JSON unmarshalling
- build: Fix build-release.sh by removing obsolete make deps command
- docs: Add JWT verification examples to reference
- docs: Fix introduction to refer to rules consistently
- topdown: Add environment variable to dump tests to disk
- topdown: Provide rewritten query vars in traces
- types: Fix constant select on array types
- wasm: Add support for built-in functions
- wasm: Extend wasm library to support shallow copying
- wasm: Fix JSON string lexing and parsing
- wasm: Fix object insertion operation
- wasm: Fix opa_json_dump to terminate keywords properly
- wasm: Fix opa_set_add to set next element correctly
- wasm: Fix planner to check call expression for false return value
- wasm: Fix planning of virtual document extent
- wasm: Improve calling convention of eval() function
- wasm: Improve planner to reuse local variables
- wasm: Fix planner to plan default rule bodies
- wasm: Remove unnecessary condition statements for scans
- wasm: Store parsed numbers as strings
- website: Replace homepage with new version

## 0.15.0

This release includes many small improvements and bug fixes.

### Built-in Functions

This release includes a few new built-in functions for string
manipulation:

- `trim_left`, `trim_right`, `trim_prefix`, and `trim_suffix` (thanks @hasit)
- `regex.find_all_string_submatch_n` and `strings.replace_n` (thanks @kenfdev)

### Fixes

- tester: Fix --timeout to apply to each test case ([#1788](https://github.com/open-policy-agent/opa/issues/1788))
- ast: Check for undefined functions before safety check ([#1141](https://github.com/open-policy-agent/opa/issues/1141))
- ast: Fix object corruption during local rewrite ([#1852](https://github.com/open-policy-agent/opa/issues/1852))
- ast: Fix virtual predicate used for rule index build ([#1863](https://github.com/open-policy-agent/opa/issues/1863))
- discovery: Fix log level message when on HTTP 304 ([#1826](https://github.com/open-policy-agent/opa/issues/1826))
- docs: Update Kubernetes primer test to avoid false-positives ([#1794](https://github.com/open-policy-agent/opa/issues/1794))
- repl: Fix unknown argument processing ([#1670](https://github.com/open-policy-agent/opa/issues/1670))
- topdown: Fix namespacing to use caller bindings ([#1814](https://github.com/open-policy-agent/opa/issues/1814))
- topdown: Fix units.parse_bytes implementation to use int64 ([#1815](https://github.com/open-policy-agent/opa/issues/1815))
- topdown: Fix base document dereference with composite ([#1057](https://github.com/open-policy-agent/opa/issues/1057))
- wasm: Add support for comprehensions ([#1120](https://github.com/open-policy-agent/opa/issues/1120))
- wasm: Add support for full virtual document model ([#1117](https://github.com/open-policy-agent/opa/issues/1117))
- wasm: Remove memory.grow calls on every malloc ([#1121](https://github.com/open-policy-agent/opa/issues/1121))

### Miscellaneous

- build: Migrate to Go modules from Glide for dependency management
- build: Fix *-debug docker images to be ":debug" tag based
- ast: Replace "var" with "some" in SomeDecl#String
- ast: Add map of rewritten vars to Compiler
- topdown: Add API to disable rule indexing for evaluation
- bundle: Add more details to manifest root errors
- bundle: Ensure data paths use `/` separators for key
- bundle: Fix for overwriting data file keys
- cmd: Ensure all errors are in JSON formatted CLI output
- cmd: Add source output format for partial eval
- cmd: Fix opa eval to specify profiler tracer correctly
- discovery: Support `resource` configuration option
- rego: Don't propagate non-threadsafe fields from Rego to preparedQuery

## 0.14.2

- topdown: Fix namespacing to use caller bindings ([#1814](https://github.com/open-policy-agent/opa/issues/1814))
- file/loader: Standardize on forward slash paths

## 0.14.1

- Fix a number of links in the OPA documentation.
- Fix issue with bundle root path comparisons on Windows.

## 0.14.0

This release includes a large number of improvements to the docs as
well as performance optimizations that improve several end-to-end
benchmarks by ~25%. Also, the `opa eval` and other sub-commands now
accept a `-b` or `--bundle` flag that tell OPA to treat file paths as
bundles (either .tar.gz or directories). This improves behaviour in
large or mixed workspaces.

### Compatibility Notes

- Status API messages now include a dump of OPA's Prometheus metric
  registry. This increases the Status API message size significantly
  (~6KB). If you are indexing the the Status API messages, consider
  removing the metrics. Nonetheless, for Status API implementations,
  having access to the Prometheus metrics is important for monitoring
  the health of the OPAs.

### Built-in Functions

This release includes a few improvements to built-in functions:

* A new function for converting SI strings (e.g., "10MB") to numbers:
  `units.num_bytes(x)`
  ([#1561](https://github.com/open-policy-agent/opa/issues/1561)). This
  is useful in the context of Kubernetes if you need to deal with
  resource limits and requests.

* The `io.jwt.verify_*` functions have been extended to support JWKs.

This release also improves support for providing custom built-in
functions to OPA. See the extensions documentation on openpolicyagent.org.

### Fixes

- ast, rego: Refactor unsafe built-in handling ([#1666](https://github.com/open-policy-agent/opa/issues/1666))
- ast: Fix ordering of rule type checking errors ([#1620](https://github.com/open-policy-agent/opa/issues/1620))
- ast: Update rule head to track assignments ([#1541](https://github.com/open-policy-agent/opa/issues/1541))
- ast: Fix bug that allowed recursion in dynamic refs ([#1565](https://github.com/open-policy-agent/opa/issues/1565))
- ast: Fix parsing of var-like scalars ([#1582](https://github.com/open-policy-agent/opa/issues/1582))
- docs: Add note about benchmark result page ([#1275](https://github.com/open-policy-agent/opa/issues/1275))
- docs: Update to show undefined example with != ([#1626](https://github.com/open-policy-agent/opa/issues/1626))
- docs: Update to use live blocks ([#1650](https://github.com/open-policy-agent/opa/issues/1650))
- format: Fix formatter to start line after writing comments ([#1560](https://github.com/open-policy-agent/opa/issues/1560))
- loader: Update to accept file:// URLs. ([#1505](https://github.com/open-policy-agent/opa/issues/1505))
- server: Improve decision log-related error messages ([#1367](https://github.com/open-policy-agent/opa/issues/1367))

### Miscellaneous

- Add support for fuzzing the ast package in CI
- Add search bar powered by Algolia to the docs
- Add "type" field to decision log events sent to the console
- Add support for := assignments at file level
- Add build commit and version to runtime info
- Fix moduleLoader to copy returned parsed Modules
- Fix panic in /health?bundle=true
- Update the --plugin-dir flag as deprecated
- Update formatter to preserve rule assigmemnts
- Update metrics object to be thread-safe
- Support loading bundles and files w/ Rego API

## 0.13.5

- Fix panic in OPA HTTP server with `/health?bundle=true` when
  using bundles loaded from CLI ([#1703](https://github.com/open-policy-agent/opa/issues/1703)).

## 0.13.4

- Fix panic in OPA HTTP server caused by concurrent map writes ([#1666](https://github.com/open-policy-agent/opa/issues/1666))

## 0.13.3

### Fixes

- Fix bundle plugin to report error in case bundle manifest roots overlap ([#1635](https://github.com/open-policy-agent/opa/issues/1635))

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
  OPA. See the [Bundles](https://www.openpolicyagent.org/docs/latest/management-bundles)
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

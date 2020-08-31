---
title: Management APIs
kind: operations
weight: 1
restrictedtoc: true
---

OPA exposes a set of APIs that enable unified, logically centralized policy
management. Read this page if you are interested in how to build a control plane
around OPA that enables policy distribution and collection of important
telemetry data like decision logs.

## Overview

OPA enables low-latency, highly-available policy enforcement by providing a
lightweight engine for distributed architectures. By default, all of the policy
and data that OPA uses to make decisions is kept in-memory:

<!--- source: https://docs.google.com/drawings/d/1-dwGFRjv_nFydo-8tOK-C-PbWyjvRObYhePC7XaLUFw/edit?usp=sharing --->

{{< figure src="integration.svg" width="55" caption="Host-local Architecture" >}}

OPA is designed to enable _distributed_ policy enforcement. You can run OPA next
to each and every service that needs to offload policy decision-making. By
colocating OPA with the services that require decision-making, you ensure that
policy decisions are rendered as fast as possible and in a highly-available
manner.

<!--- source: https://docs.google.com/drawings/d/1wFef9_Smy0gNvJj4l8n05WCTqhmzdadiyspyRGFvHuw/edit?usp=sharing --->

{{< figure src="distributed.svg" width="55" caption="Distributed Enforcement" >}}

To control and observe a set of OPAs, each OPA can be configured to connect to
management APIs that enable:

* Policy distribution (Bundles)
* Decision telemetry (Decision Logs)
* Agent telemetry (Status)
* Dynamic agent configuration (Discovery)

By configuring and implementing these management APIs you can unify control and
visibility over OPAs in your environments. OPA does not provide a control plane
service out-of-the-box today.

<!--- source: https://docs.google.com/drawings/d/1-08mHgUN5oy2phLJ6MOr7j3e0iguxg_X__3VH321iLc/edit?usp=sharing --->

{{< figure src="control-plane.svg" width="55" caption="Control Plane" >}}

## Bundles

OPA can periodically download bundles of policy and data from remote HTTP
servers. The policies and data are loaded on the fly without requiring a
restart of OPA. Once the policies and data have been loaded, they are enforced
immediately. Policies and data loaded from bundles are accessible via the
standard OPA [REST API](../rest-api).

Bundles provide an alternative to pushing policies into OPA via the REST APIs.
By configuring OPA to download bundles from a remote HTTP server, you can
ensure that OPA has an up-to-date copy of policies and data required for
enforcement at all times.

By default, the OPA REST APIs will prevent you from modifying policy and data
loaded via bundles. If you need to load policy and data from multiple sources,
see the section below.

See the [Configuration Reference](../configuration) for configuration details.

### Bundle Service API

OPA expects the service to expose an API endpoint that serves bundles. The
bundle API should allow clients to download bundles at an arbitrary URL. In
combination with a service's `url` path.

```http
GET /<service path>/<resource> HTTP/1.1
```

If the bundle exists, the server should respond with an HTTP 200 OK status
followed by a gzipped tarball in the message body.

```http
HTTP/1.1 200 OK
Content-Type: application/gzip
```

Enable bundle downloading via configuration. For example:

```yaml
services:
  - name: acmecorp
    url: https://example.com/service/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

bundles:
  authz:
    service: acmecorp
    resource: somedir/bundle.tar.gz
    polling:
      min_delay_seconds: 10
      max_delay_seconds: 20
    signing:
      keyid: my_global_key
      scope: read
```

Using this configuration, OPA will fetch bundles from
`https://example.com/service/v1/somedir/bundle.tar.gz`.

The URL is constructed as follows:

```
https://example.com/service/v1/somedir/bundle.tar.gz
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^
services[0].url                resource
```

If the `bundles[_].resource` field is not defined, the value defaults to
`bundles/<name>` where the `name` is the key value in the configuration. For the
example above this is `authz` and would default to `bundles/authz`.

Bundle names can have any valid YAML characters in them, including `/`. This can
be useful when relying on default `resource` behavior with a name like
`authz/bundle.tar.gz` which results in a `resource` of
`bundles/authz/bundle.tar.gz`.

The optional `bundles[_].signing` field can be used to specify the `keyid` and `scope` that should be used
for verifying the signature of the bundle. See [this](#bundle-signature) section for details.

See the following section for details on the bundle file format.

> Note: The `bundle` config keyword will still work with the current versions
  of OPA, but has been deprecated. It is highly recommended to switch to the
  `bundles` configuration.

#### Caching

Services implementing the Bundle Service API should set the HTTP `Etag` header
in bundle responses to identify the revision of the bundle. OPA will include the
`Etag` value in the `If-None-Match` header of bundle requests. Services can
check the `If-None-Match` header and reply with HTTP `304 Not Modified` if the
bundle has not changed since the last update.

### Bundle File Format

Bundle files are gzipped tarballs that contain policies and data. The data
files in the bundle must be organized hierarchically into directories inside
the tarball.

> The hierarchical organization indicates to OPA where to load the data files
> into the [the `data` Document](../#the-data-document).

You can list the content of a bundle with `tar`.

```bash
$ tar tzf bundle.tar.gz
.manifest
roles
roles/bindings
roles/bindings/data.json
roles/permissions
roles/permissions/data.json
http
http/example
http/example/authz
http/example/authz/authz.rego
```

In this example, the bundle contains one policy file (`authz.rego`) and two
data files (`roles/bindings/data.json` and `roles/permissions/data.json`).
The bundle may also contain an optional wasm binary file (`policy.wasm`).
It stores the WebAssembly compiled version of all the Rego policy files within
the bundle.

Bundle files may contain an optional `.manifest` file that stores bundle
metadata. The file should contain a JSON serialized object, with the following
fields:

* If the bundle service is capable of serving different revisions of the same
  bundle, the service should include a top-level `revision` field containing a
  `string` value that identifies the bundle revision.

* If you expect to load additional data into OPA from outside the
  bundle (e.g., via OPA's HTTP API) you should include a top-level
  `roots` field containing of path prefixes that declare the scope of
  the bundle. See the section below on managing data from multiple
  sources. If the `roots` field is not included in the manifest it
  defaults to `[""]` which means that ALL data and policy must come
  from the bundle.

* OPA will only load data files named `data.json` or `data.yaml` (which contain
  JSON or YAML respectively). Other JSON and YAML files will be ignored.

* The `*.rego` policy files must be valid [Modules](../policy-language/#modules)

> YAML data loaded into OPA is converted to JSON. Since JSON is a subset of
> YAML, you are not allowed to use binary or null keys in objects and boolean
> and number keys are converted to strings. Also, YAML !!binary tags are not
> supported.

For example, this manifest specifies a revision (which happens to be a Git
commit hash) and a set of roots for the bundle contents. In this case, the
manifest declares that it owns the roots `data.roles` and
`data.http.example.authz`.

```json
{
  "revision" : "7864d60dd78d748dbce54b569e939f5b0dc07486",
  "roots": ["roles", "http/example/authz"]
}
```

### Multiple Sources of Policy and Data

By default, when OPA is configured to download policy and data from a
bundle service, the entire content of OPA's policy and data cache is
defined by the bundle. However, if you need to load OPA with policy
and data from multiple sources, you can implement your bundle service
to generate bundles that are scoped to a subset of OPA's policy and
data cache.

> 🚨 We recommend that whenever possible, you implement policy and data
> aggregation centrally, however, in some cases that's not possible
> (e.g., due to latency requirements.).
> When using multiple sources there are **no** ordering guarantees for which bundle loads first and
  takes over some root. If multiple bundles conflict, but are loaded at different
  times, OPA may go into an error state. It is highly recommended to use
  the health check and include bundle state: [Monitoring OPA](../monitoring#health-checks)

To scope bundles to a subset of OPA's policy and data cache, include
a top-level `roots` key in the bundle that defines the roots of the
`data` namespace that are owned by the bundle.

For example, the following manifest would declare two roots
(`acmecorp/policy` and `acmecorp/oncall`):

```
{
    "roots": ["acmecorp/policy", "acmecorp/oncall"]
}
```

If OPA was loaded with a bundle containing this manifest it would only
erase and overwrite policy and data under these roots. Policy and data
loaded under other roots is left intact.

When OPA loads scoped bundles, it validates that:

* The roots are not overlapping (e.g., `a/b/c` and `a/b` are
  overlapped and will result in an error.) Note: This is *not*
  enforced across multiple bundles. Only within the same bundle
  manifest.

* The policies in the bundle are contained under the roots. This is
  determined by inspecting the `package` statement in each of the
  policy files. For example, given the manifest above, it would be an
  error to include a policy file containing `package acmecorp.other`
  because `acmecorp.other` is not contained in either of the roots.

* The data in the bundle is contained under the roots.

If bundle validation fails, OPA will report the validation error via
the Status API.

### Debugging Your Bundles

When you run OPA, you can provide bundle files over the command line. This
allows you to manually check that your bundles include all of the files that
you intended and that they are structured correctly. For example:

```bash
opa run bundle.tar.gz
```

### Signing

To ensure the integrity of policies (ie. the policies are coming from a trusted source), policy bundles may be
digitally signed so that industry-standard cryptographic primitives can verify their authenticity.

OPA supports digital signatures for policy bundles. Specifically, a signed bundle is a normal OPA bundle that includes
a file named `.signatures.json` that dictates which files should be included in the bundle, what their SHA hashes are,
and of course is cryptographically secure.

When OPA receives a new bundle, it checks that it has been properly signed using a (public) key that OPA has been
configured with out-of-band.  Only if that verification succeeds does OPA activate the new bundle; otherwise, OPA
continues using its existing bundle and reports an activation failure via the status API and error logging.

 > ⚠️ `opa run` performs bundle signature verification only when the `-b`/`--bundle` flag is given
> or when Bundle downloading is enabled. Sub-commands primarily used in development and debug environments
> (such as `opa eval`, `opa test`, etc.) DO NOT verify bundle signatures at this point in time.

#### Signature Format

Recall that a [policy bundle](#bundle-file-format) is a gzipped tarball that contains policies and data. A signed bundle
differs from a normal bundle in that it has a `.signatures.json` file as well.

```bash
$ tar tzf bundle.tar.gz
.manifest
.signatures.json
roles
roles/bindings
roles/bindings/data.json
```

The signatures file is a JSON file with an array of JSON Web Tokens (JWTs) that encapsulate the signatures for the bundle.
Currently, you will be limited to one signature, as shown below. In the future, we may add support to include multiple
signatures to sign different files within the bundle.

```json
{
  "signatures": [ "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiJjMjEzMTU0NGM3MTZhMjVhNWUzMWY1MDQzMDBmNTI0MGU4MjM1Y2FkYjlhNTdmMGJkMWI2ZjRiZDc0YjI2NjEyIiwiYWxnb3JpdGhtIjoiU0hBMjU2In0seyJuYW1lIjoicm9sZXMvYmluZGluZ3MvZGF0YS5qc29uIiwiaGFzaCI6IjQyY2ZlNjc2OGI1N2JiNWY3NTAzYzE2NWMyOGRkMDdhYzViODEzNTU0ZWJjODUwZjJjYzM1ODQzZTcxMzdiMWQifV0sImlhdCI6MTU5MjI0ODAyNywiaXNzIjoiSldUU2VydmljZSIsImtleWlkIjoibXlQdWJsaWNLZXkiLCJzY29wZSI6IndyaXRlIn0.ZjtUgXC6USwmhv4XP9gFH6MzZwpZrGpAL_2sTK1P-mg"]
}
```

The JWT has the standard headers `alg` (for algorithm), `typ` (always JWT), and `kid` (for key id). It has a JSON payload of the
following form:

```json
{
  "files": [
    {
      "name": ".manifest",
      "hash": "c2131544c716a25a5e31f504300f5240e8235cadb9a57f0bd1b6f4bd74b26612",
      "algorithm": "SHA-256"
    },
    {
      "name": "roles/bindings/data.json",
      "hash": "42cfe6768b57bb5f7503c165c28dd07ac5b813554ebc850f2cc35843e7137b1d"
    }
  ],
  "iat": 1592248027,
  "iss": "JWTService",
  "scope": "write"
}
```

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `files[_].name` | `string` | Yes | Path of a file in the bundle. |
| `files[_].hash` | `string` | Yes | Output of the hashing algorithm applied to the file. |
| `files[_].algorithm` | `string` | Yes | Name of the hashing algorithm. |
| `scope` | `string` | No | Represents the fragment of signings. |
| `iat` | `string` | No | Time of signature creation since epoch in seconds. For informational purposes only. |
| `iss` | `string` | No | Identifies the issuer of the JWT. For informational purposes only. |

> Note: OPA will first look for the `keyid` on the command-line. If the `keyid` is empty, OPA will look for it in it's
> configuration. If `keyid` is still empty, OPA will finally look for `kid` in the JWT header.

The following hashing algorithms are supported:

    MD5
    SHA-1
    SHA-224
    SHA-256
    SHA-384
    SHA-512
    SHA-512-224
    SHA-512-256

To calculate the digest for unstructured files (ie. all files except JSON or YAML files), apply the hash
function to the byte stream of the file.

For structured files, read the byte stream and parse into a JSON structure; then recursively order the fields of all
objects alphabetically and then apply the hash function to the result to compute the hash. This ensures
that the digital signature is independent of whitespace and other non-semantic JSON features.

To generate a `.signatures.json` file for policy and data files that will be part of a bundle, see the `opa sign` command.

#### Signature Verification

When OPA receives a policy bundle that doesn't include the `.signatures.json` file and the bundle is not configured to
use a signature, OPA does not perform signature verification and activates the bundle just as it always has.

If the actual bundle contains the `.signatures.json` file but the bundle is not configured to use a signature, verification fails.

| `.signatures.json` exists | bundle configured to verify signature | verification performed | result |
| --- | --- | --- | --- |
| `no` | `no` | `no` | `NA` |
| `no` | `yes` | `yes` | `fail` |
| `yes` | `no` | `yes` | `fail` |
| `yes` | `yes` | `yes` | `depends on the verification steps described below` |

When OPA receives a signed bundle it opens the `.signatures.json` file, grabs the JWT and performs the following steps:

* Verify the JWT signature with the appropriate public key

* Verify that the JWT payload and target directory specify the same set of files

* Verify the content of each file by checking the hash recorded in the JWT payload is the same as the hash generated
for that file

OPA activates the new bundle only if all the verification steps succeed; otherwise, it continues using its existing bundle
and reports an activation failure via the status API and error logging.

The signature verification process uses each of the fields in the JWT header and payload as follows:

* `files`: This list of files in the payload must match exactly the files in the bundle, and for each file the hash of the file must match

* `kid`: If supplied in the header, dictates which key (and algorithm) to use for verification. The actual key is supplied via
OPA out-of-band

* `scope`: If supplied in the payload, must match exactly the value provided out-of-band to OPA

* `iat`: unused for verification even if present in payload

* `iss`: unused for verification even if present in payload



## Decision Logs

OPA can periodically report decision logs to remote HTTP servers. The decision
logs contain events that describe policy queries. Each event includes the policy
that was queried, the input to the query, bundle metadata, and other information
that enables auditing and offline debugging of policy decisions.

When decision logging is enabled the OPA server will include a `decision_id`
field in API calls that return policy decisions.

See the [Configuration Reference](../configuration) for configuration details.

### Decision Log Service API

OPA expects the service to expose an API endpoint that will receive decision logs.

```http
POST /logs[/<partition_name>] HTTP/1.1
Content-Encoding: gzip
Content-Type: application/json
```

The partition name is an optional path segment that can be used to route logs
to different backends. If the partition name is not configured on the agent,
updates will be sent to `/logs`.

The message body contains a gzip compressed JSON array. Each array element (event)
represents a policy decision returned by OPA.

```json
[
  {
    "labels": {
      "app": "my-example-app",
      "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a",
      "version": "{{< current_version >}}"
    },
    "decision_id": "4ca636c1-55e4-417a-b1d8-4aceb67960d1",
    "bundles": {
      "authz": {
        "revision": "W3sibCI6InN5cy9jYXRhbG9nIiwicyI6NDA3MX1d"
      }
    },
    "path": "http/example/authz/allow",
    "input": {
      "method": "GET",
      "path": "/salary/bob"
    },
    "result": "true",
    "requested_by": "[::1]:59943",
    "timestamp": "2018-01-01T00:00:00.000000Z"
  }
]
```

Decision log updates contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `[_].labels` | `object` | Set of key-value pairs that uniquely identify the OPA instance. |
| `[_].decision_id` | `string` | Unique identifier generated for each decision for traceability. |
| `[_].revision` | `string` | (Deprecated) Bundle revision that contained the policy used to produce the decision. Omitted when `bundles` are configured.  |
| `[_].bundles` | `object` | Set of key-value pairs describing the bundles which contained policy used to produce the decision. |
| `[_].bundles[_].revision` | `string` | Revision of the bundle at the time of evaluation. |
| `[_].path` | `string` | Hierarchical policy decision path, e.g., `/http/example/authz/allow`. Receivers should tolerate slash-prefixed paths. |
| `[_].query` | `string` | Ad-hoc Rego query received by Query API. |
| `[_].input` | `any` | Input data provided in the policy query. |
| `[_].result` | `any` | Policy decision returned to the client, e.g., `true` or `false`. |
| `[_].requested_by` | `string` | Identifier for client that executed policy query, e.g., the client address. |
| `[_].timestamp` | `string` | RFC3999 timestamp of policy decision. |
| `[_].metrics` | `object` | Key-value pairs of [performance metrics](../rest-api#performance-metrics). |
| `[_].erased` | `array[string]` | Set of JSON Pointers specifying fields in the event that were erased. |
| `[_].masked` | `array[string]` | Set of JSON Pointers specifying fields in the event that were masked. |

### Local Decision Logs

Local console logging of decisions can be enabled via the `console` config option.
This does not require any remote server. Example of minimal config to enable:

```yaml
decision_logs:
    console: true
```

This will dump all decision through the OPA logging system at the `info` level. See
[Configuration Reference](../configuration) for more details.

### Masking Sensitive Data

Policy queries may contain sensitive information in the `input` document that
must be removed or modified before decision logs are uploaded to the remote API
(e.g., usernames, passwords, etc.) Similarly, parts of the policy decision itself may
be considered sensitive.

By default, OPA queries the `data.system.log.mask` path prior to encoding and
uploading decision logs or calling custom decision log plugins.

OPA provides the decision log event as input to the policy query and expects
the query to return a set of JSON Pointers that refer to fields in the decision
log event to either **erase** or **modify**.

For example, assume OPA is queried with the following `input` document:

```json
{
  "resource": "user",
  "name": "bob",
  "password": "passw0rd"
}
```

To **remove** the `password` field from decision log events related to "user"
resources, supply the following policy to OPA:

```ruby
package system.log

mask["/input/password"] {
  # OPA provides the entire decision log event as input to the masking policy.
  # Refer to the original input document under input.input.
  input.input.resource == "user"
}

# To mask certain fields unconditionally, omit the rule body.
mask["/input/ssn"]
```

When the masking policy generates one or more JSON Pointers, they will be erased
from the decision log event. The erased paths are recorded on the event itself:

```json
{
  "decision_id": "b4638167-7fcb-4bc7-9e80-31f5f87cb738",
  "erased": [
    "/input/password",
    "/input/ssn"
  ],
  "input": {
    "name": "bob",
    "resource": "user"
  },
------------------------- 8< -------------------------
  "path": "system/main",
  "requested_by": "127.0.0.1:36412",
  "result": true,
  "timestamp": "2019-06-03T20:07:16.939402185Z"
}
```

There are a few restrictions on the JSON Pointers that OPA will erase:

* Pointers must be prefixed with `/input` or `/result`.
* Pointers may be undefined. For example `/input/name/first` in the example
  above would be undefined. Undefined pointers are ignored.
* Pointers must refer to object keys. Pointers to array elements will be treated
  as undefined. For example `/input/emails/0/value` is allowed but `/input/emails/0` is not.

In order to **modify** the contents of an input field, the **mask** rule may utilize the following format.

* `"op"` -- The operation to apply when masking. All operations are done at the
  path specified.  Valid options include:
    |  op | Description  |
    |-----|--------------|
    |  `"remove"` | The `"path"` specified will be removed from the resulting log message. The `"value"` mask field is ignored for `"remove"` operations.  |
    |  `"upsert"` | The `"value"` will be set at the specified `"path"`. If the field exists it is overwritten, if it does not exist it will be added to the resulting log message.  |

* `"path"` -- A JSON pointer path to the field to perform the operation on.

Optional Fields:

* `"value"` -- Only required for `"upsert"` operations. 

> This is processed for every decision being logged, so be mindful of
  performance when performing complex operations in the mask body, eg. crypto
  operations

```ruby
package system.log

mask[{"op": "upsert", "path": "/input/password", "value": x}] {
  # conditionally upsert password if it existed in the orginal event
  input.input.password
  x := "**REDACTED**"
}
```

To always **upsert** a value, even if it didn't exist in the original event,
the following rule format can be used.

```ruby
package system.log

# always upsert, no conditions in rule body
mask[{"op": "upsert", "path": "/input/password", "value": x}] {
  x := "**REDACTED**"
}
```

The result of this mask operation on the decision log event produces
the following output. Notice that the **mask** event field exists
to track **remove** vs **upsert** mask operations.

```json
{
  "decision_id": "b4638167-7fcb-4bc7-9e80-31f5f87cb738",
  "erased": [
    "/input/ssn"
  ],
  "masked": [
    "/input/password"
  ],
  "input": {
    "name": "bob",
    "resource": "user",
    "password": "**REDACTED**"
  },
------------------------- 8< -------------------------
  "path": "system/main",
  "requested_by": "127.0.0.1:36412",
  "result": true,
  "timestamp": "2019-06-03T20:07:16.939402185Z"
}
```

## Status

OPA can periodically report status updates to remote HTTP servers. The
updates contain status information for OPA itself as well as the
[Bundles](#bundles) that have been downloaded and activated.

OPA sends status reports whenever one of the following happens:

* Bundles are downloaded and activated -- If the bundle download or activation fails for any reason, the status update
  will include error information describing the failure. This includes Discovery bundles.
* A plugin state has changed -- All plugin status is reported, and an update to any plugin will
  trigger a Status API report which contains the latest state.

The status updates will include a set of labels that uniquely identify the
OPA instance. OPA automatically includes an `id` value in the label set that
provides a globally unique identifier or the running OPA instance and a
`version` value that provides the version of OPA.

See the [Configuration Reference](../configuration) for configuration details.

### Status Service API

OPA expects the service to expose an API endpoint that will receive status
updates.

```http
POST /status[/<partition_name>] HTTP/1.1
Content-Type: application/json
```

The partition name is an optional path segment that can be used to route
status updates to different backends. If the partition name is not configured
on the agent, updates will be sent to `/status`.

```json
{
  "labels": {
    "app": "my-example-app",
    "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a",
    "version": "{{< current_version >}}"
  },
  "bundles": {
    "http/example/authz": {
      "active_revision": "ABC",
      "last_request": "2018-01-01T00:00:00.000Z",
      "last_successful_request": "2018-01-01T00:00:00.000Z",
      "last_successful_download": "2018-01-01T00:00:00.000Z",
      "last_successful_activation": "2018-01-01T00:00:00.000Z",
      "metrics": {
        "timer_rego_data_parse_ns": 12345,
        "timer_rego_module_compile_ns": 12345,
        "timer_rego_module_parse_ns": 12345
      }
    }
  },
  "plugins": {
    "bundle": {
      "state": "OK"
    },
    "discovery": {
      "state": "OK"
    },
    "status": {
      "state": "OK"
    }
  },
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
------------------------------8< SNIP 8<------------------------------
      "http_request_duration_seconds": {
        "help": "A histogram of duration for requests.",
        "metric": [
          {
            "histogram": {
              "bucket": [
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.005
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.01
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.025
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.05
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.1
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.25
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 1
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 2.5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 10
                }
              ],
              "sample_count": 1,
              "sample_sum": 0.003157399
            },
            "label": [
              {
                "name": "code",
                "value": "200"
              },
              {
                "name": "handler",
                "value": "v1/data"
              },
              {
                "name": "method",
                "value": "get"
              }
            ]
          }
        ],
        "name": "http_request_duration_seconds",
        "type": 4
      }
    }
  }
}
```

Status updates contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `labels` | `object` | Set of key-value pairs that uniquely identify the OPA instance. |
| `bundle.name` | `string` | (Deprecated) Name of bundle that the OPA instance is configured to download. Omitted when `bundles` are configured. |
| `bundle.active_revision` | `string` | (Deprecated) Opaque revision identifier of the last successful activation. Omitted when `bundles` are configured. |
| `bundle.last_successful_download` | `string` | (Deprecated) RFC3339 timestamp of last successful bundle download. Omitted when `bundles` are configured. |
| `bundle.last_successful_activation` | `string` | (Deprecated) RFC3339 timestamp of last successful bundle activation. Omitted when `bundles` are configured. |
| `bundles` | `object` | Set of objects describing the status for each bundle configured with OPA. |
| `bundles[_].name` | `string` | Name of bundle that the OPA instance is configured to download. |
| `bundles[_].active_revision` | `string` | Opaque revision identifier of the last successful activation. |
| `bundles[_].last_request` | `string` | RFC3339 timestamp of last bundle request. This timestamp should be >= to the successful request timestamp in normal operation. |
| `bundles[_].last_successful_request` | `string` | RFC3339 timestamp of last successful bundle request. This timestamp should be >= to the successful download timestamp in normal operation. |
| `bundles[_].last_successful_download` | `string` | RFC3339 timestamp of last successful bundle download. |
| `bundles[_].last_successful_activation` | `string` | RFC3339 timestamp of last successful bundle activation. |
| `bundles[_].metrics` | `object` | Metrics from the last update of the bundle. |
| `discovery.name` | `string` | Name of discovery bundle that the OPA instance is configured to download. |
| `discovery.active_revision` | `string` | Opaque revision identifier of the last successful discovery activation. |
| `discovery.last_request` | `string` | RFC3339 timestamp of last discovery bundle request. This timestamp should be >= to the successful request timestamp in normal operation. |
| `discovery.last_successful_request` | `string` | RFC3339 timestamp of last successful discovery bundle request. This timestamp should be >= to the successful download timestamp in normal operation. |
| `discovery.last_successful_download` | `string` | RFC3339 timestamp of last successful discovery bundle download. |
| `discovery.last_successful_activation` | `string` | RFC3339 timestamp of last successful discovery bundle activation. |
| `plugins` | `object` | A set of objects describing the state of configured plugins in OPA's runtime. |
| `plugins[_].state` | `string` | The state of each plugin. |
| `metrics.prometheus` | `object` | Global performance metrics for the OPA instance. |

If the bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `bundle.code` | `string` | If present, indicates error(s) occurred. |
| `bundle.message` | `string` | Human readable messages describing the error(s). |
| `bundle.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

If the discovery bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `discovery.code` | `string` | If present, indicates error(s) occurred. |
| `discovery.message` | `string` | Human readable messages describing the error(s). |
| `discovery.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

Services should reply with HTTP status `200 OK` if the status update is
processed successfully.

### Local Status Logs

Local console logging of status updates can be enabled via the `console` config option.
This does not require any remote server. Example of minimal config to enable:

```yaml
status:
    console: true
```
This will dump all status updates through the OPA logging system at the `info` level. See
[Configuration Reference](../configuration) for more details.

> Warning: Status update messages are somewhat infrequent but can be very verbose! The
>`metrics.prometheus` portion of the status update in particular can create a considerable
> amount of log text at info level.

## Discovery

OPA can be configured to download bundles of policy and data, report status, and
upload decision logs to remote endpoints. The discovery feature helps you
centrally manage the OPA configuration for these features. You should use the
discovery feature if you want to avoid managing OPA configuration updates in
number of different locations.

When the discovery feature is enabled, OPA will periodically download a
*discovery bundle*. Like regular bundles, the discovery bundle may contain JSON
and Rego files. OPA will evaluate the data and policies contained in the
discovery bundle to generate the rest of the configuration. There are two main
ways to structure the discovery bundle:

1. Include static JSON configuration files that define the OPA configuration.
2. Include Rego files that can be evaluated to produce the OPA configuration.

> If you need OPA to select which policy to download dynamically (e.g., based on
> environment variables like the region where OPA is running), use the second
> option.

If discovery is enabled, other features like bundle downloading and status
reporting **cannot** be configured manually. Similarly, discovered configuration
cannot override the original discovery settings in the configuration file that
OPA was booted with.

See the [Configuration Reference](../configuration) for configuration details.

### Discovery Service API

OPA expects the service to expose an API endpoint that serves bundles.

```http
GET /<service_url>/<discovery.resource>HTTP/1.1
```

If the bundle exists, the server should respond with an HTTP 200 OK status
followed by a gzipped tarball in the message body.

```http
HTTP/1.1 200 OK
Content-Type: application/gzip
```

You can enable discovery with an OPA configuration file similar to the example
below. In some places in the documentation, the initial configuration provided
to OPA is referred to as the "boot configuration".

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

discovery:
  name: example
  resource: /configuration/example/discovery.tar.gz
  service: acmecorp
  signing:
    keyid: my_global_key
    scope: read
```

Using the boot configuration above, OPA will fetch discovery bundles from:

```
https://example.com/control-plane-api/v1/configuration/example/discovery.tar.gz
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
services[discovery.service].url          discovery.resource
```

> The `discovery.resource` field defaults to `bundles/<discovery.name>`. The default
is convenient if you want to serve discovery bundles and normal bundles from the same API
endpoint. If only one service is defined, there is no need to set `discovery.service`.

> The `discovery.prefix` configuration option is still available but has been
  deprecated in favor of `discovery.resource`. It will eventually be removed.

> The optional `discovery.signing` field can be used to specify the `keyid` and `scope` that should be used
> for verifying the signature of the discovery bundle. See [this](#discovery-bundle-signature) section for details.


OPA generates it's subsequent configuration by querying the Rego and JSON files
contained inside the discovery bundle. The query is defined by the
`discovery.name` field from the boot configuration: `data.<discovery.name>`. For
example. with the boot configuration above, OPA executes the following query:

```
data.example
```

As an alternative, you can also provide a `decision` field, to specify the name of the query. For example, with this configuration:
```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"
discovery:
  name: example
  resource: /configuration/example/discovery.tar.gz
  decision: example/discovery
```
OPA executes the following query:
```
data.example.discovery
```

If the discovery bundle contained the following Rego file:

```ruby
package example

discovery = {
  "bundles": {
    "main": {
      "service": "acmecorp",
      "resource": bundle_name
    },
  },
  "default_decision": "acmecorp/httpauthz/allow"
}

bundle_name = "acmecorp/httpauthz"
```

The subsequent configuration would be:

```json
{
  "bundles": {
    "main": {
      "service": "acmecorp",
      "resource": "acmecorp/httpauthz"
    },
  },
  "default_decision": "acmecorp/httpauthz/allow"
}
```

The discovery bundle contents above are essentially static. The same result
could be achieved by constructing the discovery bundle with a static JSON file:

```json
{
  "example": {
    "discovery": {
      "bundles": {
        "main": {
          "service": "acmecorp",
          "resource": "acmecorp/httpauthz"
        },
      },
      "default_decision": "acmecorp/httpauthz/allow"
    }
  }
}
```

> For an example of how to configure OPA dynamically see the [Example](#example)
> section below.

The subsequent configuration does not have to specify `services` or include a
reference to a service in the `bundle`, `status,` or `decision_log` sections. If
the either the `services` or references to services are missing, OPA will
default them to the value from the boot configuration.

### Example

Let's see an example of how the discovery feature can be used to dynamically
configure an OPA to download one of two bundles based on a label in the boot
configuration. Let's say the label `region` indicates the region in which the
OPA is running and it's value will decide the bundle to download.

Below is a policy file which generates an OPA configuration.

**example.rego**

```ruby
package example

discovery = {
  "bundles": {
    "main": {
      "service": "acmecorp",
      "resource": bundle_name #line 7
    }
  }
}

rt = opa.runtime()
region = rt.config.labels.region
bundle_name = region_bundle[region]

# region-bundle information
region_bundle = {
  "US": "example/test1/p",
  "UK": "example/test2/p"
}
```

The `bundle_name` variable in `line 7` of the above policy will be dynamically
selected based on the value of the label `region`. So if an OPA was started
with `region: "US"`, then the `bundle_name` will be `example/test1/p`.

Start an OPA with a boot configuration as shown below:

**config.yaml**

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

discovery:
  name: /example/discovery

labels:
  region: "US"
```

Run OPA:

```bash
opa run -s -c config.yaml
```

You should see a log like below, which shows the bundle being downloaded. In
this case, the bundle name is `example/test1/p` as `region` is `US`.

```raw
INFO Bundle downloaded and activated successfully. name=example/test1/p plugin=bundle
```

Now start another OPA with a boot configuration as shown below. Notice the
`region` is `UK`:

**config.yaml**

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

discovery:
  name: /example/discovery

labels:
  region: "UK"
```

Run OPA:

```bash
opa run -s -c config.yaml
```

In this case, the bundle being downloaded is `example/test2/p` as `region` is
`UK`.

```raw
INFO Bundle downloaded and activated successfully. name=example/test2/p plugin=bundle
```

This shows how the discovery feature can help in centrally managing the bundle
to be downloaded by an OPA based on a configuration label. You can use the same
strategy to dynamically configure other plugins based on the running OPA's
configuration labels or environment variables.

### Limitations

In practice, discovery services do not change frequently. These configuration sections are treated as
immutable to avoid accidental configuration errors rendering OPA unable to discover a new configuration.
If the discovered configuration changes the `discovery` or `labels` sections,
those changes are ignored. If the discovered configuration changes the discovery service,
an error will be logged.

### Discovery Bundle Signature

Like regular bundles, if the discovery bundle contains a `.signatures.json` file, OPA will verify the discovery
bundle before activating it. The format of the `.signatures.json` file and the verification steps are same as that for
regular bundles. Since the discovered configuration ignores changes to the `discovery` section, any key used for
signature verification of a discovery bundle **CANNOT** be modified via discovery.

> 🚨 We recommend that if you are using discovery you should be signing the discovery bundles because those bundles
> include the keys used to verify the non-discovery bundles. However, OPA does not enforce that recommendation. You may use
> unsigned discovery bundles that themselves require non-discovery bundles to be signed.

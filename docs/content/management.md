---
title: Management
kind: operations
weight: 1
restrictedtoc: true
---

## Monitoring
### Prometheus

OPA exposes an HTTP endpoint that can be used to collect performance metrics
for all API calls. The Prometheus endpoint is enabled by default when you run
OPA as a server.

You can enable metric collection from OPA with the following `prometheus.yml` config:

```yaml
global_config:
  scrape_interval: 15s
scrape_configs:
  - job_name: "opa"
    metrics_path: "/metrics"
    static_configs:
    - targets:
      - "localhost:8181"
```

### Health Checks

OPA exposes a `/health` API endpoint that can be used to perform health checks.
See [Health API](../rest-api#health-api) for details.


## Status


OPA can periodically report status updates to remote HTTP servers. The
updates contain status information for OPA itself as well as the
[Bundles](#bundles) that have been downloaded and activated.

OPA sends status reports whenever bundles are downloaded and activated. If
the bundle download or activation fails for any reason, the status update
will include error information describing the failure.

The status updates will include a set of labels that uniquely identify the
OPA instance. OPA automatically includes an `id` value in the label set that
provides a globally unique identifier or the running OPA instance and a
`version` value that provides the version of OPA.

See the [Configuration Reference](#configuration-reference) for configuration details.

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
            "active_revision": "TODO",
            "last_successful_download": "2018-01-01T00:00:00.000Z",
            "last_successful_activation": "2018-01-01T00:00:00.000Z"
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
| `bundles[_].last_successful_download` | `string` | RFC3339 timestamp of last successful bundle download. |
| `bundles[_].last_successful_activation` | `string` | RFC3339 timestamp of last successful bundle activation. |
| `discovery.name` | `string` | Name of discovery bundle that the OPA instance is configured to download. |
| `discovery.active_revision` | `string` | Opaque revision identifier of the last successful discovery activation. |
| `discovery.last_successful_download` | `string` | RFC3339 timestamp of last successful discovery bundle download. |
| `discovery.last_successful_activation` | `string` | RFC3339 timestamp of last successful discovery bundle activation. |
| `metrics.prometheus` | `object` | Global performance metrics for the OPA instance. |

If the bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `bundle.code` | `string` | If present, indicates error(s) occurred. |
| `bundle.message` | `string` | Human readable messages describing the error(s). |
| `bundle.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

If the bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `discovery.code` | `string` | If present, indicates error(s) occurred. |
| `discovery.message` | `string` | Human readable messages describing the error(s). |
| `discovery.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

Services should reply with HTTP status `200 OK` if the status update is
processed successfully.

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

See the [Configuration Reference](#configuration-reference) for configuration details.

### Discovery Service API

OPA expects the service to expose an API endpoint that serves bundles.

```http
GET /<service_url>/<discovery.prefix>/<discovery.name> HTTP/1.1
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
  name: /example/discovery
  prefix: configuration
  service: acmecorp
```

Using the boot configuration above, OPA will fetch discovery bundles from:

```
https://example.com/control-plane-api/v1/configuration/example/discovery
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^^^
services[discovery.service].url                          |                   |
                                         + discovery.prefix  |
                                                             + discovery.name
```

> The `discovery.prefix` field defaults to `bundles`. The default is convenient if
you want to serve discovery bundles and normal bundles from the same API
endpoint. If only one service is defined, there is no need to set `discovery.service`.

OPA generates it's subsequent configuration by querying the Rego and JSON files
contained inside the discovery bundle. The query is defined by the
`discovery.name` field from the boot configuration: `data.<discovery.name>`. For
example. with the boot configuration above, OPA executes the following query:

```
data.example.discovery
```

As an alternative, you can also provide a `decision` field, to specifiy the name of the query. For example, with this configuration:
```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"
discovery:
  name: /example/discovery
  prefix: configuration
  decision: config
```
OPA executes the following query:
```
data.config
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

Below is a policy file which generates an OPA congfiguration.

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

The `bundle_name` variable in `line 7` of the above policy will be dynamically selected based on the value of the label `region`. So if an OPA was started with `region: "US"`, then the `bundle_name` will be `example/test1/p`.

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

The discovery feature cannot be used to dynamically modify `services`, `labels`
and `discovery`. This means that these configuration settings should be included
in the bootup configuration file provided to OPA.

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

See the [Configuration Reference](#configuration-reference) for configuration details.

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

> We recommend that whenever possible, you implement policy and data
> aggregation centrally, however, in some cases that's not possible
> (e.g., due to latency requirements.)

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

> **Warning!** There are *no* ordering guarantees for which bundle loads first and
  takes over some root. If multiple bundles conflict, but are loaded at different
  times, OPA may go into an error state. It is highly recommended to use
  the health check and include bundle state: [Monitoring OPA](#health-checks)

### Debugging Your Bundles

When you run OPA, you can provide bundle files over the command line. This
allows you to manually check that your bundles include all of the files that
you intended and that they are structured correctly. For example:

```bash
opa run bundle.tar.gz
```

## Decision Logs


OPA can periodically report decision logs to remote HTTP servers. The decision
logs contain events that describe policy queries. Each event includes the policy
that was queried, the input to the query, bundle metadata, and other information
that enables auditing and offline debugging of policy decisions.

When decision logging is enabled the OPA server will include a `decision_id`
field in API calls that return policy decisions.

See the [Configuration Reference](#configuration-reference) for configuration details.

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


### Local Decision Logs

Local console logging of decisions can be enabled via the `console` config option.
This does not require any remote server. Example of minimal config to enable:

```yaml
decision_logs:
    console: true
```

This will dump all decision through the OPA logging system at the `info` level. See
[Configuration Reference](#configuration-reference) for more details.


### Masking Sensitive Data

Policy queries may contain sensitive information in the `input` document that
must be removed before decision logs are uploaded to the remote API (e.g.,
usernames, passwords, etc.) Similarly, parts of the policy decision itself may
be considered sensitive.

By default, OPA queries the `data.system.log.mask` path prior to encoding and
uploading decision logs or calling custom decision log plugins.

OPA provides the decision log event as input to the policy query and expects
the query to return a set of JSON Pointers that refer to fields in the decision
log event to erase.

For example, assume OPA is queried with the following `input` document:

```json
{
  "resource": "user",
  "name": "bob",
  "password": "passw0rd"
}
```

To remove the `password` field from decision log events related to "user"
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


## Configuration Reference

This page defines the format of OPA configuration files. Fields marked as
required must be specified if the parent is defined. For example, when the
configuration contains a `status` key, the `status.service` field must be
defined.

The configuration file path is specified with the `-c` or `--config-file`
command line argument:

```bash
opa run -s -c config.yaml
```

The file can be either JSON or YAML format.


#### Example

```yaml
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

labels:
  app: myapp
  region: west
  environment: production

bundles:
  authz:
    service: acmecorp
    resource: bundles/http/example/authz.tar.gz
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120

decision_logs:
  service: acmecorp
  reporting:
    min_delay_seconds: 300
    max_delay_seconds: 600

status:
  service: acmecorp

default_decision: /http/example/authz/allow
```

#### Environment Variable Substitution
> Only supported with the OPA runtime (`opa run`).

Environment variables referenced with the `${...}` notation within the configuration
will be replaced with the value of the environment variable.

Example using `BASE_URL` and `BEARER_TOKEN` environment variables:
```yaml
services:
  acmecorp:
    url: ${BASE_URL}
    credentials:
      bearer:
        token: "${BEARER_TOKEN}"

discovery:
  name: /example/discovery
  prefix: configuration
```
The environment variables `BASE_URL` and `BEARER_TOKEN` will be substituted in when the config
file is loaded by the OPA runtime.

> If the variable is undefined then an empty string (`""`) is substituted. It will __not__
raise an error.

#### CLI Runtime Overrides
> Only supported with the OPA runtime (`opa run`).

Using `opa run` there are CLI options to explicitly set config values. These will override
any values set in the config file.

There are two options to use: `--set` and `--set-file`

Both options take in a key=value format where the key is a selector for the yaml
config structure, for example: `decision_logs.reporting.min_delay_seconds=300` is equivalent
to JSON `{"decision_logs: {"reporting": {"min_delay_seconds: 300}}}`. Multiple values can be
specified with comma separators (`key1=value,key2=value2,..`). Or with additional `--set`
parameters.

Example using several different options:
```
opa run \
  --set "default_decision=/http/example/authz/allow" \
  --set "services.acmecorp.url=https://test-env/control-plane-api/v1" \
  --set "services.acmecorp.credentials.bearer.token=\${TOKEN}"
  --set "labels.app=myapp,labels.region=west"
```
This is equivalent to a YAML config file that looks like:

```yaml
services:
  acmecorp:
    url: https://test-env/control-plane-api/v1
    credentials:
      bearer:
        token: ${TOKEN}

labels:
  app: myapp
  region: west

default_decision: /http/example/authz/allow
```

The `--set-file` option is expecting a file path for the value. This allows keeping secrets in
files and loading them into the config at run time. For Example:

With a file `/var/run/secrets/bearer_token.txt` that has contents:
```
bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm
```

Then using the `--set-file` flag for OPA

```bash
opa run --set-file "services.acmecorp.credentials.bearer.token=/var/run/secrets/bearer_token.txt"
```

It will read the contents of the file and set the config value with the token.

###### Override Limitations
####### Lists
If using arrays/lists in the configuration the `--set` and `--set-file` overrides will not be able to
patch sub-objects of the list. They will overwrite the entire index with the new object.

For example, a `config.yaml` file with contents:
```yaml
services:
  - name: acmecorp
    url: https://test-env/control-plane-api/v1
    credentials:
      bearer:
        token: ""
```
Used with overrides:
```
opa run \
  --config-file config.yaml
  --set-file "services[0].credentials.bearer.token=/var/run/secrets/bearer_token.txt"
```

Will result in configuration like:
```yaml
services:
  - credentials:
      bearer:
        token: bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm
```
Because the entire `0` index was overwritten.

It is highly recommended to use objects/maps instead of lists for configuration for this reason.

####### Empty objects
If you need to set an empty object with the CLI overrides, for example with plugin configuration like:

```yaml
decision_logger:
  plugin: my_plugin

plugins:
  my_plugin:
    # empty
```

You can do this by setting the value with `null`. For example:
```
opa run --set "decision_logger.plugin=my_plugin" --set "plugins.my_plugin=null"
```

####### Keys with Special Characters

If you have a key which contains a special character (`.`, `=`, etc), like `opa.example.com`, and want to use
the `--set` or `--set-file` options you will need to escape the character with a backslash (`\`).

For example a config section like:

```yaml
services:
  opa.example.com:
    url: https://opa.example.com
```

Could be specified with something like:

`--set services.opa\.example\.com.url=https://opa.example.com`


Note that when using it in a shell you may need to put it in quotes or escape the `\`
character too. For example:


`--set services."opa\.example\.com".url=https://opa.example.com`

_or_

`--set services.opa\\.example\\.com.url=https://opa.example.com`

Where the end result passed into OPA still has the `\.` preserved.

#### Services

Services represent endpoints that implement one or more control plane APIs
such as the Bundle or Status APIs. OPA configuration files may contain
multiple services.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].name` | `string` | Yes | Unique name for the service. Referred to by plugins. |
| `services[_].url` | `string` | Yes | Base URL to contact the service with. |
| `services[_].headers` | `object` | No | HTTP headers to include in requests to the service. |
| `services[_].allow_insecure_tls` | `bool` | No | Allow insecure TLS. |

Each service may optionally specify a credential mechanism by which OPA will authenticate
itself to the service.

#### Bearer token

OPA will authenticate using the specified bearer token and schema; to enable bearer token
authentication, the token must be specified. The schema is optional and will default to `Bearer` 
if unspecified.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.bearer.token` | `string` | Yes | Enables token-based authentication and supplies the bearer token to authenticate with. |
| `services[_].credentials.bearer.scheme` | `string` | No | Bearer token scheme to specify. |

#### Client TLS certificate

OPA will present the specified TLS certificate to authenticate. The paths to the client certificate
and the private key are required; the passphrase for the private key is only required if the
private key is encrypted.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.client_tls.cert` | `string` | Yes | The path to the client certificate to authenticate with. |
| `services[_].credentials.client_tls.private_key` | `string` | Yes | The path to the private key of the client certificate. |
| `services[_].credentials.client_tls.private_key_passphrase` | `string` | No | The passphrase to use for the private key. |

#### AWS signature

OPA will authenticate with an [AWS4 HMAC](https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html) signature. Two methods of obtaining the 
necessary credentials are available; exactly one must be specified to use the AWS signature 
authentication method.

If specifying `environment_credentials`, OPA will expect to find environment variables
for `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_REGION`, in accordance with the
convention used by the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.environment_credentials` | `{}` | Yes | Enables AWS signing using environment variables to source the configuration and credentials |

If specifying `metadata_credentials`, OPA will use the AWS metadata services for [EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html) 
or [ECS](https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html)
to obtain the necessary credentials when running within a supported virtual machine/container. 

To use the EC2 metadata service, the IAM role to use and the AWS region for the resource must both 
be specified as `iam_role` and `aws_region` respectively. 

To use the ECS metadata service, specify only the AWS region for the resource as `aws_region`. ECS
containers have at most one associated IAM role.

**N.B.** Providing a value for `iam_role` will cause OPA to use the EC2 metadata service even 
if running inside an ECS container. This may result in unexpected problems if, for example, 
there is no route to the EC2 metadata service from inside the container or if the IAM role is only available within the container and not from the hosting EC2 instance. 

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.metadata_credentials.aws_region` | `string` | Yes | The AWS region to use for the AWS signing service credential method |
| `services[_].credentials.s3_signing.metadata_credentials.iam_role` | `string` | No | The IAM role to use for the AWS signing service credential method |

> Services can be defined as an array or object. When defined as an object, the
> object keys override the `services[_].name` fields.
> For example:
```yaml
services:
  s1:
    url: https://s1/example/
  s2:
    url: https://s2/
```
Is equivalent to
```yaml
services:
  - name: s1
    url: https://s1/example/
  - name: s2
    url: https://s2/
```

### Miscellaneous

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `labels` | `object` | Yes | Set of key-value pairs that uniquely identify the OPA instance. Labels are included when OPA uploads decision logs and status information. |
| `default_decision` | `string` | No (default: `/system/main`) | Set path of default policy decision used to serve queries against OPA's base URL. |
| `default_authorization_decision` | `string` | No (default: `/system/authz/allow`) | Set path of default authorization decision for OPA's API. |
| `plugins` | `object` | No (default: `{}`) | Location for custom plugin configuration. See [Plugins](../plugins) for details. |

### Bundles

Bundles are defined with a key that is the `name` of the bundle. This `name` is used in the status API, decision logs,
server provenance, etc.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `bundles[_].resource` | `string` | No (default: `bundles/<name>`) | Resource path to use to download bundle from configured service. |
| `bundles[_].service` | `string` | Yes | Name of service to use to contact remote server. |
| `bundles[_].polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between bundle downloads. |
| `bundles[_].polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between bundle downloads. |

### Bundle (Deprecated)

>  Deprecated in favor of `bundles` (see above).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `bundle.name` | `string` | Yes | Name of the bundle to download. |
| `bundle.prefix` | `string` | No (default: `bundles`) | Path prefix to use to download bundle from remote server. |
| `bundle.service` | `string` | Yes | Name of service to use to contact remote server. |
| `bundle.polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between bundle downloads. |
| `bundle.polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between bundle downloads. |

### Status

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `status.service` | `string` | Yes | Name of service to use to contact remote server. |
| `status.partition_name` | `string` | No | Path segment to include in status updates. |

### Decision Logs

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `decision_logs.service` | `string` | No | Name of the service to use to contact remote server. If no `plugin` is specified, and `console` logging is disabled, this will default to the first `service` name defined in the Services configuration. |
| `decision_logs.partition_name` | `string` | No | Path segment to include in status updates. |
| `decision_logs.reporting.buffer_size_limit_bytes` | `int64` | No | Decision log buffer size limit in bytes. OPA will drop old events from the log if this limit is exceeded. By default, no limit is set. |
| `decision_logs.reporting.upload_size_limit_bytes` | `int64` | No (default: `32768`) | Decision log upload size limit in bytes. OPA will chunk uploads to cap message body to this limit. |
| `decision_logs.reporting.min_delay_seconds` | `int64` | No (default: `300`) | Minimum amount of time to wait between uploads. |
| `decision_logs.reporting.max_delay_seconds` | `int64` | No (default: `600`) | Maximum amount of time to wait between uploads. |
| `decision_logs.mask_decision` | `string` | No (default: `system/log/mask`) | Set path of masking decision. |
| `decision_logs.plugin` | `string` | No | Use the named plugin for decision logging. If this field exists, the other configuration fields are not required. |
| `decision_logs.console` | `boolean` | No (default: `false`) | Log the decisions locally at `info` level to the console. When enabled alongside a remote decision logging API the `service` must be configured, the default `service` selection will be disabled. | 

### Discovery

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `discovery.name` | `string` | Yes | Name of the discovery configuration to download. |
| `discovery.prefix` | `string` | No (default: `bundles`) | Path prefix to use to download configuration from remote server. |
| `discovery.decision` | `string` | No (default: value of `discovery.name` configuration field) | Name of the OPA query that will be used to calculate the configuration |
| `discovery.polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between configuration downloads. |
| `discovery.polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between configuration downloads. |

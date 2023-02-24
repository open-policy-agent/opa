---
title: "Discovery"
kind: management
weight: 5
---

OPA can be configured to download bundles of policy and data, report status, and
upload decision logs to remote endpoints. The discovery feature helps you
centrally manage the OPA configuration for these features. You should use the
discovery feature if you want to avoid managing OPA configuration updates in
a number of different locations.

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
GET /<service_url>/<discovery.resource> HTTP/1.1
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
  acmecorp:
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

discovery:
  service: acmecorp
  resource: /configuration/example/discovery.tar.gz
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


> The optional `discovery.signing` field can be used to specify the `keyid` and `scope` that should be used
> for verifying the signature of the discovery bundle. See [this](#discovery-bundle-signature) section for details.

OPA generates it's subsequent configuration by querying the Rego and JSON files
contained inside the discovery bundle. The default query is `data` however this
can be overridden by specifying the `discovery.decision`.

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

discovery := {
  "bundles": {
    "main": {
      "service": "acmecorp",
      "resource": bundle_name
    },
  },
  "default_decision": "acmecorp/httpauthz/allow"
}

bundle_name := "acmecorp/httpauthz"
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
package discovery

config := {
  "bundles": {
    "main": {
      "service": "acmecorp",
      "resource": bundle_name  # line 7
    }
  }
}

rt := opa.runtime()
region := rt.config.labels.region
bundle_name := region_bundle[region]

# region-bundle information
region_bundle := {
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
  resource: bundles/discovery.tar.gz
  decision: discovery/config
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
  resource: bundles/discovery.tar.gz
  decision: discovery/config
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

### Discovery Bundle Persistence

OPA can optionally persist the activated discovery bundle to disk for recovery purposes. To enable
persistence, set the `discovery.persist` field to `true`. When bundle
persistence is enabled, OPA will attempt to read the discovery bundle from disk on startup. This
allows OPA to start with the most recently activated bundle in case OPA cannot communicate
with the bundle server. OPA will try to load and activate the persisted discovery bundle on a best-effort basis. Any errors
encountered during the process will be surfaced in the bundle's status update. When communication between OPA and
the bundle server is restored, the latest bundle is downloaded, activated, and persisted.  Like regular bundles, only
the discovery bundle itself is persisted. The discovered configuration that is generated by evaluating the data and
policies contained in the discovery bundle will **NOT** be persisted.

{{< info >}}
By default, the discovery bundle is persisted under the current working directory of the OPA process (e.g., `./.opa/bundles/<discovery.name>/bundle.tar.gz`).
{{< /info >}}

# Discovery

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

If no discovery path is specified OPA will query the *data* document to produce
the configuration. If a discovery path is specified, OPA will translate the path
to a reference and evaluate it relative to the *data* document. For example, if
the discovery path is */example/discovery* OPA will evaluate
*data.example.discovery* to produce the configuration.

If discovery is enabled, other features like bundle downloading and status
reporting **cannot** be configured manually. Similarly, discovered configuration
cannot override the original discovery settings in the configuration file that
OPA was booted with.

See the [Configuration Reference](configuration.md) for configuration details.

## Discovery Service API

OPA expects the service to expose an API endpoint that serves bundles.

```http
GET /<service_url>/<discovery_prefix>/<discovery_path> HTTP/1.1
```

If the bundle exists, the server should respond with an HTTP 200 OK status
followed by a gzipped tarball in the message body.

```http
HTTP/1.1 200 OK
Content-Type: application/gzip
```

Discovery can be enabled using the below configuration:

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
```

OPA will fetch it's configuration from
`https://example.com/control-plane-api/v1/configuration/example/discovery` and
use that to initialize the other plugins like `bundles`, `status`, `decision
logs`. The `prefix` field is optional and by default set to `bundles`. Hence if
`prefix` is not provided, OPA will fetch it's configuration from
`https://example.com/control-plane-api/v1/bundles/example/discovery`.

Below is an example of how configuration for `decision logs` can be included
inside a policy file.

**example.rego**

```ruby
package example

discovery = {
  "decision_logs": {
    "service": "acmecorp"
  }
}
```

The same configuration can also  be provided as data.

**data.json**

```json
"example": {
  "discovery": {
    "decision_logs": {
      "service": "acmecorp"
    }
  }
}
```

In both cases, OPA's configuration is hierarchically organized under the
`discovery.name` value. If discovery is enabled, the `service` field in the
`bundles`, `status`, `decision logs` plugins is optional and will default to one
of the services from the discovery configuration.

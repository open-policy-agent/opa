# Configuration Reference

This page defines the format of OPA configuration files. Fields marked as
required must be specified if the parent is defined. For example, when the
configuration contains a `status` key, the `status.service` field must be
defined.

The configuration file path is specified with the `-c` or `--config-file`
command line argument:

```bash
opa run -s -c config.yaml
```

## Example

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

labels:
  app: myapp
  region: west
  environment: production

bundle:
  name: http/example/authz
  service: acmecorp
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

## Services

Services represent endpoints that implement one or more control plane APIs
such as the Bundle or Status APIs. OPA configuration files may contain
multiple services.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].name` | `string` | Yes | Unique name for the service. Referred to by plugins. |
| `services[_].url` | `string` | Yes | Base URL to contact the service with. |
| `services[_].headers` | `object` | No | HTTP headers to include in requests to the service. |
| `services[_].allow_insecure_tls` | `bool` | No | Allow insecure TLS. |
| `services[_].credentials.bearer.token` | `string` | No | Enables token-based authentication and supplies the bearer token to authenticate with. |
| `services[_].credentials.bearer.scheme` | `string` | No | Bearer token scheme to specify. |
| `services[_].credentials.client_tls.cert` | `string` | No | The path to the client certificate to authenticate with. |
| `services[_].credentials.client_tls.private_key` | `string` | No | The path to the private key of the client certificate. |
| `services[_].credentials.client_tls.private_key_passphrase` | `string` | No | The passphrase to use for the private key. |

> Services can be defined as an array or object. When defined as an object, the
> object keys override the `services[_].name` fields.

## Miscellaenous

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `labels` | `object` | Yes | Set of key-value pairs that uniquely identify the OPA instance. Labels are included when OPA uploads decision logs and status information. |
| `default_decision` | `string` | No (default: `/system/main`) | Set path of default policy decision used to serve queries against OPA's base URL. |
| `default_authorization_decision` | `string` | No (default: `/system/authz/allow`) | Set path of default authorization decision for OPA's API. |

## Bundles

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `bundle.name` | `string` | Yes | Name of the bundle to download. |
| `bundle.service` | `string` | Yes | Name of service to use to contact remote server. |
| `bundle.polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between bundle downloads. |
| `bundle.polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between bundle downloads. |

## Status

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `status.service` | `string` | Yes | Name of service to use to contact remote server. |
| `status.partition_name` | `string` | No | Path segment to include in status updates. |

## Decision Logs

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `decision_logs.service` | `string` | Yes | Name of the service to use to contact remote server. |
| `decision_logs.partition_name` | `string` | No | Path segment to include in status updates. |
| `decision_logs.reporting.buffer_size_limit_bytes` | `int64` | No | Decision log buffer size limit in bytes. OPA will drop old events from the log if this limit is exceeded. By default, no limit is set. |
| `decision_logs.reporting.upload_size_limit_bytes` | `int64` | No (default: `32768`) | Decision log upload size limit in bytes. OPA will chunk uploads to cap message body to this limit. |
| `decision_logs.reporting.min_delay_seconds` | `int64` | No (default: `300`) | Minimum amount of time to wait between uploads. |
| `decision_logs.reporting.max_delay_seconds` | `int64` | No (default: `600`) | Maximum amount of time to wait between uploads. |

## Discovery

OPA's configuration files contain the configuration for management functionality such as `bundles`, `status`, `decision logs` . As OPA gains more management functionality, it's configuration would have to be extended and could get difficult to manage. To simplify deployment of OPA and enable better centralized management, OPA can discover it's configuration from a remote endpoint.

Discovery can be enabled using the below configuration:

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

discovery:
  path: /foo/bar
```

OPA will fetch it's configuration from `https://example.com/control-plane-api/v1/foo/bar` and use that to initialize the other plugins like `bundles`, `status`, `decision logs`. If `discovery` is enabled,  `bundles`, `status`, `decision logs` cannot be configured manually.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `discovery.path` | `string` | Yes | Path to use to download configuration from remote server. |

The remote server can include OPA's configuration in a policy or data file and package it inside a bundle.

Below is an example of how configuration for `decision logs` can be included inside a policy file.

**example.rego**

```ruby
package foo

bar = {
  "decision_logs": {
    "service": "acmecorp"
  }
}
```

The same configuration can also  be provided as data.

**data.json**

```json
"foo": {
  "bar": {
    "decision_logs": {
      "service": "acmecorp"
    }
  }
}
```

In both cases, OPA's configuration is hierarchically organized under the `discovery.path` value.

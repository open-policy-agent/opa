---
title: Configuration Reference
kind: documentation
weight: 7
---

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


## Example

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

bundle:
  name: http/example/authz
  service: acmecorp
  prefix: bundles
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

## Environment Variable Substitution
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

## CLI Runtime Overrides
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

### Override Limitations
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

Each service may optionally specify a credential mechanism by which OPA will authenticate
itself to the service.

### Bearer token

OPA will authenticate using the specified bearer token and schema; to enable bearer token
authentication, the token must be specified. The schema is optional and will default to `Bearer` 
if unspecified.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.bearer.token` | `string` | Yes | Enables token-based authentication and supplies the bearer token to authenticate with. |
| `services[_].credentials.bearer.scheme` | `string` | No | Bearer token scheme to specify. |

### Client TLS certificate

OPA will present the specified TLS certificate to authenticate. The paths to the client certificate
and the private key are required; the passphrase for the private key is only required if the
private key is encrypted.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.client_tls.cert` | `string` | Yes | The path to the client certificate to authenticate with. |
| `services[_].credentials.client_tls.private_key` | `string` | Yes | The path to the private key of the client certificate. |
| `services[_].credentials.client_tls.private_key_passphrase` | `string` | No | The passphrase to use for the private key. |

### AWS signature

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

## Miscellaneous

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `labels` | `object` | Yes | Set of key-value pairs that uniquely identify the OPA instance. Labels are included when OPA uploads decision logs and status information. |
| `default_decision` | `string` | No (default: `/system/main`) | Set path of default policy decision used to serve queries against OPA's base URL. |
| `default_authorization_decision` | `string` | No (default: `/system/authz/allow`) | Set path of default authorization decision for OPA's API. |
| `plugins` | `object` | No (default: `{}`) | Location for custom plugin configuration. See [Plugins](../plugins) for details. |

## Bundles

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `bundle.name` | `string` | Yes | Name of the bundle to download. |
| `bundle.prefix` | `string` | No (default: `bundles`) | Path prefix to use to download bundle from remote server. |
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
| `decision_logs.mask_decision` | `string` | No (default: `system/log/mask`) | Set path of masking decision. |
| `decision_logs.plugin` | `string` | No | Use the named plugin for decision logging. If this field exists, the other configuration fields are not required. |

## Discovery

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `discovery.name` | `string` | Yes | Name of the discovery configuration to download. |
| `discovery.prefix` | `string` | No (default: `bundles`) | Path prefix to use to download configuration from remote server. |
| `discovery.polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between configuration downloads. |
| `discovery.polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between configuration downloads. |

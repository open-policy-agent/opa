---
title: Configuration
kind: operations
weight: 10
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


#### Example

```yaml
services:
  acmecorp:
    url: https://example.com/control-plane-api/v1
    response_header_timeout_seconds: 5
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
    persist: true
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120
    signing:
      keyid: global_key
      scope: write

decision_logs:
  service: acmecorp
  reporting:
    min_delay_seconds: 300
    max_delay_seconds: 600

status:
  service: acmecorp

default_decision: /http/example/authz/allow

persistence_directory: /var/opa

keys:
  global_key:
    algorithm: RS256
    key: <PEM_encoded_public_key>
    scope: read

caching:
  inter_query_builtin_cache:
    max_size_bytes: 10000000

distributed_tracing:
  type: grpc
  address: localhost:4317
  service_name: opa
  sample_percentage: 50
  encryption: "off"

server:
  encoding:
    gzip:
        min_length: 1024,
        compression_level: 9
```

#### Environment Variable Substitution
> Only supported with the OPA runtime (`opa run`).

Environment variables referenced with the `${...}` notation within the configuration
will be replaced with the value of the environment variable.

Example using `BASE_URL` and `BEARER_TOKEN` environment variables:
```yaml
services:
  acmecorp:
    url: "${BASE_URL}"
    credentials:
      bearer:
        token: "${BEARER_TOKEN}"

discovery:
  resource: /configuration/example/discovery
  decision: example
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
to JSON `{"decision_logs": {"reporting": {"min_delay_seconds": 300}}}`. Multiple values can be
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

##### Override Limitations
###### Lists
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

##### Remote Bundles Override Shorthand

When running the server to quickly try a remote public bundle — such as those published from the
[Rego Playground](https://play.openpolicyagent.org), you may find it convenient to provide the URL of the
bundle directly, rather than via repeated `--set` flags:

```shell
opa run -s https://example.com/bundles/bundle.tar.gz
```

The above shorthand command is identical to:

```shell
opa run -s --set "services.cli1.url=https://example.com" \
           --set "bundles.cli1.service=cli1" \
           --set "bundles.cli1.resource=/bundles/bundle.tar.gz" \
           --set "bundles.cli1.persist=true"
```

###### Empty objects
If you need to set an empty object with the CLI overrides, for example with plugin configuration like:

```yaml
decision_logs:
  plugin: my_plugin

plugins:
  my_plugin:
    # empty
```

You can do this by setting the value with `null`. For example:
```
opa run --set "decision_logs.plugin=my_plugin" --set "plugins.my_plugin=null"
```

###### Keys with Special Characters

If you have a key which contains a special character (`=`, `[`, `,`, `.`), like `opa.example.com`, and want to use
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
| `services[_].response_header_timeout_seconds` | `int64` | No (default: 10) | Amount of time to wait for a server's response headers after fully writing the request. This time does not include the time to read the response body. |
| `services[_].headers` | `object` | No | HTTP headers to include in requests to the service. |
| `services[_].tls.ca_cert` | `string` | No | The path to the root CA certificate.  If not provided, this defaults to TLS using the host's root CA set. |
| `services[_].tls.system_ca_required` | `bool` | No (default: `false`) | Require system certificate appended with root CA certificate. |
| `services[_].allow_insecure_tls` | `bool` | No | Allow insecure TLS. |
| `services[_].type` | `string` | No (default: empty) | Optional parameter that allows to use an "OCI" service type. This will allow bundle and discovery plugins to download bundles from an OCI registry. |

> Services can be defined as an array or object. When defined as an object, the
> object keys override the `services[_].name` fields.
> For example:
> ```yaml
> services:
>   s1:
>     url: https://s1/example/
>   s2:
>     url: https://s2/
> ```
> Is equivalent to
> ```yaml
> services:
>   - name: s1
>     url: https://s1/example/
>   - name: s2
>     url: https://s2/
> ```

Each service may optionally specify a credential mechanism by which OPA will authenticate
itself to the service.

##### Example

Using an OCI service type to download a bundle from an OCI repository.

```yaml
services:
  ghcr-registry:
    url: https://ghcr.io
    type: oci

bundles:
  authz:
    service: ghcr-registry
    resource: ghcr.io/${ORGANIZATION}/${REPOSITORY}:${TAG}
    persist: true
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120

persistence_directory: ${PERSISTENCE_PATH}
```

When using an OCI service type the downloader uses the persistence path to store the layers of the downloaded repository. This storage path should be maintained by the user.
If persistence is not configured the OCI downloader will store the layers in the system's temporary directory to allow automatic cleanup on system restart.

#### Bearer Token

OPA will authenticate using the specified bearer token and schema; to enable bearer token
authentication, either the token or the path to the token must be specified. If the latter is provided, on each request OPA will re-read the token from the file and use that token for authentication.

The `scheme` attribute is optional, and will default to `Bearer` if unspecified.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.bearer.token` | `string` | Yes | Enables token-based authentication and supplies the bearer token to authenticate with. |
| `services[_].credentials.bearer.token_path` | `string` | Yes | Enables token-based authentication and supplies the path to the bearer token to authenticate with. |
| `services[_].credentials.bearer.scheme` | `string` | No | Bearer token scheme to specify. |

#### Client TLS Certificate

OPA will present the specified TLS certificate to authenticate. The paths to the client certificate
and the private key are required; the passphrase for the private key is only required if the
private key is encrypted.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.client_tls.cert` | `string` | Yes | The path to the client certificate to authenticate with. |
| `services[_].credentials.client_tls.private_key` | `string` | Yes | The path to the private key of the client certificate. |
| `services[_].credentials.client_tls.private_key_passphrase` | `string` | No | The passphrase to use for the private key. |

#### OAuth2 Client Credentials

OPA will authenticate using a bearer token obtained through the OAuth2 [client credentials](https://tools.ietf.org/html/rfc6749#section-4.4) flow.
Following successful authentication at the token endpoint the returned token will be cached for subsequent requests for the duration of its lifetime. Note that as per the [OAuth2 standard](https://tools.ietf.org/html/rfc6749#section-2.3.1), only the HTTPS scheme is supported for the token endpoint URL.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.oauth2.token_url` | `string` | Yes | URL pointing to the token endpoint at the OAuth2 authorization server. |
| `services[_].credentials.oauth2.client_id` | `string` | Yes | The client ID to use for authentication. |
| `services[_].credentials.oauth2.client_secret` | `string` | Yes | The client secret to use for authentication. |
| `services[_].credentials.oauth2.scopes` | `[]string` | No | Optional list of scopes to request for the token. |
| `services[_].credentials.oauth2.additional_headers` | `map` | No | Map of additional headers to send to token endpoint at the OAuth2 authorization server |
| `services[_].credentials.oauth2.additional_parameters` | `map` | No | Map of additional body parameters to send token endpoint at the OAuth2 authorization server |

#### OAuth2 Client Credentials JWT authentication

OPA will authenticate using a bearer token obtained through the OAuth2 [client credentials](https://tools.ietf.org/html/rfc6749#section-4.4) flow.
Rather than providing a client secret along with the request for an access token, the client [asserts](https://tools.ietf.org/html/rfc7521#section-4.2) its identity in the form of a signed JWT.
Following successful authentication at the token endpoint the returned token will be cached for subsequent requests for the duration of its lifetime. Note that as per the [OAuth2 standard](https://tools.ietf.org/html/rfc6749#section-2.3.1), only the HTTPS scheme is supported for the token endpoint URL.

| Field                                              | Type       | Required | Description                                                                                                                                                                                     |
|----------------------------------------------------|------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `services[_].credentials.oauth2.token_url`         | `string`   | Yes      | URL pointing to the token endpoint at the OAuth2 authorization server.                                                                                                                          |
| `services[_].credentials.oauth2.grant_type`        | `string`   | No       | Defaults to `client_credentials`.                                                                                                                                                               |
| `services[_].credentials.oauth2.client_id`         | `string`   | No       | The client ID to use for authentication.                                                                                                                                                        |
| `services[_].credentials.oauth2.signing_key`       | `string`   | No       | Reference to private key used for signing the JWT. Required if `aws_kms` is not provided                                                                                                        |
| `services[_].credentials.oauth2.thumbprint`        | `string`   | No       | Certificate thumbprint to use for x5t header generation.                                                                                                                                        |
| `services[_].credentials.oauth2.additional_claims` | `map`      | No       | Map of claims to include in the JWT (see notes below)                                                                                                                                           |
| `services[_].credentials.oauth2.include_jti_claim` | `bool`     | No       | Include a uniquely generated `jti` claim in any issued JWT                                                                                                                                      |
| `services[_].credentials.oauth2.scopes`            | `[]string` | No       | Optional list of scopes to request for the token.                                                                                                                                               |
| `services[_].credentials.oauth2.aws_kms.name`      | `string`   | No       | To specify a KMS key, use its key ID, key ARN, alias name, or alias ARN. Required only for signing with AWS KMS.                                                                                |
| `services[_].credentials.oauth2.aws_kms.algorithm` | `string`   | No       | Specifies the signing algorithm used by the key `aws_kms.name` `(ECDSA_SHA_256, ECDSA_SHA_384 or ECDSA_SHA_512)`. Required only for signing with AWS KMS.                                       |
| `services[_].credentials.oauth2.aws_signing`       | `{}`       | No       | AWS credentials for signing requests. Required if `aws_kms` is provided. |

Two claims will always be included in the issued JWT: `iat` and `exp`. Any other claims will be populated from the `additional_claims` map.

{{< info >}}
For using `services[_].credentials.oauth2.aws_kms`, a method for setting the AWS credentials has to be specified in the `services[_].credentials.oauth2.aws_signing`.
The value of  `services[_].credentials.oauth2.aws_signing.service` should be `kms`. Several methods of obtaining the necessary credentials are available; exactly one must be specified,
see description for `services[_].credentials.s3_signing`.
{{< /info >}}

##### Example

Using the client credentials grant type with JWT client authentication replacing client secret as the credential used at the token endpoint.

```yaml
services:
  remote:
    url: ${BUNDLE_SERVICE_URL}
    credentials:
      oauth2:
        token_url: ${TOKEN_URL}
        grant_type: client_credentials
        client_id: opa-client
        signing_key: jwt_signing_key # references the key in `keys` below
        include_jti_claim: true
        scopes:
        - read
        - write
        additional_claims:
          sub: opa-client
          iss: opa-${POD_NAME}

bundles:
  authz:
    service: remote
    resource: bundles/http/example/authz.tar.gz

keys:
  jwt_signing_key:
    algorithm: ES512
    private_key: ${BUNDLE_SERVICE_SIGNING_KEY}
```

Using the client credentials grant type with JWT client authentication & AWS KMS signing of client assertions.

```yaml
services:
  remote:
    url: ${BUNDLE_SERVICE_URL}
    credentials:
      oauth2:
        token_url: ${TOKEN_URL}
        grant_type: client_credentials
        client_id: opa-client
        aws_kms:
          name: ${AWS_KMS_KEYID}
          algorithm: ECDSA_SHA_256
        aws_signing: # similar to s3_signing
          service: kms
          environment_credentials:
            aws_default_region: eu-west-1
        include_jti_claim: true
        scopes:
        - read
        - write
        additional_claims:
          sub: opa-client
          iss: opa-${POD_NAME}

bundles:
  authz:
    service: remote
    resource: bundles/http/example/authz.tar.gz
```
#### OAuth2 JWT Bearer Grant Type

OPA will authenticate using a bearer token obtained through the OAuth2 [JWT authorization grant](https://tools.ietf.org/html/rfc7523#section-2.1) flow.
Rather than providing a client secret along with the request for an access token, the client [asserts](https://tools.ietf.org/html/rfc7521#section-4.1) its identity in the form of a signed JWT.
Following successful authentication at the token endpoint the returned token will be cached for subsequent requests for the duration of its lifetime. Note that as per the [OAuth2 standard](https://tools.ietf.org/html/rfc6749#section-2.3.1), only the HTTPS scheme is supported for the token endpoint URL.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.oauth2.token_url` | `string` | Yes | URL pointing to the token endpoint at the OAuth2 authorization server. |
| `services[_].credentials.oauth2.grant_type` | `string` | No | Must be set to `jwt_bearer` for JWT bearer grant type. Defaults to `client_credentials`. |
| `services[_].credentials.oauth2.signing_key` | `string` | Yes | Reference to private key used for signing the JWT. |
| `services[_].credentials.oauth2.additional_claims` | `map` | No | Map of claims to include in the JWT (see notes below) |
| `services[_].credentials.oauth2.include_jti_claim` | `bool` | No | Include a uniquely generated `jti` claim in any issued JWT |
| `services[_].credentials.oauth2.scopes` | `[]string` | No | Optional list of scopes to request for the token. |

Two claims will always be included in the issued JWT: `iat` and `exp`. Any other claims will be populated from the `additional_claims` map.

##### Example

Using a [Google Cloud Storage](https://cloud.google.com/storage/) bucket as a bundle service backend from outside the
cloud account (for access from inside the account, see the [GCP Metadata Token](#gcp-metadata-token) section).

```yaml
services:
  gcp:
    url: https://storage.googleapis.com/storage/v1/b/${BUCKET_NAME}/o
    credentials:
      oauth2:
        token_url: https://oauth2.googleapis.com/token
        grant_type: jwt_bearer
        signing_key: jwt_signing_key # references the key in `keys` below
        scopes:
        - https://www.googleapis.com/auth/devstorage.read_only
        additional_claims:
          aud: https://oauth2.googleapis.com/token
          iss: opa-client@my-account.iam.gserviceaccount.com

bundles:
  authz:
    service: gcp
    resource: 'bundles/http/example/authz.tar.gz?alt=media'

keys:
  jwt_signing_key:
    algorithm: RS256
    private_key: ${BUNDLE_SERVICE_SIGNING_KEY}
```

{{< danger >}}
OPA masks services authentication secrets which make use of the `credentials` field, in order to prevent the exposure of sensitive tokens.
It is important to note that the [/v1/config API](../rest-api/#config-api) allows clients to read the runtime configuration of OPA. As such, any credentials used by
custom configurations not utilizing the credentials field will be exposed to the caller.
Consider requiring authentication in order to prevent unauthorized read access to OPA's runtime configuration.
{{< /danger >}}

#### AWS Signature

OPA will authenticate with an [AWS Version 4](https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html) or version 4A signature. While version 4 is the default, version 4A must be used when making requests that might be handled by more than one region, such as an [S3 Multi-Region Access Point](https://docs.aws.amazon.com/AmazonS3/latest/userguide/MultiRegionAccessPoints.html). You must use version 4A for this or requests will fail when routed to a different region than the one indicated in a version 4 signature. Furthermore, using version 4a also requires that temporary credentials are retrieved from a [regional AWS STS endpoint](https://docs.aws.amazon.com/sdkref/latest/guide/feature-sts-regionalized-endpoints.html), rather than the global STS endpoint.

Several methods of obtaining the necessary credentials are available; exactly one must be specified to use the AWS signature authentication method.

The AWS service for which to sign the request can be specified in the `service` field. If omitted, the default is `s3`.

The AWS signature version to sign the request with can be specified in the `signature_version` field. If omitted, the default is `4`. The only other valid value is `4a`.

| Field                                                  | Type | Required | Description                                                                    |
|--------------------------------------------------------| --- | --- |--------------------------------------------------------------------------------|
| `services[_].credentials.s3_signing.service`           | `string` | No | The AWS service to sign requests with, eg `execute-api` or `s3`. Default: `s3` |
| `services[_].credentials.s3_signing.signature_version` | `string` | No | The AWS signature version to sign requests with, eg `4` or `4a`. Default: `4`  |

##### Using Static Environment Credentials
If specifying `environment_credentials`, OPA will expect to find environment variables
for `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_REGION`, in accordance with the
convention used by the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html).

Please note that if you are using temporary IAM credentials (e.g. assumed IAM role credentials) you have to provide additional `AWS_SESSION_TOKEN` or `AWS_SECURITY_TOKEN` environment variable.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.environment_credentials` | `{}` | Yes | Enables AWS signing using environment variables to source the configuration and credentials |

##### Using Named Profile Credentials
If specifying `profile_credentials`, OPA will expect to find the `access key id`, `secret access key` and
`session token` from the [named profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html)
stored in the [credentials](https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html) file on disk. On each
request OPA will re-read the credentials from the file and use them for authentication.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.profile_credentials.path` | `string` | No | The path to the shared credentials file. If empty, OPA will look for the `AWS_SHARED_CREDENTIALS_FILE` env variable. If the variable is not set, the path defaults to the current user's home directory. `~/.aws/credentials` (Linux & Mac) or `%USERPROFILE%\.aws\credentials` (Windows) |
| `services[_].credentials.s3_signing.profile_credentials.profile` | `string` | No | AWS Profile to extract credentials from the credentials file. If empty, OPA will look for the `AWS_PROFILE` env variable. If the variable is not set, the `default` profile will be used |
| `services[_].credentials.s3_signing.profile_credentials.aws_region` | `string` | No | The AWS region to use for the AWS signing service credential method. If unset, the `AWS_REGION` environment variable must be set |

##### Using EC2 Metadata Credentials
If specifying `metadata_credentials`, OPA will use the AWS metadata services for [EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)
or [ECS](https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html)
to obtain the necessary credentials when running within a supported virtual machine/container.

To use the EC2 metadata service, the IAM role to use and the AWS region for the resource must both
be specified as `iam_role` and `aws_region` respectively.

To use the ECS metadata service, specify only the AWS region for the resource as `aws_region`. ECS
containers have at most one associated IAM role.

> Providing a value for `iam_role` will cause OPA to use the EC2 metadata service even
> if running inside an ECS container. This may result in unexpected problems if, for example,
> there is no route to the EC2 metadata service from inside the container or if the IAM role is only available within the container and not from the hosting EC2 instance.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.metadata_credentials.aws_region` | `string` | No | The AWS region to use for the AWS signing service credential method. If unset, the `AWS_REGION` environment variable must be set |
| `services[_].credentials.s3_signing.metadata_credentials.iam_role` | `string` | No | The IAM role to use for the AWS signing service credential method |

##### Using EKS IAM Roles for Service Account (Web Identity) Credentials
If specifying `web_identity_credentials`, OPA will expect to find environment variables for `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`, in accordance with the convention used by the [AWS EKS IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.s3_signing.web_identity_credentials.aws_region` | `string` | Yes | The AWS region to use for the sts regional endpoint. Uses the global endpoint by default |
| `services[_].credentials.s3_signing.web_identity_credentials.session_name` | `string` | No | The session name used to identify the assumed role session. Default: `open-policy-agent` |
| `services[_].credentials.s3_signing.web_identity_credentials.aws_domain` | `string` | No | The AWS domain name to use. Default: `amazonaws.com`. Can also be set via the `AWS_DOMAIN` environment variable (config takes precedence) |

#### GCP Metadata Token

OPA will authenticate with a GCP [access token](https://cloud.google.com/run/docs/securing/service-identity#access_tokens) or [identity token](https://cloud.google.com/run/docs/securing/service-identity) fetched from the [Compute Metadata Server](https://cloud.google.com/compute/docs/storing-retrieving-metadata). When one or more `scopes` is provided an access token is fetched. When a non-empty `audience` is provided an identity token is fetched. An audience or `scopes` array is required.

When authenticating to native GCP services such as [Google Cloud Storage](https://cloud.google.com/storage) an access token should be used with the appropriate set of scopes required by the target resource. When authenticating to a third party application such as an application hosted on Google Cloud Run an identity token should be used.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
|`services[_].credentials.gcp_metadata.audience`|`string`|No|The audience to use when fetching identity tokens.|
|`services[_].credentials.gcp_metadata.endpoint`|`string`|No|The metadata endpoint to use.|
|`services[_].credentials.gcp_metadata.scopes`|`array`|No|The set of scopes to use when fetching access token.|
|`services[_].credentials.gcp_metadata.access_token_path`|`string`|No|The access token metadata path to use.|
|`services[_].credentials.gcp_metadata.id_token_path`|`string`|No|The identity token metadata path to use.|

##### Example

Using a [Cloud Run](https://cloud.google.com/run) service as a bundle service backend.

```yaml
services:
  cloudrun:
    url: ${BUNDLE_SERVICE_URL}
    response_header_timeout_seconds: 5
    credentials:
      gcp_metadata:
        audience: ${BUNDLE_SERVICE_URL}

bundles:
  authz:
    service: cloudrun
    resource: bundles/http/example/authz.tar.gz
    persist: true
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120
```

Using [Google Cloud Storage](https://cloud.google.com/storage) as a bundle service backend.

```yaml
services:
  gcs:
    url: https://storage.googleapis.com/storage/v1/b/${BUCKET_NAME}/o
    response_header_timeout_seconds: 5
    credentials:
      gcp_metadata:
        scopes:
          - "https://www.googleapis.com/auth/devstorage.read_only"

bundles:
  authz:
    service: gcs
    resource: 'bundle.tar.gz?alt=media'
    persist: true
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120
```

#### Azure Managed Identities Token

OPA will authenticate with an [Azure managed identities](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview) token.
The [token request](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token#get-a-token-using-http)
can be configured via the plugin to customize the base URL, API version, and resource. Specific managed identity IDs can be optionally provided as well.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `services[_].credentials.azure_managed_identity.endpoint` | `string` | No | Request endpoint. (default: `http://169.254.169.254/metadata/identity/oauth2/token`, the Azure Instance Metadata Service endpoint (recommended))|
| `services[_].credentials.azure_managed_identity.api_version` | `string` | No | API version to use. (default: `2018-02-01`, the minimum version) |
| `services[_].credentials.azure_managed_identity.resource` | `string` | No | App ID URI of the target resource. (default: `https://storage.azure.com/`) |
| `services[_].credentials.azure_managed_identity.object_id` | `string` | No | Optional object ID of the managed identity you would like the token for. Required, if your VM has multiple user-assigned managed identities. |
| `services[_].credentials.azure_managed_identity.client_id` | `string` | No | Optional client ID of the managed identity you would like the token for. Required, if your VM has multiple user-assigned managed identities. |
| `services[_].credentials.azure_managed_identity.mi_res_id` | `string` | No | Optional Azure Resource ID of the managed identity you would like the token for. Required, if your VM has multiple user-assigned managed identities. |

##### Example
Use an [Azure storage account](https://docs.microsoft.com/en-us/azure/storage/common/storage-account-overview) as a bundle service backend.
Note that the `x-ms-version` header must be specified for the storage account service, and a minimum version of `2017-11-09` must be provided as per [Azure documentation](https://docs.microsoft.com/en-us/rest/api/storageservices/authorize-with-azure-active-directory#call-storage-operations-with-oauth-tokens).

```yaml
services:
  azure_storage_account:
    url: ${STORAGE_ACCOUNT_URL}
    headers:
      x-ms-version: 2017-11-09
    response_header_timeout_seconds: 5
    credentials:
      azure_managed_identity: {}

bundles:
  authz:
    service: azure_storage_account
    resource: bundles/http/example/authz.tar.gz
    persist: true
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120
```

#### Custom Plugin

If none of the existing credential options work for a service, OPA can authenticate using a custom plugin, enabling support for any authentication scheme.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
|`services[_].credentials.plugin`|`string`|No|The name of the plugin to use for authentication|

##### Example

Using a custom plugin for service credentials:

```yaml
services:
  my_service:
    url: https://example.com/v1
    credentials:
      plugin: my_custom_auth
plugins:
  my_custom_auth:
    foo: bar
```

```go

package plugins

import (
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/util"
)

type Config struct {
	Foo string `json:"foo"`
}

type PluginFactory struct{}

type Plugin struct {
	manager  *plugins.Manager
	config   Config
	stop     chan chan struct{}
	reconfig chan interface{}
}

func (p *PluginFactory) Validate(manager *plugins.Manager, config []byte) (interface{}, error) {
	var parsedConfig Config
	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}
	return &parsedConfig, nil
}

func (p *PluginFactory) New(manager *plugins.Manager, config interface{}) plugins.Plugin {
	return &Plugin{
		config:   *config.(*Config),
		manager:  manager,
		stop:     make(chan chan struct{}),
		reconfig: make(chan interface{}),
	}
}

func (p *Plugin) Start(ctx context.Context) error {
	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
	return nil
}

func (p *Plugin) Stop(ctx context.Context) {
	done := make(chan struct{})
	p.stop <- done
	<-done
	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
	return
}

func (p *Plugin) Reconfigure(ctx context.Context, config interface{}) {
	p.reconfig <- config
	return
}

func (p *Plugin) NewClient(c rest.Config) (*http.Client, error) {
	t, err := rest.DefaultTLSConfig(c)
	if err != nil {
		return nil, err
	}
	return rest.DefaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

func (p *Plugin) Prepare(req *http.Request) error {
	req.Header.Add("X-Custom-Auth-Protocol", "knock knock")
	return nil
}

func init() {
	runtime.RegisterPlugin("my_custom_auth", &PluginFactory{})
}

```

### Using private image from OCI repositories

When using a private image from an OCI registry you need to specify an authentication method. Supported authentication methods are listed in the [Services](#services) section. The Azure managed identity plugin
is not supported at this point in time.

Examples of setting credentials for pulling private images:
*AWS ECR* private images usually require at least basic authentication. The credentials to authenticate can be obtained using the AWS CLI command `aws ecr get-login` and those can be passed to the service configuration as basic bearer credentials as follows:
```yaml
credentials:
  bearer:
    scheme: "Basic"
    token: "<username>:<password>"
```

Other AWS authentication methods also work:
```yaml
credentials:
  s3_signing:
    service: "ecr"
    metadata_credentials:
      aws_region: us-east-1
```

Note, that the authentication method `s3_signing` does work for
signing requests to other AWS services.

A special case is that bearer authentication works differently to normal service authentication. The OCI downloader base64-encodes the credentials for you so that they need to be supplied in plain text.

For *GHCR* (Github Container Registry) you can use a developer PAT (personal access token) when downloading a private image. These can be supplied as:
```yaml
credentials:
  bearer:
    scheme: "Bearer"
    token: "<PAT>"
```

### Miscellaneous

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `labels` | `object` | Yes | Set of key-value pairs that uniquely identify the OPA instance. Labels are included when OPA uploads decision logs and status information. |
| `default_decision` | `string` | No (default: `/system/main`) | Set path of default policy decision used to serve queries against OPA's base URL. |
| `default_authorization_decision` | `string` | No (default: `/system/authz/allow`) | Set path of default authorization decision for OPA's API. |
| `persistence_directory` | `string` | No (default `$PWD/.opa`) | Set directory to use for persistence with options like `bundles[_].persist`. |
| `plugins` | `object` | No (default: `{}`) | Location for custom plugin configuration. See [Plugins](../plugins) for details. |
| `nd_builtin_cache` | `boolean` | No (default: `false`) | Enable the non-deterministic builtins caching system during policy evaluation, and include the contents of the cache in decision logs. Note that decision logs that are larger than `upload_size_limit_bytes` will drop the `nd_builtin_cache` key from the log entry before uploading. |

### Keys

Keys is a dictionary mapping the key name to the actual key and optionally the algorithm and scope.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `keys[_].key` | `string` | Yes (unless `private_key` provided) | PEM encoded public key to use for signature verification. |
| `keys[_].private_key` | `string` | Yes (unless `key` provided`) | PEM encoded private key to use for signing. |
| `keys[_].algorithm` | `string` | No (default: `RS256`) | Name of the signing algorithm. |
| `keys[_].scope` | `string` | No | Scope to use for bundle signature verification. |

> Note: If the `scope` is provided in a bundle's `signing` configuration (ie. `bundles[_].signing.scope`),
> it takes precedence over `keys[_].scope`.

The following signing algorithms are supported:

| Name | Description |
| --- | --- |
| `ES256` | ECDSA using P-256 and SHA-256 |
| `ES384` | ECDSA using P-384 and SHA-384 |
| `ES512` | ECDSA using P-521 and SHA-512 |
| `HS256` | HMAC using SHA-256 |
| `HS384` | HMAC using SHA-384 |
| `HS512` | HMAC using SHA-512 |
| `PS256` | RSASSA-PSS using SHA256 and MGF1-SHA256 |
| `PS384` | RSASSA-PSS using SHA384 and MGF1-SHA384 |
| `PS512` | RSASSA-PSS using SHA512 and MGF1-SHA512 |
| `RS256` | RSASSA-PKCS-v1.5 using SHA-256 |
| `RS384` | RSASSA-PKCS-v1.5 using SHA-384 |
| `RS512` | RSASSA-PKCS-v1.5 using SHA-512 |

### Caching

Caching represents the configuration of the inter-query cache that built-in functions can utilize.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `caching.inter_query_builtin_cache.max_size_bytes` | `int64` | No | Inter-query cache size limit in bytes. OPA will drop old items from the cache if this limit is exceeded. By default, no limit is set. |

### Bundles

Bundles are defined with a key that is the `name` of the bundle. This `name` is used in the status API, decision logs,
server provenance, etc.

Each bundle can be configured to verify a bundle signature using the `keyid` and `scope` fields. The `keyid` is the name of
one of the keys listed under the [keys](#keys) entry.

Signature verification fails if the `bundles[_].signing` field is configured on a bundle but no `.signatures.json` file is
included in the actual bundle gzipped tarball.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `bundles[_].resource` | `string` | No (default: `bundles/<name>`) | Resource path to use to download bundle from configured service. |
| `bundles[_].service` | `string` | Yes | Name of service to use to contact remote server. |
| `bundles[_].polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between bundle downloads. |
| `bundles[_].polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between bundle downloads. |
| `bundles[_].trigger` | `string`  (default: `periodic`) | No | Controls how bundle is downloaded from the remote server. Allowed values are `periodic` and `manual`. |
| `bundles[_].polling.long_polling_timeout_seconds` | `int64` | No | Maximum amount of time the server should wait before issuing a timeout if there's no update available. |
| `bundles[_].persist` | `bool` | No | Persist activated bundles to disk. |
| `bundles[_].signing.keyid` | `string` | No | Name of the key to use for bundle signature verification. |
| `bundles[_].signing.scope` | `string` | No | Scope to use for bundle signature verification. |
| `bundles[_].signing.exclude_files` | `array` | No | Files in the bundle to exclude during verification. |
| `bundles[_].size_limit_bytes` | `int64` | No (default: `1073741824`) | Size limit for individual files contained in the bundle. |

### Status

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `status.service` | `string` | Yes | Name of service to use to contact remote server. |
| `status.partition_name` | `string` | No | Path segment to include in status updates. |
| `status.console` | `boolean` | No (default: `false`) | Log the status updates locally to the console. When enabled alongside a remote status update API the `service` must be configured, the default `service` selection will be disabled. |
| `status.prometheus` | `boolean` | No (default: `false`) | Export the status (bundle and plugin) metrics to prometheus (see [the monitoring documentation](../monitoring/#prometheus)). When enabled alongside a remote status update API the `service` must be configured, the default `service` selection will be disabled. |
| `status.plugin` | `string` | No | Use the named plugin for status updates. If this field exists, the other configuration fields are not required. |
| `status.trigger` | `string`  (default: `periodic`) | No | Controls how status updates are reported to the remote server. Allowed values are `periodic` and `manual`. |

### Decision Logs

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `decision_logs.service` | `string` | No | Name of the service to use to contact remote server. If no `plugin` is specified, and `console` logging is disabled, this will default to the first `service` name defined in the Services configuration. |
| `decision_logs.partition_name` | `string` | No | Deprecated: Use `resource` instead. Path segment to include in status updates. |
| `decision_logs.resource` | `string` | No (default: `/logs`) | Full path to use for sending decision logs to a remote server. |
| `decision_logs.reporting.buffer_size_limit_bytes` | `int64` | No | Decision log buffer size limit in bytes. OPA will drop old events from the log if this limit is exceeded. By default, no limit is set. Only one of `buffer_size_limit_bytes`, `max_decisions_per_second` may be set. |
| `decision_logs.reporting.max_decisions_per_second` | `float64` | No | Maximum number of decision log events to buffer per second. OPA will drop events if the rate limit is exceeded. Only one of `buffer_size_limit_bytes`, `max_decisions_per_second` may be set. |
| `decision_logs.reporting.upload_size_limit_bytes` | `int64` | No (default: `32768`) | Decision log upload size limit in bytes. OPA will chunk uploads to cap message body to this limit. |
| `decision_logs.reporting.min_delay_seconds` | `int64` | No (default: `300`) | Minimum amount of time to wait between uploads. |
| `decision_logs.reporting.max_delay_seconds` | `int64` | No (default: `600`) | Maximum amount of time to wait between uploads. |
| `decision_logs.reporting.trigger` | `string` | No (default: `periodic`) | Controls how decision logs are reported to the remote server. Allowed values are `periodic` and `manual`. |
| `decision_logs.mask_decision` | `string` | No (default: `/system/log/mask`) | Set path of masking decision. |
| `decision_logs.drop_decision` | `string` | No (default: `/system/log/drop`) | Set path of drop decision. |
| `decision_logs.plugin` | `string` | No | Use the named plugin for decision logging. If this field exists, the other configuration fields are not required. |
| `decision_logs.console` | `boolean` | No (default: `false`) | Log the decisions locally to the console. When enabled alongside a remote decision logging API the `service` must be configured, the default `service` selection will be disabled. |

### Discovery

| Field | Type | Required | Description                                                                                                                                                 |
| --- | --- | --- |-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `discovery.resource` | `string` | Yes | Resource path to use to download bundle from configured service.                                                                                            |
| `discovery.service` | `string` | No | Name of the service to use to contact remote server. If omitted, the configuration must contain exactly one service. Discovery will default to this service. |
| `discovery.decision` | `string` | No | The path of the decision to evaluate in the discovery bundle. By default, OPA will evaluate `data` in the discovery bundle to produce the configuration.    |
| `discovery.polling.min_delay_seconds` | `int64` | No (default: `60`) | Minimum amount of time to wait between configuration downloads.                                                                                             |
| `discovery.polling.max_delay_seconds` | `int64` | No (default: `120`) | Maximum amount of time to wait between configuration downloads.                                                                                             |
| `discovery.trigger` | `string`  (default: `periodic`) | No | Controls how bundle is downloaded from the remote server. Allowed values are `periodic` and `manual`.                                                       |
| `discovery.polling.long_polling_timeout_seconds` | `int64` | No | Maximum amount of time the server should wait before issuing a timeout if there's no update available.                                                      |
| `discovery.signing.keyid` | `string` | No | Name of the key to use for bundle signature verification.                                                                                                   |
| `discovery.signing.scope` | `string` | No | Scope to use for bundle signature verification.                                                                                                             |
| `discovery.signing.exclude_files` | `array` | No | Files in the bundle to exclude during verification.                                                                                                         |
| `discovery.persist` | `bool` | No | Persist activated discovery bundle to disk.                                                                                                                 |

> ⚠️ The plugin trigger mode configured on the discovery plugin will be inherited by the bundle, decision log
> and status plugins. For example, if the discovery plugin is configured to use the manual trigger mode, all other
> plugins will use manual triggering as well. If any of the plugins explicitly specify a different mode (for ex. periodic),
> OPA will generate a configuration error.

The following `discovery` configuration fields are supported but deprecated:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `discovery.prefix` | `string` | No (default: `bundles`) | Deprecated: Use `resource` instead. Path prefix to use to download configuration from remote server. |
| `discovery.name` | `string` | No | Deprecated: Use `resource` instead. Name of the discovery configuration to download. If `discovery.name` is specified and `discovery.resource` is unset, the `discovery.decision` field will default to the `discovery.name` value. |

### Distributed tracing

Distributed tracing represents the configuration of the OpenTelemetry Tracing.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `distributed_tracing.type` | `string` | No | Setting this to "grpc" enables distributed tracing with an collector gRPC endpoint |
| `distributed_tracing.address` | `string` | No (default: `localhost:4317`) | Address of the OpenTelemetry Collector gRPC endpoint. |
| `distributed_tracing.service_name` | `string` | No (default: `opa`) | Logical name of the service. |
| `distributed_tracing.sample_percentage` | `int` | No (default: `100`) | Percentage of traces that are sampled and exported. |
| `distributed_tracing.encryption` | `string` | No (default: `off`) | Configures TLS. |
| `distributed_tracing.allow_insecure_tls` | `bool` | No (default: `false`) | Allow insecure TLS. |
| `distributed_tracing.tls_ca_cert_file` | `string` | No | The path to the root CA certificate. |
| `distributed_tracing.tls_cert_file` | `string` | No (unless `encryption` equals `mtls`) | The path to the client certificate to authenticate with. |
| `distributed_tracing.tls_private_key_file` | `string` | No (unless `tls_cert_file` provided)  | The path to the private key of the client certificate. |

The following encryption methods are supported:

| Name | Description |
| --- | --- |
| `off` | Disable TLS |
| `tls` | Enable TLS |
| `mtls` | Enable mutual TLS |

### Disk Storage

The `storage` configuration key allows for enabling, and configuring, the
persistent on-disk storage of an OPA instance.

If `disk` is set to something, the server will enable the on-disk store
with data put into the configured `directory`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `storage.disk.directory` | `string` | Yes | This is the directory to use for storing the persistent database. |
| `storage.disk.auto_create` | `bool` | No (default: `false`) | If set to true, the configured directory will be created if it does not exist. |
| `storage.disk.partitions` | `array[string]` | No | Non-overlapping `data` prefixes used for partitioning the data on disk. |
| `storage.disk.badger` | `string` | No (default: empty) | "Superflags" passed to Badger allowing to modify advanced options. |

See [the docs on disk storage](../misc-disk/) for details about the settings.

### Server

The `server` configuration sets the gzip compression settings for `/v0/data`, `/v1/data` and `/v1/compile` HTTP `POST` endpoints
The gzip compression settings are used when the client sends `Accept-Encoding: gzip`

| Field                                    | Type  | Required            | Description                                                                                                                                                                                                                                  |
|------------------------------------------|-------|---------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `server.encoding.gzip.min_length`        | `int` | No, (default: 1024) | Specifies the minimum length of the response to compress                                                                                                                                                                                     |
| `server.encoding.gzip.compression_level` | `int` | No, (default: 9)    | Specifies the compression level. Accepted values: a value of either 0 (no compression), 1 (best speed, lowest compression) or 9 (slowest, best compression). See https://pkg.go.dev/compress/flate#pkg-constants |

# Security

This document provides guidelines for deploying OPA inside untrusted
environments. You should read this document if you are deploying OPA as a
service.

Securing the API involves configuring OPA to use TLS, authentication, and
authorization so that:

- Traffic between OPA and clients is encrypted.
- Clients verify the OPA API endpoint identity.
- OPA verifies client identities.
- Clients are only granted access to specific APIs or sections of [The `data` Document](/how-does-opa-work.md#the-data-document).

## TLS and HTTPS

HTTPS is configured by specifying TLS credentials via command line flags at
startup:

- ``--tls-cert-file=<path>`` specifies the path of the file containing the TLS certificate.
- ``--tls-private-key-file=<path>`` specifies the path of the file containing the TLS private key.

OPA will exit immediately with a non-zero status code if only one of these flags
is specified.

By default, OPA ignores insecure HTTP connections when TLS is enabled. To allow
insecure HTTP connections in addition to HTTPS connections, provide another
listening address with `--insecure-addr`.

### 1. Generate the TLS credentials for OPA (Example)

```bash
openssl genrsa -out private.key 2048
openssl req -new -x509 -sha256 -key private.key -out public.crt -days 1
```

> We have generated a self-signed certificate for example purposes here. DO NOT
rely on self-signed certificates outside of development without understanding
the risks.

### 2. Start OPA with TLS enabled

```bash
opa run --server --log-level debug \
    --tls-cert-file public.crt \
    --tls-private-key-file private.key
```

### 3. Try to access the API with HTTP

```bash
curl http://localhost:8181/v1/data
```

### 4. Access the API with HTTPS

```bash
curl -k https://localhost:8181/v1/data
```

> We have to use cURL's `-k/--insecure` flag because we are using a
> self-signed certificate.

## Authentication and Authorization

This section shows how to configure OPA to authenticate and authorize client
requests. Client-side authentication of the OPA API endpoint should be handled
with TLS.

Authentication and authorization allow OPA to:

- Verify client identities.
- Control client access to APIs and data.

Both are configured via command line flags:

- ``--authentication=<scheme>`` specifies the authentication scheme to use.
- ``--authorization=<scheme>`` specifies the authorization scheme to use.

By default, OPA does not perform authentication or authorization and these flags
default to `off`.

For authentication, OPA supports:

- [Bearer tokens](/rest-api.md#bearer-tokens): Bearer tokens are enabled by
starting OPA with ``--authentication=token``. When the `token` authentication
mode is enabled, OPA will extract the Bearer token from incoming API requests
and provide to the authorization handler. When you use the `token`
authentication, you must configure an authorization policy that checks the
tokens. If the client does not supply a Bearer token, the `input.identity`
value will be undefined when the authorization policy is evaluated.

For authorization, OPA relies on policy written in Rego. Authorization is
enabled by starting OPA with ``--authorization=basic``.

When the `basic` authorization scheme is enabled, a minimal authorization policy
must be provided on startup. The authorization policy must be structured as follows:

```ruby
# The "system" namespace is reserved for internal use
# by OPA. Authorization policy must be defined under
# system.authz as follows:
package system.authz

default allow = false  # Reject requests by default.

allow {
  # Logic to authorize request goes here.
}
```

When OPA receives a request, it executes a query against the document defined
`data.system.authz.allow`. The implementation of the policy may span multiple
packages however it is recommended that administrators keep the policy under the
`system` namespace.

If the document produced by the ``allow`` rule is ``true``, the request is
processed normally. If the document is undefined or **not** ``true``, the
request is rejected immediately.

OPA provides the following input document when executing the authorization
policy:

```ruby
{
    "input": {
        # Identity established by authentication scheme.
        # When Bearer tokens are used, the identity is
        # set to the Bearer token value.
        "identity": <String>,

        # One of {GET, POST, PUT, PATCH, DELETE}.
        "method": <HTTP Method>,

        # URL path represented as an array.
        # For example: /v1/data/exempli-gratia
        # is represented as [v1, data, exampli-gratia]
        "path": <HTTP URL Path>,
    }
}
```

At a minimum, the authorization policy should grant access to a special root
identity:

```ruby
package system.authz

default allow = false          # Reject requests by default.

allow {                        # Allow request if...
    "secret" = input.identity  # Identity is the secret root key.
}
```

When OPA is configured with this minimal authorization policy, requests without
authentication are rejected:

```http
GET /v1/policies HTTP/1.1
```

Response:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
```

```json
{
  "code": "unauthorized",
  "message": "request rejected by administrative policy"
}
```

However, if Bearer token authentication is enabled and the request includes the
secret from above, the request is allowed:

```http
GET /v1/policies HTTP/1.1
Authorization: Bearer secret
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

When Bearer tokens are used for authentication, the policy should at minimum
validate the identity:

```ruby
package system.authz

# Tokens may defined in policy or pushed into OPA as data.
tokens = {
    "my-secret-token-foo": {
        "roles": ["admin"]
    },
    "my-secret-token-bar": {
        "roles": ["service-1"]
    },
    "my-secret-token-baz": {
        "roles": ["service-2", "service-3"]
    }
}

default allow = false          # Reject requests by default.

allow {                        # Allow request if...
    input.identity = "secret"  # Identity is the secret root key.
}

allow {                        # Allow request if...
    tokens[input.identity]     # Identity exists in "tokens".
}
```

To complete this example, the policy could further restrict tokens to specific
documents:

```ruby
package system.authz

# Rights may be defined in policy or pushed into OPA as data.
rights = {
    "admin": {
        "path": "*"
    },
    "service-1": {
        "path": ["v1", "data", "exempli", "gratia"]
    },
    "service-2": {
        "path": ["v1", "data", "par", "example"]
    }
}

# Tokens may be defined in policy or pushed into OPA as data.
tokens = {
    "my-secret-token-foo": {
        "roles": ["admin"]
    },
    "my-secret-token-bar": {
        "roles": ["service-1"]
    },
    "my-secret-token-baz": {
        "roles": ["service-2", "service-3"]
    }
}

default allow = false               # Reject requests by default.

allow {                             # Allow request if...
    identity_rights[right]          # Rights for identity exist, and...
    right.path = "*"                # Right.path is '*'.
} {                                 # Or...
    identity_rights[right]          # Rights for identity exist, and...
    right.path = input.path         # Right.path matches input.path.
}

identity_rights[right] {            # Right is in the identity_rights set if...
    token = tokens[input.identity]  # Token exists for identity, and...
    role = token.roles[_]           # Token has a role, and...
    right = rights[role]            # Role has rights defined.
}
```

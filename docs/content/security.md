---
title: Security
kind: operations
weight: 40
---

This document provides guidelines for deploying OPA inside untrusted
environments. You should read this document if you are deploying OPA as a
service.

Securing the API involves configuring OPA to use TLS, authentication, and
authorization so that:

- Traffic between OPA and clients is encrypted.
- Clients verify the OPA API endpoint identity.
- OPA verifies client identities.
- Clients are only granted access to specific APIs or sections of [The `data` Document](../philosophy/#the-opa-document-model).

## TLS and HTTPS

HTTPS is configured by specifying TLS credentials via command line flags at
startup:

- ``--tls-cert-file=<path>`` specifies the path of the file containing the TLS certificate.
- ``--tls-private-key-file=<path>`` specifies the path of the file containing the TLS private key.

OPA will exit immediately with a non-zero status code if only one of these flags
is specified.

The server can track the certificate and key files' contents, and reload them if necessary:

- ``--tls-cert-refresh-period=<duration>`` specifies how often OPA should check the TLS certificate and
  private key file for changes (defaults to 0s, disabling periodic refresh). This argument accepts
  any duration, such as "30s", "5m" or "24h".

Note that for using TLS-based authentication, a CA cert file can be provided:

- ``--tls-ca-cert-file=<path>`` specifies the path of the file containing the CA cert.

If provided, it will be used to validate clients' TLS certificates when using TLS
authentication (see below).

By default, OPA ignores insecure HTTP connections when TLS is enabled. To allow
insecure HTTP connections in addition to HTTPS connections, provide another
listening address with `--addr`. For example:

```bash
opa run --server \
  --log-level debug \
  --tls-cert-file public.crt \
  --tls-private-key-file private.key \
  --addr https://0.0.0.0:8181 \
  --addr http://localhost:8282
```

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

{{< info >}}
We have to use cURL's `-k/--insecure` flag because we are using a self-signed certificate.
{{< /info >}}

## Interface Binding

OPA can be configured to listen on specific interfaces using the `--addr` flag. For example:

```bash
opa run --server \
  --log-level debug \
  --addr localhost:8181 \
```

By default, OPA binds to the 0.0.0.0 interface, which allows the OPA server to be exposed to services running outside of the same machine. It's important to note that binding OPA to the 0.0.0.0 interface by itself is not inherently insecure in a trusted environment, exposing OPA to the outside world would also require opening ports and likely a similar procedure on a gateway layer above.

In situations where OPA is not intended to be exposed to remote services, it is recommended to bind OPA to the localhost interface, which only allows connections from the same machine. If it is necessary to expose OPA to remote services, ensure to follow the security recommendations on this page, such as requiring authentication.


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

- [Bearer tokens](../rest-api#bearer-tokens): Bearer tokens are enabled by
starting OPA with ``--authentication=token``. When the `token` authentication
mode is enabled, OPA will extract the Bearer token from incoming API requests
and provide to the authorization handler. When you use the `token`
authentication, you must configure an authorization policy that checks the
tokens. If the client does not supply a Bearer token, the `input.identity`
value will be undefined when the authorization policy is evaluated.
- Client TLS certificates: Client TLS authentication is enabled by starting
OPA with ``--authentication=tls``. When this authentication mode is enabled,
OPA will require all clients to provide a client certificate. It is verified
against the CA certificate(s) provided via `--tls-ca-cert-file`. Upon successful
verification, the `input.identity` value is set to the TLS certificate's
subject.

  Note that TLS authentication does not disable non-HTTPS listeners. To ensure
  that all your communication is secured, it should be paired with an
  authorization policy (see below) that at least requires the client identity
  (`input.identity`) to _be set_.

For authorization, OPA relies on policy written in Rego. Authorization is
enabled by starting OPA with ``--authorization=basic``.

When the `basic` authorization scheme is enabled, a minimal authorization policy
must be provided on startup. The authorization policy must be structured as follows:

```live:system_ns:module:read_only
# The "system" namespace is reserved for internal use
# by OPA. Authorization policy must be defined under
# system.authz as follows:
package system.authz

default allow := false  # Reject requests by default.

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
request is rejected immediately. The count of requests rejected by an OPA instance
are surfaced via the performance metrics in the [Status](../management-status) information.

OPA provides the following `input` document when executing the authorization
policy:

<!-- TODO(sr): check if "jsonc" looks alright on netlify -->
```jsonc
{
    # Identity value established by authentication scheme.
    # When Bearer tokens are used, the identity is
    # set to the Bearer token value.
    # When TLS client certificates are used, the identity
    # is set to the certificate subject RDNSequence.
    # E.g. "OU=opa-client-01,O=Example"
    # Note: client certificate data is available in the
    # 'client_certificates' key.
    "identity": "",
    
    # Client certificates provided by the client when calling OPA
    # over an mTLS connection. Represented in input as a list of
    # Go x509.Certificate objects marshalled as JSON.
    "client_certificates": [],

    # One of {"GET", "POST", "PUT", "PATCH", "DELETE"}.
    "method": "",

    # URL path represented as an array.
    # For example: /v1/data/exempli-gratia
    # is represented as ["v1", "data", "exampli-gratia"]
    "path": [...],

    # URL parameters represented as an object of string arrays.
    # For example: metrics&explain=true is represented as
    # {"metrics": [""], "explain": ["true"]}
    "params": {"...": ...},

    # Request headers represented as an object of string arrays.
    #
    # Example Request Headers:
    #
    #   host: acmecorp.com
    #   x-custom: secretvalue
    #
    # Example input.headers Value:
    #
    #   {"Host": ["acmecorp.com"], "X-Custom": ["mysecret"]}
    #
    # Example header check:
    #
    #   input.headers["X-Custom"][_] == "mysecret"
    #
    # Header keys follow canonical MIME form. The first character and any
    # characters following a hyphen are uppercase. The rest are lowercase.
    # If the header key contains space or invalid header field bytes,
    # no conversion is performed.
    "headers": {"...": [...]},

    # Request message body if present for applicable APIs.
    #
    # Example Request:
    #
    #   POST v1/data HTTP/1.1
    #   Content-Type: application/json
    #
    #   {"input": {"action": "trade", "stock": "ACME"}}
    #
    # Example input.body Value:
    #
    #   {"input": {"action": "trade", "stock": "ACME"}}
    #
    # Example body check:
    #
    #   input.body.input.stock == "ACME"
    #
    # The 'body' field is provided for the following APIs:
    #
    #   * POST v1/data
    #   * POST v0/data
    #   * POST /
    "body": ...,
}
```

At a minimum, the authorization policy should grant access to a special root
identity:

```live:system_authz_secret:module:read_only
package system.authz

default allow := false           # Reject requests by default.

allow {                         # Allow request if...
    "secret" == input.identity  # Identity is the secret root key.
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

Besides boolean responses, authorization policies can change the message included
in the deny response. Do do that, policy decisions must yield an object response as
follows:

```live:system_authz_object_resp:module:read_only
package system.authz

default allow := {
    "allowed": false,
    "reason": "unauthorized resource access"
}

allow := { "allowed": true } {   # Allow request if...
    "secret" == input.identity  # identity is the secret root key.
}

allow := { "allowed": false, "reason": reason } {
    not input.identity
    reason := "no identity provided"
}
```

### Token-based Authentication Example

When Bearer tokens are used for authentication, the policy should at minimum
validate the identity:

```live:system_authz_bearer:module:read_only
package system.authz

# Tokens may defined in policy or pushed into OPA as data.
tokens := {
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

default allow := false           # Reject requests by default.

allow {                         # Allow request if...
    input.identity == "secret"  # Identity is the secret root key.
}

allow {                        # Allow request if...
    tokens[input.identity]     # Identity exists in "tokens".
}
```

To complete this example, the policy could further restrict tokens to specific
documents:

```live:system_authz_bearer_complete:module:read_only
package system.authz

# Rights may be defined in policy or pushed into OPA as data.
rights := {
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
tokens := {
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

default allow := false               # Reject requests by default.

allow {                             # Allow request if...
    some right
    identity_rights[right]          # Rights for identity exist, and...
    right.path == "*"               # Right.path is '*'.
}

allow {                             # Allow request if...
    some right
    identity_rights[right]          # Rights for identity exist, and...
    right.path == input.path        # Right.path matches input.path.
}

identity_rights[right] {             # Right is in the identity_rights set if...
    token := tokens[input.identity]  # Token exists for identity, and...
    role := token.roles[_]           # Token has a role, and...
    right := rights[role]            # Role has rights defined.
}
```

### TLS-based Authentication Example

To set up authentication based on mutual TLS, we will need three certificates:

1. the CA cert (self-signed),
2. the server cert (signed by the CA), and
3. the client cert (signed by the CA).

We use `openssl` to create the example certificates and keys used in this demo. In production, creation of certificates
and keys should be handled by an automated process out of scope for this tutorial.

Note that we also create an extra client cert (client-2). While this certificate is signed by the same CA, it's identity
is different. We'll use this to show our authorization policy in action.

```bash
# CA
openssl ecparam -out ca-key.pem -name prime256v1 -genkey 
openssl req -x509 -new -nodes -key ca-key.pem -days 30 -out ca.pem -subj "/CN=my-ca"

# client 1
cat <<EOF >req.cnf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
subjectAltName = @alt_names

[alt_names]
URI.1 = spiffe://example.com/client-1
EOF
openssl ecparam -out client-key-1.pem -name prime256v1 -genkey 
openssl req -new -key client-key-1.pem -out csr.pem -subj "/CN=client-1" -config req.cnf 
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out client-cert-1.pem -days 10 -extensions v3_req -extfile req.cnf -sha256

# client 2
cat <<EOF >req.cnf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
subjectAltName = @alt_names

[alt_names]
URI.1 = spiffe://example.com/client-2
EOF
openssl ecparam -out client-key-2.pem -name prime256v1 -genkey 
openssl req -new -key client-key-2.pem -out csr.pem -subj "/CN=client-2" -config req.cnf
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out client-cert-2.pem -days 10 -extensions v3_req -extfile req.cnf -sha256

# create server cert with IP and DNS SANs
cat <<EOF >req.cnf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = opa.example.com
IP.1 = 127.0.0.1
URI.1 = spiffe://example.com/server
EOF
openssl ecparam -out server-key.pem -name prime256v1 -genkey 
openssl req -new -key server-key.pem -out csr.pem -subj "/CN=server" -config req.cnf
openssl x509 -req -in csr.pem -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 10 -extensions v3_req -extfile req.cnf -sha256
```

We also create an example authorization policy file, called `check.rego`. This example `system.authz` policy will check
the certificate ID against a list of allowed paths as defined in a simple Access Control List.

{{< danger >}}
When choosing messages to return to unauthorized clients in `system.authz` policies, be careful not to expose sensitive
information such as which paths are allowed.
{{< /danger >}}

```live:system_authz_x509:module:read_only
package system.authz

import future.keywords.if
import future.keywords.in

id_uri := input.client_certificates[0].URIs[0]
id_string := sprintf("%s://%s%s", [id_uri.Scheme, id_uri.Host, id_uri.Path])

# client_acl represents an access control list and may defined in policy or pushed into OPA as data changes.
client_acl := {
  "spiffe://example.com/client-1": [["v1", "data"]],
  "spiffe://example.com/client-2": [],
}

default allow := {"allowed": false, "reason": "Access denied: unknown caller"}

allow := { "allowed": true } if {
  input.path in client_acl[id_string]
} else := {
  "allowed": false,
  "reason": sprintf("%s is not allowed to call /%s", [
    id_string,
    concat("/", input.path),
  ])
}
```

Now, we're ready to starting the server with `-authentication=tls` and the
certificate-related parameters:
```console
$ opa run -s \
  --tls-cert-file server-cert.pem \
  --tls-private-key-file server-key.pem \
  --tls-ca-cert-file ca.pem \
  --authentication=tls \
  --authorization=basic \
  -a https://127.0.0.1:8181 \
  check.rego
{"addrs":["https://127.0.0.1:8181"],"diagnostic-addrs":[],"level":"info","msg":"Initializing server.","time":"2023-01-04T10:31:12Z"}
```

We can use `curl` to validate our TLS-based authentication setup:

First, we use the client certificate that was signed by the CA, and has a subject
matching our authorization policy:

```console
$ curl --key client-key-1.pem \
  --cert client-cert-1.pem \
  --cacert ca.pem \
  --resolve opa.example.com:8181:127.0.0.1 \
  https://opa.example.com:8181/v1/data
{"result":{}}
```

Note that we're passing the CA cert to curl -- this is done to have curl accept
the server's certificate, which has been signed by our CA cert.

Since we've set up an IP SAN, we may also `curl https://127.0.0.1:8181/v1/data`
directly. (To keep our examples focused, we'll do that from here on.)

Using a valid certificate whose subject will be declined by our authorization
policy:

```console
$ curl --key client-key-2.pem \
  --cert client-cert-2.pem \
  --cacert ca.pem \
  https://127.0.0.1:8181/v1/data
{
  "code": "unauthorized",
  "message": "spiffe://example.com/client-2 is not allowed to call /v1/data"
}
```

Finally, we'll attempt to query without a client certificate:
```console
$ curl --cacert ca.pem https://127.0.0.1:8181/v1/data
curl: (56) LibreSSL SSL_read: error:1404C412:SSL routines:ST_OK:sslv3 alert bad certificate, errno 0
```

As you can see, TLS-based authentication disallows these request before even invoking the `system.authz` policy.

## Secure Health and Monitoring

Often OPA is deployed locally to the host where the client resides (side-car or
similar model). In these deployments it is ideal to only expose the API via
`localhost` to prevent any remote clients from reaching OPA at all. The downside
to this approach is that it blocks remote monitoring systems that require access
to `/health` or `/metrics`.

The solution is to configure OPA with a separate diagnostic listener by
providing the `--diagnostic-addr` flag, for example:

```
$ opa run \
  -s \
  --addr localhost:8181 \
  --diagnostic-addr :8282
```
The configuration above would expose only `/health` and `/metrics` API's on port
`8282` while keeping the normal REST API bound to `localhost:8181`.

> When the diagnostic listener is enabled, the `/metrics` and `/health` APIs will
> still be exposed on the normal listener.

## Hardened Configuration Example

You can run a hardened OPA deployment with minimal configuration. There are a
few things to keep in mind:

* Limit API access to host-local clients executing policy queries.
* Configure TLS (for localhost TCP) or a UNIX domain socket.
* Do not pass credentials as command-line arguments.
* Run OPA as a non-root user ideally inside it's own account.

With OPA configured to fetch policies using the [Bundles](../management-bundles) feature
you can configure OPA with a restrictive authorization policy that only grants
clients access to the default policy decision, i.e., `POST /`:

```live:hardened_example:module:read_only
package system.authz

# Deny access by default.
default allow := false

# Allow anonymous access to the default policy decision.
allow {
    input.method == "POST"
    input.path == [""]
}
```

The example below shows flags that tell OPA to:

* Authorize all API requests (`--authorization=basic`)
* Listen on localhost for HTTPS (not HTTP!) connections (`--addr`, `--tls-cert-file`, `--tls-private-key-file`)
* Download bundles from a remote HTTPS endpoint (`--set` flags and `--set-file` flag)

```bash
opa run \
    --server \
    --authorization=basic \
    --addr=https://localhost:8181 \
    --tls-cert-file=/var/tmp/server.crt \
    --tls-private-key-file=/var/tmp/server.key \
    --set=bundles.authz.service=default \
    --set=bundles.authz.resource=myapp_authz_bundle \
    --set=services.default.url=https://control.acmecorp.com \
    --set-file=services.default.credentials.bearer.token=/var/tmp/secret-bearer-token
```

> The `/var/tmp/secret-bearer-token` will store the credential in plaintext. You
> should make sure that file permission(s) are setup to limit access.

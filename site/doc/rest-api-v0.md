# REST API (V0)

This document is the authoritative specification of the OPA REST API (V0).
The V0 API represents the simplest way to evaluate policies.

Use the V0 API if you are enforcing policy decisions via webhooks that have
pre-defined request/response formats that are incompatible with the [V1
API](../rest-v1).

The V0 API does not support policy management, ad-hoc queries, or higher-order
functionality on Data API queries such as profiling or explanations.

## <a name="data-api"></a> Data API

The Data API exposes endpoints for reading and writing documents in OPA. For an
introduction to the different types of documents in OPA see [How Does OPA
Work?](../../how-does-opa-work/).

### Get a Document

```
POST /v0/data/{path:.+}
Content-Type: application/json
```

Get a document.

The request message body defines the content of the [The input Document](../../how-does-opa-work#the-input-document). The request message body may be empty. The path separator is used to access values inside object and array documents.

The examples below assume the following policy:

```ruby
package opa.examples

import input.example.flag

allow_request { flag = true }
```

#### Example Request

```http
POST /v0/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "example": {
    "flag": true
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
true
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If the requested document is missing or undefined, the server will return 404 and the message body will contain an error object.

## Authentication

The V0 API supports the same authentication methods as the V1 API. See the  See the [Authentication section of the V1 API](../rest-v1#authentication) for more details.

## Errors

The V0 API supports the same error format as the V1 API. See the [Errors section of the V1 API](../rest-v1#errors) for more details.

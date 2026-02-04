---
sidebar_position: 5
title: API Reference
---

## Overview

The OCP server exposes a REST API for managing OPA bundles, sources, stacks, and data. All API endpoints except `/health` require Bearer token authentication.

## Authentication

- **Method**: Bearer token authentication
- **Header**: `Authorization: Bearer <api-key>`
- **Middleware**: All `/v1/*` endpoints require valid API key
- **Unauthorized Response**: HTTP 401 with "Unauthorized" message

### Example Authentication Header

```yaml
Authorization: Bearer admin-api-key-change-me
```

## Base URL Structure

- **Health Check**: `/health`
- **API Endpoints**: `/v1/*`

---

## Health Check Endpoint

### `GET /health`

**Description**: Health check endpoint to verify server readiness

**Authentication**: None required

**Response**:

- **200 OK**: Server is healthy and ready
- **500 Internal Server Error**: Server is not ready

**Example Response**:

```json
{}
```

---

## Bundle Management

Bundles are collections of policies and data that can be distributed to OPA instances.

### `GET /v1/bundles`

**Description**: List all bundles with pagination support

**Query Parameters**:

- `limit` (optional): Number of results to return (max 100, default 100\)
- `cursor` (optional): Pagination cursor for next page
- `pretty` (optional): Pretty-print JSON response (default true if param present with no value)

**Example Response**:

```json
{
  "result": [
    {
      "name": "main-bundle",
      "labels": {
        "env": "production",
        "team": "platform"
      },
      "object_storage": {
        "aws": {
          "bucket": "my-policy-bundles",
          "key": "bundles/my-app/bundle.tar.gz",
          "region": "us-east-1",
          "credentials": "aws-creds"
        }
      },
      "requirements": [
        {
          "source": "my-app-policies"
        },
        {
          "source": "shared-policies",
          "path": "library",
          "prefix": "shared.lib"
        }
      ],
      "excluded_files": [
        "*.test.rego",
        ".git/*"
      ]
    }
  ],
  "next_cursor": "eyJpZCI6IjEyMyIsInRzIjoxNjkwMjM0NTY3fQ=="
}
```

### `GET /v1/bundles/{bundle}`

**Description**: Get a specific bundle by name

**Path Parameters**:

- `bundle`: URL-encoded bundle name

**Example Response**:

```json
{
  "result": {
    "name": "my-app-bundle",
    "labels": {
      "env": "production",
      "version": "1.0.0"
    },
    "object_storage": {
      "filesystem": {
        "path": "/bundles/my-app/bundle.tar.gz"
      }
    },
    "requirements": [
      {
        "source": "my-app-policies"
      },
      {
        "git": {
          "commit": "abc123def456"
        }
      }
    ],
    "excluded_files": [
      "*.md",
      "docs/*"
    ]
  }
}
```

### `PUT /v1/bundles/{bundle}`

**Description**: Create or update a bundle

**Path Parameters**:

- `bundle`: URL-encoded bundle name

**Validation**:

- Bundle name in path must match name in body (if provided)
- If no name in body, path name is used

**Example Request**:

```json
{
  "labels": {
    "env": "staging",
    "team": "security"
  },
  "object_storage": {
    "aws": {
      "bucket": "policy-bundles-staging",
      "key": "bundles/auth-service/bundle.tar.gz",
      "region": "us-west-2",
      "credentials": "aws-staging-creds"
    }
  },
  "requirements": [
    {
      "source": "auth-policies"
    },
    {
      "source": "common-utils",
      "path": "utils",
      "prefix": "shared.utils"
    }
  ],
  "excluded_files": [
    "*.test.rego",
    "examples/*"
  ]
}
```

**Example Response**:

```json
{}
```

### `DELETE /v1/bundles/{bundle}`

**Description**: Delete a bundle

**Path Parameters**:

- `bundle`: URL-encoded bundle name

**Example Response**:

```json
{}
```

---

## Source Management

Sources define where policies and data come from (Git repositories, local files, HTTP endpoints, etc.).

### `GET /v1/sources`

**Description**: List all sources with pagination support

**Query Parameters**: Same as bundles endpoint

**Example Response**:

```json
{
  "result": [
    {
      "name": "myorg-git-policies",
      "git": {
        "repo": "https://github.com/myorg/policies.git",
        "reference": "refs/heads/main",
        "path": "src/policies",
        "included_files": ["*.rego"],
        "excluded_files": ["*.test.rego"],
        "credentials": "github-creds"
      },
      "requirements": [
        {
          "source": "base-policies"
        }
      ]
    },
    {
      "name": "local-policies",
      "directory": "/local/policies",
      "paths": [
        "authz.rego",
        "utils.rego"
      ],
      "datasources": [
        {
          "name": "user-data",
          "path": "external/users",
          "type": "http",
          "config": {
            "url": "https://api.example.com/users"
          },
          "credentials": "api-creds"
        }
      ]
    }
  ],
  "next_cursor": null
}
```

### `GET /v1/sources/{source}`

**Description**: Get a specific source by name

**Path Parameters**:

- `source`: URL-encoded source name

**Example Response**:

```json
{
  "result": {
    "name": "entitlements",
    "builtin": "styra.entitlements.v1",
    "requirements": [
      {
        "source": "foundation-policies"
      }
    ]
  }
}
```

### `PUT /v1/sources/{source}`

**Description**: Create or update a source

**Path Parameters**:

- `source`: URL-encoded source name

**Example Request (Git Source)**:

```json
{
  "git": {
    "repo": "https://github.com/myorg/security-policies.git",
    "reference": "refs/heads/production",
    "commit": "abc123def456789",
    "path": "policies",
    "included_files": ["*.rego"],
    "excluded_files": ["*.test.rego", "examples/*"],
    "credentials": "github-readonly"
  },
  "requirements": [
    {
      "source": "shared-utilities"
    }
  ]
}
```

**Example Request (Directory Source)**:

```json
{
  "directory": "/local/policies",
  "paths": [
    "main.rego",
    "utils.rego"
  ],
  "datasources": [
    {
      "name": "user-directory",
      "path": "external/users",
      "type": "http",
      "config": {
        "url": "https://api.company.com/users",
        "headers": {
          "Accept": "application/json",
          "User-Agent": "OPA-Control-Plane/1.0"
        }
      },
      "credentials": "api-credentials",
      "transform_query": "{user.id: user | user := input.users[_]}"
    }
  ]
}
```

**Example Response**:

```json
{}
```

### `DELETE /v1/sources/{source}`

**Description**: Delete a source

**Path Parameters**:

- `source`: URL-encoded source name

**Validation**:

- Source may not be in use by a bundle or a stack or another source

**Example Response**:

```json
{}
```

---

## Source Data Management

Manage runtime data that gets injected into policy bundles.

### `GET /v1/sources/{source}/data/{path}`

**Description**: Retrieve data from a source at a specific path

**Path Parameters**:

- `source`: URL-encoded source name
- `path`: Data path (automatically appends `/data.json`)

**Example Response**:

```json
{
  "result": {
    "users": [
      {
        "id": "user123",
        "name": "John Doe",
        "roles": ["admin", "user"]
      },
      {
        "id": "user456",
        "name": "Jane Smith",
        "roles": ["user"]
      }
    ],
    "last_updated": "2025-08-07T10:30:00Z"
  }
}
```

### `POST|PUT /v1/sources/{source}/data/{path}`

**Description**: Upload data to a source at a specific path

**Path Parameters**:

- `source`: URL-encoded source name
- `path`: Data path (automatically appends `/data.json`)

**Example Request**:

```json
{
  "permissions": {
    "read": ["admin", "user"],
    "write": ["admin"],
    "delete": ["admin"]
  },
  "resources": [
    {
      "id": "resource1",
      "type": "document",
      "owner": "user123"
    }
  ]
}
```

**Example Response**:

```json
{}
```

### `DELETE /v1/sources/{source}/data/{path}`

**Description**: Delete data from a source at a specific path

**Path Parameters**:

- `source`: URL-encoded source name
- `path`: Data path (automatically appends `/data.json`)

**Example Response**:

```json
{}
```

---

## Stack Management

Stacks define how bundles are distributed to different environments or services based on selectors.

### `GET /v1/stacks`

**Description**: List all stacks with pagination support

**Query Parameters**: Same as bundles endpoint

**Example Response**:

```json
{
  "result": [
    {
      "name": "prod-stack",
      "selector": {
        "environment": ["production"],
        "service": ["auth-service", "api-gateway"]
      },
      "exclude_selector": {
        "region": ["us-west-1"]
      },
      "requirements": [
        {
          "source": "production-policies"
        },
        {
          "source": "security-baseline"
        }
      ]
    }
  ],
  "next_cursor": "eyJpZCI6IjQ1NiIsInRzIjoxNjkwMjM0NTY3fQ=="
}
```

### `GET /v1/stacks/{stack}`

**Description**: Get a specific stack by name

**Path Parameters**:

- `stack`: URL-encoded stack name

**Example Response**:

```json
{
  "result": {
    "name": "default-stack",
    "selector": {
      "team": ["platform", "security"],
      "environment": ["staging", "production"]
    },
    "requirements": [
      {
        "source": "team-policies"
      },
      {
        "source": "compliance-rules"
      }
    ]
  }
}
```

### `PUT /v1/stacks/{stack}`

**Description**: Create or update a stack

**Path Parameters**:

- `stack`: URL-encoded stack name

**Example Request**:

```json
{
  "selector": {
    "environment": ["development"],
    "team": ["backend"]
  },
  "exclude_selector": {
    "deprecated": ["true"]
  },
  "requirements": [
    {
      "source": "dev-policies"
    },
    {
      "source": "testing-utils",
      "path": "utils.testing",
      "prefix": "test.utils"
    }
  ]
}
```

**Example Response**:

```json
{}
```

### `DELETE /v1/stacks/{stack}`

**Description**: Delete a stack

**Path Parameters**:

- `stack`: URL-encoded stack name

**Example Response**:

```json
{}
```

---

## Secrets Management

Secrets can be used in HTTP datasources or Git references.
As a rule, the HTTP API will **never return** a secret value, it only allows authorized requests to get (lists of) their name(s).

### `GET /v1/secrets`

**Description**: List all secrets with pagination support, omitting values

**Query Parameters**: Same as bundles endpoint

**Example Response**:

```json
{
  "result": [
    "api-token",
    "github-ssh-key"
  ],
  "next_cursor": "eyJpZCI6IjQ1NiIsInRzIjoxNjkwMjM0NTY3fQ=="
}
```

### `GET /v1/secrets/{secret}`

**Description**: Get a specific secret by name

**Path Parameters**:

- `secret`: URL-encoded secret name

**Example Response**:

```json
{
  "result": "api-token"
}
```

### `PUT /v1/secrets/{secret}`

**Description**: Create or update a secret

**Path Parameters**:

- `secret`: URL-encoded secret name

**Validation**:

- Secret name in path must match name in body (if provided)
- If no name in body, path name is used

**Example Request**:

```json
{
  "name": "api-token-2",
  "value": {
    "type": "token_auth",
    "token": "open-sesame"
  }
}
```

**Example Response**:

```json
{}
```

### `DELETE /v1/secrets/{secret}`

**Description**: Delete a secret

**Path Parameters**:

- `secret`: URL-encoded secret name

** Validation **

- The secret may not be in use.

**Example Response**:

```json
{}
```

---

## Error Responses

### Standard Error Format

```json
{
  "code": "error_code",
  "message": "error description"
}
```

### Error Codes and Examples

#### 400 Bad Request

```json
{
  "code": "invalid_parameter",
  "message": "bundle name must match path"
}
```

#### 401 Unauthorized

```
HTTP/1.1 401 Unauthorized
Content-Type: text/plain

Unauthorized
```

#### 403 Forbidden

```json
{
  "code": "not_authorized",
  "message": "user does not have permission to access this resource"
}
```

#### 404 Not Found

```json
{
  "code": "not_found",
  "message": "bundle 'my-bundle' not found"
}
```

#### 500 Internal Server Error

```json
{
  "code": "internal_error",
  "message": "database connection failed"
}
```

---

## Configuration Examples

### Object Storage Types

#### AWS S3 Storage

```json
{
  "object_storage": {
    "aws": {
      "bucket": "my-policy-bundles",
      "key": "bundles/my-app/bundle.tar.gz",
      "region": "us-east-1",
      "credentials": "aws-credentials",
      "url": "https://custom-s3-endpoint.com"
    }
  }
}
```

#### Filesystem Storage

```json
{
  "object_storage": {
    "filesystem": {
      "path": "/local/bundles/my-app/bundle.tar.gz"
    }
  }
}
```

#### GCP Cloud Storage

```json
{
  "object_storage": {
    "gcp": {
      "project": "my-gcp-project",
      "bucket": "policy-bundles",
      "object": "bundles/my-app/bundle.tar.gz",
      "credentials": "gcp-service-account"
    }
  }
}
```

#### Azure Blob Storage

```json
{
  "object_storage": {
    "azure": {
      "account_url": "https://mystorageaccount.blob.core.windows.net",
      "container": "policy-bundles",
      "path": "bundles/my-app/bundle.tar.gz",
      "credentials": "azure-credentials"
    }
  }
}
```

### Git Configuration Examples

#### Basic Git Source

```json
{
  "git": {
    "repo": "https://github.com/myorg/policies.git",
    "reference": "refs/heads/main",
    "credentials": "github-token"
  }
}
```

#### Git with Path and File Filtering

```json
{
  "git": {
    "repo": "git@github.com:myorg/monorepo.git",
    "reference": "refs/heads/production",
    "commit": "abc123def456789",
    "path": "services/auth/policies",
    "included_files": ["*.rego", "*.json"],
    "excluded_files": ["*.test.rego", "examples/*"],
    "credentials": "ssh-key"
  }
}
```

### Datasource Examples

#### HTTP Datasource

```json
{
  "datasources": [
    {
      "name": "user-directory",
      "path": "external/users",
      "type": "http",
      "config": {
        "url": "https://api.company.com/users",
        "headers": {
          "Accept": "application/json",
          "User-Agent": "OPA-Control-Plane/1.0"
        }
      },
      "credentials": "api-credentials",
      "transform_query": "{user.id: user | user := input.users[_]}"
    }
  ]
}
```

---

## API Features

### Pagination

- All list endpoints support pagination
- `limit`: Max 100 items per page (default 100\)
- `cursor`: Opaque cursor for next page
- `next_cursor` in response indicates more data available

### Pretty Printing

- Add `?pretty` or `?pretty=true` to format JSON responses
- Default is pretty-printed if `pretty` parameter is present without value

### URL Encoding

- All path parameters support URL encoding for special characters
- Names with spaces, slashes, etc. should be URL-encoded

### Content Type

- All API endpoints expect and return `application/json`
- Request bodies must be valid JSON for PUT/POST operations

---

## Common HTTP Status Codes

- **200**: Success
- **400**: Bad Request (invalid parameters)
- **401**: Unauthorized (missing/invalid API key)
- **403**: Forbidden (not authorized for resource)
- **404**: Not Found (resource doesn't exist)
- **500**: Internal Server Error

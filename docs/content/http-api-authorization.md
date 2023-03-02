---
title: "HTTP APIs"
kind: tutorial
weight: 1
---

Anything that exposes an HTTP API (whether an individual microservice or an application as a whole) needs to control who can run those APIs and when.  OPA makes it easy to write fine-grained, context-aware policies to implement API authorization.

## Goals

In this tutorial, you'll use a simple HTTP web server that accepts any HTTP GET
request that you issue and echoes the OPA decision back as text. OPA will fetch
policy bundles from a simple bundle server. Both OPA, the bundle server and the
web server will be run as containers.

For this tutorial, our desired policy is:

* People can see their own salaries (`GET /finance/salary/{user}` is permitted for `{user}`)
* A manager can see their direct reports' salaries (`GET /finance/salary/{user}` is permitted for `{user}`'s manager)

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run a demo web server along with OPA.

## Steps

### 1. Create a policy bundle.

Create a policy that allows users to request their own salary as well as the salary of their direct subordinates.

**First** create a directory named `bundles` and cd into it.
```sh
mkdir bundles
cd bundles
```

**example.rego**:

```live:example:module:openable
package httpapi.authz

# bob is alice's manager, and betty is charlie's.
subordinates := {"alice": [], "charlie": [], "bob": ["alice"], "betty": ["charlie"]}

default allow := false

# Allow users to get their own salaries.
allow {
    input.method == "GET"
    input.path == ["finance", "salary", input.user]
}

# Allow managers to get their subordinates' salaries.
allow {
    some username
    input.method == "GET"
    input.path = ["finance", "salary", username]
    subordinates[input.user][_] == username
}
```

**Then**, build a bundle.

```shell
opa build example.rego
cd ..
```

You should now see a policy bundle (`bundle.tar.gz`) in your working directory (`./bundles/bundle.tar.gz`).

### 2. Bootstrap the tutorial environment using Docker Compose.

Next, create a `docker-compose.yml` file that runs OPA, a bundle server and the demo web server.

**docker-compose.yml**:

```yaml
version: '2'
services:
  opa:
    image: openpolicyagent/opa:{{< current_docker_version >}}
    ports:
    - 8181:8181
    # WARNING: OPA is NOT running with an authorization policy configured. This
    # means that clients can read and write policies in OPA. If you are
    # deploying OPA in an insecure environment, be sure to configure
    # authentication and authorization on the daemon. See the Security page for
    # details: https://www.openpolicyagent.org/docs/security.html.
    command:
    - "run"
    - "--server"
    - "--log-format=json-pretty"
    - "--set=decision_logs.console=true"
    - "--set=services.nginx.url=http://bundle_server"
    - "--set=bundles.nginx.service=nginx"
    - "--set=bundles.nginx.resource=bundles/bundle.tar.gz"
    depends_on:
    - bundle_server
  api_server:
    image: openpolicyagent/demo-restful-api:0.3
    ports:
    - 5000:5000
    environment:
    - OPA_ADDR=http://opa:8181
    - POLICY_PATH=/v1/data/httpapi/authz
    depends_on:
    - opa
  bundle_server:
    image: nginx:1.20.0-alpine
    ports:
    - 8888:80
    volumes:
    - ./bundles:/usr/share/nginx/html/bundles
```

Then run `docker-compose` to pull and run the containers.

**NOTE:** if running "Docker Desktop" (Mac or Windows) you may instead use the `docker compose` command.

```shell
docker-compose -f docker-compose.yml up
```

Every time the demo web server receives an HTTP request, it
asks OPA to decide whether an HTTP API is authorized or not
using a single RESTful API call.  An example code is [here](https://github.com/open-policy-agent/contrib/blob/main/api_authz/docker/echo_server.py),
but the crux of the (Python) code is shown below.

```python

# Grab basic information. We assume user is passed on a form.
http_api_user = request.form['user']

# Get the path as a list (removing leading and trailing /)
# Example: "/finance/salary/" will become ["finance", "salary"]
http_api_path_list = request.path.strip("/").split("/")

input_dict = {  # create input to hand to OPA
    "input": {
        "user": http_api_user,
        "path": http_api_path_list, # Ex: ["finance", "salary", "alice"]
        "method": request.method  # HTTP verb, e.g. GET, POST, PUT, ...
    }
}
# ask OPA for a policy decision
# (in reality OPA URL would be constructed from environment)
rsp = requests.post("http://127.0.0.1:8181/v1/data/httpapi/authz", json=input_dict)
if rsp.json()["allow"]:
  # HTTP API allowed
else:
  # HTTP API denied

```

### 3. Check that `alice` can see her own salary.

The following command will succeed.

```shell
curl --user alice:password localhost:5000/finance/salary/alice
```

The webserver queries OPA to authorize the request. In the query, the webserver
includes JSON data describing the incoming request.

```live:example:input
{
  "method": "GET",
  "path": ["finance", "salary", "alice"],
  "user": "alice"
}
```

When the webserver queries OPA it asks for a specific policy decision. In this
case, the integration is hardcoded to ask for `/v1/data/httpapi/authz`. OPA
translates this URL path into a query:

```live:example:query
data.httpapi.authz
```

The answer returned by OPA for the input above is:

```live:example:output
```

### 4. Check that `bob` can see `alice`'s salary (because `bob` is `alice`'s manager.)

```shell
curl --user bob:password localhost:5000/finance/salary/alice
```

### 5. Check that `bob` CANNOT see `charlie`'s salary.

`bob` is not `charlie`'s manager, so the following command will fail.

```shell
curl --user bob:password localhost:5000/finance/salary/charlie
```

### 6. Change the policy.

Suppose the organization now includes an HR department. The organization wants
members of HR to be able to see any salary. Let's extend the policy to handle
this.

**example-hr.rego**:

```live:hr_example:module:read_only,openable
package httpapi.authz

# Allow HR members to get anyone's salary.
allow {
    input.method == "GET"
    input.path = ["finance", "salary", _]
    input.user == hr[_]
}

# David is the only member of HR.
hr := [
    "david",
]
```

Build a new bundle with the new policy included.

```shell
opa build example.rego example-hr.rego
```

The updated bundle will automatically be served by the bundle server, but note that it  might take up to the
configured `max_delay_seconds` for the new bundle to be downloaded by OPA. If you plan to make frequent policy
changes you might want to adjust this value in `docker-compose.yml` accordingly.

For the sake of the tutorial we included `manager_of` and `hr` data directly
inside the policies. In real-world scenarios that information would be imported
from external data sources.

### 7. Check that the new policy works.
Check that `david` can see anyone's salary.

```shell
curl --user david:password localhost:5000/finance/salary/alice
curl --user david:password localhost:5000/finance/salary/bob
curl --user david:password localhost:5000/finance/salary/charlie
curl --user david:password localhost:5000/finance/salary/david
```

### 8. (Optional) Use JSON Web Tokens to communicate policy data.
OPA supports the parsing of JSON Web Tokens via the builtin function `io.jwt.decode`.
To get a sense of one way the subordinate and HR data might be communicated in the
real world, let's try a similar exercise utilizing the JWT utilities of OPA.

**example-jwt.rego**:

```live:jwt_example:module:openable
package httpapi.authz

default allow := false

# Allow users to get their own salaries.
allow {
    some username
    input.method == "GET"
    input.path = ["finance", "salary", username]
    token.payload.user == username
    user_owns_token
}

# Allow managers to get their subordinate' salaries.
allow {
    some username
    input.method == "GET"
    input.path = ["finance", "salary", username]
    token.payload.subordinates[_] == username
    user_owns_token
}

# Allow HR members to get anyone's salary.
allow {
    input.method == "GET"
    input.path = ["finance", "salary", _]
    token.payload.hr == true
    user_owns_token
}

# Ensure that the token was issued to the user supplying it.
user_owns_token { input.user == token.payload.azp }

# Helper to get the token payload.
token := {"payload": payload} {
    [header, payload, signature] := io.jwt.decode(input.token)
}
```

```live:jwt_example:input:hidden
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM",
  "method": "GET",
  "path": ["finance", "salary", "alice"],
  "user": "alice"
```

Build a new bundle for the new policy.

```shell
opa build example-jwt.rego
```

For convenience, we'll want to store user tokens in environment variables (they're really long).

```shell
export ALICE_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM"
export BOB_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYm9iIiwiYXpwIjoiYm9iIiwic3Vib3JkaW5hdGVzIjpbImFsaWNlIl0sImhyIjpmYWxzZX0.n_lXN4H8UXGA_fXTbgWRx8b40GXpAGQHWluiYVI9qf0"
export CHARLIE_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiY2hhcmxpZSIsImF6cCI6ImNoYXJsaWUiLCJzdWJvcmRpbmF0ZXMiOltdLCJociI6ZmFsc2V9.EZd_y_RHUnrCRMuauY7y5a1yiwdUHKRjm9xhVtjNALo"
export BETTY_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYmV0dHkiLCJhenAiOiJiZXR0eSIsInN1Ym9yZGluYXRlcyI6WyJjaGFybGllIl0sImhyIjpmYWxzZX0.TGCS6pTzjrs3nmALSOS7yiLO9Bh9fxzDXEDiq1LIYtE"
export DAVID_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiZGF2aWQiLCJhenAiOiJkYXZpZCIsInN1Ym9yZGluYXRlcyI6W10sImhyIjp0cnVlfQ.Q6EiWzU1wx1g6sdWQ1r4bxT1JgSHUpVXpINMqMaUDMU"
```

These tokens encode the same information as the policies we did before (`bob` is `alice`'s manager, `betty` is `charlie`'s, `david` is the only HR member, etc).
If you want to inspect their contents, start up the OPA REPL and execute `io.jwt.decode(<token here>, [header, payload, signature])` or open the example above in the Playground.

Let's try a few queries (note: you may need to escape the `?` characters in the queries for your shell):

Check that `charlie` can't see `bob`'s salary.

```shell
curl --user charlie:password "localhost:5000/finance/salary/bob?token=$CHARLIE_TOKEN"
```

Check that `charlie` can't pretend to be `bob` to see `alice`'s salary.

```shell
curl --user charlie:password "localhost:5000/finance/salary/alice?token=$BOB_TOKEN"
```

Check that `david` can see `betty`'s salary.

```shell
curl --user david:password "localhost:5000/finance/salary/betty?token=$DAVID_TOKEN"
```

Check that `bob` can see `alice`'s salary.

```shell
curl --user bob:password "localhost:5000/finance/salary/alice?token=$BOB_TOKEN"
```

Check that `alice` can see her own salary.

```shell
curl --user alice:password "localhost:5000/finance/salary/alice?token=$ALICE_TOKEN"
```

## Wrap Up

Congratulations for finishing the tutorial!

You learned a number of things about API authorization with OPA:

* OPA gives you fine-grained policy control over APIs once you set up the
  server to ask OPA for authorization.
* You write allow/deny policies to control which APIs can be executed by whom.
* You can import external data into OPA and write policies that depend on
  that data.
* You can use OPA data structures to define abstractions over your data.
* You can use a remote bundle server for distributing policy and data.

The code for this tutorial can be found in the
[open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.

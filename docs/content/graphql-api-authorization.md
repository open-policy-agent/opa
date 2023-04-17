---
title: "GraphQL APIs"
kind: tutorial
weight: 1
---

GraphQL APIs have become a popular way to query a variety of datastores and microservices, and any application or service providing a GraphQL API generally needs to control which users can run queries, mutations, and so on.
OPA makes it easy to write fine-grained, context-aware policies to implement GraphQL query authorization.

## Goals

In this tutorial, you'll use a simple GraphQL server that accepts any GraphQL request that you issue, and echoes the OPA decision back as text.
OPA will fetch policy bundles from a simple bundle server.
OPA, the bundle server, and the GraphQL server will all be run as containers.

For this tutorial, our desired policy is:

* People can see their own salaries (`query user($id: <user>) { salary }` is permitted for `<user>`)
* A manager can see their direct reports' salaries (`query user($id: <user>) { salary }` is permitted for `<user>`'s manager)


{{< danger >}} GraphQL API Authorization with OPA is currently experimental and the following tutorial is intended for demonstration purposes only. {{< /danger >}}

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run a demo web server along with OPA.

## Steps

### 1. Define our GraphQL schema.

Most modern GraphQL frameworks encourage starting with a schema, so we'll follow suit, and begin by defining the schema for this example.

**schema.gql**:
```graphql
type Employee {
  id: String!
  salary: Int!
}

schema {
  query: Query
}

type Query {
  employeeByID(id: String!): Employee
}
```

Every GraphQL service has a `query` type, and may or may not have a `mutation` type.
These types are special because they define the entry points of *every* GraphQL query for the API covered by that schema.

For our example above, we've defined exactly one query entry point, the parameterized query `employeeByID(id: String!)`.

### 2. Create a policy bundle.

GraphQL APIs allow surprising flexibility in how queries can be constructed, which makes writing policies for them a bit more challenging than for a REST API, which usually has a more fixed structure.

To protect a particular endpoint or field, we need to see if they are referenced in the incoming GraphQL query.
By using `graphql.parse`, we can extract an [abstract syntax tree][wikipedia-ast] (AST) from the incoming query, and then walk down the tree to its leaves to see if our endpoint is the target of the query.

We can then use separate rules to enforce conditions around the `salary` field, and who is allowed to access it.

The policy below does all of the above in parts:
 - Obtains the query AST (and validates it against our schema with `graphql.parse`).
 - Recursive traversal with `walk()` to obtain chunks of the AST with queries of interest present.
   - Selection of nodes of interest by name and structure.
 - Salary field selected.
 - Every query of interest found has to pass one of the `allowed_query` rules, or the entire query is rejected.
   - Constant/variable cases for both employees and their managers.

   [wikipedia-ast]: https://en.wikipedia.org/wiki/Abstract_syntax_tree

**example.rego**:
```live:example:module:openable
package graphqlapi.authz

import future.keywords.in
import future.keywords.every

subordinates = {"alice": [], "charlie": [], "bob": ["alice"], "betty": ["charlie"]}

query_ast := graphql.parse(input.query, input.schema)[0] # If validation fails, the rules depending on this will be undefined.

default allow := false

allow {
    employeeByIDQueries != {}
    every query in employeeByIDQueries {
      allowed_query(query)
    }
}

# Allow users to see the salaries of their subordinates. (variable case)
allowed_query(q) {
    selected_salary(q)
    varname := variable_arg(q, "id")
    input.variables[varname] in subordinates[input.user] # Do value lookup from the 'variables' object.
}
# Allow users to see the salaries of their subordinates. (constant value case)
allowed_query(q) {
    selected_salary(q)
    username := constant_string_arg(q, "id")
    username in subordinates[input.user]
}

# Helper rules.

# Allow users to get their own salaries. (variable case)
allowed_query(q) {
    selected_salary(q)
    varname := variable_arg(q, "id")
    input.user == input.variables[varname] # Do value lookup from the 'variables' object.
}

# Allow users to get their own salaries. (constant value case)
allowed_query(q) {
    selected_salary(q)
    username := constant_string_arg(q, "id")
    input.user == username
}


# Helper functions.

# Build up an object with all queries of interest as values.
employeeByIDQueries[value] {
    some value
	walk(query_ast, [_, value])
    value.Name == "employeeByID"
    count(value.SelectionSet) > 0 # Ensure we latch onto an employeeByID query.
}

# Extract the string value of a constant value argument.
constant_string_arg(value, argname) := arg.Value.Raw {
    some arg in value.Arguments
    arg.Name == argname
    arg.Value.Kind == 3
}

# Extract the variable name for a variable argument.
variable_arg(value, argname) := arg.Value.Raw {
    some arg in value.Arguments
    arg.Name == argname
    arg.Value.Kind == 0
}

# Ensure we're dealing with a selection set that includes the "salary" field.
selected_salary(value) := value.SelectionSet[_].Name == "salary"

```

Then, build a bundle.

```shell
mkdir bundles
opa build example.rego
mv bundle.tar.gz ./bundles
```

You should now see a policy bundle (`bundle.tar.gz`) in your working directory.

### 3. Bootstrap the tutorial environment using Docker Compose.

Next, create a `docker-compose.yml` file that runs OPA, a bundle server and the demo GraphQL server.


**docker-compose.yml**:

```yaml
services:
  opa:
    image: openpolicyagent/opa:{{< current_docker_version >}}
    ports:
      - "8181:8181"
    command:
      - "run"
      - "--server"
      - "--log-format=json-pretty"
      - "--set=decision_logs.console=true"
      - "--set=services.nginx.url=http://bundle_server"
      - "--set=bundles.nginx.service=nginx"
      - "--set=bundles.nginx.resource=bundle.tar.gz"
      - "--set=bundles.nginx.polling.min_delay_seconds=10"
      - "--set=bundles.nginx.polling.max_delay_seconds=30"
    depends_on:
      - bundle_server
  api_server:
    image: openpolicyagent/demo-graphql-api:0.1
    ports:
      - "6000:5000"
    environment:
      - OPA_ADDR=http://opa:8181
      - POLICY_PATH=/v1/data/graphqlapi/authz
    depends_on:
      - opa
  bundle_server:
    image: nginx:1.20.0-alpine
    ports:
      - 8888:80
    volumes:
      - ./bundles/:/usr/share/nginx/html/
```

Then run `docker-compose` to pull and run the containers.

{{< info >}}
If running "Docker Desktop" (Mac or Windows) you may instead use the `docker compose` command.
{{< /info >}}

```shell
docker-compose -f docker-compose.yml up
```

Every time the demo GraphQL server receives an HTTP request, it asks OPA to decide whether an GraphQL query is authorized or not using a single RESTful API call.
An example codebase is [here][graphql-example-repo], but the crux of the (JavaScript, Apollo framework) code is shown below.

   [graphql-example-repo]: https://github.com/StyraInc/graphql-apollo-example

```javascript
  // we assume user is passed in as part of the request context.
  var user = req.user;

  // we feed in the query and schema strings, as well as the variables object.
  var input = {
    input: {
      schema: schema, // GraphQL schema text.
      query: query, // GraphQL query text.
      user: user,
      variables: variables // GraphQL variable bindings.
    }
  };

  await axios
    // ask OPA for a policy decision.
    // (in reality OPA URL would be constructed from environment)
    .post("http://127.0.0.1:8181/v1/data/graphqlapi/authz", input)
    .then(res => {
      // GraphQL query allowed.
    })
    .catch(error => {
      // GraphQL query denied.
    });
```


### 4. Check that `alice` can see her own salary.

We'll define a quick shell function to make the following examples cleaner on the command line:

```shell
gql-query() {
   curl --user "$1" -H "Content-Type: application/json" "$2" --data-ascii "$3"
}
```

The following command will succeed.

```shell
gql-query alice:password "localhost:6000/" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
```

The GraphQL server queries OPA to authorize the request.
In the query, the server includes JSON data describing the incoming request.

```live:example:input
{
  "schema": "type Employee {\n  id: ...",
  "query": "query { employeeByID(id: \"alice\") { salary }}",
  "user": "alice",
  "variables": {}
}
```

When the GraphQL server queries OPA it asks for a specific policy decision.
In this case, the integration is hardcoded to ask for `/v1/data/graphqlapi/authz`.
OPA translates this URL path into a query:

```live:example:query
data.graphqlapi.authz
```

The answer returned by OPA for the input above is:

```live:example:output
```

### 4. Check that `bob` can see `alice`'s salary (because `bob` is `alice`'s manager.)

```shell
gql-query bob:password "localhost:6000/" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
```

### 5. Check that `bob` CANNOT see `charlie`'s salary.

`bob` is not `charlie`'s manager, so the following command will fail.

```shell
gql-query bob:password "localhost:6000/" '{"query":"query { employeeByID(id: \"charlie\") { salary }}"}'
```

### 6. Change the policy.

Suppose the organization now includes an HR department.
The organization wants members of HR to be able to see any salary.
Let's extend the policy to handle this.

**example-hr.rego**:

```live:hr_example:module:read_only,openable
package graphqlapi.authz

# Allow HR members to get anyone's salary.
allowed_query(q) {
  selected_salary(q)
  input.user == hr[_]
}

# David is the only member of HR.
hr = [
  "david",
]
```

Build a new bundle with the new policy included.

```shell
opa build example.rego example-hr.rego
mv bundle.tar.gz ./bundles
```

The updated bundle will automatically be served by the bundle server, but note that it  might take up to the configured `max_delay_seconds` for the new bundle to be downloaded by OPA.
If you plan to make frequent policy changes you might want to adjust this value in `docker-compose.yml` accordingly.

For the sake of the tutorial we included `manager_of` and `hr` data directly inside the policies.
In real-world scenarios that information would be imported from external data sources.

### 7. Check that the new policy works.
Check that `david` can see anyone's salary.

```shell
gql-query david:password "localhost:6000/" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
gql-query david:password "localhost:6000/" '{"query":"query { employeeByID(id: \"bob\") { salary }}"}'
gql-query david:password "localhost:6000/" '{"query":"query { employeeByID(id: \"charlie\") { salary }}"}'
gql-query david:password "localhost:6000/" '{"query":"query { employeeByID(id: \"david\") { salary }}"}'
```

### 8. (Optional) Use JSON Web Tokens to communicate policy data.
OPA supports the parsing of JSON Web Tokens via the builtin function `io.jwt.decode`.
To get a sense of one way the subordinate and HR data might be communicated in the real world, let's try a similar exercise utilizing the JWT utilities of OPA.

**example-jwt.rego**:

```live:jwt_example:module:hidden
package graphqlapi.authz

import future.keywords.in
import future.keywords.every

query_ast := graphql.parse(input.query, input.schema)[0] # If validation fails, the rules depending on this will be undefined.

# Helper rules.

# Allow users to see the salaries of their subordinates. (variable case)
allowed_query(q) {
    selected_salary(q)
    varname := variable_arg(q, "id")
    input.variables[varname] in token.payload.subordinates # Do value lookup from the 'variables' object.
}
# Allow users to see the salaries of their subordinates. (constant value case)
allowed_query(q) {
    selected_salary(q)
    username := constant_string_arg(q, "id")
    username in token.payload.subordinates
}

# Allow users to get their own salaries. (variable case)
allowed_query(q) {
    selected_salary(q)
    varname := variable_arg(q, "id")
    token.payload.user == input.variables[varname] # Do value lookup from the 'variables' object.
}

# Allow users to get their own salaries. (constant value case)
allowed_query(q) {
    selected_salary(q)
    username := constant_string_arg(q, "id")
    token.payload.user == username
}

# Allow HR members to get anyone's salary.
allowed_query(q) {
    selected_salary(q)
    token.payload.hr == true
}

# Helper functions.

# Build up a set with all queries of interest as values.
employeeByIDQueries[value] {
    some value
	walk(query_ast, [_, value])
    value.Name == "employeeByID"
    count(value.SelectionSet) > 0 # Ensure we latch onto an employeeByID query.
}

# Extract the string value of a constant value argument.
constant_string_arg(value, argname) := arg.Value.Raw {
    some arg in value.Arguments
    arg.Name == argname
    arg.Value.Kind == 3
}

# Extract the variable name for a variable argument.
variable_arg(value, argname) := arg.Value.Raw {
    some arg in value.Arguments
    arg.Name == argname
    arg.Value.Kind == 0
}

# Ensure we're dealing with a selection set that includes the "salary" field.
selected_salary(value) := value.SelectionSet[_].Name == "salary"

```

```live:jwt_example/new_rules:module:openable

default allow := false

allow {
    employeeByIDQueries != {}
    user_owns_token  # Ensure we validate the JWT token.
    every query in employeeByIDQueries {
      allowed_query(query)
    }
}

# Helper rules ... (Same as example.rego)

# Helper functions ... (Same as example.rego)

# -------------------------------------------------------------
# JWT Token Support

# Ensure that the token was issued to the user supplying it.
user_owns_token { input.user == token.payload.azp }

# Helper to get the token payload.
token = {"payload": payload} {
  [_, payload, _] := io.jwt.decode(input.token)
}
```

```live:jwt_example:input:hidden
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM",
  "schema": "type Employee {\n  id: ...",
  "query": "query { employeeByID(id: \"alice\") { salary }}",
  "user": "alice",
  "variables": {}
}
```

Build a new bundle for the new policy.

```shell
opa build example-jwt.rego example-hr.rego
mv bundle.tar.gz ./bundles
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
gql-query charlie:password "localhost:5000/?token=$CHARLIE_TOKEN" '{"query":"query { employeeByID(id: \"bob\") { salary }}"}'
```

Check that `charlie` can't pretend to be `bob` to see `alice`'s salary.

```shell
gql-query charlie:password "localhost:5000/?token=$BOB_TOKEN" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
```

Check that `david` can see `betty`'s salary.

```shell
gql-query david:password "localhost:5000/?token=$DAVID_TOKEN" '{"query":"query { employeeByID(id: \"betty\") { salary }}"}'
```

Check that `bob` can see `alice`'s salary.

```shell
gql-query bob:password "localhost:5000/?token=$BOB_TOKEN" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
```

Check that `alice` can see her own salary.

```shell
gql-query alice:password "localhost:5000/?token=$ALICE_TOKEN" '{"query":"query { employeeByID(id: \"alice\") { salary }}"}'
```

## Wrap Up

Congratulations for finishing the tutorial!

You learned a number of things about API authorization with OPA:

* OPA gives you fine-grained policy control over GraphQL APIs once you set up the server to ask OPA for authorization.
* You write allow/deny policies to control which endpoints and fields can be accessed by whom.
* You can import external data into OPA and write policies that depend on that data.
* You can use OPA data structures to define abstractions over your data.
* You can use a remote bundle server for distributing policy and data.

The code for this tutorial can be found in the
[StyraInc/graphql-apollo-example][graphql-example-repo]
repository.

# Get Started

The Open Policy Agent (OPA) is an open source, general-purpose policy engine that enables unified, context-aware policy enforcement across the entire stack.

OPA provides a high-level **declarative language** for authoring policies and
simple APIs to answer policy queries. Using OPA, you can offload policy
decisions from your service such as:

* Should this API call be allowed? E.g., `true` or `false`.
* How much quota remains for this user? E.g., `1048`.
* Which hosts can this container be deployed on? E.g., `["host1", "host40", ..., "host329"]`.
* What updates must be applied to this resource? E.g., `{"labels": {"team": "products}}`.

This tutorial shows how to get started with OPA using an interactive shell or [REPL (read-eval-print loop)](https://en.wikipedia.org/wiki/Read–eval–print_loop).

## Goals

REPLs are great for learning new languages and running quick experiments. You
can use OPA's REPL to experiment with policies and prototype new ones.

To introduce the REPL, you will use dummy data and an example policy. In English, the policy can be stated as follows:

- Servers that open an unencrypted HTTP port must not be connected to a public network.

Inside the REPL, you will define rules that codify the policy stated above.

Once you finish this tutorial, you will be familiar with:

  * Running OPA as an interactive shell/REPL.
  * Writing ad-hoc queries in [Rego](/how-do-i-write-policies.md).

## Prerequisites

If this is your first time using OPA, download the latest executable for your system.

On macOS (64-bit):

```shell
curl -L -o opa https://github.com/open-policy-agent/opa/releases/download/v0.8.1/opa_darwin_amd64
```

On Linux (64-bit):

```shell
curl -L -o opa https://github.com/open-policy-agent/opa/releases/download/v0.8.1/opa_linux_amd64
```

> Windows users can obtain the OPA executable from [GitHub Releases](https://github.com/open-policy-agent/opa/releases). The steps below are the same for Windows users except the executable name will be different.

Set permissions on the OPA executable:

```shell
chmod 755 ./opa
```

## Steps

### 1. Make sure you can run the REPL on your machine.

```shell
./opa run
```

Without any data, you can experiment with simple expressions to get the hang of it:

```ruby
> true
true
> 3.14
3.14
> ["hello", "world"]
[
  "hello",
  "world"
]
```

You can also test simple boolean expressions:

```ruby
> true = false
false
> 3.14 > 3
true
> "hello" != "goodbye"
true
```

Most REPLs let you define variables that you can reference later on. OPA allows you to do something similar. For example, you can define a `pi` constant as follows:

```ruby
> pi = 3.14
```

Once "pi" is defined, you query for the value and write expressions in terms of it:

```ruby
> pi
3.14
> pi > 3
true
```

One thing to watch out for in the REPL is that `=` is used both for assigning variables values and for testing the value of variables. For example `p = q` sometimes assigns `p` the value of `q` and sometimes checks if the values of `p` and `q` are the same. The REPL decides between assignment and test based on whether `p` already has a value or not. If `p` has a value, `p = q` is a test (returning `true` or `false`), and if `p` has no value `p = q` is an assignment. To unset a value for a variable, use the `unset` command. (This ambiguity is only really an issue in the REPL. When writing policy, the duality of `=` is actually beneficial.)

```ruby
> pi = 3
false
> unset pi
> pi = 3
> pi
3
```

In addition to running queries, the REPL also lets you define rules:

```ruby
> items = ["pizza", "apples", "bread", "coffee"]
> users = {"bob": {"likes": [0, 2]}, "alice": {"likes": [1, 2, 3]}}
> likes[[name, item]] { index = users[name].likes[_]; item = items[index] }
```

The likes rule above defines a set of tuples specifying what each user likes.

```
> likes[["alice", item]] # what does alice like?
+----------+
|   item   |
+----------+
| "apples" |
| "bread"  |
| "coffee" |
+----------+
> likes[[name, "bread"]] # who likes bread?
+---------+
|  name   |
+---------+
| "bob"   |
| "alice" |
+---------+
```

When you enter expressions into the OPA REPL, you are effectively running *queries*. The REPL output shows the values of variables in the expression that make the query `true`. If there is no set of variables that would make the query `true`, the REPL prints `false`. If there are no variables in the query and the query evaluates successfully, then the REPL just prints `true`.

Quit out of the REPL by pressing Control-D or typing `exit`:

```ruby
> exit
Exiting
```

### 2. Create a data file and a policy module.

Let's define a bit of JSON data that will be used in the tutorial:

```shell
cat >data.json <<EOF
{
    "servers": [
        {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
        {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
        {"id": "s3", "name": "cache", "protocols": ["memcache"], "ports": ["p3"]},
        {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
    ],
    "networks": [
        {"id": "n1", "public": false},
        {"id": "n2", "public": false},
        {"id": "n3", "public": true}
    ],
    "ports": [
        {"id": "p1", "networks": ["n1"]},
        {"id": "p2", "networks": ["n3"]},
        {"id": "p3", "networks": ["n2"]}
    ]
}
EOF
```

Also, let's include a rule that defines a set of servers that are attached to public networks:

```shell
cat >example.rego <<EOF
package opa.example

import data.servers
import data.networks
import data.ports

public_servers[s] {
    s = servers[_]
    s.ports[_] = ports[i].id
    ports[i].networks[_] = networks[j].id
    networks[j].public = true
}
EOF
```

### 3. Run the REPL with the data file and policy module as input.

```shell
./opa run data.json example.rego
```

You can now run queries against the various documents:

```ruby
> data.servers[_].id
+--------------------+
| data.servers[_].id |
+--------------------+
| "s1"               |
| "s2"               |
| "s3"               |
| "s4"               |
+--------------------+
> data.opa.example.public_servers[x]
+-------------------------------------------------------------------------------+
|                                       x                                       |
+-------------------------------------------------------------------------------+
| {"id":"s1","name":"app","ports":["p1","p2","p3"],"protocols":["https","ssh"]} |
| {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]}             |
+-------------------------------------------------------------------------------+
```

One powerful thing about Rego and the REPL is that you can run queries using the same syntax that you would use to lookup values.

For example if `i` has value `0` then `data.servers[i]` returns the first value in the `data.servers` array:

```ruby
> i = 0
> data.servers[i]
{
  "id": "s1",
  "name": "app",
  "ports": [
      "p1",
      "p2",
      "p3"
  ],
  "protocols": [
      "https",
      "ssh"
  ]
}
```

That same expression `data.servers[i]` when `i` has no value defines a query that returns all the values of `i` and `data.servers[i]`:

```ruby
> unset i
> data.servers[i]
+---+-------------------------------------------------------------------------------+
| i |                                data.servers[i]                                |
+---+-------------------------------------------------------------------------------+
| 0 | {"id":"s1","name":"app","ports":["p1","p2","p3"],"protocols":["https","ssh"]} |
| 1 | {"id":"s2","name":"db","ports":["p3"],"protocols":["mysql"]}                  |
| 2 | {"id":"s3","name":"cache","ports":["p3"],"protocols":["memcache"]}            |
| 3 | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]}             |
+---+-------------------------------------------------------------------------------+
```

### 4. Import and export documents.

The REPL also understands the [Import and Package](/how-do-i-write-policies.md#modules) directives.

```ruby
> import data.servers
> servers[i].ports[_] = "p2"; servers[i].id = id
+---+------+
| i |  id  |
+---+------+
| 0 | "s1" |
| 3 | "s4" |
+---+------+
```

```ruby
> package opa.example
> public_servers[x].protocols[_] = "http"
+-------------------------------------------------------------------+
|                                 x                                 |
+-------------------------------------------------------------------+
| {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
+-------------------------------------------------------------------+
```

### 5. Define a rule to identify servers in violation of our security policy.

```ruby
> import data.servers
> violations[s] {
  s = servers[_]
  s.protocols[_] = "http"
  public_servers[s]
}

> violations[server]
+-------------------------------------------------------------------+
|                              server                               |
+-------------------------------------------------------------------+
| {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
+-------------------------------------------------------------------+
```

> The REPL accepts multi-line input and will change appearance when it detects multi-line input.

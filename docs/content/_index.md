---
title: "Introduction"
kind: documentation
weight: 1
---

The Open Policy Agent (OPA, pronounced "oh-pa") is an open source,
general-purpose policy engine that unifies policy enforcement across the stack.
OPA provides a high-level declarative language that lets you specify policy as
code and simple APIs to offload policy decision-making from your software. You
can use OPA to enforce policies in microservices, Kubernetes, CI/CD pipelines,
API gateways, and more.

OPA was originally created by [Styra](https://www.styra.com) and is proud to be
a graduated project in the [Cloud Native Computing Foundation
(CNCF)](https://www.cncf.io/) landscape. For details read the CNCF
[announcement](https://www.cncf.io/announcements/2021/02/04/cloud-native-computing-foundation-announces-open-policy-agent-graduation/).

Read this page to learn about the core concepts in OPA's policy language
([Rego](policy-language)) as well as how to download, run, and integrate OPA.

## Overview

OPA [decouples](philosophy#policy-decoupling) policy decision-making from policy
enforcement. When your software needs to make policy decisions it **queries**
OPA and supplies structured data (e.g., JSON) as input. OPA accepts arbitrary
structured data as input.

<!--- source: https://docs.google.com/drawings/d/10_iIoQO6VgORsMpXyQ--fu8F5xSe9E4vIz5qRAIon78/edit --->

{{< figure src="opa-service.svg" width="65" caption="Policy Decoupling" >}}

OPA generates policy decisions by evaluating the query input against
policies and data. OPA and Rego are domain-agnostic so you can describe almost
any kind of invariant in your policies. For example:

* Which users can access which resources.
* Which subnets egress traffic is allowed to.
* Which clusters a workload must be deployed to.
* Which registries binaries can be downloaded from.
* Which OS capabilities a container can execute with.
* Which times of day the system can be accessed at.

Policy decisions are not limited to simple yes/no or allow/deny answers. Like
query inputs, your policies can generate arbitrary structured data as output.

Let's look at an example.

## Example

Imagine you work for an organization with the following system:

<!--- source: https://docs.google.com/drawings/d/1nPZ08-G8zEZyTlVTmLGnlnS-CdBEltiV8kds0cwd_qQ/edit --->

{{< figure src="system.svg" width="65" caption="Example System" >}}

There are three kinds of components in the system:

* Servers expose zero or more protocols (e.g., `http`, `ssh`, etc.)
* Networks connect servers and can be public or private. Public networks are connected to the Internet.
* Ports attach servers to networks.

All of the servers, networks, and ports are provisioned by a script. The script
receives a JSON representation of the system as input:

```live:example:input
{
    "servers": [
        {"id": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
        {"id": "db", "protocols": ["mysql"], "ports": ["p3"]},
        {"id": "cache", "protocols": ["memcache"], "ports": ["p3"]},
        {"id": "ci", "protocols": ["http"], "ports": ["p1", "p2"]},
        {"id": "busybox", "protocols": ["telnet"], "ports": ["p1"]}
    ],
    "networks": [
        {"id": "net1", "public": false},
        {"id": "net2", "public": false},
        {"id": "net3", "public": true},
        {"id": "net4", "public": true}
    ],
    "ports": [
        {"id": "p1", "network": "net1"},
        {"id": "p2", "network": "net3"},
        {"id": "p3", "network": "net2"}
    ]
}
```

Earlier in the day your boss told you about a new security policy that has to be
implemented:

```
1. Servers reachable from the Internet must not expose the insecure 'http' protocol.
2. Servers are not allowed to expose the 'telnet' protocol.
```

The policy needs to be enforced when servers, networks, and ports are
provisioned and the compliance team wants to periodically audit the system to
find servers that violate the policy.

Your boss has asked you to determine if OPA would be a good fit for implementing
the policy.

## Rego

OPA policies are expressed in a high-level declarative language called Rego.
Rego (pronounced "ray-go") is purpose-built for expressing policies over complex
hierarchical data structures. For detailed information on Rego see the [Policy
Language](policy-language) documentation.

{{< info >}}
💡 The examples below are interactive! If you edit the input data above
containing servers, networks, and ports, the output will change below.
Similarly, if you edit the queries or rules in the examples below the output
will change. As you read through this section, try changing the input, queries,
and rules and observe the difference in output.

💻 They can also be run locally on your machine using the [`opa eval` command, here are setup instructions.](#running-opa)

Note that the examples in this section try to represent the best practices.
As such, they make use of keywords that are meant to become standard keywords
at some point in time, but have been introduced gradually.
[See the docs on _future keywords_](./policy-language/#future-keywords) for more information.
{{< /info >}}

### References

```live:example/refs:module:hidden
package example
import future.keywords
```

When OPA evaluates policies it binds data provided in the query to a global
variable called `input`. You can refer to data in the input using the `.` (dot)
operator.

```live:example/refs/1:query:merge_down
input.servers
```
```live:example/refs/1:output
```

To refer to array elements you can use the familiar square-bracket syntax:

```live:example/refs/array:query:merge_down
input.servers[0].protocols[0]
```
```live:example/refs/array:output
```

> 💡 You can use the same square bracket syntax if keys contain other than
> `[a-zA-Z0-9_]`. E.g., `input["foo~bar"]`.

If you refer to a value that does not exist, OPA returns _undefined_. Undefined
means that OPA was not able to find any results.

```live:example/refs/undefined:query:merge_down
input.deadbeef
```
```live:example/refs/undefined:output:expect_undefined
```

### Expressions (Logical AND)

```live:example/exprs:module:hidden
package example
import future.keywords
```

To produce policy decisions in Rego you write expressions against input and
other data.

```live:example/exprs/eq:query:merge_down
input.servers[0].id == "app"
```
```live:example/exprs/eq:output
```

OPA includes a set of built-in functions you can use to perform common
operations like string manipulation, regular expression matching, arithmetic,
aggregation, and more.

```live:example/exprs/builtins:query:merge_down
count(input.servers[0].protocols) >= 1
```
```live:example/exprs/builtins:output
```

For a complete list of built-in functions supported in OPA out-of-the-box see
the [Policy Reference](./policy-reference) page.

Multiple expressions are joined together with the `;` (AND) operator. For
queries to produce results, all of the expressions in the query must be true or
defined. The order of expressions does not matter.

```live:example/exprs/multi:query:merge_down
input.servers[0].id == "app"; input.servers[0].protocols[0] == "https"
```
```live:example/exprs/multi:output
```

You can omit the `;` (AND) operator by splitting expressions across multiple
lines. The following query has the same meaning as the previous one:

```live:example/exprs/multi_line:query:merge_down
input.servers[0].id == "app"
input.servers[0].protocols[0] == "https"
```
```live:example/exprs/multi_line:output
```

If any of the expressions in the query are not true (or defined) the result is
undefined. In the example below, the second expression is false:

```live:example/exprs/multi_undefined:query:merge_down
input.servers[0].id == "app"
input.servers[0].protocols[0] == "telnet"
```
```live:example/exprs/multi_undefined:output:expect_undefined
```

### Variables

```live:example/vars:module:hidden
package example
import future.keywords
```

You can store values in intermediate variables using the `:=` (assignment)
operator. Variables can be referenced just like `input`.

```live:example/vars/1:query:merge_down
s := input.servers[0]
s.id == "app"
p := s.protocols[0]
p == "https"
```
```live:example/vars/1:output
```

When OPA evaluates expressions, it finds values for the variables that make all
of the expressions true. If there are no variable assignments that make all of
the expressions true, the result is undefined.

```live:example/vars/undefined:query:merge_down
s := input.servers[0]
s.id == "app"
s.protocols[1] == "telnet"
```
```live:example/vars/undefined:output:expect_undefined
```

Variables are immutable. OPA reports an error if you try to assign the same
variable twice.

```live:example/vars/assign_error:query:merge_down
s := input.servers[0]
s := input.servers[1]
```
```live:example/vars/assign_error:output:expect_assigned_above
```

OPA must be able to enumerate the values for all variables in all expressions.
If OPA cannot enumerate the values of a variable in any expression, OPA will
report an error.

```live:example/vars/unsafe:query:merge_down
x := 1
x != y  # y has not been assigned a value
```
```live:example/vars/unsafe:output:expect_unsafe_var
```

### Iteration

```live:example/iter:module:hidden
package example
import future.keywords
```

Like other declarative languages (e.g., SQL), iteration in Rego happens
implicitly when you inject variables into expressions.

There are explicit iteration constructs to express _FOR ALL_ and _FOR SOME_, [see
below](#for-some-and-for-all).

To understand how iteration works in Rego, imagine you need to check if any
networks are public. Recall that the networks are supplied inside an array:

```live:example/iter/recall:query:merge_down
input.networks
```
```live:example/iter/recall:output
```

One option would be to test each network in the input:

```live:example/iter/check_public_0:query:merge_down
input.networks[0].public == true
```
```live:example/iter/check_public_0:output:merge_down
```
```live:example/iter/check_public_1:query:merge_down
input.networks[1].public == true
```
```live:example/iter/check_public_1:output:merge_down
```
```live:example/iter/check_public_2:query:merge_down
input.networks[2].public == true
```
```live:example/iter/check_public_2:output
```

This approach is problematic because there may be too many networks to list
statically, or more importantly, the number of networks may not be known in
advance.

In Rego, the solution is to substitute the array index with a variable.

```live:example/iter/1:query:merge_down
some i; input.networks[i].public == true
```
```live:example/iter/1:output
```

Now the query asks for values of `i` that make the overall expression true. When
you substitute variables in references, OPA automatically finds variable
assignments that satisfy all of the expressions in the query. Just like
intermediate variables, OPA returns the values of the variables.

You can substitute as many variables as you want. For example, to find out if
any servers expose the insecure `"http"` protocol you could write:

```live:example/iter/double:query:merge_down
some i, j; input.servers[i].protocols[j] == "http"
```
```live:example/iter/double:output
```

If variables appear multiple times the assignments satisfy all of the
expressions. For example, to find the ids of ports connected to public networks,
you could write:

```live:example/iter/join:query:merge_down
some i, j
id := input.ports[i].id
input.ports[i].network == input.networks[j].id
input.networks[j].public
```
```live:example/iter/join:output
```

Providing good names for variables can be hard. If you only refer to the
variable once, you can replace it with the special `_` (wildcard variable)
operator. Conceptually, each instance of `_` is a unique variable.

```live:example/iter/wildcard:query:merge_down
input.servers[_].protocols[_] == "http"
```
```live:example/iter/wildcard:output
```

Just like references that refer to non-existent fields or expressions that fail
to match, if OPA is unable to find any variable assignments that satisfy all of
the expressions, the result is undefined.

```live:example/iter/undefined:query:merge_down
some i; input.servers[i].protocols[i] == "ssh"  # there is no assignment of i that satisfies the expression
```
```live:example/iter/undefined:output:expect_undefined
```

#### FOR SOME and FOR ALL

While plain iteration serves as a powerful building block, Rego also features ways
to express _FOR SOME_ and _FOR ALL_ more explicitly.

{{< info >}}
To ensure backwards-compatibility, the keywords discussed below introduced slowly.
In the first stage, users can opt-in to using the new keywords via a special import:
`import future.keywords.every` introduces the `every` keyword described here.
(Importing `every` means also importing `in` without an extra `import` statement.)

At some point in the future, the keyword will become _standard_, and the import will
become a no-op that can safely be removed. This should give all users ample time to
update their policies, so that the new keyword will not cause clashes with existing
variable names.
[See the docs on _future keywords_](./policy-language/#future-keywords) for more information.
{{< /info >}}

##### FOR SOME (`some`)

`some ... in ...` is used to iterate over the collection (its last argument),
and will bind its variables (key, value position) to the collection items.
It introduces new bindings to the evaluation of the rest of the rule body.

Using `some`, we can express the rules introduced above in different ways:

```live:example/iter/some1:module:merge_down
public_network contains net.id if {
    some net in input.networks # some network exists and..
    net.public                 # it is public.
}

shell_accessible contains server.id if {
    some server in input.servers
    "telnet" in server.protocols
}

shell_accessible contains server.id if {
    some server in input.servers
    "ssh" in server.protocols
}
```
```live:example/iter/some1:query:merge_down
shell_accessible
```
```live:example/iter/some1:output
```

For details on `some ... in ...`, see [the documentation of the `in` operator](policy-language/#membership-and-iteration-in).

##### FOR ALL (`every`)

Expanding on the examples above, `every` allows us to succinctly express that
a condition holds for all elements of a domain.

```live:example/iter/every2:module:merge_down
no_telnet_exposed if {
    every server in input.servers {
        every protocol in server.protocols {
            "telnet" != protocol
        }
    }
}

no_telnet_exposed_alt if { # alternative: every + not-in
    every server in input.servers {
        not "telnet" in server.protocols
    }
}

no_telnet_exposed_alt2 if { # alternative: not + rule + some
    not any_telnet_exposed
}

any_telnet_exposed if {
    some server in input.servers
    "telnet" in server.protocols
}
```
```live:example/iter/every2:input:merge_down
{
    "servers": [
        {
            "id": "busybox",
            "protocols": ["http", "ftp"]
        },
        {
            "id": "db",
            "protocols": ["mysql", "ssh"]
        },
        {
            "id": "web",
            "protocols": ["https"]
        }
    ]
}
```
```live:example/iter/every2:query:merge_down
no_telnet_exposed
```
```live:example/iter/every2:output
```

For all the details, see [Every Keyword](policy-language/#every-keyword).

### Rules

Rego lets you encapsulate and re-use logic with rules. Rules are just if-then
logic statements. Rules can either be "complete" or "partial".

```live:example/complete:module:hidden
package example.rules
import future.keywords
```

#### Complete Rules

Complete rules are if-then statements that assign a single value to a variable.
For example:

```live:example/complete/1:module:openable
any_public_networks := true if {
    some net in input.networks # some network exists and..
    net.public                 # it is public.
}
```

Every rule consists of a _head_ and a _body_. In Rego we say the rule head
is true _if_ the rule body is true for some set of variable assignments. In
the example above `any_public_networks := true` is the head and `some net in
input.networks; net.public` is the body.

You can query for the value generated by rules just like any other value:

```live:example/complete/1:query
any_public_networks
```
```live:example/complete/1:output
```

All values generated by rules can be queried via the global `data` variable.

```live:example/complete/1/abs:query:merge_down
data.example.rules.any_public_networks
```
```live:example/complete/1/abs:output:merge_down
```

> 💡 You can query the value of any rule loaded into OPA by referring to it with an
> absolute path. The path of a rule is always:
> `data.<package-path>.<rule-name>`.

If you omit the `= <value>` part of the rule head the value defaults to `true`.
You could rewrite the example above as follows without changing the meaning:

```live:example/complete/elided:module:read_only,openable
any_public_networks if {
    some net in input.networks
    net.public
}
```

To define constants, omit the rule body. When you omit the rule body it defaults
to `true`. Since the rule body is true, the rule head is always true/defined.

```live:example/complete_constant:module:openable
package example.constants

pi := 3.14
```

Constants defined like this can be queried just like any other values:

```live:example/complete_constant:query:merge_down
pi > 3
```
```live:example/complete_constant:output
```

If OPA cannot find variable assignments that satisfy the rule body, we say that
the rule is undefined. For example, if the `input` provided to OPA does not
include a public network then `any_public_networks` will be undefined (which is
not the same as false.) Below, OPA is given a different set of input networks
(none of which are public):

```live:example/complete/1/undefined:input:merge_down
{
    "networks": [
        {"id": "n1", "public": false},
        {"id": "n2", "public": false}
    ]
}
```
```live:example/complete/1/undefined:query:merge_down
any_public_networks
```
```live:example/complete/1/undefined:output:expect_undefined
```

#### Partial Rules

```live:example/partial_set:module:hidden
package example
import future.keywords
```

Partial rules are if-then statements that generate a set of values and
assign that set to a variable. For example:

```live:example/partial_set/1:module:openable
public_network contains net.id if {
    some net in input.networks # some network exists and..
    net.public                 # it is public.
}
```

In the example above `public_network[net.id]` is the rule head and `net :=
input.networks[_]; net.public` is the rule body. You can query for the entire
set of values just like any other value:

```live:example/partial_set/1/extent:query:merge_down
public_network
```
```live:example/partial_set/1/extent:output
```

Iteration over the set of values can be done with the `some ... in ...` expression:

```live:example/partial_set/1/iteration_some:query:merge_down
some net in public_network
```
```live:example/partial_set/1/iteration_some:output
```

With a literal, or a bound variable, you can check if the value exists in the set
via `... in ...`:

```live:example/partial_set/1/exists_in:query:merge_down
"net3" in public_network
```
```live:example/partial_set/1/exists_in:output
```

You can also iterate over the set of values by referencing the set elements with a
variable:

```live:example/partial_set/1/iteration:query:merge_down
some n; public_network[n]
```
```live:example/partial_set/1/iteration:output
```

Lastly, you can check if a value exists in the set using the same syntax:

```live:example/partial_set/1/exists:query:merge_down
public_network["net3"]
```
```live:example/partial_set/1/exists:output
```

In addition to partially defining sets, You can also partially define key/value
pairs (aka objects). See
[Rules](https://www.openpolicyagent.org/docs/latest/policy-language/#rules) in
the language guide for more information.

#### Logical OR

When you join multiple expressions together in a query you are expressing
logical AND. To express logical OR in Rego you define multiple rules with the
same name. Let's look at an example.

Imagine you wanted to know if any servers expose protocols that give clients
shell access. To determine this you could define a complete rule that declares
`shell_accessible` to be `true` if any servers expose the `"telnet"` or `"ssh"`
protocols:

```live:example/logical_or/complete:module:openable,merge_down
package example.logical_or

default shell_accessible := false

shell_accessible := true {
    input.servers[_].protocols[_] == "telnet"
}

shell_accessible := true {
    input.servers[_].protocols[_] == "ssh"
}
```
```live:example/logical_or/complete:input:merge_down
{
    "servers": [
        {
            "id": "busybox",
            "protocols": ["http", "telnet"]
        },
        {
            "id": "web",
            "protocols": ["https"]
        }
    ]
}
```
```live:example/logical_or/complete:query:merge_down
shell_accessible
```
```live:example/logical_or/complete:output
```

> 💡 The `default` keyword tells OPA to assign a value to the variable if all of
> the other rules with the same name are undefined.

When you use logical OR with partial rules, each rule definition contributes
to the set of values assigned to the variable. For example, the example above
could be modified to generate a set of servers that expose `"telnet"` or
`"ssh"`.

```live:example/logical_or/partial_set:module:openable,merge_down
package example.logical_or

shell_accessible[server.id] {
    server := input.servers[_]
    server.protocols[_] == "telnet"
}

shell_accessible[server.id] {
    server := input.servers[_]
    server.protocols[_] == "ssh"
}
```
```live:example/logical_or/partial_set:input:merge_down
{
    "servers": [
        {
            "id": "busybox",
            "protocols": ["http", "telnet"]
        },
        {
            "id": "db",
            "protocols": ["mysql", "ssh"]
        },
        {
            "id": "web",
            "protocols": ["https"]
        }
    ]
}
```
```live:example/logical_or/partial_set:query:merge_down
shell_accessible
```
```live:example/logical_or/partial_set:output
```

<!---TBD: explain conflicts --->
### Putting It Together

The sections above explain the core concepts in Rego. To put it all together
let's review the desired policy (in English):

```
1. Servers reachable from the Internet must not expose the insecure 'http' protocol.
2. Servers are not allowed to expose the 'telnet' protocol.
```

At a high-level the policy needs to identify servers that violate some
conditions. To implement this policy we could define rules called `violation`
that generate a set of servers that are in violation.

For example:

```live:example/final:module:openable,merge_down
package example
import future.keywords.every # "every" implies "in"

allow := true {                                     # allow is true if...
    count(violation) == 0                           # there are zero violations.
}

violation[server.id] {                              # a server is in the violation set if...
    some server in public_servers                   # it exists in the 'public_servers' set and...
    "http" in server.protocols                      # it contains the insecure "http" protocol.
}

violation[server.id] {                              # a server is in the violation set if...
    some server in input.servers                    # it exists in the input.servers collection and...
    "telnet" in server.protocols                    # it contains the "telnet" protocol.
}

public_servers[server] {                            # a server exists in the public_servers set if...
    some server in input.servers                    # it exists in the input.servers collection and...

    some port in server.ports                       # it references a port in the input.ports collection and...
    some input_port in input.ports
    port == input_port.id

    some input_network in input.networks            # the port references a network in the input.networks collection and...
    input_port.network == input_network.id
    input_network.public                            # the network is public.
}
```
```live:example/final:query:merge_down
some x; violation[x]
```
```live:example/final:output
```

## Running OPA

This section explains how you can query OPA directly and interact with it on
your own machine.

### 1. Download OPA

To get started download an OPA binary for your platform from GitHub releases:

On macOS (64-bit):

```shell
curl -L -o opa https://openpolicyagent.org/downloads/{{< current_version >}}/opa_darwin_amd64
```

On Linux (64-bit):

```shell
curl -L -o opa https://openpolicyagent.org/downloads/{{< current_version >}}/opa_linux_amd64_static
```

{{< info >}}
Windows users can obtain the OPA executable from [here](https://openpolicyagent.org/downloads/{{< current_version >}}/opa_windows_amd64.exe).
The steps below are the same for Windows users except the executable name will be different.
{{< /info >}}

Set permissions on the OPA executable:

```shell
chmod 755 ./opa
```

{{< info >}}
You can also download and run OPA via Docker. The latest stable image tag is
`openpolicyagent/opa:latest`.
{{< /info >}}

#### Checksums

Checksums for all binaries are available in the download path by appending `.sha256` to the binary filename.
Verify the macOS binary checksum:

```shell
curl -L -o opa_darwin_amd64 https://openpolicyagent.org/downloads/{{< current_version >}}/opa_darwin_amd64
curl -L -o opa_darwin_amd64.sha256 https://openpolicyagent.org/downloads/{{< current_version >}}/opa_darwin_amd64.sha256
shasum -c opa_darwin_amd64.sha256
```

### 2. Try `opa eval`

The simplest way to interact with OPA is via the command-line using the [`opa eval` sub-command](cli/#opa-eval).
It is a swiss-army knife that you can use to evaluate arbitrary Rego expressions and policies.
`opa eval` supports a large number of options for controlling evaluation.
Commonly used flags include:

| Flag | Short | Description |
| --- | --- | --- |
| `--bundle` | `-b`| Load a [bundle file](management-bundles/#bundle-file-format) or directory into OPA. This flag can be repeated. |
| `--data` | `-d` | Load policy or data files into OPA. This flag can be repeated. |
| `--input` | `-i` | Load a data file and use it as `input`. This flag cannot be repeated. |
| `--format` | `-f` | Set the output format to use. The default is `json` and is intended for programmatic use. The `pretty` format emits more human-readable output. |
| `--fail` | n/a | Exit with a non-zero exit code if the query is undefined. |
| `--fail-defined` | n/a | Exit with a non-zero exit code if the query is not undefined. |

For example:

**input.json**:

```live:example/using_opa:input
{
    "servers": [
        {"id": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
        {"id": "db", "protocols": ["mysql"], "ports": ["p3"]},
        {"id": "cache", "protocols": ["memcache"], "ports": ["p3"]},
        {"id": "ci", "protocols": ["http"], "ports": ["p1", "p2"]},
        {"id": "busybox", "protocols": ["telnet"], "ports": ["p1"]}
    ],
    "networks": [
        {"id": "net1", "public": false},
        {"id": "net2", "public": false},
        {"id": "net3", "public": true},
        {"id": "net4", "public": true}
    ],
    "ports": [
        {"id": "p1", "network": "net1"},
        {"id": "p2", "network": "net3"},
        {"id": "p3", "network": "net2"}
    ]
}
```

**example.rego**:

```live:example/using_opa:module:openable,read_only
package example

default allow := false                              # unless otherwise defined, allow is false

allow := true {                                     # allow is true if...
    count(violation) == 0                           # there are zero violations.
}

violation[server.id] {                              # a server is in the violation set if...
    some server
    public_server[server]                           # it exists in the 'public_server' set and...
    server.protocols[_] == "http"                   # it contains the insecure "http" protocol.
}

violation[server.id] {                              # a server is in the violation set if...
    server := input.servers[_]                      # it exists in the input.servers collection and...
    server.protocols[_] == "telnet"                 # it contains the "telnet" protocol.
}

public_server[server] {                             # a server exists in the public_server set if...
    some i, j
    server := input.servers[_]                      # it exists in the input.servers collection and...
    server.ports[_] == input.ports[i].id            # it references a port in the input.ports collection and...
    input.ports[i].network == input.networks[j].id  # the port references a network in the input.networks collection and...
    input.networks[j].public                        # the network is public.
}
```

```bash
# Evaluate a trivial expression.
./opa eval "1*2+3"

# Evaluate a policy on the command line.
./opa eval -i input.json -d example.rego "data.example.violation[x]"

# Evaluate a policy on the command line and use the exit code.
./opa eval --fail-defined -i input.json -d example.rego "data.example.violation[x]"
echo $?
```

### 3. Try `opa run` (interactive)

OPA includes an interactive shell or REPL (Read-Eval-Print-Loop) accessible via
the [`opa run` sub-command](cli/#opa-run).
You can use the REPL to experiment with policies and prototype new ones.

To start the REPL just:

```bash
./opa run
```

When you enter statements in the REPL, OPA evaluates them and prints the result.


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

Most REPLs let you define variables that you can reference later on. OPA allows
you to do something similar. For example, you can define a `pi` constant as
follows:

```ruby
> pi := 3.14
```

Once "pi" is defined, you query for the value and write expressions in terms of
it:

```ruby
> pi
3.14
> pi > 3
true
```

Quit out of the REPL by pressing Control-D or typing `exit`:

```ruby
> exit
```

You can load policy and data files into the REPL by passing them on the command
line. By default, JSON and YAML files are rooted under `data`.

```
opa run input.json
```

Run a few queries to poke around the data:

```ruby
> data.servers[0].protocols[1]
```
```ruby
> data.servers[i].protocols[j]
```
```ruby
> net := data.networks[_]; net.public
```

To set a data file as the `input` document in the REPL prefix the file path:

```bash
opa run example.rego repl.input:input.json
```

```ruby
> data.example.public_server[s]
```

{{< info >}}
Prefixing file paths with a reference controls where file is loaded under
`data`. By convention, the REPL sets the `input` document that queries see by
reading `data.repl.input` each time a statement is evaluated. See `help input`
for details in the REPL.
{{< /info >}}

Quit out of the REPL by pressing Control-D or typing `exit`:

```ruby
> exit
```

### 4. Try `opa run` (server)

To integrate with OPA you can run it as a server and execute queries over HTTP.
You can start OPA as a server with `-s` or `--server`:

```bash
./opa run --server ./example.rego
```

By default OPA listens for HTTP connections on `0.0.0.0:8181`. See `opa run
--help` for a list of options to change the listening address, enable TLS, and
more.

Inside of another terminal use `curl` (or a similar tool) to access OPA's HTTP
API. When you query the `/v1/data` HTTP API you must wrap input data inside of a
JSON object:

```json
{
    "input": <value>
}
```

Create a copy the input file for sending via `curl`:

```
cat <<EOF > v1-data-input.json
{
    "input": $(cat input.json)
}
EOF
```

Execute a few `curl` requests and inspect the output:

```bash
curl localhost:8181/v1/data/example/violation -d @v1-data-input.json -H 'Content-Type: application/json'
curl localhost:8181/v1/data/example/allow -d @v1-data-input.json -H 'Content-Type: application/json'
```

By default `data.system.main` is used to serve policy queries without a path.
When you execute queries without providing a path, you do not have to wrap the
input. If the `data.system.main` decision is undefined it is treated as an
error:

```bash
curl localhost:8181 -i -d @input.json -H 'Content-Type: application/json'
```

You can restart OPA and configure to use any decision as the default decision:

```
./opa run --server --set=default_decision=example/allow ./example.rego
```

Re-run the last `curl` command from above:

```bash
curl localhost:8181 -i -d @input.json -H 'Content-Type: application/json'
```

### 5. Try OPA as a Go library

OPA can be embedded inside Go programs as a library. The simplest way to embed
OPA as a library is to import the `github.com/open-policy-agent/opa/rego`
package.

```golang
import "github.com/open-policy-agent/opa/rego"
```

Call the `rego.New` function to create an object that can be prepared or
evaluated:

```golang
r := rego.New(
    rego.Query("x = data.example.allow"),
    rego.Load([]string{"./example.rego"}, nil))
```

The `rego.Rego` supports several options that let you customize evaluation. See
the [GoDoc](https://godoc.org/github.com/open-policy-agent/opa/rego) page for
details. After constructing a new `rego.Rego` object you can call
`PrepareForEval()` to obtain an executable query. If `PrepareForEval()` fails it
indicates one of the options passed to the `rego.New()` call was invalid (e.g.,
parse error, compile error, etc.)

```golang
ctx := context.Background()
query, err := r.PrepareForEval(ctx)
if err != nil {
    // handle error
}
```

The prepared query object can be cached in-memory, shared across multiple
goroutines, and invoked repeatedly with different inputs. Call `Eval()` to
execute the prepared query.

```golang
bs, err := ioutil.ReadFile("./input.json")
if err != nil {
    // handle error
}

var input interface{}

if err := json.Unmarshal(bs, &input); err != nil {
    // handle error
}

rs, err := query.Eval(ctx, rego.EvalInput(input))
if err != nil {
    // handle error
}
```

The policy decision is contained in the results returned by the `Eval()` call.
You can inspect the decision and handle it accordingly:

```golang
// In this example we expect a single result (stored in the variable 'x').
fmt.Println("Result:", rs[0].Bindings["x"])
```

You can combine the steps above into a simple command-line program that
evaluates policies and outputs the result:

**main.go**:

```golang
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/open-policy-agent/opa/rego"
)

func main() {

	ctx := context.Background()

	// Construct a Rego object that can be prepared or evaluated.
	r := rego.New(
		rego.Query(os.Args[2]),
		rego.Load([]string{os.Args[1]}, nil))

	// Create a prepared query that can be evaluated.
	query, err := r.PrepareForEval(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Load the input document from stdin.
	var input interface{}
	dec := json.NewDecoder(os.Stdin)
	dec.UseNumber()
	if err := dec.Decode(&input); err != nil {
		log.Fatal(err)
	}

	// Execute the prepared query.
	rs, err := query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		log.Fatal(err)
	}

    // Do something with the result.
	fmt.Println(rs)
}
```

Run the code above as follows:

```bash
go run main.go example.rego 'data.example.violation' < input.json
```

## Next Steps

Congratulations on making it through the introduction to OPA. If you made it
this far you have learned the core concepts behind OPA's policy language as well
as how to get OPA and run it on your own.

If you have more questions about how to write policies in Rego check out:

* The [Policy Reference](policy-reference) page for reference documentation on built-in functions.
* The [Policy Language](policy-language) page for complete descriptions of all language features.

If you want to try OPA for a specific use case check out:

* The [Ecosystem](ecosystem) page which showcases various of OPA integrations.

Some popular tutorials include:

* The [Kubernetes](kubernetes-introduction) page for how to use OPA as an admission controller in Kubernetes.
* The [Envoy](envoy-introduction) page for how to use OPA as an external authorizer with Envoy.
* The [Terraform](terraform) page for how to use OPA to validate Terraform plans.

Don't forget to install the OPA (Rego) Plugin for your favorite [IDE or Text Editor](editor-and-ide-support)

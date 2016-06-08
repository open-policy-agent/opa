---
layout: docs
title: REPL
section: examples
sort_order: 1001
---

REPL
----

This examples shows how to run OPA as an interactive shell or [REPL (read-eval-print loop)](https://en.wikipedia.org/wiki/Read–eval–print_loop).

REPLs are great for learning new languages and running quick experiments. You can take advantage of OPA's REPL to quickly
run ad-hoc queries or prototype policies.

In this example, we will use the REPL to define rules to identify servers that violate a security policy: servers exposing (unencrypted) HTTP must not be connected to a public network.

Once you finish this example, you will be familiar with:

- Running OPA as an interactive shell/REPL.
- Writing ad-hoc queries in [Rego](/docs/lang.html).

Prerequisites
=============

- This example requires that you have the latest version of OPA. You can download the latest version of OPA at [openpolicyagent.org](/index.html)

- This example assumes you have the OPA executable in your $PATH.

Steps
=====

1. First, make sure you can run the REPL on your machine:

        opa run

    Without any data, you can experiment with simple boolean expressions to get the hang of it:

        > a = 1, b = 2, a != b
        +---+---+
        | A | B |
        +---+---+
        | 1 | 2 |
        +---+---+
        > a = 1, b = 2, a = b
        false
        > 10 > 9
        true
        > a = [1,2,3,4], a[i] > 2
        +-----------+---+
        |     A     | I |
        +-----------+---+
        | [1,2,3,4] | 2 |
        | [1,2,3,4] | 3 |
        +-----------+---+

    When you enter expressions into the REPL, you are effectively running *queries* against OPA. The REPL output shows the values of variables in the expression that make the query **true**. If there is not set of variables that would make the query true, the REPL prints **false**. If there are no variables in the query and the query evaluates successfully, then the REPL just prints **true**.

1. In addition to running queries, the REPL also lets you define rules:

        > p[x] :- a = [1,2,3,4], a[x] = _
        defined
        > p[x]
        +---+
        | X |
        +---+
        | 0 |
        | 1 |
        | 2 |
        | 3 |
        +---+

1. Quit out of the REPL by pressing Control-C or typing "exit":

        > exit
        Exiting

1. The REPL can also take policies and data as input. Let's define a bit of JSON data that will be used in the example:

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

    Also, let's include a rule that defines a set of servers that are attached to public networks:

        cat >example.rego <<EOF
        package opa.example

        import data.servers
        import data.networks
        import data.ports

        public_servers[s] :-
            s = servers[_],
            s.ports[_] = ports[i].id,
            ports[i].networks[_] = networks[j].id,
            networks[j].public = true
        EOF

1. Run the REPL with the two files as input:

        opa run data.json example.rego

    You can now run queries against the various documents:

        > data.servers[_].id = id
        +------+
        |  ID  |
        +------+
        | "s1" |
        | "s2" |
        | "s3" |
        | "s4" |
        +------+
        > data.opa.example.public_servers[x]
        +-------------------------------------------------------------------------------+
        |                                       X                                       |
        +-------------------------------------------------------------------------------+
        | {"id":"s1","name":"app","ports":["p1","p2","p3"],"protocols":["https","ssh"]} |
        | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]}             |
        +-------------------------------------------------------------------------------+

1. The REPL also understands the [Import and Package](/docs/lang.html#modules) directives.

        > import data.servers
        > servers[i].ports[_] = "p2", servers[i].id = id
        +---+------+
        | I |  ID  |
        +---+------+
        | 3 | "s4" |
        | 0 | "s1" |
        +---+------+
        > package opa.example
        > public_servers[x], x.protocols[_] = "http"
        +-------------------------------------------------------------------+
        |                                 X                                 |
        +-------------------------------------------------------------------+
        | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
        +-------------------------------------------------------------------+

1. Finally, we can define a rule to identify servers in violation of our security policy:

        > violations[s] :-
          s = servers[_],
          s.protocols[_] = "http",
          public_servers[s]
        > violations[server]
        +-------------------------------------------------------------------+
        |                              SERVER                               |
        +-------------------------------------------------------------------+
        | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
        +-------------------------------------------------------------------+

    > The REPL will accept multi-line input. The REPL prompt will change to indicate that it's accepting multi-line input.
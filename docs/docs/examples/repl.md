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

    Without any data, you can experiment with simple expressions to get the hang of it:

        > true
        true
        > 3.14
        3.14
        > ["hello", "world"]
        [
          "hello",
          "world"
        ]

    You can also test simple boolean expressions:

        > true = false
        false
        > 3.14 > 3
        true
        > "hello" != "goodbye"
        true

    Most REPLs let you define variables that you can reference later on. OPA allows you to do something similiar. For example, we can define a "pi" constant as follows:

        > pi = 3.14

    Once "pi" is defined, you query for the value and write expressions in terms of it:

        > pi
        3.14
        > pi > 3
        true

    One thing to watch out for in the REPL is that = is used both for assigning variables values and for testing the value of variables. For example p = q sometimes assigns p the value of q and sometimes checks if the values of p and q are the same. The REPL decides between assignment and test based on whether p already has a value or not. If p has a value, p = q is a test (returning true or false), and if p has no value p = q is an assignment. To unset a value for a variable, use the 'unset' command. (This ambiguity is only really an issue in the REPL--when writing policy the duality of = is actually beneficial.)

        > pi = 3
        false
        > unset pi
        > pi = 3
        > pi
        3

    In addition to running queries, the REPL also lets you define rules:

        > p[x] :- a = [1,2,3,4], a[x]
        > p[x], x > 1
        +---+
        | x |
        +---+
        | 2 |
        | 3 |
        +---+

    The rule above defines a set of values that are the indices of elements in the array "a".

    When you enter expressions into the REPL, you are effectively running *queries* against OPA. The REPL output shows the values of variables in the expression that make the query **true**. If there is no set of variables that would make the query true, the REPL prints **false**. If there are no variables in the query and the query evaluates successfully, then the REPL just prints **true**.

    Quit out of the REPL by pressing Control-C or typing "exit":

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

1. The REPL also understands the [Import and Package](/docs/lang.html#modules) directives.

        > import data.servers
        > servers[i].ports[_] = "p2", servers[i].id = id
        +---+------+
        | i |  id  |
        +---+------+
        | 0 | "s1" |
        | 3 | "s4" |
        +---+------+
        > package opa.example
        > public_servers[x], x.protocols[_] = "http"
        +-------------------------------------------------------------------+
        |                                 x                                 |
        +-------------------------------------------------------------------+
        | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
        +-------------------------------------------------------------------+

1. Finally, we can define a rule to identify servers in violation of our security policy:

        > import data.servers
        > violations[s] :-
          s = servers[_],
          s.protocols[_] = "http",
          public_servers[s]
        > violations[server]
        +-------------------------------------------------------------------+
        |                              server                               |
        +-------------------------------------------------------------------+
        | {"id":"s4","name":"dev","ports":["p1","p2"],"protocols":["http"]} |
        +-------------------------------------------------------------------+

    > The REPL will accept multi-line input. The REPL prompt will change to indicate that it's accepting multi-line input.
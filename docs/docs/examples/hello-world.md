---
layout: docs
header_title: Examples
title: Hello World
section: examples
show_in_header: true
sort_order: 1000
---

Hello World
-----------

This example demonstrates how to run OPA and load policy definitions and data
via the REST APIs.

Once you finish this example you will be familiar with:

- Running OPA in server mode locally on your machine.
- Loading policy definitions and [Base Documents](/docs/arch.html#data-model) via the REST APIs.
- Querying Base and [Virtual Documents](/docs/arch.html#data-model) via the REST APIs.

In this example, we will write rules to identify servers that are in violation of
a security policy. The security policy states that servers connected to a public network
must not expose unencrypted HTTP endpoints.

Prerequisites
=============

The steps below assume you have the latest version of OPA on your machine. You can download the
latest executable by following the link for your platform at [openpolicyagent.org](http://openpolicyagent.org).

The steps below assume the OPA executable is in your PATH.

Steps
=====

1. Create a directory that OPA can store policy definitions in:

        mkdir -p policies

1. Run OPA in server mode and enable console logging:

        opa run -s --alsologtostderr 1 --v 2 --policy-dir policies

1. Open another shell and create a policy definition that defines two rules ("violations" and "public_servers"):

        cat > example.rego <<EOF
        package opa.example

        import data.servers
        import data.networks
        import data.ports

        violations[server] :-
            server = servers[_],
            server.protocols[_] = "http",
            public_servers[server]

        public_servers[server] :-
            server = servers[_],
            server.ports[_] = ports[i].id,
            ports[i].networks[_] = networks[j].id,
            networks[j].public = true
        EOF

1. Load the policy definition into OPA via the REST API.

        curl -X PUT --data-binary @example.rego \
            http://localhost:8181/v1/policies/example

1. Load "servers" data into OPA via the REST API:

        cat > servers.json <<EOF
        [
            {
                "op": "add", "path": "/",
                "value": []
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "s3", "name": "cache", "protocols": ["memcache"], "ports": ["p3"]}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
            }
        ]
        EOF

        curl -X PATCH -d @servers.json \
            http://localhost:8181/v1/data/servers \
            -H "Content-Type: application/json-patch+json"

1. Load "networks" data into OPA via the REST API:

        cat > networks.json <<EOF
        [
            {
                "op": "add", "path": "/",
                "value": []
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "n1", "public": false}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "n2", "public": false}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "n3", "public": true}
            }
        ]
        EOF

        curl -X PATCH -d @networks.json \
            http://localhost:8181/v1/data/networks \
            -H "Content-Type: application/json-patch+json"

1. Load the "ports" into OPA via the REST API:

        cat > ports.json <<EOF
        [
            {
                "op": "add", "path": "/",
                "value": []
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "p1", "networks": ["n1"]}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "p2", "networks": ["n3"]}
            },
            {
                "op": "add", "path": "-",
                "value": {"id": "p3", "networks": ["n2"]}
            }
        ]
        EOF

        curl -X PATCH -d @ports.json \
            http://localhost:8181/v1/data/ports \
            -H "Content-Type: application/json-patch+json"

1. Query the "public_servers" document. Because "s1" and "s4" are connected to network "n3" they will be contained in the response:

        curl http://localhost:8181/v1/data/opa/example/public_servers

1. Query the "violations" document. Because "s4" is the only *public server* exposing unencrypted HTTP, it will be the only server in the document:

        curl http://localhost:8181/v1/data/opa/example/violations

1. Update "s4" to reflect that it no longer exposes unencrypted HTTP:

        curl -X PATCH -d '[{"op": "remove", "path": "/protocols/0"}]' \
            http://localhost:8181/v1/data/servers/3 \
            -H "Content-Type: application/json-patch+json"

1. Query the "violations" document again. The document will be empty now.

        curl http://localhost:8181/v1/data/opa/example/violations
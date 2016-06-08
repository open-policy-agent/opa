---
layout: docs
title: Authorization
section: examples
sort_order: 1000
---

Authorization
-------------

This example helps you get started with OPA and shows how you can start policy enabling an existing
app or service. This example introduces core concepts in OPA including the language used to define policies.

> Policy enabling an application decouples the policy implementation from the business logic so that administrators can define policy without changing the application while still keeping up with the size, complexity, and dynamic nature of modern applications.

In this example, we will write policies to authorize access to Docker. There are many examples of policies around access control. For demonstration purposes, we want to prevent the following:

- Containers with insecure configurations.
- Users modifying the system without sufficient read+write access.

There are two key ideas that this example illustrates:

1. The policy definition is decoupled from the implementation of the app (in this case Docker). The administrator is empowered to define and manage policies without requiring changes to any of the apps.

1. Both the data relevant to policy and the policy definitions themselves can change rapidly.

Once you finish this example, you will be familiar with:

- Running OPA as a server/daemon.
- Loading policy definitions and data via the REST APIs.
- Querying data via the REST APIs.

You will also be introduced to the language used to write policies in OPA: [Rego](/docs/lang.html).

Prerequisites
=============

The example has been tested on the following platforms:

- Ubuntu 16.04 (64-bit)

If you are using a different distro, OS, or architecture the steps will be the same however there will be slight differences in the commands you need to run.

- This example requires Docker Engine 1.11 or newer to be installed on your machine.

- This examples requires that you have root or sudo access on your machine.

Steps
=====

1. Create a directory that OPA can store policy definitions in:

        mkdir -p policies

1. Download the latest version of OPA:

        curl -L https://github.com/open-policy-agent/opa/releases/download/v0.1.0-rc2/opa_linux_amd64 > opa
        chmod u+x opa

1. Run OPA in server mode with logging enabled:

        ./opa run -s --alsologtostderr 1 --v 2 --policy-dir policies

    OPA will run until it receives a signal to stop. Open another terminal to continue with the rest of the example.

1. Download the [open-policy-agent/docker-authz-plugin](https://github.com/open-policy-agent/docker-authz-plugin) executable from GitHub:

        curl -L https://github.com/open-policy-agent/docker-authz-plugin/releases/download/v0.1.0-rc1/docker-authz-plugin_linux_amd64 > docker-authz-plugin
        chmod u+x docker-authz-plugin

    The open-policy-agent/docker-authz-plugin repository hosts a small [Docker Authorization Plugin](https://docs.docker.com/engine/extend/plugins_authorization/).
    Docker's authorization plugin system allows an external process to receive all requests sent to the Docker daemon. The
    authorization plugin replies, instructing the Docker daemon to allow or reject the request.

1. Create an empty policy definition that will allow all requests:

        cat >example.rego <<EOF
        package opa.example
        allow_request = true :- true
        EOF

    This policy definition is about simple as it can be. The policy includes a single rule named allow_request that is defined
    to always be true. Once all of the components are running, we will come back and extend the policy.

1. Run the executable downloaded in the previous step and then open another terminal:

        sudo ./docker-authz-plugin

    > This step requires sudo access because the Docker plugin framework will attempt to update the Docker daemon configuration.
    If you run without sudo you may encounter a permission error.

1. <a name="step-6"></a>Update Docker's configuration to include the following command line argument:

        --authorization-plugin=docker-authz-plugin

    On Ubuntu 16.04 with systemd, this can be done as follows (requires root):

        sudo mkdir -p /etc/systemd/system/docker.service.d
        sudo tee -a /etc/systemd/system/docker.service.d/override.conf > /dev/null <<EOF
        [Service]
        ExecStart=
        ExecStart=/usr/bin/docker daemon -H fd:// --authorization-plugin=docker-authz-plugin
        EOF
        sudo systemctl daemon-reload
        sudo service docker restart

    If you are using a different Linux distribution or you are not running systemd the step will be slightly different.

1. Run a simple Docker command to make sure everything is still working:

        docker ps

    If everything is setup correctly, the command should exit successfully. You can expect to see log messages
    from OPA and the plugin.

1. Test the policy definition is working by modifying it to **deny** all requests:

        cat >example.rego <<EOF
        package opa.example
        allow_request = true :- false
        EOF

    In OPA, rules defines the content of documents, e.g., objects, arrays, strings, booleans, etc.

    In steps above, we created a rule named allow_request that defines a document that is the boolean value true.
    When all of the expressions on the body of the rule evaluate to true, we say the document is defined.
    If any of the expressions evaluate to false, we say the document is *undefined*.
    In this case, the document will always be undefined because the body of the rule is false.

        docker ps

    The output from the last command will be:

        Error response from daemon: authorization denied by plugin docker-authz-plugin: request rejected by administrative policy

    To learn more about how rules define the content of documents, see the [Data Model](/docs/arch.html#data-model) section in the architectural overview.

    With this policy in place, users will not be able to run any Docker commands. Go ahead and try other commands such as `docker run` or `docker pull`,
    they will all be rejected. Let's change the policy so that it's a bit more useful.

1. Update the policy to reject requests with the unconfined [seccomp](https://en.wikipedia.org/wiki/Seccomp) profile:

        cat >example.rego <<EOF
        package opa.example

        import request as req

        seccomp_unconfined :-
            # This expression asserts that the string on the right hand side exists
            # within the array SecurityOpt referenced on the left hand side.
            req.Body.HostConfig.SecurityOpt[_] = "seccomp:unconfined"

        allow_request = true :- not seccomp_unconfined
        EOF

    The plugin is watching the policy definition file for changes. Each time we change the file, the plugin reads the file and sends it to OPA. To manually send the policy to OPA, you can use the following API:

        curl -X PUT --data-binary @example.rego \
            http://localhost:8181/v1/policies/example_policy

    This API is idempotent so sending the policy multiple times is fine. Go ahead and try it yourself.

1. Test the policy is working by running a simple container:

        docker run hello-world

    Now try running the same container but disable seccomp (which should be prevented by the policy):

        docker run --security-opt seccomp=unconfined hello-world

    When Docker processes the run command it contacts the plugin to see if the request(s) should be allowed. The plugin takes the request and executes a query against OPA using the request as input data to the query. The same API call that the plugin makes can be executed using curl:

        curl -v -G http://localhost:8181/v1/data/opa/example/allow_request \
            --data-urlencode \
            'global=request:{"Body":{"HostConfig":{"SecurityOpt":["seccomp=unconfined"]}}}'

    Because the document generated by the allow_request rule is undefined in this case, OPA responds with a 404.

    You can re-run the same query with the default seccomp profile and see that it succeeds:

        curl -G http://localhost:8181/v1/data/opa/example/allow_request \
            --data-urlencode \
            'global=request:{"Body":{"HostConfig":{"SecurityOpt":["seccomp=default"]}}}'

    Congratulations! You have successfully prevented containers from running without seccomp!

    So far, the policy has been defined in terms of input data from the plugin. In many cases, it's necessary to write policies against multiple data sources.

    The rest of the example shows how you can grant fine grained access to specific clients. To do so, we will insert fake user data into OPA to simulate an authentication system.

1. Create a configuration file to include an HTTP header in all of the requests sent to the Docker daemon. This header will identify the user:

        mkdir -p ~/.docker

        # Backup your existing Docker configuration, just in case. You can replace the configuration
        # which we generate below after you are done the example.
        cp ~/.docker/config.json ~/.docker/config.json~

        cat >~/.docker/config.json <<EOF
        {
            "HttpHeaders": {
                "Authz-User": "bob"
            }
        }
        EOF

    Currently, Docker does not provide a way to authenticate clients. In Docker 1.12, clients can be authenticated using TLS and there are plans to include other means of authentication. For the purpose of this example, we assume that an authentication system is place.

1. Add user data directly into OPA:

        cat >users.json <<EOF
        [
            {
                "op": "add",
                "path": "/",
                "value": {
                    "alice": {
                        "readOnly": false
                    },
                    "bob": {
                        "readOnly": true
                    }
                }
            }
        ]
        EOF

        curl -X PATCH -d @users.json \
            http://localhost:8181/v1/data/users \
            -H "Content-Type: application/json"

    This data represents information about users that could either come from an external system or be included in policy definitions.

    To see that the user data has been added, we can query the Data API. This shows the properties associated with the user "alice":

        curl http://localhost:8181/v1/data/users/alice

1. Update the policy definition to include basic user access controls:

        cat >example.rego <<EOF
        package opa.example

        import request as req
        import data.users

        allow_request = true :- valid_user_role

        # valid_user_role defines a document that is the boolean value true if this is
        # a write request and the user is allowed to perform writes.
        valid_user_role :-
            user_id = req.Headers["Authz-User"],
            user = users[user_id],
            user.readOnly = false

        # valid_user_role is defined again here to handle read requests. When a rule
        # like this is defined multiple times, the rule definition must ensure that
        # only one instance evaluates successfully in a given query. If multiple
        # instances evaluated successfully, it indicates a conflict.
        valid_user_role :-
            user_id = req.Headers["Authz-User"],
            user = users[user_id],
            req.Method = "GET",
            user.readOnly = true
        EOF

    In the new policy, the valid_user_role rules reference the "users" document created in the previous step.

1. Attempt to run a container. Because the configured user is "bob", the request is rejected:

        docker run hello-world

1. Change the user to "alice" and re-run the container:

        cat > ~/.docker/config.json <<EOF
        {
            "HttpHeaders": {
                "Authz-User": "alice"
            }
        }
        EOF

        docker run hello-world

That's it!

Remember to undo the configuration changes from [Step 6](#step-6) and [Step 12](#step-12) after you are
finished with the example.
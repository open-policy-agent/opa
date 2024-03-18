---
title: Debugging OPA
kind: documentation
weight: 75
---

This section outlines the various tools and techniques that can be used to debug OPA, both as a component in a 
distributed system and as a policy engine evaluating Rego.

# Debugging Rego Policies

At its core, OPA is a policy engine evaluating policies written in Rego. There are a number of options available for
users to debug Rego policies.

## OPA REPL and Playground

Often it can take a few tries to get a Rego policy correct, the OPA REPL and Playground are great tools for
reducing the feedback loop when debugging policies.

The REPL can be run locally and loaded with the policy and data files you are working on:

```shell
opa run [policy-files] [data-files]
```

The [Rego Playground](http://play.openpolicyagent.org) is a web-based Rego development environment that can be
used to test policies with different inputs and data. If you are interested in asking for help in the
[OPA Slack](http://slack.openpolicyagent.org), the playground is a great way to share your policy and data with
others.

## Performance Profiling

Sometimes the issue isn't the correctness of the policy but rather the performance. The
[Policy Performance](../policy-performance) section of the documentation outlines various techniques for
profiling and optimizing Rego policies.

## Using the `print` Built-in Function

The `print` built-in function can be used to print values to stdout, this can be useful for checking values
during policy evaluation as well as seeing how many times a particular line of code is executed.

See the [print function documentation](../policy-reference/#debugging) for more details on how to use
the `print` built-in function in different contexts.

## Ecosystem Projects

{{< ecosystem_feature_embed key="debugging-rego" topic="Debugging Rego" >}}

# Debugging OPA Instances in Distributed Systems

Debugging problems in distributed systems poses a number of challenges. Since OPA is commonly deployed in a distributed
fashion, as part of a larger platform, it is helpful to understand the various tools and techniques available for
debugging OPA in these environments.

## OPA Logs

OPA logs are a great place to start when debugging issues. The logs can be used to understand what OPA is doing
at any given time. Common issues such as failing to load in policy or data bundles will be shown here.

You can also enable debug logging to get more detailed information about what OPA is doing with `--log-level debug`.
This is documented in the [CLI documentation](../cli/#options-10) for `opa run`.

### Decision Logging

When OPA responds to a query, it is making a decision based on the policy and data that it has loaded. With the default
logging configuration, these are not logged in detail to the OPA logs. However, it is possible to enable console decision
logging by setting the following in OPA's config file:

```yaml
decision_logs:
  console: true
```

It might be preferable to send these logs to an HTTP endpoint or other system, to learn more about decision logging,
take a look at the [Decision Logging documentation](../management-decision-logs).

## Metrics, Health and Status APIs

Like other cloud-native tools, OPA exposes `/metrics` and `/health` endpoints that can be used to understand the
state of an OPA instance at any given time.

* `/metrics` - exposes Prometheus metrics about the OPA instance's memory use, bundle loading and HTTP requests.
  Read more in the [Metrics documentation](../monitoring).
* `/health` - shows information about the instance's readiness to serve requests, there are options available to also
  show information about the loading of bundles and other plugins. Read more about the endpoint in the
  [Health API documentation](../rest-api/#health-api).
* `/status` - is a JSON formatted endpoint that shows both health and metrics information. Read more in
  [Status API documentation](../rest-api/#status-api).

## Manually Querying OPA

In distributed systems, it's common that an OPA instance is being invoked by another service, sometimes it can be helpful
to isolate the OPA instance and query it directly. This can be done using the [REST API](../rest-api).

For example, to get a snapshot of the data that OPA has loaded, you can use the following command:

```shell
curl --silent https://$OPA_HOSTNAME/v1/data
```

Or to manually evaluate a policy rule with some input:

```shell
curl -X POST https://$OPA_HOSTNAME/v0/data/example_package/example_rule -d '{"foo": "bar"}'
```

## Load a Production Bundle Locally

Sometimes there are too many moving parts in a distributed system to debug an issue effectively on a live system.
In these cases, it can be helpful to load a bundle into a local OPA instance and debug the issue there.

You can quickly start an OPA instance with a remote bundle using the following command:

```shell
opa run -s https://example.com/bundles/bundle.tar.gz
```

If you need to configure the OPA instance with other options, you can use a config file to
make more detailed configurations. Read more in the [Configuration documentation](../configuration) documentation.

---
title: Kafka
kind: tutorial
weight: 1
---

[Apache Kafka](https://kafka.apache.org/) is a high-performance distributed
streaming platform deployed by thousands of companies. In many deployments,
administrators require fine-grained access control over Kafka topics to
enforce important requirements around confidentiality and integrity.

## Goals

This tutorial shows how to enforce fine-grained access control over Kafka
topics. In this tutorial you will use OPA to define and enforce an
authorization policy stating:

* Consumers of topics containing Personally Identifiable Information (PII) must be on allow list.
* Producers to topics with _high fanout_ must be on allow list.

In addition, this tutorial shows how to break up a policy with small helper
rules to reuse logic and improve overall readability.

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run Kafka, ZooKeeper, and OPA.

Additionally, we'll use Nginx for serving policy and data bundles to OPA. This component is however easily replaceable
by any other bundle server [implementation](../management-bundles/#implementations).

## Steps

### 1. Bootstrap the tutorial environment using Docker Compose.

First, let's create some directories. We'll create one for our policy files, a second one for built bundles, and a third
one or the OPA authorizer plugin.

```bash
mkdir policies bundles plugin
```

Next, create an OPA policy that allows all requests. You will update this policy later in the tutorial.

**policies/tutorial.rego**:

```live:start:module:read_only
package kafka.authz

allow := true
```

With the policy in place, build a bundle from the contents of the `policies` directory and place it in the `bundles`
directory. The `bundles` directory will later be mounted into the Nginx container in order to distribute policy updates
to OPA.

```shell
opa build --bundle policies/ --output bundles/bundle.tar.gz
```

#### Kafka Authorizer JAR File

Next, download the latest version of the [Open Policy Agent plugin for Kafka authorization](https://github.com/StyraInc/opa-kafka-plugin)
plugin from the projects [release pages](https://github.com/StyraInc/opa-kafka-plugin/releases).

Store the plugin in the `plugin` directory (replace `${version}` with the version number of the plugin just downloaded):
```bash
mv opa-authorizer-${version}-all.jar plugin/
```

For more information on how to configure the OPA plugin for Kafka, see the plugin [repository](https://github.com/StyraInc/opa-kafka-plugin).

Next, create a `docker-compose.yaml` file that runs OPA, Nginx, ZooKeeper, and Kafka.

**docker-compose.yaml**:

```yaml
services:
  nginx:
    image: nginx:1.21.4
    volumes:
      - "./bundles:/usr/share/nginx/html"
    ports:
      - "80:80"
  opa:
    image: openpolicyagent/opa:{{< current_docker_version >}}-rootless
    ports:
      - "8181:8181"
    command:
      - "run"
      - "--server"
      - "--set=decision_logs.console=true"
      - "--set=services.authz.url=http://nginx"
      - "--set=bundles.authz.service=authz"
      - "--set=bundles.authz.resource=bundle.tar.gz"
    depends_on:
      - nginx
  zookeeper:
    image: confluentinc/cp-zookeeper:6.2.1
    ports:
      - "2181:2181"
    environment:
      - ALLOW_ANONYMOUS_LOGIN=yes
      - ZOOKEEPER_CLIENT_PORT=2181
  broker:
    image: confluentinc/cp-kafka:6.2.1
    ports:
      - "9093:9093"
    environment:
      # Set cache expiry to low value for development in order to see decisions
      KAFKA_OPA_AUTHORIZER_CACHE_EXPIRE_AFTER_SECONDS: 10
      KAFKA_OPA_AUTHORIZER_URL: http://opa:8181/v1/data/kafka/authz/allow
      KAFKA_AUTHORIZER_CLASS_NAME: org.openpolicyagent.kafka.OpaAuthorizer
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: "zookeeper:2181"
      KAFKA_ADVERTISED_LISTENERS: SSL://localhost:9093
      KAFKA_SECURITY_INTER_BROKER_PROTOCOL: SSL
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_SSL_KEYSTORE_FILENAME: server.keystore
      KAFKA_SSL_KEYSTORE_CREDENTIALS: credentials.txt
      KAFKA_SSL_KEY_CREDENTIALS: credentials.txt
      KAFKA_SSL_TRUSTSTORE_FILENAME: server.truststore
      KAFKA_SSL_TRUSTSTORE_CREDENTIALS: credentials.txt
      KAFKA_SSL_CLIENT_AUTH: required
      CLASSPATH: "/plugin/*"
    volumes:
      - "./plugin:/plugin"
      - "./cert/server:/etc/kafka/secrets"
    depends_on:
      - opa
      - zookeeper
```

#### Authentication

The Docker Compose file defined above requires **SSL client authentication**
for clients that connect to the broker. Enabling SSL client authentication
allows for service identities to be provided as input to your policy. The
example below shows the input structure.

```json
{
  "action": {
    "logIfAllowed": true,
    "logIfDenied": true,
    "operation": "WRITE",
    "resourcePattern": {
      "name": "credit-scores",
      "patternType": "LITERAL",
      "resourceType": "TOPIC",
      "unknown": false
    },
    "resourceReferenceCount": 1
  },
  "requestContext": {
    "clientAddress": "/172.22.0.1",
    "clientInformation": {
      "softwareName": "apache-kafka-java",
      "softwareVersion": "2.8.1"
    },
    "connectionId": "172.22.0.5:9093-172.22.0.1:62744-2",
    "header": {
      "headerVersion": 2,
      "name": {
        "clientId": "consumer-console-producer-63933-1",
        "correlationId": 10,
        "requestApiKey": 1,
        "requestApiVersion": 12
      }
    },
    "listenerName": "SSL",
    "principal": {
      "name": "CN=anon_producer,OU=Developers",
      "principalType": "User"
    },
    "securityProtocol": "SSL"
  }
}
```

The client identity is extracted from the SSL certificates that clients
present when they connect to the broker. The user identity information is
encoded in the `input.requestContext.principal.name` field.
This field can be used inside the policy.

A detailed rundown of generating SSL certificates and JKS files required
for SSL client authentication is outside the scope of this tutorial, but the plugin
repository provides an [example script](https://github.com/StyraInc/opa-kafka-plugin/tree/main/example/opa_tutorial/create_cert.sh)
that demonstrates the creation of client certificates for the four different
users used in this tutorial:

* `anon_producer`
* `anon_consumer`
* `pii_consumer`
* `fanout_producer`

Lets' download the script and run it:

```shell
curl -O https://raw.githubusercontent.com/StyraInc/opa-kafka-plugin/main/example/opa_tutorial/create_cert.sh
chmod +x create_cert.sh
./create_cert.sh
```

We should now find a new `cert` directory created by the script,
containing the server and client certificates we'll need for TLS
authentication.

Note: Do not rely on these SSL certificates in real-world scenarios.
They are only provided for convenience/test purposes.

If you'd rather set up these users by other means, like one
of the available SASL mechanisms Kafka provides, that should work
just as well. Just make sure to update the Docker compose file
accordingly.

Once you have created the files needed for authentication,
you may launch the containers for this tutorial.

```bash
docker-compose --project-name opa-kafka-tutorial up
```

Now that the tutorial environment is running, we can define an authorization policy using OPA and test it.


### 2. Define a policy to restrict consumer access to topics containing Personally Identifiable Information (PII).

Update the `policies/tutorial.rego` with the following content.

```live:example:module:openable
#-----------------------------------------------------------------------------
# High level policy for controlling access to Kafka.
#
# * Deny operations by default.
# * Allow operations if no explicit denial.
#
# The kafka-authorizer-opa plugin will query OPA for decisions at
# /kafka/authz/allow. If the policy decision is _true_ the request is allowed.
# If the policy decision is _false_ the request is denied.
#-----------------------------------------------------------------------------
package kafka.authz

import future.keywords.in

default allow := false

allow {
	not deny
}

deny {
	is_read_operation
	topic_contains_pii
	not consumer_is_allowlisted_for_pii
}

#-----------------------------------------------------------------------------
# Data structures for controlling access to topics. In real-world deployments,
# these data structures could be loaded into OPA as raw JSON data. The JSON
# data could be pulled from external sources like AD, Git, etc.
#-----------------------------------------------------------------------------

consumer_allowlist := {"pii": {"pii_consumer"}}

topic_metadata := {"credit-scores": {"tags": ["pii"]}}

#-----------------------------------
# Helpers for checking topic access.
#-----------------------------------

topic_contains_pii {
	"pii" in topic_metadata[topic_name].tags
}

consumer_is_allowlisted_for_pii {
	principal.name in consumer_allowlist.pii
}

#-----------------------------------------------------------------------------
# Helpers for processing Kafka operation input. This logic could be split out
# into a separate file and shared. For conciseness, we have kept it all in one
# place.
#-----------------------------------------------------------------------------

is_write_operation {
    input.action.operation == "WRITE"
}

is_read_operation {
	input.action.operation == "READ"
}

is_topic_resource {
	input.action.resourcePattern.resourceType == "TOPIC"
}

topic_name := input.action.resourcePattern.name {
	is_topic_resource
}

principal := {"fqn": parsed.CN, "name": cn_parts[0]} {
	parsed := parse_user(input.requestContext.principal.name)
	cn_parts := split(parsed.CN, ".")
}

# If client certificates aren't used for authentication
else := {"fqn": "", "name": input.requestContext.principal.name}

parse_user(user) := {key: value |
	parts := split(user, ",")
	[key, value] := split(parts[_], "=")
}
```

The Kafka authorization plugin is configured to query for the
`data.kafka.authz.allow` decision. If the response is `true` the operation is
allowed, otherwise the operation is denied. When the integration queries OPA it
supplies a JSON representation of the operation, resource, client, and principal.

```live:example:query:hidden
data.kafka.authz.allow
```

```live:example:input
{
  "action": {
    "logIfAllowed": true,
    "logIfDenied": true,
    "operation": "READ",
    "resourcePattern": {
      "name": "credit-scores",
      "patternType": "LITERAL",
      "resourceType": "TOPIC",
      "unknown": false
    },
    "resourceReferenceCount": 1
  },
  "requestContext": {
    "clientAddress": "/172.22.0.1",
    "clientInformation": {
      "softwareName": "apache-kafka-java",
      "softwareVersion": "2.8.1"
    },
    "connectionId": "172.22.0.5:9093-172.22.0.1:62744-2",
    "header": {
      "headerVersion": 2,
      "name": {
        "clientId": "consumer-console-producer-63933-1",
        "correlationId": 10,
        "requestApiKey": 1,
        "requestApiVersion": 12
      }
    },
    "listenerName": "SSL",
    "principal": {
      "name": "CN=pii_consumer,OU=developers",
      "principalType": "User"
    },
    "securityProtocol": "SSL"
  }
}
```

With the input value above, the answer is:

```live:example:output
```

The `./bundles` directory is mounted into the Docker container running Nginx.
When the bundle under this directory change, OPA is notified via the bundle API,
and the policies are automatically reloaded.

You can update the bundle at any time by rebuilding it.
```shell
opa build --bundle policies/ --output bundles/bundle.tar.gz
```

At this point, you can exercise the policy.

### 3. Exercise the policy that restricts consumer access to topics containing PII.

This step shows how you can grant fine-grained access to services using
Kafka. In this scenario, some services are allowed to read PII data while
others are not.

First, run `kafka-console-producer` to generate some data on the
`credit-scores` topic.

> This tutorial uses the `kafka-console-producer` and `kafka-console-consumer` scripts provided by Kafka to generate and display Kafka messages. These scripts read from STDIN and write to STDOUT and are frequently used to send and receive data via Kafka over the command line. If you are not familiar with these scripts you can learn more in Kafka's [Quick Start](https://kafka.apache.org/documentation/#quickstart) documentation.


```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    bash -c 'for i in {1..10}; do echo "{\"user\": \"bob\", \"score\": $i}"; done | kafka-console-producer --topic credit-scores --broker-list broker:9093 -producer.config /tmp/client/anon_producer.properties'
```

This command will send 10 messages to the `credit-scores` topic. Bob's credit
score seems to be improving.

Next, run `kafka-console-consumer` and try to read data off the topic. Use
the `pii_consumer` credentials to simulate a service that is allowed to read
PII data.

```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    kafka-console-consumer --bootstrap-server broker:9093 --topic credit-scores --from-beginning --consumer.config /tmp/client/pii_consumer.properties
```

This command will output the 10 messages sent to the topic in the first part
of this step. Once the 10 messages have been printed, exit out of the script
(^C).

Finally, run `kafka-console-consumer` again but this time try to use the
`anon_consumer` credentials. The `anon_consumer` credentials simulate a
service that has **not** been explicitly granted access to PII data.

```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    kafka-console-consumer --bootstrap-server broker:9093 --topic credit-scores --from-beginning --consumer.config /tmp/client/anon_consumer.properties
```

Because the `anon_consumer` is not allowed to read PII data, the request will
be denied and the consumer will output an error message.

```
Not authorized to read from topic credit-scores.
...
Processed a total of 0 messages
```

### 4. Extend the policy to prevent services from accidentally writing to topics with large fanout.

First, add the following content to the policy file (`./policies/tutorial.rego`):

```live:example/deny:module:openable
deny {
    is_write_operation
    topic_has_large_fanout
    not producer_is_allowlisted_for_large_fanout
}

producer_allowlist := {
    "large-fanout": {
        "fanout_producer",
    }
}

topic_has_large_fanout {
    topic_metadata[topic_name].tags[_] == "large-fanout"
}

producer_is_allowlisted_for_large_fanout {
    producer_allowlist["large-fanout"][_] == principal.name
}
```

Next, update the `topic_metadata` data structure in the same file to indicate
that the `click-stream` topic has a high fanout.

```live:updated_metadata:module:read_only
topic_metadata := {
    "click-stream": {
        "tags": ["large-fanout"],
    },
    "credit-scores": {
        "tags": ["pii"],
    }
}
```

Last, build a bundle from the updated policy.
```shell
opa build --bundle policies/ --output bundles/bundle.tar.gz
```

### 5. Exercise the policy that restricts producer access to topics with high fanout.

First, run `kafka-console-producer` and simulate a service with access to the
`click-stream` topic.

```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    bash -c 'for i in {1..10}; do echo "{\"user\": \"alice\", \"button\": $i}"; done | kafka-console-producer --topic click-stream --broker-list broker:9093 -producer.config /tmp/client/fanout_producer.properties'
```

Next, run the `kafka-console-consumer` to confirm that the messages were published.

```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    kafka-console-consumer --bootstrap-server broker:9093 --topic click-stream --from-beginning --consumer.config /tmp/client/anon_consumer.properties
```

Once you see the 10 messages produced by the first part of this step, exit the console consumer (^C).

Lastly, run `kafka-console-producer` to simulate a service that should **not**
have access to _high fanout_ topics.

```bash
docker run -v $(pwd)/cert/client:/tmp/client --rm --network opa-kafka-tutorial_default \
    confluentinc/cp-kafka:6.2.1 \
    bash -c 'echo "{\"user\": \"alice\", \"button\": \"bogus\"}" | kafka-console-producer --topic click-stream --broker-list broker:9093 -producer.config /tmp/client/anon_producer.properties'
```

Because `anon_producer` is not authorized to write to high fanout topics, the
request will be denied and the producer will output an error message.

```
Not authorized to access topics: [click-stream]
```

## Wrap Up

Congratulations on finishing the tutorial!

At this point you have learned how to enforce fine-grained access control
over Kafka topics. In addition, you have seen how to break down policies into
smaller rules that can be reused and improve the overall readability over the
policy.

If you want to use the Kafka Authorizer plugin that integrates Kafka with
OPA, see the build and install instructions in the
[opa-kafka-plugin](https://github.com/StyraInc/opa-kafka-plugin)
repository.

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

* Consumers of topics containing Personally Identifiable Information (PII) must be whitelisted.
* Producers to topics with _high fanout_ must be whitelisted.

In addition, this tutorial shows how to break up a policy with small helper
rules to reuse logic and improve overall readability.

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run Kafka, ZooKeeper, and OPA.

## Steps

### 1. Bootstrap the tutorial environment using Docker Compose.

First, create an OPA policy that allows all requests. You will update this policy later in the tutorial.

```bash
mkdir -p policies
```

**policies/tutorial.rego**:

```live:start:module:read_only
package kafka.authz

allow = true
```

Next, create a `docker-compose.yaml` file that runs OPA, ZooKeeper, and Kafka.

**docker-compose.yaml**:

```yaml
version: "2"
services:
  opa:
    hostname: opa
    image: openpolicyagent/opa:{{< current_docker_version >}}
    ports:
      - 8181:8181
    # WARNING: OPA is NOT running with an authorization policy configured. This
    # means that clients can read and write policies in OPA. If you are deploying
    # OPA in an insecure environment, you should configure authentication and
    # authorization on the daemon. See the Security page for details:
    # https://www.openpolicyagent.org/docs/security.html.
    command: "run --server --watch /policies"
    volumes:
      - ./policies:/policies
  zookeeper:
    image: confluentinc/cp-zookeeper:4.0.0-3
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      zk_id: "1"
  kafka:
    hostname: kafka
    image: openpolicyagent/demo-kafka:1.0
    links:
      - zookeeper
      - opa
    ports:
      - "9092:9092"
    environment:
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: "1"
      KAFKA_ZOOKEEPER_CONNECT: "zookeeper:2181"
      KAFKA_ADVERTISED_LISTENERS: "SSL://:9093"
      KAFKA_SECURITY_INTER_BROKER_PROTOCOL: SSL
      KAFKA_SSL_CLIENT_AUTH: required
      KAFKA_SSL_KEYSTORE_FILENAME: kafka.broker.keystore.jks
      KAFKA_SSL_KEYSTORE_CREDENTIALS: broker_keystore_creds
      KAFKA_SSL_KEY_CREDENTIALS: broker_sslkey_creds
      KAFKA_SSL_TRUSTSTORE_FILENAME: kafka.broker.truststore.jks
      KAFKA_SSL_TRUSTSTORE_CREDENTIALS: broker_truststore_creds
      KAFKA_AUTHORIZER_CLASS_NAME: com.lbg.kafka.opa.OpaAuthorizer
      KAFKA_OPA_AUTHORIZER_URL: "http://opa:8181/v1/data/kafka/authz/allow"
      KAFKA_OPA_AUTHORIZER_ALLOW_ON_ERROR: "false"
      KAFKA_OPA_AUTHORIZER_CACHE_INITIAL_CAPACITY: 100
      KAFKA_OPA_AUTHORIZER_CACHE_MAXIMUM_SIZE: 100
      KAFKA_OPA_AUTHORIZER_CACHE_EXPIRE_AFTER_MS: 600000
```

For more information on how to configure the OPA plugin for Kafka, see the [github.com/open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.

Once you have created the file, launch the containers for this tutorial.

```bash
docker-compose --project-name opa-kafka-tutorial up
```

Now that the tutorial environment is running, we can define an authorization policy using OPA and test it.

#### Authentication

The Docker Compose file defined above requires **SSL client authentication**
for clients that connect to the broker. Enabling SSL client authentication
allows for service identities to be provided as input to your policy. The
example below shows the input structure.

```json
{
  "operation": {
    "name": "Write"
  },
  "resource": {
    "resourceType": {
      "name": "Topic"
    },
    "name": "credit-scores"
  },
  "session": {
    "principal": {
      "principalType": "User"
    },
    "clientAddress": "172.21.0.5",
    "sanitizedUser": "CN%3Danon_producer.tutorial.openpolicyagent.org%2COU%3DTUTORIAL%2CO%3DOPA%2CL%3DSF%2CST%3DCA%2CC%3DUS"
  }
}
```

The client identity is extracted from the SSL certificates that clients
present when they connect to the broker. The client identity information is
encoded in the `input.session.sanitizedUser` field. This field can be decoded
inside the policy.

Generating SSL certificates and JKS files required for SSL client
authentication is outside the scope of this tutorial. To simplify the steps
below, the Docker Compose file uses an extended version of the
[confluentinc/cp-kafka](https://hub.docker.com/r/confluentinc/cp-kafka/)
image from Docker Hub. The extended image includes **pre-generated SSL
certificates** that the broker and clients use to identify themselves.

Do not rely on these pre-generated SSL certificates in real-world scenarios.
They are only provided for convenience/test purposes.

#### Kafka Authorizer JAR File

The Kafka image used in this tutorial includes a pre-installed JAR file that implements the [Kafka Authorizer](https://kafka.apache.org/documentation/#security_authz) interface. For more information on the authorizer see [open-policy-agent/contrib/kafka_authorizer](https://github.com/open-policy-agent/contrib/tree/main/kafka_authorizer).

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

default allow = false

allow {
	not deny
}

deny {
	is_read_operation
	topic_contains_pii
	not consumer_is_whitelisted_for_pii
}

#-----------------------------------------------------------------------------
# Data structures for controlling access to topics. In real-world deployments,
# these data structures could be loaded into OPA as raw JSON data. The JSON
# data could be pulled from external sources like AD, Git, etc.
#-----------------------------------------------------------------------------

consumer_whitelist = {"pii": {"pii_consumer"}}

topic_metadata = {"credit-scores": {"tags": ["pii"]}}

#-----------------------------------
# Helpers for checking topic access.
#-----------------------------------

topic_contains_pii {
	topic_metadata[topic_name].tags[_] == "pii"
}

consumer_is_whitelisted_for_pii {
	consumer_whitelist.pii[_] == principal.name
}

#-----------------------------------------------------------------------------
# Helpers for processing Kafka operation input. This logic could be split out
# into a separate file and shared. For conciseness, we have kept it all in one
# place.
#-----------------------------------------------------------------------------

is_write_operation {
    input.operation.name == "Write"
}

is_read_operation {
	input.operation.name == "Read"
}

is_topic_resource {
	input.resource.resourceType.name == "Topic"
}

topic_name = input.resource.name {
	is_topic_resource
}

principal = {"fqn": parsed.CN, "name": cn_parts[0]} {
	parsed := parse_user(urlquery.decode(input.session.sanitizedUser))
	cn_parts := split(parsed.CN, ".")
}

parse_user(user) = {key: value |
	parts := split(user, ",")
	[key, value] := split(parts[_], "=")
}
```

The Kafka authorization plugin is configured to query for the
`data.kafka.authz.allow` decision. If the response is `true` the operation is
allowed, otherwise the operation is denied. When the integration queries OPA it
supplies a JSON representation of the operation, resource, and principal.

```live:example:query:hidden
data.kafka.authz.allow
```

```live:example:input
{
  "operation": {
    "name": "Write"
  },
  "resource": {
    "resourceType": {
      "name": "Topic"
    },
    "name": "credit-scores"
  },
  "session": {
    "principal": {
      "principalType": "User"
    },
    "clientAddress": "172.21.0.5",
    "sanitizedUser": "CN%3Danon_producer.tutorial.openpolicyagent.org%2COU%3DTUTORIAL%2CO%3DOPA%2CL%3DSF%2CST%3DCA%2CC%3DUS"
  }
}
```

With the input value above, the answer is:

```live:example:output
```

The `./policies` directory is mounted into the Docker container running OPA.
When the files under this directory change, OPA is notified and the policies
are automatically reloaded.

At this point, you can exercise the policy.

### 3. Exercise the policy that restricts consumer access to topics containing PII.

This step shows how you can grant fine-grained access to services using
Kafka. In this scenario, some services are allowed to read PII data while
others are not.

First, run `kafka-console-producer` to generate some data on the
`credit-scores` topic.

> This tutorial uses the `kafka-console-producer` and `kafka-console-consumer` scripts to generate and display Kafka messages. These scripts read from STDIN and write to STDOUT and are frequently used to send and receive data via Kafka over the command line. If you are not familiar with these scripts you can learn more in Kafka's [Quick Start](https://kafka.apache.org/documentation/#quickstart) documentation.


```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    bash -c 'for i in {1..10}; do echo "{\"user\": \"bob\", \"score\": $i}"; done | kafka-console-producer --topic credit-scores --broker-list kafka:9093 -producer.config /etc/kafka/secrets/anon_producer.ssl.config'
```

This command will send 10 messages to the `credit-scores` topic. Bob's credit
score seems to be improving.

Next, run `kafka-console-consumer` and try to read data off the topic. Use
the `pii_consumer` credentials to simulate a service that is allowed to read
PII data.

```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    kafka-console-consumer --bootstrap-server kafka:9093 --topic credit-scores --from-beginning --consumer.config /etc/kafka/secrets/pii_consumer.ssl.config
```

This command will output the 10 messages sent to the topic in the first part
of this step. Once the 10 messages have been printed, exit out of the script
(^C).

Finally, run `kafka-console-consumer` again but this time try to use the
`anon_consumer` credentials. The `anon_consumer` credentials simulate a
service that has **not** been explicitly granted access to PII data.

```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    kafka-console-consumer --bootstrap-server kafka:9093 --topic credit-scores --from-beginning --consumer.config /etc/kafka/secrets/anon_consumer.ssl.config
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
  not producer_is_whitelisted_for_large_fanout
}

producer_whitelist = {
  "large-fanout": {
    "fanout_producer",
  }
}

topic_has_large_fanout {
  topic_metadata[topic_name].tags[_] == "large-fanout"
}

producer_is_whitelisted_for_large_fanout {
  producer_whitelist["large-fanout"][_] == principal.name
}
```

Next, update the `topic_metadata` data structure in the same file to indicate
that the `click-stream` topic has a high fanout.

```live:updated_metadata:module:read_only
topic_metadata = {
  "click-stream": {
    "tags": ["large-fanout"],
  },
  "credit-scores": {
    "tags": ["pii"],
  }
}
```

### 5. Exercise the policy that restricts producer access to topics with high fanout.

First, run `kafka-console-producer` and simulate a service with access to the
`click-stream` topic.

```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    bash -c 'for i in {1..10}; do echo "{\"user\": \"alice\", \"button\": $i}"; done | kafka-console-producer --topic click-stream --broker-list kafka:9093 -producer.config /etc/kafka/secrets/fanout_producer.ssl.config'
```

Next, run the `kafka-console-consumer` to confirm that the messages were published.

```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    kafka-console-consumer --bootstrap-server kafka:9093 --topic click-stream --from-beginning --consumer.config /etc/kafka/secrets/anon_consumer.ssl.config
```

Once you see the 10 messages produced by the first part of this step, exit the console consumer (^C).

Lastly, run `kafka-console-producer` to simulate a service that should **not**
have access to _high fanout_ topics.

```bash
docker run --rm --network opakafkatutorial_default \
    openpolicyagent/demo-kafka:1.0 \
    bash -c 'echo "{\"user\": \"alice\", \"button\": \"bogus\"}" | kafka-console-producer --topic click-stream --broker-list kafka:9093 -producer.config /etc/kafka/secrets/anon_producer.ssl.config'
```

Because `anon_producer` is not authorized to write to high fanout topics, the
request will be denied and the producer will output an error message.

```
Not authorized to access topics: [click-stream]
```

## Wrap Up

Congratulations for finishing the tutorial!

At this point you have learned how to enforce fine-grained access control
over Kafka topics. In addition, you have seen how to break down policies into
smaller rules that can be reused and improve the overall readability over the
policy.

If you want to use the Kafka Authorizer plugin that integrates Kafka with
OPA, see the build and install instructions in the
[github.com/open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.

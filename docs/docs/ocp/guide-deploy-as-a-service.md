---
sidebar_position: 3
sidebar_label: "Tutorial: Deploy as a Service"
---

# Deploy as a Service to Kubernetes, AWS…

This example expands on [Kick the tires](./#kick-the-tires),
illustrating a more comprehensive and realistic configuration. This will
showcase a practical, more complete server configuration and demonstrate its
operational aspects.

**Key aspects to explore in this expanded configuration:**

- **Deployment Environment:**
  - Running the OCP server within Kubernetes
- **Database Integration:**
  - Connecting the OCP server to an external database. For this example, we will set up a PostgreSQL database within the Kubernetes environment, but a more likely solution would be a managed SQL database.
- **Source control integration**:
  - This example will utilize git as the source of the policies
- **Advanced Bundle Management:**
  - Using an HTTP datasource to include data in multiple bundles that OCP manages. Also, deploying the bundle to S3.
- **Configuration Management:**
  - Illustrating the dynamic pushing of configuration to the OCP server using `curl` commands. This will encompass updating configuration in real-time.

## Prerequisites

- A Git repository and valid credentials to read from it
- Access and credentials to an S3 bucket
- A Kubernetes environment (a local environment with minikube or other tools is fine)

## Complete manifest

The complete manifest can be found [here](https://github.com/open-policy-agent/opa-control-plane/blob/main/docs/k8s-manifests.yaml).

The rest of this section will just highlight portions of the full manifests to explain the concepts.

## Database Integration

As mentioned we will use a PostgreSQL deployment/service for this example, but for a production install you will likely want to use an external/managed database. Example configuration can be found in the [Database Configuration](./configuration.md#database-configuration) section.

The configuration to connect to the database will be in a configuration file. In our example it will look like this:

```yaml
database:
  sql:
    driver: postgres
    dsn: "postgres://opactl:password@postgres-service:5432/opactl?sslmode=disable"
```

### Database Migrations

The initial schema and incremental updates need to be applied to the database.
For simple setups, like OCP running as a single server, with full control over its database, pass `--apply-migrations` to `opactl run`.
That will make OCP apply all pending migrations on server startup.

For more complex scenarios, you can run `opactl db migrate` during rollout.
It uses the same configuration format as `opactl run`, see `opactl db migrate --help`.
Notably, it comes with a `--dry-run` flag for _not actually applying_ the migrations.

When OCP starts without `--apply-migrations`, it will
1. warn if there are un-applied migrations known to the binary
2. warn if the binary appears to be stale (migration state surpasses the binary's migrations)
3. finally, attempt to use the database as-is.

## OCP Server

The OCP server is fairly straightforward to setup, there is a docker [image](#docker-image) provided and you can configure it with one or more configuration files. In this case we will use multiple configuration files in a configmap to do the configuration. One important piece of configuration is the token setup for accessing the OCP API. Details of this configuration can be found in the [Authentication and permissions](./authentication.md) section, we will do a basic setup adds an admin and viewer API token:

```yaml
tokens:
  admin-user:
    api_key: "admin-api-key-change-me"
    scopes:
    - role: administrator
  viewer-user:
    api_key: "viewer-api-key-change-me"
    scopes:
    - role: viewer
```

Then the k8s manifest will look something like:

```yaml
containers:
- name: opactl
  image: openpolicyagent/opactl
  args:
  - "run"
  - "--addr=0.0.0.0:8282"
  - "--data-dir=/data"
  - "--config=/config.d/config.yaml"
  - "--config=/config.d/tokens.yaml"
  - "--config=/config.d/credentials.yaml"
  - "--config=/config.d/my-alpha-app.yaml"
  - "--config=/config.d/my-beta-app.yaml"
  - "--config=/config.d/my-shared-datasource.yaml"
  # - "--reset-persistence"                     # Not suitable for production, but useful for testing
  - "--log-level=debug"
```

`(other environment variables and mounts can be found in the complete manifest)`

## Git Source setup

The normal setup for a system will be to get the policies from a git repository. [Sources](./concepts.md#sources) have several options, but we will use a basic git config:

```yaml
sources:
  my-alpha-app:
    git:
      repo: https://github.com/your-org/my-alpha-app.git # Change to your Git repository URL
      reference: refs/heads/main
      # path: path/to/rules                                       # Path within the Git repo
      excluded_files:
      - .*/*
      credentials: git-creds
```

Credentials can be [configured](./concepts.md#secrets) to be either ssh key or basic auth, we will use basic auth (getting the actual creds from env variables so they can be injected with secrets):

```yaml
secrets:
  git-creds:
    type: "basic_auth"
    username: "${GIT_USERNAME}"
    password: "${GIT_PASSWORD}"
```

## S3 bundle deploy

OCP does not act as a bundle server so we need to put the bundle somewhere where OPA can retrieve it. OCP can deploy [bundles](./concepts.md#bundle-configuration-fields) to cloud storage services, we will deploy to S3 in our example. We will also include the source from our previously configured git repo and a yet to be configured datasource:

```yaml
bundles:
  my-alpha-app:
    object_storage:
      aws:
        bucket: my-aws-bucket-name # Change to your S3 bucket name
        key: bundles/my-alpha-app/bundle.tar.gz
        region: us-east-2 # Change to your AWS region
    requirements:
    - source: my-alpha-app
    - source: my-shared-datasource
```

Note: the name `my-alpha-app` in the requirements is specifically referencing the name under sources (from the previous step). You will oftentimes name the bundle with the same name to “link” them logically together, these will normally (but not required to) be configured together in their own configuration file.

You may notice that there are no credentials configured for the S3 bucket. In this case they will be pulled from environment variables configured in the OCP deployment.

```yaml
env:
- name: AWS_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_ACCESS_KEY_ID
- name: AWS_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_SECRET_ACCESS_KEY
- name: AWS_REGION
  valueFrom:
    secretKeyRef:
      name: aws-credentials
      key: AWS_REGION
```

## Shared Datasource

One of the powerful concepts in OCP is the ability to share policies and data across multiple bundles. To do this we create another [source](./concepts.md#sources) for this and require it in the bundle. We will set up a http datasouce to share, but you could just as easily do this for rego. Full datasource configuration can be found [here](./concepts.md#sources), but for our purposes we will call out to httpbin using a bearer token (other authn can be found [here](./concepts.md#secrets)):

```yaml
sources:
  my-shared-datasource:
    datasources:
    - name: httpbin-json
      path: httpbin
      type: http
      config:
        url: https://httpbin.org/bearer
        credentials: httpbin-credentials

secrets:
  httpbin-credentials:
    type: "token_auth"
    token: "my-fake-token"
```

## Deploy

This should now give you a configuration that is ready to deploy. The full manifest also includes a second application that also uses the shared datasoure, it’s configured much like the previous example using the filesystem for rego and bundles. Once OCP is up and running you should see bundles being automatically updated in S3 (or exec into the OCP container for the second app), sources are checked in a forever loop about every 30-60 seconds. The manifest also includes an OPA configured for the `my-alpha-app` S3 bundle. If you want to test through the OPA endpoints you can do a port forward:

```
kubectl port-forward svc/opa-service 8181:8181
```

## API Access

All the configuration to this point has been done through the configuration files and this may be suitable for many (likely smaller) installs, but for larger installs this might not be efficient. While OCP doesn’t have a UI it does have an [API](./api-reference.md) for doing configuration of [sources](./concepts.md#sources)/[bundles](./concepts.md#bundles)/[stacks](./concepts.md#stacks).

In order to use the api you will need to expose the opactl-server with whatever sort of ingress matches your infrastructure, but for testing you can use a simple port forward:

```shell
kubectl port-forward svc/opactl-service 8282:8282
```

To do a basic get of the bundles would look like this:

```shell
curl --request GET \
  --url http://localhost:8282/v1/bundles \
  --header 'Authorization: Bearer admin-api-key-change-me'
```

The output should look similar to this:

```json
{
  "result": [
    {
      "object_storage": {
        "aws": {
          "bucket": "my-aws-bucket-name",
          "key": "bundles/my-alpha-app/bundle.tar.gz",
          "region": "us-east-2"
        }
      },
      "requirements": [
        {
          "source": "my-alpha-app",
          "git": {}
        },
        {
          "source": "my-shared-datasource",
          "git": {}
        }
      ]
    },
    {
      "object_storage": {
        "filesystem": {
          "path": "bundles/my-beta-app/bundle.tar.gz"
        }
      },
      "requirements": [
        {
          "source": "my-beta-app",
          "git": {}
        },
        {
          "source": "my-shared-datasource",
          "git": {}
        }
      ]
    }
  ]
}
```

If you want to add a new bundle you can upsert it with a PUT request:

```shell
curl --request PUT \
  --url http://localhost:8282/v1/bundles/my-alpha-app-no-shared \
  --header 'Authorization: Bearer admin-api-key-change-me' \
  --header 'Content-Type: application/json' \
  --data '{
    "object_storage": {
                "aws": {
                    "bucket": "my-aws-bucket-name",
                    "key": "bundles/my-alpha-app-no-shared/bundle.tar.gz",
                    "region": "us-east-2"
                }
            },
            "requirements": [
                {
                    "source": "my-alpha-app",
                    "git": {}
                }
            ]

}'
```

Then you can get that specific bundle like this:

```shell
curl --request GET \
  --url http://localhost:8282/v1/bundles/my-alpha-app-no-shared \
  --header 'Authorization: Bearer admin-api-key-change-me'
```

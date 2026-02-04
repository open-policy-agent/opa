---
sidebar_position: 2
title: Concepts
---

## Bundles

Bundles are the primary packaging and distribution unit in OCP. Each bundle contains Rego policies, data files, and is intended to be consumed by any number of OPA instances. The OCP configuration for the bundle specifies a set of **requirements** that list the sources (Rego, data, etc.) to include in the bundle.

OCP builds [OPA Bundles](https://www.openpolicyagent.org/docs/management-bundles) and pushes them to external object storage systems (e.g., S3, GCS, Azure Cloud Storage, File System). OPA instances are configured to download bundles directly from these storage systems. See the [OPA Configuration](https://www.openpolicyagent.org/docs/configuration) documentation for more information how to configure authentication and bundle downloads for different cloud providers

### Namespacing

In OCP, bundles must not require multiple sources with overlapping packages. When OCP builds bundles it checks that no two (distinct) sources being included in a bundle contain packages that are the same or prefix each other. This rule is applied transitively to all sources included in the bundle. If two sources contain overlapping packages OCP will report a build error:

`requirement "lib1" contains conflicting package x.y.z`
`- package x.y from "system"`

In this example:

- lib1 is the name of a source that declares package x.y.z
- system is the name of another source that declares package x.y
- because x.y is a prefix of x.y.z, they overlap

If you are interested in seeing this restriction relaxed please leave a comment [on Issue #30](https://github.com/open-policy-agent/opa-control-plane/issues/30) including any details you can share about your use case.

### Bundle Configuration Fields

- **object\_storage:**
  Configure the storage backend (S3, GCS, Azure Cloud Storage, or filesystem, etc.) for bundle distribution. OCP will write bundles to the object storage backend and the bundles will be served from there.
  - Filesystem:
    - Path: Path where the bundle will be created
      - Example: `bundles/prod-app.tar.gz`
  - Amazon S3 (aws):
    - Bucket: Name of the bucket
      - Example: `my-prod-bucket`
    - Key: Path and or name of the bundle to be built
      - Example: `prod/bundle.tar.gz`
    - Region: Aws region bucket was created in
    - Credentials: Reference a named Secret for authenticating with the target object store.
  - GCP Cloud Storage (gcp):
    - Project: GCP project the bucket is a part of
    - Bucket: Name of the bucket
    - Object: Name of the bundle, including the path
    - Credentials: Reference a named Secret for authenticating with the target object store.
  - Azure Blob Storage (azure):
    - Account URL: URL to the Azure account
    - Container: Name of the blob storage container
    - Path: Path and name of the bundle to be created
    - Credentials: Reference a named Secret for authenticating with the target object store.
- **labels:**
  Add metadata to bundles to describe environment, team, system-type, etc. Labels are used by Stacks (see below) for bundle selection and policy composition.
- **requirements:**
  Specify policies or data (from Sources) that must be included in the bundle. Requirements can include optional `path` and `prefix` settings to rewrite package names and data paths.
- **Excluded\_files**: (optional)
  A list of files to be excluded from the bundle during build for example any hidden files

#### Examples

**Filesystem:**

```yaml
bundles:
  prod-app:
    object_storage:
      filesystem:
        path: bundles/prod-app.tar.gz
    labels:
      environment: prod
      team: payments
    requirements:
    - source: app-policy
```

**Amazon S3 (aws):**

```yaml
bundles:
  prod-app:
    object_storage:
      aws:
        bucket: my-prod-bucket
        key: prod/bundle.tar.gz
        url: https://s3.amazonaws.com
        region: us-east-1
        credentials: s3-prod-creds
```

**GCP Cloud Storage (gcp):**

```yaml
bundles:
  prod-app:
    object_storage:
      gcp:
        project: my-gcp-project
        bucket: policy-bundles
        object: bundles/my-app/bundle.tar.gz
        credentials: gcp-service-account
```

**Azure Blob Storage (azure):**

```yaml
bundles:
  prod-app:
    object_storage:
      azure:
        account_url: https://mystorageaccount.blob.core.windows.net
        container: policy-bundles
        key: bundles/my-app/bundle.tar.gz
        credentials: azure-credentials
```

## Sources

Sources define how OCP pulls Rego and data from external systems, local files, or built-in libraries to compose and build bundles.

### Types of Sources

- **git:** Pull policy code and data from a Git repository (HTTPS, with token/basicauth credentials).
  - Repo: Repository url either https or ssh
    - Example: `https://github.com/example/app-policy.git`
  - Reference (Optional): git reference
    - Example: `refs/head/main`
  - Commit (Optional): Commit sha of the commit you want the bundle built from
    - Example: `d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3`
  - Path (Optional): Git path to the files to be included in the policy
    - Example: `policies/authz`
  - Include Files (Optional): Files to explicitly include in the bundle
  - Excluded Files (Optional): Files to be explicitly excluded from the bundle
    - Example: `.*/*`
  - Credentials: Reference a named Secret for authenticating with the target git repository
- **datasources:** Configure HTTP(S) endpoints, APIs, or other external data services as data sources for policy evaluation.
  - Name
  - Path
  - Type
    - http
  - Transform Query
  - Config
  - Credentials
- **files:** Local embedded files provided to OCP at build time.
- **directory:** Local directories provided to OCP at build time
- **paths:** Paths to individual rego or datasource files to be used during bundle build
- **builtin:** Reference built-in policy or library modules shipped with OCP.
- **requirements:** Specify dependencies on other sources or builtins for composable policy development.
- **credentials:** Reference a named Secret for accessing Git/datasource endpoints.

**Example:**
Configuration sourcing policy from git (`app-policy`), data from file (`global-data`)
and additional data pulled via http from Amazon S3 (`s3-data`). Information about used credentials is available in [Secrets](#secrets) section.

```yaml
sources:
  app-policy:
    git:
      repo: https://github.com/example/app-policy.git
      reference: refs/heads/main
      excluded_files:
      - .*/*
      credentials: github-token
  global-data:
    paths:
      - global/common.json
  s3-data:
    datasources:
      - name: s3-datasource
        type: http
        path: data/from/s3
        config:
          url: https://my-bucket.s3.my-region.amazonaws.com/s3-data.json
          credentials: aws_auth
```

### Path and Prefix Rewrites

When including sources in bundles, stacks, or as requirement of other sources, you can use `path` and `prefix` settings to rewrite Rego package names and data paths. This allows you to mount sources into different namespaces to avoid conflicts, import external policies and data under controlled prefixes, and select only specific subtrees of data or policies from a source.

#### Configuration

Each source requirement can specify:

- **path**: Selects a subtree of `data` to include (default: `data`, meaning everything)
- **prefix**: The new prefix where the selected content will be mounted (default: `data`, meaning no change)

#### Examples

**Basic path and prefix usage:**

```yaml
bundles:
  my-app:
    requirements:
    - source: library-policies
      path: library
      prefix: imported.lib.v1
```

This configuration selects everything under `data.library` from the `library-policies` source, rewrites package names from `data.library.authz` to `data.imported.lib.v1.authz`, moves data from `data.library` to `data.imported.lib.v1`, and adjusts all references accordingly.

**Mounting everything with a prefix:**

```yaml
requirements:
- source: external-policies
  prefix: external.policies
```

This mounts all content from `external-policies` under the `data.external.policies` namespace.

**Selecting a specific subtree:**

```yaml
requirements:
- source: shared-utils
  path: utils.validation
  prefix: app.validation
```

This takes only the `data.utils.validation` subtree and mounts it at `data.app.validation`.

#### Important Notes

- The `data.` prefix can be omitted for convenience: `path: library` is equivalent to `path: data.library`
- Only one `path`/`prefix` pair is allowed per source requirement
- Path selection for data depends on the filesystem structure - you can only select paths that correspond to actual directory boundaries
- Mounts are applied transitively - if source A depends on source B with mount settings, both mount configurations are applied

## Stacks

Stacks enforce that certain policies are distributed to OPAs managed by OCP. When OCP builds bundles it identifies the applicable stacks (via [Selectors](#selectors)) and then adds the required sources (declared via `requirements`) to the bundle. Consider using stacks if:

- You have ephemeral OPA deployments that need to have a consistent set of policies applied (e.g., CI/CD pipelines, Kubernetes clusters, etc.)
- You have global or hierarchical rules implementing organization-wide policies that you want to enforce automatically in many OPA deployments.

Let's look at an example:

- Your organization deploys microservices that use OPA to enforce API authorization rules
- Each microservice and bundle is owned by a separate team
- You want to enforce a global policy that blocks users contained in a blocklist

Stacks provide a convenient and scalable way of enforcing this policy. Instead of manually modifying the policy for each microservice or requiring that each team write policies that call into a common library, you can define this policy once and configure a stack to inject it into the bundles for each microservice.

Because Stacks inherently involve multiple policy decisions, _conflicts_ can arise. See the [Conflict Resolution](#conflict-resolution) section for more information.

### Selectors

When OCP builds a bundle it includes all of the sources from all stacks that apply. A stack applies if both:

- The selector matches the bundle's labels AND EITHER
- The exclude selector DOES NOT match the bundle's labels OR
- The exclude selector is undefined

The selector and exclude selector are evaluated the same way. A selector matches if:

- It is empty

OR

- All of the keys in the selector exist in the labels

AND EITHER

- At least one selector value matches the corresponding label value

OR

- The selector value is empty (\[\])

A selector value matches the label value if:

- The selector value and the label value are the same OR
- The selector value contains a glob pattern (\*) that matches the label value.
  - OCP implements the same glob matching as [OPA's glob built-in functions](https://www.openpolicyagent.org/docs/policy-reference/builtins/glob).

### Conflict Resolution

If a stack policy and a bundle policy generate different decisions we refer to this as a _conflict_. Similarly, when multiple stacks are included in a bundle they may also generate conflicting decisions. Before returning the final decision to the application, the overall policy should resolve any potential conflicts by combining the different decisions. Below we provide examples of how to implement common conflict resolution patterns for different use cases. In general, conflict resolution involves:

- the bundle policy that produces a decision
- one or more stack policies that each produce a separate decision
- an **entrypoint** policy that produces by the final decision by composing all of the above

#### Pattern: stack deny overrides bundle allow

The following example shows how to implement a common pattern where:

- bundle owners define policies that generate allow decisions
- a single stack owner defines policies that generate deny decisions
- the final decision returned to the application should set allow to true IF
  - the bundle policy generates an allow (i.e., allow is true) AND
  - the stack policy does not generate a deny (i.e., deny is undefined or false)

To illustrate this pattern we will use a simple example with two bundle policies and a stack policy. The bundle policies allow access to microservice APIs (for a "petshop" service and a "notifications" service) and the stack policy will deny access based on a blocklist. Finally, there is an entrypoint policy that composes the bundle and stack policies to produce the final decision.

The petshop service will define a policy that allows:

- anyone to view pet profiles
- employees to update pet profiles

```rego
package service
import rego.v1

allow if {
  input.action == "view_pets"
}

allow if {
  input.action == "update_pets"
  input.principal.is_employee
}
```

The notifications service will define a policy that allows customers to subscribe to newsletters:

```rego
package service
import rego.v1

allow if {
  input.action == "subscribe_to_newsletter"
  input.principal.is_customer
}
```

The stack policy will deny users that are contained in the blocklist datasource.

```rego
package globalsecurity
import rego.v1

deny if {
  input.principal.username in data.blocklist
}
```

Finally, the entrypoint policy will combine the service and stack policy to produce the final decision:

```rego
package main
import rego.v1

main if {
  data.service.allow
  not data.mandatory.globalsecurity.deny
}
```

The configuration below illustrates how the bundles, sources, and stacks are tied together:

```yaml
bundles:
  petshop-svc:
    labels:
      environment: prod
    requirements:
    - source: petshop-svc
  notifications-svc:
    labels:
      environment: prod
    requirements:
    - source: notifications-svc

stacks:
  mandatory:
    selector:
      environment: [prod]
    requirements:
    - source: main
      automount: false
    - source: globalsecurity

sources:
  petshop-svc: ...
  notifications-svc: ...
  globalsecurity: ...
  main: ...
```

#### Pattern: unioning stack and bundle denies

The following example shows how to implement a common pattern where:

- bundle owners define a policy that generates a set of deny reasons
- stack owners also define policies that generate sets of deny reasons
- the final decision returned to the application should be the union of all the deny reasons

To illustrate this pattern we will use a simple example with a single bundle policy and two stack policies. The final decision will be generated by the entrypoint policy by unioning the bundle and stack decisions. For this example, we will assume that application querying OPA is a job running in a CI/CD pipeline that provides a set of build artifacts to deploy.

The bundle policy will deny deployments that contain artifacts that do not contain a "qa" attestation.

```rego
package pipeline
import rego.v1

deny contains msg if {
  some artifact in input.artifacts
  "qa" in artifact.attestations
  msg := sprintf("deployment contains untested artifact: %v", [artifact.name])
}
```

The first stack policy will block deployments that do not contain an SBOM:

```rego
package pipelines.stacks.sbom
import rego.v1

deny contains "deployments must contain sbom" if {
  not input.sbom
}
```

The second stack policy will block deployments if an artifact has critical CVEs:

```rego
package pipelines.stacks.cves
import rego.v1

deny contains msg if {
    some artifact in input.artifacts
    some cve in data.cves[artifact.sha]
    cve.level == "critical"
  msg := sprintf("artifact contains critical cve: %v", [cve.id])
}
```

The entrypoint policy will union all of the deny reasons to produce the final set. Since stacks are added to bundles dynamically at build-time, the entrypoint policy iterates over the `stacks` namespace. Only applicable stacks will be present in the bundle.

```rego
package pipelines
import rego.v1

deny contains msg if {
    some msg in data.pipeline.deny
}

deny contains msg if {
  some stackname
  some msg in data.pipelines.stacks[stackname].deny
}
```

The configuration below illustrates how the bundles, sources, and stacks are tied together:

```yaml
bundles:
  pipeline-a1234:
    labels:
      environment: prod
      type: pipeline
    requirements:
    - source: pipeline-a1234
    options:
      no_default_stack_mount: true

stacks:
  sbom:
    selector:
      environment: [prod]
      type: [pipeline]
    requirements:
    - source: sbom
  cves:
    selector:
      environment: [prod]
      type: [pipeline]
    requirements:
    - source: cves
  pipelines:
    selector:
      type: [pipeline]
    requirements:
    - source: pipelines

sources:
  pipeline-a1234: ...
  sbom: ...
  cves: ...
  pipelines: ...
```

## Secrets

**Goal:**
Secrets enable OCP to securely communicate with external systems (object storage, Git, datasources, etc.) without hardcoding credentials in configuration files.

### **Supported Secret Types**

- **aws\_auth:** For S3/MinIO storage (access key/secret key)
  - access\_key\_id:
  - secret\_access\_key:
  - session\_token:
- **basic\_auth:** For Git or HTTP(S) sources (username/password or token)
  - username:
  - password:
  - headers:
- **gcp\_auth:** For Google Cloud Storage
  - api\_key:
  - credentials: JSON credentials file
- **azure\_auth:** For Azure Blob Storage
  - account\_name:
  - account\_key:
- **github\_app\_auth:** For authentication as a GitHub App
  - integration\_id:
  - installation\_id:
  - Private\_key: Private key of the app as a PEM
- **ssh\_key:** For Authentication with an ssh key
  - key: Path to ssh key
  - passphrase:
  - Fingerprints: Optional ssh key fingerprints
- **token\_auth**: For authentication with a Bearer token or JWT token.
  - token:
- **password:** For password based authentication with datasource or the database
  - password:

**Example:**

```yaml
secrets:
  s3-prod-creds:
    type: aws_auth
    access_key_id: ${S3_ACCESS_KEY_ID}
    secret_access_key: ${S3_SECRET_ACCESS_KEY}
  github-token:
    type: basic_auth
    username: ${GITHUB_USERNAME}
    password: ${GITHUB_TOKEN}
```

# Configuration Organization

OCP configuration files can be organized as a single file or split across
multiple files and directories. For small or simple deployments, a single
configuration file may be sufficient and easier to manage. When defining an
"application," it is common practice to group related bundles and sources
together in the same configuration file. This approach keeps the application's
policy logic and its data sources tightly coupled, making updates and reviews
straightforward.

Best practices suggest keeping secrets and environment-specific overrides in
separate files or directories, while grouping each application's bundles and
sources together. Use lexical naming and directory structure to avoid conflicts.
For collaborative environments, version control each file and use
directory-based organization to support team workflows and automated deployment
pipelines. Choose the level of granularity that matches your operational
complexity—favor modularity for larger teams and environments, but keep things
simple for smaller setups.

## Multiple Configuration Files and Overrides

When you execute OCP commands you specify the path to configuration files or
directories with \-c/–config. The flag can point at individual files or
directories. If a directory is provided, OCP will load the contents of the
directory and all subdirectories (recursively) and merge them.

By default, OCP will merge object keys and override scalar values. Files are
loaded in lexical order and the last file to set a scalar or list value wins. If
the –merge-conflict-fail argument is specified, then scalar and list values are
never overridden and an error will be returned if two files set the same field
to a different value.

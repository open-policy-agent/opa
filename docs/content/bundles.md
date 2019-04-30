---
title: Bundles
kind: documentation
weight: 9
---

OPA can periodically download bundles of policy and data from remote HTTP
servers. The policies and data are loaded on the fly without requiring a
restart of OPA. Once the policies and data have been loaded, they are enforced
immediately. Policies and data loaded from bundles are accessible via the
standard OPA [REST API](../rest-api).

Bundles provide an alternative to pushing policies into OPA via the REST APIs.
By configuring OPA to download bundles from a remote HTTP server, you can
ensure that OPA has an up-to-date copy of policies and data required for
enforcement at all times.

OPA can only be configured to download one bundle at a time. You
cannot configure OPA to download multiple bundles. By default, the OPA
REST APIs will prevent you from modifying policy and data loaded via
bundles. If you need to load policy and data from multiple sources,
see the section below.

See the [Configuration Reference](../configuration) for configuration details.

## Bundle Service API

OPA expects the service to expose an API endpoint that serves bundles. The
bundle API should allow clients to download named bundles.

```http
GET /<bundle_prefix>/<name> HTTP/1.1
```

If the bundle exists, the server should respond with an HTTP 200 OK status
followed by a gzipped tarball in the message body.

```http
HTTP/1.1 200 OK
Content-Type: application/gzip
```

Enable bundle downloading via configuration. For example:

```yaml
services:
  - name: acmecorp
    url: https://example.com/service/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

bundle:
  name: authz/bundle.tar.gz
  prefix: somedir
  service: acmecorp
  polling:
      min_delay_seconds: 10
      max_delay_seconds: 20
```

Using this configuration, OPA will fetch bundles from
`https://example.com/service/v1/somedir/authz/bundle.tar.gz`.

The URL is constructed as follows:

```
https://example.com/service/v1/somedir/authz/bundle.tar.gz
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^ ^^^^^^^^^^^^^^^^^^^
services[0].url                prefix  name
```

If the `bundle.prefix` field is not defined, the value defaults to `bundles`.

See the following section for details on the bundle file format.

### Caching

Services implementing the Bundle Service API should set the HTTP `Etag` header
in bundle responses to identify the revision of the bundle. OPA will include the
`Etag` value in the `If-None-Match` header of bundle requests. Services can
check the `If-None-Match` header and reply with HTTP `304 Not Modified` if the
bundle has not changed since the last update.

## Bundle File Format

Bundle files are gzipped tarballs that contain policies and data. The data
files in the bundle must be organized hierarchically into directories inside
the tarball.

> The hierarchical organization indicates to OPA where to load the data files
> into the [the `data` Document](../how-does-opa-work#the-data-document).

You can list the content of a bundle with `tar`.

```bash
$ tar tzf bundle.tar.gz
.manifest
roles
roles/bindings
roles/bindings/data.json
roles/permissions
roles/permissions/data.json
http
http/example
http/example/authz
http/example/authz/authz.rego
```

In this example, the bundle contains one policy file (`authz.rego`) and two
data files (`bindings/data.json` and `permissions/data.json`).

Bundle files may contain an optional `.manifest` file that stores bundle
metadata. The file should contain a JSON serialized object.

* If the bundle service is capable of serving different revisions of the same
  bundle, the service should include a top-level `revision` field containing a
  `string` value that identifies the bundle revision.

* If you expect to load additional data into OPA from outside the
  bundle (e.g., via OPA's HTTP API) you should include a top-level
  `roots` field containing of path prefixes that declare the scope of
  the bundle. See the section below on managing data from multiple
  sources. If the `roots` field is not included in the manifest it
  defaults to `[""]` which means that ALL data and policy must come
  from the bundle.

* OPA will only load data files named `data.json`, i.e., you MUST name files
  that contain data (which you want loaded into OPA) `data.json` -- otherwise
  they will be ignored.

* The `*.rego` policy files must be valid [Modules](/docs/{{< current_version >}}/how-do-i-write-policies#modules)

## Multiple Sources of Policy and Data

By default, when OPA is configured to download policy and data from a
bundle service, the entire content of OPA's policy and data cache is
defined by the bundle. However, if you need to load OPA with policy
and data from multiple sources, you can implement your bundle service
to generate bundles that are scoped to a subset of OPA's policy and
data cache.

> We recommend that whenever possible, you implement policy and data
> aggregation centrally, however, in some cases that's not possible
> (e.g., due to latency requirements.)

To scope bundles to a subset of OPA's policy and data cache, include
a top-level `roots` key in the bundle that defines the roots of the
`data` namespace that are owned by the bundle.

For example, the following manifest would declare two roots
(`acmecorp/policy` and `acmecorp/oncall`):

```
{
    "roots": ["acmecorp/policy", "acmecorp/oncall"]
}
```

If OPA was loaded with a bundle containing this manifest it would only
erase and overwrite policy and data under these roots. Policy and data
loaded under other roots is left intact.

When OPA loads scoped bundles, it validates that:

* The roots are not overlapping (e.g., `a/b/c` and `a/b` are
  overlapped and will result in an error.)

* The policies in the bundle are contained under the roots. This is
  determined by inspecting the `package` statement in each of the
  policy files. For example, given the manifest above, it would be an
  error to include a policy file containing `package acmecorp.other`
  because `acmecorp.other` is not contained in either of the roots.

* The data in the bundle is contained under the roots.

If bundle validation fails, OPA will report the validation error via
the Status API.

## Debugging Your Bundles

When you run OPA, you can provide bundle files over the command line. This
allows you to manually check that your bundles include all of the files that
you intended and that they are structured correctly. For example:

```bash
opa run bundle.tar.gz
```

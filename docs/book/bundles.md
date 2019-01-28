# Bundles

OPA can periodically download bundles of policy and data from remote HTTP
servers. The policies and data are loaded on the fly without requiring a
restart of OPA. Once the policies and data have been loaded, they are enforced
immediately. Policies and data loaded from bundles are accessible via the
standard OPA [REST API](rest-api.md).

Bundles provide an alternative to pushing policies into OPA via the REST APIs.
By configuring OPA to download bundles from a remote HTTP server, you can
ensure that OPA has an up-to-date copy of policies and data required for
enforcement at all times.

OPA can only be configured to download one bundle at a time. You
cannot configure OPA to download multiple bundles. If you need to
provide OPA with data or policies from multiple sources, you should
merge the data and policies within your bundle service.

See the [Configuration Reference](configuration.md) for configuration details.

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

## Bundle File Format

Bundle files are gzipped tarballs that contain policies and data. The data
files in the bundle must be organized hierarchically into directories inside
the tarball.

> The hierarchical organization indicates to OPA where to load the data files
> into the [the `data` Document](how-does-opa-work.md#the-data-document).

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

OPA will only load data files named `data.json`, i.e., you MUST name files
that contain data (which you want loaded into OPA) `data.json` -- otherwise
they will be ignored.

## Debugging Your Bundles

When you run OPA, you can provide bundle files over the command line. This
allows you to manually check that your bundles include all of the files that
you intended and that they are structured correctly. For example:

```bash
opa run bundle.tar.gz
```

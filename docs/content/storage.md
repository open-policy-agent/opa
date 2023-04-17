---
title: Storage
kind: operations
weight: 60
---

## Disk

This page outlines configuration options relevant to using the disk storage
feature of OPA.
Configuration options are to be found in [the configuration docs](../configuration/#disk-storage).

{{< info >}}
The persistent disk storage enables OPA to work with data that does not fit
into the memory resources granted to the OPA server.
It is **not** supposed to be used as the primary source of truth for that data.

The on-disk storage should be considered ephemeral: you need to secure the
means to restore that data.
Backup and restore, or repair procedures for data corruption are not provided
at this time.
{{< /info >}}

### Partitions

Partitions determine how the JSON data is split up when stored in the
underlying key-value store.
For example, this table shows how an example document would be stored given
different configured partitions:

```json
{
  "users": {
    "alice": { "roles": ["admin"] },
    "bob": { "roles": ["viewer"] }
  }
}
```

| Partitions | Keys | Values |
| --- | --- | --- |
| (1) none | `/users` | `{"alice": {"roles": ["admin"]}, "bob": {"roles": ["viewer"]}}}` |
| --- | --- | --- |
| (2) `/users` | `/users/alice` | `{"roles": ["admin"]}`  |
|          | `/users/bob`   | `{"roles": ["viewer"]}` |
| --- | --- | --- |
| (3) `/users/*` | `/users/alice/roles` | `["admin"]`  |
|            | `/users/bob/roles`   | `["viewer"]` |

Partitioning has consequences on performance: in the example above, the
number of keys to retrieve from the database (and the amount of data of
its values) varies.

| Query | Partitions | Number of keys read |
| --- | --- | --- |
| `data.users` | (1) | 1 |
|              | (2) | 2 |
|              | (3) | 2 |
| --- | --- | --- |
| `data.users.alice` | (1) | 1 with `bob` data thrown away|
|                    | (2) | 2 |
|                    | (3) | 2 |

For example, retrieving the full extent of `data.users` from the disk store
will require a single key fetch with the partitions of (1).
With (2), the storage engine will fetch two keys and their values.

Retrieving a single user's data, e.g. `data.users.alice`, will require
reading a single key and all the users data with (1); but throw away most
of it: all the data not belonging to `alice`.

There is no one-size-fits-all setting for partitions: good settings depend
on the actual usage, and that comes down to the policies that are used with
OPA.
Commonly, you would optimize the partition settings for those queries that
are performance critical.

To figure out suboptimal partitioning, please have a look at the exposed
metrics.

OPA stores some internal values (such as bundle metadata) in the data store,
under `/system`. Partitions for that part of the data store are managed by
OPA, and providing any overlapping partitions in the config will raise an
error.

### Metrics

Using the [REST API](../rest-api/), you can include the `?metrics` query string
to gain insights into the disk storage access related to a certain OPA query.

```
$ curl 'http://localhost:8181/v1/data/tenants/acme1/bindings/user1?metrics' | opa eval -I 'input.metrics' -fpretty
{
  "counter_disk_read_bytes": 339,
  "counter_disk_read_keys": 3,
  "counter_server_query_cache_hit": 1,
  "timer_disk_read_ns": 40736,
  "timer_rego_external_resolve_ns": 251,
  "timer_rego_input_parse_ns": 656,
  "timer_rego_query_eval_ns": 66616,
  "timer_server_handler_ns": 117539
}
```

The `timer_disk_*_ns` timers give an indication about how much time
was spent with the different disk operations.

Available timers are

- `timer_disk_read_ns`
- `timer_disk_write_ns`
- `timer_disk_commit_ns`

Also note the `counter_disk_*` counters in the metrics:

- `counter_disk_read_keys`: number of keys retrieved
- `counter_disk_written_keys`: number of keys written
- `counter_disk_deleted_keys`: number of keys deleted
- `counter_disk_read_bytes`: bytes retrieved

Suboptimal partition settings can be spotted when the amount of
keys and bytes retrieved for a query is unproportional to the
actual data returned: the query likely had to retrieve a giant
JSON object, and most of it was thrown away.

### Debug Logging

Pass `--log-level debug` to `opa run` to see all the underlying storage
engine's logs.

When debug logging is _enabled_, the service will output some
statistics about the configured disk partitions and their key
sizes.

```
[DEBUG] partition /tenants/acme3/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme4/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme8/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme9/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme0/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme2/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
[DEBUG] partition /tenants/acme6/bindings (pattern /tenants/*/bindings): key count: 10000 (estimated size 598890 bytes)
```

Note that this process will iterate over all database keys.
It only happens on startup, when debug logging is enabled.

### Fine-tuning Badger settings (superflags)

While partitioning should be the first thing to look into to tune the memory usage and
performance of the on-disk storage engine, this configurable gives you the means to
change many internal aspects of how Badger uses memory and disk storage.

{{< danger >}}
To be used with care!

Any of the Badger settings used by OPA can be overridden using this feature.
There is no validation happening for configurables set using this flag.

When the embedded Badger version changes, these configurables could change,
too.
{{< /danger >}}

The configurables correspond to Badger options that can be set on [the library's Options
struct](https://pkg.go.dev/github.com/dgraph-io/badger/v3#Options).

The following configurables can _not_ be overridden:

- `dir`
- `valuedir`
- `detectconflicts`

Aside from conflict detection, Badger in OPA uses the default options [you can find here](https://github.com/dgraph-io/badger/blob/v3.2103.2/options.go#L128-L187).

Conflict detection is disabled because the locking scheme used within OPA does not allow
for having multiple concurrent writes.

#### Example

```yaml
storage:
  disk:
    directory: /tmp/disk
    badger: nummemtables=1; numgoroutines=2; maxlevels=3
```

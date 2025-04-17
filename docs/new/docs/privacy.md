---
title: Privacy
kind: operations
weight: 50
---

This document provides details about OPA's anonymous information reporting feature.

## Overview

OPA periodically reports its version and specific anonymous runtime statistics to a publicly hosted, external service.
The reports contain the OPA version number (e.g., v0.12.3), a randomly generated UUID and the following runtime statistics:

* heap usage in bytes

This feature is only applicable to the `opa run` and `opa version` commands.

In case of the `opa run` command, this feature is **ON by-default** and can be easily disabled by specifying
the `--disable-telemetry` flag. When OPA is started in either the server or repl mode, OPA calls the external service
on a best-effort basis and shares the version it's running and other statistics such as current memory usage.
The time taken to execute the remote call and process the subsequent response from the external service does not
delay OPA's start-up.

In case of the `opa version` command, this feature can be enabled by specifying the `--check` or `-c` flag.

## External Service

OPA uploads its information by default at [telemetry.openpolicyagent.org](https://telemetry.openpolicyagent.org).
The environment variable `OPA_TELEMETRY_SERVICE_URL` can be used to configure the external service OPA reports to.

Sample HTTP request from OPA to the external service looks like this:

```http
POST /v1/version HTTP/1.1
Host: telemetry.openpolicyagent.org
Content-Type: application/json
User-Agent: "Open Policy Agent/v0.12.3 (darwin, amd64)"
```

```json
{
  "id": "08c1d850-6065-478a-b9b5-a8f9f464ad33",
  "version": "v0.12.3",
  "heap_usage_bytes": "596000"
}
```

The *id* field in the request body above is a version 4 random UUID generated when OPA starts.

The external service checks the OPA version reported by a remote OPA client and responds with information about the
latest OPA release. This information includes a link to download the latest OPA version, release notes etc.

Sample response from the external service looks like this:

```json
{
  "latest": {
    "download": "https://openpolicyagent.org/downloads/v0.19.2/opa_darwin_amd64",
    "release_notes": "https://github.com/open-policy-agent/opa/releases/tag/v0.19.2",
    "latest_release": "v0.19.2"
  }
}
```

The external service response contains a link to download the latest released OPA binary for client's platform, and a link
to the OPA release notes.

## Benefits

* OPA's anonymous version reporting feature provides users with up-to-date information about new OPA versions while
still executing the familiar OPA `run` and `version` commands. It helps users stay abreast of OPA's latest capabilities
and hence empowers them to make informed decisions while upgrading their OPA deployments.

* OPA maintainers and the [Cloud Native Computing Foundation](https://cncf.io) (CNCF) executive staff can use the version
reports for obtaining more information about OPA usage and engagement. For example, the information can be used in 
making better decisions about OPA's deprecation cycle.

* Reporting a running OPA's memory usage can help to better understand how much memory an OPA instance is consuming and
thereby drive optimization efforts around better resource utilization. Some users have concerns around OPA's memory usage
and hence this information can help OPA maintainers quantify the number of impacted OPA deployments and also guide future
features and priorities for the project.

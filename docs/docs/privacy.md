---
title: Privacy
---

OPA checks for the latest release version by querying the GitHub API. This
feature only retrieves version information and does not send any data about your
OPA instance to external services. This feature is applicable to the `opa run`
and `opa version` commands.

For the `opa run` command, this feature is **ON by default** and can be disabled
by specifying the `--skip-version-check` flag. When OPA is started in either
server or REPL mode, OPA queries the GitHub API on a best-effort basis to check
if a newer version is available. The time taken to execute this check does not
delay OPA's start-up.

For the `opa version` command, this feature can be enabled by specifying the
`--check` or `-c` flag.

OPA checks the latest release version by querying the GitHub
API at `https://api.github.com`. The environment variable
`OPA_VERSION_CHECK_SERVICE_URL` can be used to configure an alternative service
URL.

Sample HTTP request from OPA to the GitHub API:

```http
GET /repos/open-policy-agent/opa/releases/latest HTTP/1.1
Host: api.github.com
User-Agent: OPA-Version-Checker
```

No data about your OPA instance is sent in the request. OPA simply retrieves
information about the latest release. The GitHub API responds with release
information including the tag name and release notes URL. OPA uses this
information to determine if a newer version is available and constructs a
download link for your platform. Sample response from the GitHub API:

```json
{
  "tag_name": "v1.12.2",
  "html_url": "https://github.com/open-policy-agent/opa/releases/tag/v1.12.2",
  ...
}
```

Based on this response, OPA constructs a platform-specific download link and
displays update information if a newer version is available.

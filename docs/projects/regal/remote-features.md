---
sidebar_position: 11
---


# Remote Features

This page outlines the features of Regal that need internet access to function.

## Checking for Updates

Regal will check for updates on startup. If a new version is available,
Regal will notify you by writing a message in stderr.

An example of such a message is:

```txt
A new version of Regal is available (v0.23.1). You are running v0.23.0.
See https://github.com/open-policy-agent/regal/releases/tag/v0.23.1 for the latest release.
```

This message is based on the local version set in the Regal binary, and **no
user data is sent** to GitHub where the releases are hosted.

This same function will also write to the file at: `$HOME/.config/regal/latest_version.json`,
this is used as a cache of the latest version to avoid consuming excessive
GitHub API rate limits when using Regal.

This functionality can be disabled in two ways:

* Using `.regal/config.yaml` / `.regal.yaml`: set `features.remote.check-version` to `false`.
* Using an environment variable: set `REGAL_DISABLE_CHECK_VERSION` to `true`.

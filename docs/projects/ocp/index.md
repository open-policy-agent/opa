---
sidebar_position: 1
---
# OPA Control Plane


OPA Control Plane (OCP) simplifies how you manage policies for your OPA
deployments. It provides a centralized management system to control how OPAs
receive the policies and data they need to make decisions. OCP provides:

* **Git-based Policy Management.** Build bundles based on Rego from multiple Git
  repositories and implement environment promotion strategies natively with Git.
* **External Datasources.** Fetch and bundle external data required by your
  policies build-time using HTTP push and pull datasources.
* **Highly-Available & Scalable Bundle Serving.** Distribute bundles to cloud
  object storage like AWS S3, Google Cloud Storage, or Azure Blob Storage and
  ensure your OPAs can quickly and reliably serve policy decisions.
* **Global and hierarchical policies.** Enforce organization-wide rules by
  defining global policies that get injected into bundles at build-time based on
  label selectors. Global policies can override other policies based on custom
  conflict resolution logic written in Rego.

# Learn More

* [Deploy as a service](./guide-deploy-as-a-service.md) - Run OCP as a standalone service in Kubernetes
* [Concepts](./concepts.md) - Learn how OCP works
* [Configuration](./configuration.md) - Learn how to configure the server
* [API Reference](./api-reference.md) - Learn about the OCP REST API
* [Authentication](./authentication.md) - Learn how to secure the server API

# Kick the tires

Follow this section to get a quick example running on your laptop. By following
these instructions, you will be able to:

* Install OCP on your local machine.
* Define a basic bundle with a test policy.
* Use OCP to build the bundle
* Configure OPA to use the OCP build bundle
* Test the policy's enforcement and observe its effects.

This example is designed for rapid iteration and learning, making it ideal for new users who want to understand OCP's fundamental concepts and operational flow in a controlled, personal setting. We'll focus on simplicity and clarity, ensuring that each step is easy to follow and the outcomes are immediately visible.

## 1. Install binary

Install the opactl tool using one of the install methods listed below.

## 2. Define a bundle

The bundle is defined by a configuration file normally in the `config.d` directory.  More details can be found in the [Concepts](#concepts) section, but for now lets use this configuration.  In your working directory add the following to `./config.d/hello.yaml`

```yaml
bundles:
  hello-world:
    object_storage:
      filesystem:
        path: bundles/hello-world/bundle.tar.gz
    requirements:
      - source: hello-world
sources:
  hello-world:
    directory: files/sources/hello-world
    paths:
      - rules/rules.rego
```

We also will want to define a simple policy for this bundle.  Add the following to `./files/sources/hello-world/rules/rules.rego`

```rego
package rules

import rego.v1

default allow := false
allow if {
  input.user == "alice"
}
```

## 3. Build the bundle

In your working directory run the `build` command:

`opactl build`

## 4. Configure OPA to use the bundle

You could set up a simple server to serve up the bundle, but for now we can just use OPA to watch the bundle.  Run this in your working directory:

```shell
opa run -s -w ./bundles/hello-world/bundle.tar.gz
```

## 5. Test the policy

You should now be able to test the policy running in OPA.  Using the following curl:

```shell
curl localhost:8181/v1/data/rules/allow -d \
'{"input":{"user":"alice"}}'
```

You can also try changing the policy in `./files/sources/hello-world/rules/rules.rego`.  After you make the change, rerun the build command from above to see the changes reflected in OPA.

# Installation

## Download the OCP Binary

The OCP binary can be downloaded directly. After downloading, make the binary executable and verify it works by running the version command.

### macOS \- Apple silicon (ARM)

```shell
curl -L -o opactl https://openpolicyagent.org/downloads/latest/opactl_darwin_arm64_static
chmod +x opactl
./opactl version
```

### macOS \- Intel-based

```shell
curl -L -o opactl https://openpolicyagent.org/downloads/latest/opactl_darwin_amd64
chmod +x opactl
./opactl version
```

### Linux/Unix \- amd64

```shell
curl -L -o opactl https://openpolicyagent.org/downloads/latest/opactl_linux_amd64
chmod +x opactl
./opactl version
```

### Linux/Unix \- arm64

```shell
curl -L -o opactl https://openpolicyagent.org/downloads/latest/opactl_linux_arm64_static
chmod +x opactl
./opactl version
```

### Windows \- via Powershell

```shell
Invoke-WebRequest -Uri "https://openpolicyagent.org/downloads/latest/opactl_windows_amd64.exe" -OutFile "opactl.exe"
.\opactl.exe version
```

### Windows \- via Curl

```shell
curl -L -o opactl.exe https://openpolicyagent.org/downloads/latest/opactl_windows_amd64.exe
.\opactl.exe version
```

### Docker image {#docker-image}

OCP Docker images are available on Docker Hub.

```shell
openpolicyagent/opactl
```

## Adding opactl to PATH (Optional)

For easier usage, you can move the binary to a directory in your PATH:

**macOS/Linux:**
```shell
# Move to /usr/local/bin (requires sudo)
sudo mv opactl /usr/local/bin/

# Or move to a user directory (create if it doesn't exist)
mkdir -p ~/bin
mv opactl ~/bin/
export PATH="$HOME/bin:$PATH"  # Add to ~/.bashrc or ~/.zshrc for persistence
```

**Windows:**
```shell
# Move to a directory in your PATH or add current directory to PATH
move opactl.exe C:\Windows\System32\
```

After adding to PATH, you can run `opactl version` from anywhere.

# Build from source

To build the OCP binary locally run the following command from the root folder.
You will need to have a recent version of Go installed.

```shell
make build
```

The binary will be created in the form `opactl_<OS>_<ARCH>` (e.g., `opactl_darwin_amd64`, `opactl_linux_amd64`).

**Verify the build:**
```shell
# Example for macOS/Linux (adjust filename for your platform)
chmod +x ./opactl_darwin_amd64
./opactl_darwin_amd64 version
```

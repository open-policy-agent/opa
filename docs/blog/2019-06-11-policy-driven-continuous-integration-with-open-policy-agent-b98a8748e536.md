---
title: "Policy-driven continuous integration with Open Policy Agent"
authors: ["lperkins"]
date: 2019-06-11
slug: policy-driven-continuous-integration-with-open-policy-agent-b98a8748e536
---

_Take sound security policy to the source_

One of the things that I love most about [Open Policy Agent](https://openpolicyagent.org) (**OPA**) is that it was built to be interoperable with other systems. Anything that produces JSON — and nowadays most things do — can provide OPA with inputs for rendering policy judgments. Due to this interoperability, you can use OPA with container-based development tools like [Docker](https://www.openpolicyagent.org/docs/latest/docker-authorization/), infrastructure provisioning tools like [Terraform](https://www.openpolicyagent.org/docs/latest/terraform/), container orchestration platforms like [Kubernetes](https://www.openpolicyagent.org/docs/latest/kubernetes-admission-control/), and that's just scratching the surface.

## OPA and continuous integration

Because OPA can integrate with just about anything, virtually every single part of a modern software "stack" can be policy driven, _including_ continuous integration. With OPA you can create policies that govern which artifacts are allowed to be built _in the first place_, providing a powerful lever for keeping potentially malicious jobs and services from _ever_ running on your systems (make sure those are also governed by OPA policies!).

Incidents like the recent `event-stream` debacle (see [details](https://blog.npmjs.org/post/180565383195/details-about-the-event-stream-incident)) — amongst many others — demonstrate just how necessary these safeguards are.

In fact, you're probably _already_ applying policies at the CI level but doing so in an ad hoc way via a loose assemblage of scripts. OPA provides an excellent platform for making those implicit policies explicit and declarative. I'll provide a straightforward example for what policy-driven CI may look like in the next section.

> "You're probably already _applying policies at the CI level but doing so in an ad hoc way. OPA enables you to make your implicit policies explicit and declarative._"

## Dependency blacklisting in action

As an example, let's say that I'm a developer working on a Node.js web server in a large organization. That organization enforces CI policies using a policy written in [Rego](https://www.openpolicyagent.org/docs/latest/how-do-i-write-policies), OPA's policy language. The CI provider is [GitHub Actions](https://developer.github.com/actions/), though the example could easily be ported to other CI providers. The code for this example is in the [lucperkins/opa-ci-example](https://github.com/lucperkins/opa-ci-example/) repository on GitHub.

_The Rego policy governing package.json `dependencies`_

```rego
package ci

# The package.json is presumed faulty
default allow = false

# Packages that aren't allowed
blacklist = {
    "event-stream",
    "left-pad"
}

# Records dependencies that are on the blacklist
violations[pkg] {
    input.dependencies[pkg]
    blacklist[pkg]
}

# Returns true only if there are no violations
allow {
    count(violations) == 0
}
```

This policy takes each project's `package.json` file as an input and applies the policy to that (notice the `input.dependencies`). Two things to note about the policy:

- `default allow = false` means that my `package.json` dependencies are _presumed_ faulty and must pass muster before the next CI stage (the installation stage) is reached. This is generally a good practice for Rego policies.
- The `violations[pkg]` block creates a list of blacklist-violating packages that is returned in the evaluation output in case of violations, making it easier for developers to know _why_ the evaluation is failing.
- The evaluation succeeds (i.e. the script returns an exit code of 0) only if there are no violations (`count(violations) == 0`); otherwise, it fails.

So that covers our dependencies policy. Now let's dive into the GitHub Action workflow definition.

_The GitHub Actions workflow for this application_

```hcl
workflow "OPA evaluation" {
  on = "push"
  resolves = ["install"]
}

# Determines whether the policy has been violated
action "evaluate" {
  uses = "docker://openpolicyagent/opa:0.11.0"
  args = [
    "eval",
    "--fail-defined", "data.ci.violations[pkg]",
    "--input", "package.json",
    "--data", "ci.rego",
    "--format", "pretty"
  ]
}

# Installs the dependencies in package.json
# iff the evaluate action succeeds
action "install" {
  uses = "nuxt/actions-yarn@master"
  args = "install"
  needs = "evaluate"
}
```

There are two Actions in this workflow: `evaluate` and `install` (in a more fleshed-out scenario there may be other stages, like `build-container` or `deploy-to-k8s`). The `evaluate` action runs the following script, using the `openpolicyagent/opa:0.11.0` Docker image:

```bash
opa eval \
    --fail-defined 'data.ci.violations[pkg]' \
    --input package.json \
    --data ci.rego \
    --format pretty
```

The OPA evaluation fails any time the `violations[pkg]` directive is satisfied, i.e. any time a dependency is both in the `dependencies` block in `package.json` and on the package blacklist. The `--format pretty` flag dictates that the failure output includes a visually appealing table like this:

```
+----------------+-------------------------+
|      pkg       | data.ci.violations[pkg] |
+----------------+-------------------------+
| "dependency-1" | "dependency-1"          |
| "dependency-2" | "dependency-2"          |
+----------------+-------------------------+
```

If I'm a developer working on this project, I get highly readable, actionable feedback about policy violations directly in the CI output. So how is my build currently faring?

## Results

Well… not so good. On my current [`dev`](https://github.com/lucperkins/opa-ci-example/tree/dev) branch, my `package.json` file looks like this.

_The doomed-to-fail package.json_

```json
{
  "private": true,
  "dependencies": {
    "event-stream": "^4.0.1",
    "express": "^4.17.1",
    "left-pad": "^1.3.0"
  }
}
```

I've included two dependencies here — `event-stream` and `left-pad` — that very obviously violate the blacklist. You can see the [resulting CI failure run](https://github.com/lucperkins/opa-ci-example/runs/142847261). Let's fix this!

[This pull request](https://github.com/lucperkins/opa-ci-example/pull/2) gets the job done. It removes the offending dependencies from the `package.json`. As you can see from the [results](https://github.com/lucperkins/opa-ci-example/runs/142848310) of the `evaluate` action, the `opa eval …` command returns `undefined` instead of a table listing violations. And because the `evaluate` action has passed, the [`install` action](https://github.com/lucperkins/opa-ci-example/runs/142848384) has been successfully invoked.

> You can see the failing policy and input in action in the [Open Policy Agent Playground](https://play.openpolicyagent.org/p/kipUorP7ui). Correct the inputs on your own to fix the build!

## Related efforts: testing Kubernetes configs

For another fully fleshed-out example of using OPA as part of a build pipeline, I highly recommend [Unit Testing Your Kubernetes Configurations Using Open Policy Agent](https://www.youtube.com/watch?v=AfTuzonH93U&app=desktop) from [Gareth Rushgrove](https://twitter.com/garethr/) (slides on [Speaker Deck](https://speakerdeck.com/garethr/unit-testing-your-kubernetes-configuration-with-open-policy-agent)), presented at KubeCon/CloudNativeCon EU 2019 in Barcelona.

Though Gareth's project has different aims from mine, it provides a very nice illustration of using OPA to prevent certain classes of problems from ever arising in production environments by vetting Kubernetes configurations.

If you have other examples — blog posts, code snippets, anything — please feel free to add a comment to share your work!

## Implications

What I've presented here is just a small taste of what's possible. You could use Open Policy Agent to build a much more robust system of CI checks. To give a few examples, you could write policies for:

- **Linters and formatters**, specifying allowable thresholds for deviance from desired norms
- **Code coverage checkers**, with requirements specified for each language and domain within your organization
- **Configuration files** for systems like [Kubernetes](https://kubernetes.io), [Prometheus](https://prometheus.io), [Envoy](https://envoyproxy.io), and many others
- **Utilize existing integrations** with other tools, such as Terraform and [Docker](https://www.openpolicyagent.org/docs/latest/docker-authorization/), [Terraform](https://www.openpolicyagent.org/docs/latest/terraform/), [Puppet](https://github.com/open-policy-agent/contrib/tree/master/puppet_example), and other CI-related tools.

Making your production systems policy driven is of the utmost importance, and that has to include sanitizing the inputs to those systems whenever possible. OPA quite simply provides the most robust and flexible platform in the open source world for doing so.

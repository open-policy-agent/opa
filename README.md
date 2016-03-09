# OPA: Open Policy Agent

OPA is an open source project which can help policy enable any application.

## What is Policy?

Policy defines expected behavior in response to specific events within an application. The implementation of policy can vary greatly within and across applications, e.g, it may be maintained manually by administrators, documented on wiki pages, hardcoded in applications, exposed through configuration, etc. Often, these policies are not part of the application's core business logic.

There are many examples of functionality within applications which can benefit from rich policy control, e.g, API authorization, VM and container placement, auto-scaling, auto-healing, etc.

## What is Policy Enabling?

Policy enabling an application decouples the policy implementation from the business logic so that administrators can define policy without changing the application while still keeping up with the size, complexity, and dynamic nature of modern applications.

## What does OPA provide?

OPA provides an open source policy engine implementation which simplifies the task of policy enabling applications. Application developers do not need to design a policy language, build a compiler or interpreter, or implement policy language analysis tools in order to policy enable their applications.

## Project Information

- License: [Apache Version 2.0](./LICENSE)
- Bugs, Features: [Github Issues](https://github.com/open-policy-agent/opa/issues)
- Mailing List: [open-policy-agent on Google Groups](https://groups.google.com/forum/?hl=en#!forum/open-policy-agent)
- Roadmap: [ROADMAP.md](./ROADMAP.md)
- Continuous Integration: [![Build Status](https://travis-ci.org/open-policy-agent/opa.svg?branch=master)](https://travis-ci.org/open-policy-agent/opa)
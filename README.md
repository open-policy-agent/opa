# OPA: Open Policy Agent

OPA is an open source project which can help policy enable any application.

## What is Policy?

Policy defines expected behavior in response to specific events within an application.

There are many examples of functionality within applications which can benefit from rich policy control, e.g, API authorization, VM and container placement, auto-scaling, auto-healing, etc.

The approach to policy can vary greatly within and across applications. For example, policy is often written in natural language (English), stored in documents such as wiki pages, and enforced manually by the administrators of the the application. 

When the cost of manually maintaining and enforcing policy is too high, policy is often defined in way that computers can understand: the application's implementation is updated to include the definition of policy and the means for enforcing it. Because most applications are implemented in **imperative languages** (such as Java or Python) the policy definition tends to focus on *how* the policy is enforced rather than *what* the expected behavior or state should be. This makes it hard for people unfamiliar with the application's implementation to know what the expected behavior or state should be. Furthermore, because policy is defined in the application's implementation, it is hard to adapt the policy without rebuilding, retesting, and redeploying the application. Deploy-time configuration may address this to some extent but is often incomplete or too course-grained.

A better approach is to define policy in a **declarative language** which can be understood by a computer. By using a declarative language, the policy definition can focus on *what* the expected behavior or state should be. This approach decouples policy from the application's implementation and lifecycle so that policy can be updated independently. When policy is defined this way, it can be readily reused in multiple systems. Finally, this approach to policy enables various features such as conflict detection, change tracking and access control independent from the application's source, sharing and re-use of policy, visualization, etc.

## What is Policy Enabling?

Policy enabling an application decouples the policy implementation from the business logic so that administrators can define policy without changing the application while still keeping up with the size, complexity, and dynamic nature of modern applications.

Policy enabling an application involves providing policy statements to an engine and then integrating the application with the engine to provide answers to questions such as:

- Can a specific operation be performed?
- What options are available for some operation?
- What policy violations currently exist?

To answer questions like these, policies must be written in a declarative language and then compiled and executed by a policy engine that knows about the state of the world (which is relevant to policy).

## What does OPA provide?

OPA provides an open source policy engine that simplifies the task of policy enabling applications.

OPA exposes several APIs to simplify integration:

- Query APIs with rich support for accessing stored data.
- Simple CRUD APIs to manage policies and data.
- Transactional APIs to operate on consistent snapshots of data.
- Asynchronous APIs to register for notification when policy is violated.

OPA's policy engine supports a purpose built declarative language for policy. Developers do not have to design a policy language, build a compiler or interpreter, or implement other language analysis tools to policy enable their applications.

## Project Information

- License: [Apache Version 2.0](./LICENSE)
- Bugs, Features: [Github Issues](https://github.com/open-policy-agent/opa/issues)
- Mailing List: [open-policy-agent on Google Groups](https://groups.google.com/forum/?hl=en#!forum/open-policy-agent)
- Roadmap: [ROADMAP.md](./ROADMAP.md)
- Continuous Integration: [![Build Status](https://travis-ci.org/open-policy-agent/opa.svg?branch=master)](https://travis-ci.org/open-policy-agent/opa)
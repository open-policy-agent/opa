---
nav_id: MAIN_DOCUMENTATION
doc_id: WHAT_IS_POLICY_ENABLEMENT
layout: documentation

title: What is Policy Enablement?
---

{% contentfor header %}

# What Is Policy Enablement?

A policy is a set of rules that governs the behavior of a service.

Policy-enablement empowers users to read, write, and manage these rules without needing specialized development or operational expertise.

When your users can implement policies without recompiling your source code, then your service is policy enabled.

{% endcontentfor %}

{% contentfor body %}

## Policies

All organizations have policies. Policies are essential to the long-term success of organizations because they encode important knowledge about how to comply with legal requirements, work within technical constraints, avoid repeating mistakes, and so on.

In their simplest form, policies can be applied manually based on rules that are written down or conventions that are unspoken but permeate an organization’s culture. Policies may also be enforced with application logic or statically configured at deploy time.

## Policy-Enabled Services

Policy-enabled services allow policies to be specified declaratively, updated at any time without recompiling or redeploying, and enforced automatically (which is especially valuable when decisions need to be made faster than humanly possible). They make deployments more adaptable to changing business requirements, improve the ability to discover violations and conflicts, increase the consistency of policy compliance, and mitigate the risk of human error.

A policy-enabled service is able to answer questions by comparing relevant input from its environment to policy statements written by administrators. For example, a cloud computing service could answer questions such as:

  * Can I add compute capacity?
  * In what regions can I add compute capacity?
  * Which instances are currently running in the wrong region?

## Open Policy Agent

OPA is a lightweight, self-contained and extensible agent co-located with the your service. OPA simplifies the task of policy enabling your service by implementing a full-featured policy engine for you. It provides:

  * Declarative policy authoring
  * Secure policy management
  * Interactive queries
  * Transactional data consistency
  * Asynchronous events

With OPA, you do not have to design a policy language, build a compiler or interpreter, or implement other language analysis tools to policy enable your service.

{% endcontentfor body %}

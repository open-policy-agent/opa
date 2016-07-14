---
nav_id: MAIN_HOME
layout: homepage

title: Open Policy Agent
---

{% contentfor header %}

# Open Policy Agent
{: .opa-header--title}

Decouple governance logic from application logic.
{: .opa-header--subtitle}

Open Policy Agent (OPA) simplifies the task of policy enabling your service. OPA provides an extensible framework for declarative policy authoring, secure policy management, interactive queries, transactional data consistency, asynchronous events, and more.
{: .opa-header--text}

So you can focus on other things.
{: .opa-header--text}

{% endcontentfor %}

{% contentfor experience %}

## Deliver the experience you want your users to have.
{: .opa-homepage--section--title}

Your users need deployments to comply with legal requirements, technical constraints, and their own team conventions. OPA policies are decoupled from your code so users can create and manage policies without modifying your service.
{: .opa-homepage--section--text}

{% img '{{assets["experience.svg"].logical_path}}' alt:'With OPA, admins define how your service should behave using a declarative language. Attempts to use your service are posed as questions that OPA answers.' %}
{: .opa-homepage--section--infographic}

{% endcontentfor %}

{% contentfor benefits %}

## Don’t roll your own policy engine.
{: .opa-homepage--section--title}

Do you really have time and resources to devote to designing, implementing, testing, and maintaining a policy engine?
{: .opa-homepage--section--text}

{% img '{{assets["benefits.svg"].logical_path}}' alt:'Without OPA, you need to implement policy management for your service from scratch. Required components must be carefully designed, implemented, and tested to ensure correct behavior and a positive user experience. That’s a lot of work. But OPA already includes everything you need in order to policy enable any service.' %}
{: .opa-homepage--section--infographic}

{% endcontentfor %}

{% contentfor features %}

## Is OPA right for you?
{: .opa-homepage--section--title}

OPA is purpose-built for modern deployment environments. OPA is…
{: .opa-homepage--section--text}

<div class="opa-homepage--section--feature-list">
<div class="opa-homepage--section--feature" markdown="1">

### Decoupled
{: .opa-homepage--section--feature--title}

Users don’t change your source code. Instead, they write policies in an easy-to-use, declarative language developed especially for OPA.
{: .opa-homepage--section--feature--text}

</div>

<div class="opa-homepage--section--feature" markdown="1">

### Easy to Deploy
{: .opa-homepage--section--feature--title}

OPA has zero deployment dependencies. It runs as a daemon side-by-side with your service and shares its fate for the purposes of high availabilty.
{: .opa-homepage--section--feature--text}

</div>

<div class="opa-homepage--section--feature" markdown="1">

### Compatible
{: .opa-homepage--section--feature--title}

OPA’s RESTful APIs use JSON over HTTP so you can integrate OPA with your service no matter which programming language you use.
{: .opa-homepage--section--feature--text}

</div>

<div class="opa-homepage--section--feature" markdown="1">

### Responsive
{: .opa-homepage--section--feature--title}

OPA is designed from scratch with latency-sensitive applications in mind, enforcing policies with minimal performance impact.
{: .opa-homepage--section--feature--text}

</div>

<div class="opa-homepage--section--feature" markdown="1">

### Interactive
{: .opa-homepage--section--feature--title}

Anyone can use OPA’s interactive shell to quickly experiment with queries and data sets.
{: .opa-homepage--section--feature--text}

</div>

<div class="opa-homepage--section--feature" markdown="1">

### Embeddable
{: .opa-homepage--section--feature--title}

Services written with Go can use OPA as a library and do not need to run a separate daemon.
{: .opa-homepage--section--feature--text}

{% endcontentfor %}

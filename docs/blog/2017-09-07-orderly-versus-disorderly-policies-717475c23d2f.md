---
title: "Orderly versus Disorderly Policies"
authors: ["timhinrichs"]
date: 2017-09-07
slug: orderly-versus-disorderly-policies-717475c23d2f
---

![Banner image for the Orderly versus Disorderly Policies blog post](/img/blog/orderly-versus-disorderly-policies-717475c23d2f/banner.webp)

One of the age-old questions with policy languages is: does the order of policy statements matter?

- For Orderly policies where statement-order matters, the policy engine makes snap judgments. It uses the first statement that applies to make a decision. Examples are Firewalls and IPTables.
- For Disorderly policies where statement-order doesn't matter, the policy engine is more contemplative. It finds all the statements that apply and combines them to make a decision. Examples are query languages like SQL and Datalog.

Are Orderly or Disorderly policies better? In this blog post we lay out the tradeoffs and conclude there's no clear winner (which is why the [Open Policy Agent](http://openpolicyagent.org) supports **both**). You want a policy language that lets you mix and match Orderly and Disorderly policies so you can leverage the strengths of each along the following dimensions.

- **Conflict resolution**: Implicit (Orderly) vs. Explicit (Disorderly)
- **Overrides and Defaults**: Easy (Orderly) vs. Difficult (Disorderly)
- **Composability**: Difficult (Orderly) vs. Easy (Disorderly)
- **Performance**: At most 1 (Orderly) vs. More than 1 (Disorderly)
- **Understandability:** Misleading (Orderly) vs. Requires Tools (Disorderly)

## Terminology

Throughout this blog post we'll use the following terms.

- A **policy** is a collection of statements.
- A **statement** makes a decision, such as whether an HTTP API call should be allowed or denied.
- A **policy language** defines what statements are possible. It defines what a statement, policy, or collection of policies means and what decisions it makes.
- A **policy engine** is a piece of software that ingests policies and evaluates them to make decisions.

As a running example, we'll use HTTP API authorization. The input to the policy has two components: a user making an API request and the HTTP path for that request. For example, the user might be `alice` and the HTTP path might be `/finance/alice/salary`. The policy must decide whether to allow or deny the input.

## Conflict Resolution

A conflict happens when a policy says something contradictory, like a request is both allowed and denied. A policy language that can't express conflicts is usually too impoverished in practice, so a policy language must have a way of eliminating conflicts if it's to make an actual decision.

For example, the following policy has a conflict: any URL of the form `/finance/<user>/salary` is both allowed and denied.

```
Deny /finance   
Allow /finance/<user>/salary
```

An Orderly policy resolves conflicts automatically because there's only ever 1 statement that applies: the first one. You order your policy statements so that conflicts get resolved the way you want.

A Disorderly policy on the other hand requires you to choose an explicit conflict resolution strategy for when multiple statements make contradicting decisions. For example, you might dictate that Allow overrides Deny (or vice versa).

The snippets below show the different options for the example above.

Option 1: Orderly

```
Deny /finance  
Allow /finance/<user>  
Final decision: Deny
```

Option 2: Orderly (statements in a different order)

```
Allow /finance/<user>  
Deny /finance  
Final decision: Allow
```

Option 3: Disorderly with Deny overrides Allow

```
Deny /finance  
Allow /finance/<user>  
Final decision: Deny
```

Option 4: Disorderly with Allow overrides Deny

```
Deny /finance  
Allow /finance/<user>  
Final decision: Allow
```

On the surface, Orderly policies seem to handle conflicts better than Disorderly policies, but in reality you're thinking about conflicts in both cases. For Orderly policies, you think about conflicts when you figure out what order to put your statements in, and for Disorderly policies you think about conflicts when choosing a conflict-resolution strategy. Here there is no clear winner.

## Overrides and Defaults

One of the best applications of an Orderly policy is to handle default conditions and overrides. Imagine you have an existing policy and temporarily need to block all requests to a particular HTTP API because of a recent security vulnerability or bug.

Using an Orderly policy you can add the appropriate policy statements to the top of the policy and rest assured that those statements will be the ones that get applied. In contrast, for a Disorderly policy you figure out what all statements are that conflict with your new statement and modify them so they no longer apply.

In our running example, imagine you have 4 different policy statements and now you want to block all API calls to `/finance`. With an Orderly policy, you simply add a single statement `Deny /finance` to the top of the policy. You don't even need to know what the other policy statements say.

**Orderly**

```
Deny /finance  
Allow /finance/<user> by <user>  
Allow /finance/<user> for HR group members  
Allow /finance/<user> for <user>'s manager  
Deny /finance/<user> for subordinates of <user>
```

In a Disorderly policy where Allow overrides Deny, we would need to identify and then temporarily remove or disable all of the Allow statements shown above.

Orderly policies clearly excel at the override use-case.

## Composability and Collaboration

Based on the last section, it might seem that Orderly policies are better because they make defaults and overrides so easy. But as we're about to see Disorderly policies are vastly better for the case of composition (the ability to combine different policies).

Composing (combining) different policies happens whenever multiple people are collaboratively defining a policy. Imagine you write policy A and someone else writes policy B (both about HTTP APIs). For Disorderly policies combining A and B is easy: union the statements of A and B together. Any conflicts between A and B are handled by conflict resolution.

In contrast, for Orderly policies, there's no clear way to combine A and B into a single policy. Put policy A before B, and all conflicts are resolved in your favor. Put policy B before A, and all of policy B's decisions win out. Orderly policies don't have conflicts or conflict resolution, so you need additional tooling to even identify conflicts between A and B.

For example, if you were to combine Orderly policies A and B below, which order would the authors of A and B agree on? (A then B does NOT produce the Orderly policy given in the last section.) If the policies are Disorderly, combining them is easy.

**Policy A (Ordered)**

```
Allow /finance/<user> by <user>  
Allow /finance/<user> for <user>'s manager
```

**Policy B (Ordered)**

```
Allow /finance/<user> for HR group members  
Deny /finance/<user> for subordinates of <user>
```

You might wonder how often composition happens. Anytime you have hierarchically organized resources (like files in a directory structure or HTTP API resources), you'll typically end up composing policies. It's natural to attach different policies to different points in the hierarchy and then make decisions based on the policies attached at a parent and child in the hierarchy. Sometimes you want the policy at the lower level to take precedence over the policy at the higher level, and sometimes the reverse is true.

Composability is the single greatest strength of Disorderly policies. People (and machines) can contribute what they know as policy statements, and the language lets you understand where the disagreements are, and what the final decision is.

## Performance

For Orderly policies, evaluation only executes until it finds one applicable statement. If that statement is number 10 out of 100,000 then the performance improvement is substantial. While it is unknown how many policy statements will need to be evaluated, at most 1 will ever be fully evaluated.

For Disorderly policies, it's important to implement indexing algorithms. Indexing algorithms analyze a policy and organize its statements so that by inspecting a request, it can quickly zero in on the (hopefully) small number of statements that apply.

In our running example, an indexer might identify that just the following two statements need to be evaluated. If it's smart, it can even conclude that the answer must be Allow without evaluating anything because all of the statements Allow the result.

```
Allow /finance/<user> by <user>  
Allow /finance/<user> for HR group members
```

Of course, you can use indexing for ordered evaluation, but it requires deeper analysis because statement k only applies if none of statements 1…k-1 apply, meaning that the conditions for statement k include some of the conditions from previous statements.

In terms of performance, it's hard to say whether Orderly or Disorderly policies are better. Indexing algorithms can be complex and lead to variability, but can also improve performance asymptotically (infinitely).

## Familiarity and Understandability

Technical considerations are important, but the best technical solution in the world won't be used if people don't understand it. How familiar and understandable a policy language is has a great impact on its adoption.

Orderly policies require the reader/writer to think top-to-bottom. That's the same way we train people to read, and it's the same way we train programmers to write most code in imperative, object-oriented, and functional languages. Orderly policies are familiar to a wide range of people.

Just because they're familiar doesn't mean Orderly policies are easy to understand (especially at scale). To know what statement number k means, you need to know which of the 1…k-1 statements conflict with statement k since they all take precedence over statement k. So to understand statement number 1,000 you need to understand those 999 statements that come before it. The worst part is that when people authored the policy, they may have known that there were only 5 statements that mattered, but they were forced to write them so that it looks like all 999 statements matter.

For example, understanding which requests get denied by the last statement in the Orderly policy below means understanding which requests get allowed by the first 3 statements. Understanding what happens to a request for a user's financial data by a member of HR only requires looking at the first 2 statements — it's safe to ignore all the rest.

**Orderly**

```
Allow /finance/<user> by <user>  
Allow /finance/<user> for HR group members  
Allow /finance/<user> for <user>'s manager  
Deny /finance/<user> for subordinates of <user>
```

Disorderly statements are no less familiar to the general populace than ordered. Every time you read two different newspaper articles, you get different sides of the story and need to figure out what really happened. For programmers, the order in which you define functions usually makes no difference; any time you create a set, use a boolean OR, launch multiple threads, or work on a distributed system you're working with unorderedness.

A single statement in a Disorderly policy stands on its own. Once you understand that statement, you need to understand which other statements conflict with it and how those conflicts are resolved. Tooling helps immensely here.

For example, in our running example you may want to understand the impact of the statement `Allow /finance/<user> for HR group members`. The statement by itself is simple enough, but you need to find all the statements that might conflict with it (e.g. the last statement conflicts if one of the HR group members is a subordinate), and how that conflict would be resolved.

**Disorderly**

```
Allow /finance/<user> by <user>  
Allow /finance/<user> for HR group members  
Allow /finance/<user> for <user>'s manager  
Deny /finance/<user> for subordinates of <user>
```

In the end, understanding any single statement in any policy (Orderly or Disorderly) requires understanding all the other statements that conflict with it. Orderly policies tell you that the conflicts all come before the statement you're looking at, but force you to order statements that have no inherent ordering, resulting in the appearance of conflicts when none exist. Disorderly policies give you no indication about which policy statements might conflict and rely on tooling to help you find conflicting statements.

## Summary

When all is said and done, there are times when you want statement order to matter (overrides), and there are times when you don't want statement order to matter (composition and hierarchies). If you have a tightly-constrained use case, you might be able to choose one or the other. But if you're interested in a general-purpose policy language that works for many different use cases, across many different domains, and is used by a wide population, there's no clear winner. You'll want your policy language to support both Orderly and Disorderly policies.

Given that this is a blog post for the [Open Policy Agent](http://openpolicyagent.org), it's probably not surprising that OPA supports both options and lets you mix and match as appropriate. See the [docs](http://www.openpolicyagent.org/docs/) and [FAQ](http://www.openpolicyagent.org/docs/faq.html) for more details, or reach out on the [slack channel](http://a5ec585d42ace11e7aaad0260e83477e-1095713357.us-west-2.elb.amazonaws.com).

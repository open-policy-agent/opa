---
title: "OPA's Full Stack Policy Language"
authors: ["timhinrichs"]
date: 2017-12-14
slug: opas-full-stack-policy-language-caeaadb1e077
---

![Banner image for OPA's full stack policy language post](/img/blog/opas-full-stack-policy-language-caeaadb1e077/banner.webp)

The [Open Policy Agent](http://www.openpolicyagent.org/) (OPA) has been used to policy-enable software across several different domains across several layers of the stack: container management (Kubernetes), servers (Linux), public cloud infrastructure (Terraform), and microservice APIs (Istio, Linkerd, CloudFoundry). In this post we describe how OPA's policy language Rego manages to work for all these different domains and layers of the stack without requiring any changes or extensions to the language.

## Rego Overview

Rego's sole purpose is to make policy decisions that other products/services need to take action. For example…

- Is this API request allowed or denied?
- What's the hostname of the backup server for this application?
- What's the risk score for this proposed infrastructure change?
- What list of clusters should this container be deployed to for high-availability?
- What's the routing information that should be used for this microservice?

Rego lets you write policy to answer all of those questions (and many more). It lets you write policy about any layer of the stack and any domain (e.g. APIs, servers, infrastructure, clusters, networking), without requiring you to change or extend the language. There are two key insights to Rego:

> Every domain can be encoded as JSON/YAML data.
>
> Policy is logic applied to data.

When you're writing policy, you should only be thinking about the domain you care about, how to encode that domain as data, and the logic you need to make a policy decision given that data. In this post, we show you how to do exactly that with Rego. Along the way you'll learn that:

- Rego data can be any JSON/YAML data
- Rego is a query language. You write logic to search and combine JSON/YAML data from different sources.

## Rego Data is JSON/YAML

As mentioned above, Rego is a general-purpose policy language, meaning that it works for any layer of the stack and any domain. The key idea is that while you as an author are thinking about servers, containers, or APIs, Rego just sees JSON/YAML data. So you can write policy about any domain as long as the information you need to make a decision can be stuffed into JSON/YAML.

For example, you can think of authorizing an HTTP API call as making a true/false decision about the YAML data shown below.

```yaml
user: alice
method: GET
path: /finance/salary/bob
headers:
- JWT: …
```

And you can think of authorizing someone to SSH into a server as a true/false decision about:

```yaml
user: alice
server_id: s12345
role: webapp
environment: prod
```

Besides the data describing the input to the decision, Rego lets you incorporate background information from many different sources of JSON/YAML to make a decision. You could, for example, tell Rego who each person's manager is:

```yaml
manager:
  charlie: bob
  dave: bob
  bob: alice
```

Then you could authorize managers to execute API calls that return their subordinates' salaries, or that a manager can SSH into their subordinates' desktop servers.

The key point here is that Rego does not understand what the data (or even the schema for the data) means in the real world. Because of that, you can use Rego to make policy decisions about APIs, servers, containers, risk-management, or any other domain you can imagine. It's the policy author that knows what the data means in the real world and writes logic to make a policy decision.

## Rego is a Query Language

To make a policy decision in Rego, you write logical tests on the data that comes in as input (such as the API or SSH data from the last section).

For example, if you want to allow a user to run the API call that reports her own salary, you write a policy where the input is a JSON/YAML document representing the API call (shown below tweaked slightly to represent the URL as an array instead of a string) and call it `input`.

```yaml
input:
  user: alice
  method: GET
  path: ["finance", "salary", "bob"]   # /finance/salary/bob
  headers:
  - JWT: …
```

Then you write boolean logic that decides whether or not the API call represented by that JSON/YAML data is authorized. For example, you could write conditions that authorize `bob` to see his own salary.

```rego
input.method = "GET"
input.path = ["finance", "salary", "bob"]
input.user = "bob"
```

Implicitly all of the conditions above are ANDed together. In Rego, logic like this doesn't stand on its own — it needs a name. Below we've given the logic the name `allow`.

```rego
allow {
    input.method = "GET"
    input.path = ["finance", "salary", "bob"]
    input.user = "bob"
}
```

Of course, you don't want to write one `allow` statement for every employee in the company. You want to use a variable so that the statement applies to everyone. In Rego, a variable is basically any symbol that's not a string or number, so to allow all employees to see their own salary, you replace `"bob"` with a variable like `employee`.

```rego
allow {
    input.method = "GET"
    input.path = ["finance", "salary", employee]
    input.user = employee
}
```

This statement says that `allow` is true if there is some value for the `employee` variable that makes all of the conditions true. Rego treats a set of conditions as a query and finds variable assignments that makes them all true. So while Rego has a syntax closer to programming languages than to SQL, it's really a query language underneath.

Now that you've let everyone see their own salary, you might decide to allow managers to see their subordinates' salaries. But to do that the Rego policy needs to know who manages whom, information that isn't included in the API call. In OPA, you make managerial data available by inserting it into the `data` namespace. (OPA has exactly two toplevel namespaces for JSON/YAML data: `data` and `input`.)

```yaml
data:
  manager:
    charlie: bob
    dave: bob
    bob: alice
  ...
```

Now you can write a second query also named `allow` as shown below. When you have multiple queries with the same name, they are ORed together.

```rego
allow {
    input.method = "GET"
    input.path = ["finance", "salary", employee]
    input.user = data.manager[employee]
}
```

This query looks up whether the user requesting an employee's salary is the manager of that employee using the `manager` dictionary. In addition to looking up key/value pairs in a dictionary, Rego lets you search through JSON data to find values and even cross-reference (or join) multiple JSON data sources during a search.

Additionally, Rego lets you make policy decisions that are more sophisticated than the allow/deny decisions shown above. You can make decisions that are numbers (e.g. rate-limits), strings (e.g. hostnames), arrays (e.g. servers), or dictionaries (microservice route-mappings). For more examples, see the [Open Policy Agent tutorials](http://www.openpolicyagent.org/docs/get-started.html).

The key takeaway is that Rego logic lets you write queries about multiple JSON/YAML data-sources to make a policy decision, and a decision can be a boolean, number, string, array, or even a dictionary.

## Rego is for Policy, not Programming

The goal of Rego is to help you tell software systems how to behave in the world by writing logic about (collections of) JSON/YAML data. Programming languages (e.g. C, Java, Go, Python) are the usual solution to this problem, but Rego was purpose-built to let you focus on just the data that represents the world and the logic that makes policy decisions about that data. Below we contrast policy and programming by showing what you SHOULD be thinking about when writing policy (logic and data) and what you SHOULD NOT be thinking about when writing policy (programming).

When writing policy about HTTP APIs…

- you SHOULD be thinking about whether the `method` is a `GET` or `POST`
- you SHOULD be thinking about whether the `employee` is requesting her own salary or someone else's.
- you SHOULD be thinking about whether the `manager` data has an entry stating that the `employee` requesting a salary is the manager of the person's salary in the request.

But…

- You SHOULD NOT be thinking about opening a network socket to retrieve the `manager` data.
- You SHOULD NOT be thinking about the object classes or class inheritance used to store the managerial data.
- You SHOULD NOT be thinking about which methods to use to access the fields of a class.
- You SHOULD NOT be deciding between different looping constructs (`for`, `do-until`, `while`, `iterators`, `recursion`) to search through the data or worrying about if those loops will terminate.
- You should NOT be thinking about a multitude of data structures and their subtle tradeoffs, like splay trees versus red-black trees.

Rego has some of the same primitives as programming languages for sharing common logic and coping with a large number of policies (modules and functions), but you're always thinking about exactly two things: logic and data. You're not thinking about sockets, object classes, method calls, non-terminating loops, or binary trees. You're thinking about logic and data.

## Wrap Up

Hopefully that helps shed some light on Rego and how OPA works for any domain and any layer of the stack. Here are the key takeaways:

- Rego lets you write policy about any domain and any layer of the stack: APIs, servers, risk-management, containers, networking.
- Rego makes you think about policy (logic and data), not programming (sockets, object classes, method calls, non-terminating loops, binary trees).
- Rego operates over JSON data. You can supply JSON data as input to every decision and as background information for making decisions.
- Rego logic is all queries. A query finds values for variables that make boolean conditions true.

For more information, check out the [Open Policy Agent](http://www.openpolicyagent.org/) project.

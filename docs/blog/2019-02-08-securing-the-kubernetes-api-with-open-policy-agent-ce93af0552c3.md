---
title: "Securing the Kubernetes API with Open Policy Agent"
authors: ["timhinrichs"]
date: 2019-02-08
slug: securing-the-kubernetes-api-with-open-policy-agent-ce93af0552c3
---

![Banner image for securing the Kubernetes API with OPA](/img/blog/securing-the-kubernetes-api-with-open-policy-agent-ce93af0552c3/banner.webp)

Kubernetes is being rolled out for production — it's mission critical. But it presents unique challenges around the age-old problem of who-can-do-what. Those challenges are exactly the ones the Open Policy Agent was designed to solve.

## TL;DR

This post highlights several key ideas:

- Controlling who-can-do-what on Kubernetes has unique challenges because to make an access control decision you need to inspect an arbitrary chunk of YAML, e.g. the images in all containers in all pods must come from a trusted repository.
- The Open Policy Agent was designed around the premise that sometimes you need to write and enforce access control decisions over arbitrary JSON/YAML, so it's a perfect match for Kubernetes's challenges.
- OPA supports a class of access control decisions called "context-aware" that enable you to make decisions based on the Kubernetes resources already in the cluster, e.g. no conflicting ingresses.

## KubeCon Seattle 2018 Debrief

After each KubeCon we try to document the answer to the question we heard most often while talking to folks at the Open Policy Agent (OPA) booth. For those who don't know, OPA is a general-purpose policy engine for the cloud-native stack and has been applied to solve policy and authorization problems in several different domains, e.g. microservice authorization, data protection, ssh/sudo control, terraform risk-analysis, and most popular at KubeCon this year: Kubernetes admission control. The most common question we heard was

> Why is OPA so well-suited for securing the Kubernetes API through admission control?

People heard about this use case in several different talks throughout the week, which is why we think so many people were asking about it at the booth. Here are links to those talks:

- [Securing Kubernetes With Admission Controllers from Dave Strebel (Microsoft](https://sched.co/GrZQ))
- [User Studies from Zach Abrahamson (Capital One) and Todd Ekenstam (Intuit)](https://sched.co/Grbn)
- [Liz Rice's keynote](https://youtu.be/McDzaTnUVWs?t=418)

In this post, when we talk about securing the Kubernetes, we're talking about the Kubernetes API itself — the container management system. We're talking about helping you, the Kubernetes cluster admin, put guardrails in place so that the developers running applications on top of Kubernetes don't need be constantly referring to wikis or PDFs that detail what policies the organization has decided on around Kubernetes. OPA lets you codify those wikis and PDF policies into policy-as-code and enforce them directly on the cluster. For example:

- every container image must come from a trusted, corporate repository
- every application exposed to the internet must use an approved domain name
- every resource must include a `costcenter` label
- business critical storage volumes must use the `retain` storage policy

One thing people sometimes mean when they say "Kubernetes" is the applications running on top of the Kubernetes container management system. That's another use case for OPA, but not the one covered in this post. Here are a few references if you're interested in using OPA to provide API security for cloud-native applications themselves (whether or not they run on Kubernetes).

- [How Netflix is Solving Authorization (with OPA), KubeCon Austin 2017](https://www.youtube.com/watch?v=R6tUNpRpdnY)
- [Implementing Authorization (for your app), KubeCon Shanghai 2018](https://sched.co/FuKQ)
- [OPA tutorial for HTTP-API authorization](https://www.openpolicyagent.org/docs/http-api-authorization.html)
- OPA integrations for [Java Spring](https://github.com/open-policy-agent/contrib/tree/master/spring_authz), [Istio/Envoy](https://github.com/open-policy-agent/opa-istio-plugin), [Linkerd](https://github.com/open-policy-agent/contrib/tree/master/linkerd_authz)

## The Kubernetes YAML-centric API

The key reason OPA is such a good choice for securing Kubernetes is that the Kubernetes API is pretty unique, and that presents challenges for authorization and API security. Within the community people refer to it as the Kubernetes Resource Model ([Brian Grant's doc](https://docs.google.com/document/d/1RmHXdLhNbyOWPW_AtnnowaRfGejw-qlKQIuLKQWlwzs/edit), [Tim Hockin tweet](https://twitter.com/thockin/status/1092091311572701184)).

Each Kubernetes API call requires you to specify the desired-state for one of Kubernetes's many objects: pods, services, ingresses, deployments, etc. For example, here is you define the desired state for an nginx workload.

```yaml
# nginx-pod.yaml
kind: Pod
apiVersion: v1
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  containers:
  - image: nginx
    name: nginx
```

To create this workload, you use `kubectl` and hand it the YAML file (`-f`) above.

```bash
kubectl create -f nginx-pod.yaml
```

Updates happen similarly. Say you want to change the version of `nginx`, mount an external volume, or provide additional configuration. You update the `nginx-pod.yaml` file to whatever the desired state should be and use `kubectl` again, this time using `apply` instead of `create`.

```bash
kubectl apply -f nginx-pod.yaml
```

## The Challenge of Securing the Kubernetes API

Imagine now that you want to require all images to come from a trusted repository (say, `hooli.com`). Anytime someone runs, say,`kubectl create`, the access control system needs to make a decision based on the user, the action `create` and the YAML that describes the pod, e.g.

```yaml
kind: Pod
metadata:
  labels:
    app: nginx
  name: nginx-1493591563-bvl8q
  namespace: production
spec:
  containers:
  - image: nginx
    name: nginx
    securityContext:
      privileged: true
  - image: hooli.com/frontend
    name: frontend
    securityContext:
      privileged: true
  dnsPolicy: ClusterFirst
  nodeName: minikube
  restartPolicy: Always
```

To make the right decision, the access control system needs to extract the list of image names (e.g. `nginx` and `hooli.com/frontend`) and do string manipulation to extract the name of the repository (e.g. the default repo and `hooli.com`). To complicate matters, Kubernetes supports Custom Resource Definitions, which means we can't just build an access control system that knows the layout of these YAML files. We need the access control system to be expressive enough for all of the following:

- Descending through the hierarchical structure of a YAML file.
- Iterating over elements in an array.
- Manipulating strings, IPs, numbers, etc.

## Securing the Kubernetes API with Open Policy Agent

This is where the Open Policy Agent shines. OPA was designed to express access control policies (as well as other kinds of policies) over arbitrary JSON/YAML, along with a complete toolkit for testing, dry-running, auditing, profiling, and integrating those policies into third party projects. The list of requirements from the last section are first-class citizens in OPA's policy language: dot-notation, iteration, and built-in functions. That means that encoding the policy that says, "all images must come from the repository `hooli.com`" is just a few lines in OPA.

```rego
# deny any pod with an image not from the repository hooli.com
deny {
  image_name := input.spec.containers[_].image
  not startswith(image_name, "hooli.com")
}
```

The logic shown above denies the API call if there is ANY container in the pod whose image fails to start with `hooli.com`. If you want to understand how the code works, the following notes should help:

- `input` is an OPA keyword that stores the JSON/YAML document representing the Kubernetes YAML shown earlier.
- The dot-notation (e.g. `input.spec.containers`) does the obvious thing — descending through the YAML hierarchy.
- The underscore (`_`) iterates over all the containers. If the body of the `deny` rule is true for ANY of the containers in the array, the pod is rejected. Note: iteration is not limited to only `_` — see the [docs](https://www.openpolicyagent.org/docs/how-do-i-write-policies.html#variable-keys) for details.
- `startswith` is one of [50+ builtins for string, numeric, IP, etc. manipulation](https://www.openpolicyagent.org/docs/language-reference.html).

## OPA's Context-aware Kubernetes Policies

That image-repository example is actually one of the simpler access control policies you might need to write for Kubernetes because you can make the decision using just the one YAML file describing the pod. But sometimes you need to know what other resources exist in the cluster to make an allow/deny decision. For example, it's possible to accidentally create two applications serving internet traffic using Kubernetes `ingresses` where one application steals traffic from the other. The policy that prevents that needs to compare a new ingress that's being created/updated with all of the existing ingresses. That leads to another requirement for a Kubernetes access control system that OPA supports:

- Conditioning decisions based on external information about the world.

To see the OPA policy that prohibits conflicting ingresses, here is an example `ingress` YAML.

```yaml
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - http:
      paths:
      - path: /testpath
        backend:
          serviceName: test
          servicePort: 80
```

Below is the (essence of the) OPA policy that stops an ingress from being created/updated if there is an _existing_ ingress that it would conflict with.

```rego
deny {
    input.kind == "Ingress"
    host := input.request.object.spec.rules[_].host
    host == data.kubernetes.ingresses[_][_].spec.rules[_].host
}
```

The only new part of this policy is the reference to `data.kubernetes`. Below are some notes explaining that code.

- `data` is a keyword in OPA (similar to `input`) that stores all of the information about the external world.
- The reference `data.kubernetes.ingresses` is a dictionary mapping each namespace into the array of ingresses in that namespace.
- `data.kubernetes.ingresses[_][_]` iterates over all ingresses over all namespaces.
- The last line checks if the `host` for the new/updated ingress is the same as the host on any of the rules over any of the ingresses in any namespace. (3 "any"s means you need 3 underscores.)

How you load information about the external world into OPA varies depending on the use case and the kind of data. Typically the information loaded into OPA is eventually-consistent, meaning it's a copy of the data and could be out of date — whether that matters depends entirely on the use case and can be mitigated by using OPA's offline auditing capabilities. To load Kubernetes data, OPA has a sidecar that watches the API server to replicate Kubernetes resources into OPA.

In this post, we've given 2 simple and common examples of policies using the core Kubernetes objects (pods and ingresses), but there's nothing special about those resources. If you're using Custom Resource Definitions, you can still go ahead and write whatever policies you need, e.g. [knative](https://github.com/knative) or [istio](https://github.com/istio). Any resource managed by Kubernetes is something you can write policy over with OPA — as far as OPA is concerned they're all just YAML/JSON.

## Summary

In this post, we dug into the API security challenges faced by Kubernetes and how OPA addresses those challenges.

- Kubernetes's API is YAML-centric, meaning that the arguments to API calls are (at least conceptually) arbitrary chunks of YAML.
- A YAML-centric API is challenging for access control because it requires analyzing that YAML to make a decision. For example, the policy "ensure all images come from a trusted repository" requires navigating the YAML to find the list of all containers, iterating over that list, extracting the image name, and string-parsing that image name to extract the repository.
- The Open Policy Agent's declarative policy language was designed to express policy over arbitrary JSON/YAML, so it includes implicit iteration, dot-notation, and 50+ builtins.
- OPA also supports _context-aware_ policies that let you analyze both the resource that a user is trying to create/update and all of the other Kubernetes resources that already exist. It's all just JSON/YAML to OPA. For example, the policy "prohibit ingresses with conflicting hostnames" requires comparing any new ingress that is being created to all the existing ingresses.

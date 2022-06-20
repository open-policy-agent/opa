---
title: Overview & Architecture
kind: kubernetes
weight: 1
---

In Kubernetes, [Admission
Controllers](https://kubernetes.io/docs/admin/admission-controllers/) enforce
policies on objects during create, update, and delete operations. Admission
control is fundamental to policy enforcement in Kubernetes.

For example, by deploying OPA as an admission controller you can:

* Require specific labels on all resources.
* Require container images come from the corporate image registry.
* Require all Pods specify resource requests and limits.
* Prevent conflicting Ingress objects from being created.

Admission controllers can also mutate incoming objects. By deploying OPA as a
mutating admission controller you can:

* Inject sidecar containers into Pods.
* Set specific annotations on all resources.
* Rewrite container images to point at the corporate image registry.
* Include node and pod (anti-)affinity selectors on Deployments.

These are just examples of policies you can enforce with admission controllers
and OPA. There are dozens of other policies you will want to enforce in your
Kubernetes clusters for security, cost, and availability reasons.

## What is OPA Gatekeeper?

[OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper) is a specialized
project providing first-class integration between OPA and Kubernetes. For
background information see this [blog
post](https://kubernetes.io/blog/2019/08/06/opa-gatekeeper-policy-and-governance-for-kubernetes)
on kubernetes.io.

OPA Gatekeeper adds the following on top of plain OPA:

* An extensible, parameterized policy library.
* Native Kubernetes CRDs for instantiating the policy library (aka "constraints").
* Native Kubernetes CRDs for extending the policy library (aka "constraint templates").
* Audit functionality.

If you want to kick the tires:

* See the [Installation
  Instructions](https://open-policy-agent.github.io/gatekeeper/website/docs/install/)
  in the README.
* See the
  [demo/basic](https://github.com/open-policy-agent/gatekeeper/tree/master/demo/basic)
  and
  [demo/agilebank](https://github.com/open-policy-agent/gatekeeper/tree/master/demo/agilebank)
  directories for examples policies and setup scripts.

**Recommendation**: OPA Gatekeeper is **the go-to project** for using OPA for
Kubernetes admission control. Plain OPA and Kube-mgmt (see below) are alternatives
that can be reached for if you want to use the management features of OPA, such as
status logs, decision logs, and bundles.

## How Does It Work With Plain OPA and Kube-mgmt?

The Kubernetes API Server is configured to query OPA for admission control
decisions when objects (e.g., Pods, Services, etc.) are created, updated, or
deleted.

{{< figure src="kubernetes-admission-flow.png" width="80" caption="Admission Control Flow" >}}

The API Server sends the entire Kubernetes object in the webhook request to OPA.
OPA evaluates the policies it has loaded using the admission review as `input`.
For example, the following policy denies objects that include container images
referring to illegal registries:

```live:container_image:module:openable
package kubernetes.admission

deny[reason] {
  some container
  input_containers[container]
  not startswith(container.image, "hooli.com/")
  reason := "container image refers to illegal registry (must be hooli.com)"
}

input_containers[container] {
  container := input.request.object.spec.containers[_]
}

input_containers[container] {
  container := input.request.object.spec.template.spec.containers[_]
}
```

When `deny` is evaluated with the input defined below the answer is:

```live:container_image:query:hidden
deny
```
```live:container_image:output
```

The `input` document contains the following fields:

* `input.request.kind` specifies the type of the object (e.g., `Pod`, `Service`,
  etc.)
* `input.request.operation` specifies the type of the operation, i.e., `CREATE`,
  `UPDATE`, `DELETE`, `CONNECT`.
* `input.request.userInfo` specifies the identity of the caller.
* `input.request.object` contains the entire Kubernetes object.
* `input.request.oldObject` specifies the previous version of the Kubernetes
  object on `UPDATE` and `DELETE`.

Here is an example of a Pod being created:

```live:container_image:input
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1",
  "request": {
    "kind": {
      "group": "",
      "version": "v1",
      "kind": "Pod"
    },
    "resource": {
      "group": "",
      "version": "v1",
      "resource": "pods"
    },
    "namespace": "opa-test",
    "operation": "CREATE",
    "userInfo": {
      "username": "system:serviceaccount:kube-system:replicaset-controller",
      "uid": "439dea65-3e4e-4fa8-b5f8-8fdc4bc7cf53",
      "groups": [
        "system:serviceaccounts",
        "system:serviceaccounts:kube-system",
        "system:authenticated"
      ]
    },
    "object": {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "creationTimestamp": "2019-08-13T16:01:54Z",
        "generateName": "nginx-7bb7cd8db5-",
        "labels": {
          "pod-template-hash": "7bb7cd8db5",
          "run": "nginx"
        },
        "name": "nginx-7bb7cd8db5-dbplk",
        "namespace": "opa-test",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "blockOwnerDeletion": true,
            "controller": true,
            "kind": "ReplicaSet",
            "name": "nginx-7bb7cd8db5",
            "uid": "7b6a307f-d9b4-4b65-a916-5d0b96305e87"
          }
        ],
        "uid": "266d2c8b-e43e-42d9-a19c-690bb6103900"
      },
      "spec": {
        "containers": [
          {
            "image": "nginx",
            "imagePullPolicy": "Always",
            "name": "nginx",
            "resources": {},
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "volumeMounts": [
              {
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
                "name": "default-token-6h4dn",
                "readOnly": true
              }
            ]
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "enableServiceLinks": true,
        "priority": 0,
        "restartPolicy": "Always",
        "schedulerName": "default-scheduler",
        "securityContext": {},
        "serviceAccount": "default",
        "serviceAccountName": "default",
        "terminationGracePeriodSeconds": 30,
        "tolerations": [
          {
            "effect": "NoExecute",
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "tolerationSeconds": 300
          },
          {
            "effect": "NoExecute",
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "tolerationSeconds": 300
          }
        ],
        "volumes": [
          {
            "name": "default-token-6h4dn",
            "secret": {
              "secretName": "default-token-6h4dn"
            }
          }
        ]
      },
      "status": {
        "phase": "Pending",
        "qosClass": "BestEffort"
      }
    },
    "oldObject": null
  }
}
```

The policies you give to OPA ultimately generate an admission review response
that is sent back to the API Server. Here is an example of the policy decision
sent back to the API Server.

```json
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1",
  "response": {
    "allowed": false,
    "status": {
      "message": "container image refers to illegal registry (must be hooli.com)"
    }
  }
}
```

> The API Server implements a "deny overrides" conflict resolution strategy. If
> any admission controller denies the request, the request is denied (even if
> one of the later admission controllers were to allow the request.)

Policies can be loaded into OPA dynamically via ConfigMap objects using the
[kube-mgmt](https://github.com/open-policy-agent/kube-mgmt) sidecar container.
The kube-mgmt sidecar container can also load any other Kubernetes object into
OPA as JSON under `data`. This lets you enforce policies that rely on an
eventually consistent snapshot of the Kubernetes cluster as context.

{{< figure src="kubernetes-watchers.png" width="60" caption="Policy and Data Caching" >}}

See the [Policy Authoring](../kubernetes-primer) and [Tutorial: Ingress
Validation](../kubernetes-tutorial) pages for more details.

## Additional Resources

See the following pages on [kubernetes.io](https://kubernetes.io) for more
information on admission control:

* [A Guide to Kubernetes Admission
  Controllers](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
  for a quick primer on admission controllers.
* [Dynamic Admission
  Control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
  for details on configuring external admission controllers.

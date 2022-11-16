---
title: Policy Primer via Examples
kind: kubernetes
weight: 2
---

Read this page if you are new to Kubernetes admission control with OPA and want
to learn how to write policies for Kubernetes.  It covers the version
that uses kube-mgmt. The [OPA Gatekeeper version](https://open-policy-agent.github.io/gatekeeper)
has its own docs.

## Writing Policies

To get started, let's look at a common policy: ensure all images come from a
trusted registry.

```live:container_images:module:openable
package kubernetes.admission                                                # line 1

deny[msg] {                                                                 # line 2
    input.request.kind.kind == "Pod"                                        # line 3
    image := input.request.object.spec.containers[_].image                  # line 4
    not startswith(image, "hooli.com/")                                     # line 5
    msg := sprintf("image '%v' comes from untrusted registry", [image])     # line 6
}
```

### Packages

In line 1 the `package kubernetes.admission` declaration gives the (hierarchical) name `kubernetes.admission` to the rules in the remainder of the policy.  The default installation of OPA as an admission controller assumes your rules are in the package `kubernetes.admission`.

### Deny Rules

For admission control, you write `deny` statements.  Order does not matter.  (OPA is far more flexible than this, but we recommend writing just `deny` statements to start.)  In line 2, the *head* of the rule `deny[msg]` says that the admission control request should be rejected and the user handed the error message `msg` if the conditions in the *body* (the statements between the `{}`) are true.

`deny` is the *set* of error messages that should be returned to the user.  Each rule you write adds to that set of error messages.

For example, suppose you tried to create the Pod below with nginx and mysql images.

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp
spec:
  containers:
  - image: nginx
    name: nginx-frontend
  - image: mysql
    name: mysql-backend
```

The admission review request to sent to OPA would look like this:

```live:container_images:input
{
  "kind": "AdmissionReview",
  "request": {
    "kind": {
      "kind": "Pod",
      "version": "v1"
    },
    "object": {
      "metadata": {
        "name": "myapp"
      },
      "spec": {
        "containers": [
          {
            "image": "nginx",
            "name": "nginx-frontend"
          },
          {
            "image": "mysql",
            "name": "mysql-backend"
          }
        ]
      }
    }
  }
}
```

When the `deny` rule is evaluated with the input above, the answer is:

```live:container_images:query:hidden
```

```live:container_images:output
```

### Input Document

In OPA, `input` is a reserved, global variable whose value is the  Kubernetes AdmissionReview object that the API server hands to any admission control webhook.

AdmissionReview objects have many fields.  The rule above uses `input.request.kind`, which includes the usual group/version/kind information.  The rule also uses `input.request.object`, which is the YAML that the user provided to `kubectl` (augmented with defaults, timestamps, etc.).  The full `input` object is 50+ lines of YAML, so below we show just the relevant parts.

```yaml
apiVersion: admission.k8s.io/v1
kind: AdmissionReview
request:
  kind:
    group:
    kind: Pod
    version: v1
  object:
    metadata:
      name: myapp
    spec:
      containers:
        - image: nginx
          name: nginx-frontend
        - image: mysql
          name: mysql-backend
```

### Dot Notation

In line 3 `input.request.kind.kind == "Pod"`, the expression `input.request.kind.kind` does the obvious thing: it descends through the YAML hierarchy.  The dot (.) operator never throws any errors; if the path does not exist the value of the expression is `undefined`.

```live:container_images/kind:query:merge_down
input.request.kind
```
```live:container_images/kind:output:merge_down
```
```live:container_images/kind/kind:query:merge_down
input.request.kind.kind
```
```live:container_images/kind/kind:output:merge_down
```
```live:container_images/spec:query:merge_down
input.request.object.spec.containers
```
```live:container_images/spec:output
```

### Equality

Lines 3, 4, 6 all use a form of equality.  There are 3 forms of equality in OPA.

* `x := 7` declares a local variable `x` and assigns it a value of 7.  The compiler throws an error if `x` already has a value.
* `x == 7` returns true if `x` has a value of 7.  The compiler throws an error if `x` has no value.
* `x = 7` either assigns the value 7 to `x` if `x` has no value or compares `x`'s value to 7 if it has a value.  The compiler never throws an error.

The recommendation for rule-writing is to use `:=` and `==` wherever possible.  Rules written with `:=` and `==` are easier to write and to read.  `=` is invaluable in more advanced use cases, and outside of rules is the only supported form of equality.

### Arrays

Lines 4-5 find images in the Pod that don't come from the trusted registry.  To do that, they use the `[]` operator, which does what you expect: index into the array.

Continuing the example from earlier:

```live:container_images/arrays:query:merge_down
input.request.object.spec.containers[0]
```
```live:container_images/arrays:output:merge_down
```
```live:container_images/arrays/image:query:merge_down
input.request.object.spec.containers[0].image
```
```live:container_images/arrays/image:output
```

The `[]` operators let you use variables to index into the array as well.

```live:container_images/arrays/vars:query:merge_down
i := 0; input.request.object.spec.containers[i]
```
```live:container_images/arrays/vars:output
```

### Iteration

The containers array has an unknown number of elements, so to implement an image registry check you need to iterate over them.  Iteration in OPA requires no new syntax.  In fact, OPA is always iterating--it's always searching for all variable assignments that make the conditions in the rule true. It's just that sometimes the search is so easy people don't think of it as iteration/search.

To iterate over the indexes in the `input.request.object.spec.containers` array, you just put a variable that has no value in for the index.  OPA will do what it always does: find values for that variable that make the conditions true.

OPA detects when there will be multiple answers and displays all the results in a table.

```live:container_images/iteration:query:merge_down
some j; input.request.object.spec.containers[j]
```
```live:container_images/iteration:output
```

Often you don't want to invent new variable names for iteration.  OPA provides the special anonymous variable `_` for exactly that reason.  So in line (4) `image := input.request.object.spec.containers[_].image` finds all the images in the containers array and assigns each to the `image` variable one at a time.

### Builtins

On line 5 the *builtin* `startswith` checks if one string is a prefix of the other.  The builtin `sprintf` on line 6 formats a string with arguments.  OPA has 150+ builtins detailed in [the Policy Reference](../policy-reference/#built-in-functions).
Builtins let you analyze and manipulate:

* Numbers, Strings, Regexs, Networks
* Aggregates, Arrays, Sets
* Types
* Encodings (base64, YAML, JSON, URL, JWT)
* Time

## Testing Policies

When you write policies, you should use the OPA unit-test framework *before* sending the policies out into the OPA that is running on your cluster.  The debugging process will be much quicker and effective.  Here's an example test for the policy from the last section.

```live:container_images/test:module:read_only,openable
package kubernetes.test_admission                         # line 1

import data.kubernetes.admission                          # line 2

test_image_safety {                                       # line 3
  unsafe_image := {                                       # line 4
    "request": {
      "kind": {"kind": "Pod"},
      "object": {
        "spec": {
          "containers": [
            {"image": "hooli.com/nginx"},
            {"image": "busybox"}
          ]
        }
      }
    }
  }
  expected := "image 'busybox' comes from untrusted registry"
  admission.deny[expected] with input as unsafe_image     # line 5
}
```

**Different Package**. On line 1 the `package` directive puts these tests in a different package than admission control policy itself.  This is the recommended best practice.

**Import**.  On line 2 `import data.kubernetes.admission` allows us to reference the admission control policy using the name `admission` everywhere in the test package.  `import` is not strictly necessary--it simply sets up an alias; you could instead reference `data.kubernetes.admission` inside the rules.

**Unit Test**.  On line 3 `test_image_safety` defines a unittest.  If the rule evaluates to true the test passes; otherwise it fails.  When you use the OPA test runner, anything in any package starting with `test` is treated as a test.

**Assignment**. On line 4 `unsafe_image` is the input we want to use for the test.  Ideally this would be a real AdmissionReview object, though those are so long that in this example we hand-rolled a partial input.

**Dot for packages**.  On line 5 we use the Dot operator on a package.  `admission.deny[expected]` runs the `deny` rule(s) in package `admission` and checks if the message is contained in the set defined by `deny`.

**Test Input**.  Also on line 5 the stanza `with input as unsafe_image` sets the value of `input` to be `unsafe_image` while evaluating `admission.deny[expected]`.

**Running Tests**. If you've created the files *image-safety.rego* and *test-image-safety.rego* in the current directory then you run the tests by naming the files explicitly as shown below or by handing the `opa test` command the directory (and subdirectories) of files to load: `opa test .`

```
$ opa test image-safety.rego test-image-safety.rego
PASS: 1/1
```

## Using Context in Policies

The image-repository example shows an example where you can make a policy decision using just the one JSON/YAML file describing the resource in question. But sometimes you need to know what other resources exist in the cluster to make an allow/deny decision.

For example, it’s possible to accidentally configure two Kubernetes ingresses so that one steals traffic from the other. The policy that prevents conflicting ingresses needs to compare the ingress that’s being created/updated with all of the existing ingresses.  Just knowing the new/updated ingress isn't enough information to make an allow/deny decision.

Below is a partial example of the input OPA sees when someone creates an ingress.  To avoid conflicts, we want to prevent two ingresses from having the same `request.object.spec.rules.host`.  If OPA has only this one ingress configuration it doesn't have enough information to make an allow/deny decision; it also needs the configurations for all of the existing ingresses.

```yaml
apiVersion: admission.k8s.io/v1
kind: AdmissionReview
request:
  kind:
    group: networking.k8s.io
    kind: Ingress
    version: v1
  object:
    metadata:
      name: prod
    spec:
      rules:
      - host: initech.com
        http:
          paths:
          - path: /finance
            pathType: Prefix
            backend:
              service:
                name: banking
                port:
                  number: 443
```

To avoid conflicting ingresses, you write a policy like the one that follows.

```live:ingress_conflicts:module:read_only
package kubernetes.admission

deny[msg] {
  some namespace, name
  input.request.kind.kind == "Ingress"                                            # line 1
  newhost := input.request.object.spec.rules[_].host                              # line 2
  oldhost := data.kubernetes.ingresses[namespace][name].spec.rules[_].host        # line 3
  newhost == oldhost                                                              # line 4
  input.request.object.metadata.namespace != namespace                            # line 5
  input.request.object.metadata.name != name                                      # line 6
  msg := sprintf("ingress host conflicts with ingress %v/%v", [namespace, name])  # line 7
}
```
The first part of the rule you already understand:
* Line (1) checks if the `input` is an Ingress
* Line (2) iterates over all the rules in the `input` ingress and looks up the `host` field for each of its rules.

**Existing K8s Resources** Line (3) iterates over ingresses that already exist in Kubernetes. `data` is a global variable where (among other things) OPA has a record of the current resources inside Kubernetes.  The line `oldhost := data.kubernetes.ingresses[namespace][name].spec.rules[_].host` finds all ingresses in all namespaces, iterates over all the `rules` inside each of those and assigns the `host` field to the variable `oldhost`.  Whenever `newhost == oldhost`, there's a conflict, and the OPA rule includes an appropriate error message into the `deny` set.

In this case the rule uses explicit variable names `namespace` and `name` for iteration so that it can use those variables again when constructing the error message in line (7).

**Schema Differences**.  Both `input` and `data.kubernetes.ingresses[namespace][name]` represent ingresses, but they do it differently.

* `input` is a Kubernetes AdmissionReview object.  It includes several fields in addition to the Kubernetes Ingress object itself.
* `data.kubernetes.ingresses[namespace][name]` is a native Kubernetes Ingress object as returned by the API.

Here are two examples.

<div>
<div style="float: left; width: 49%; padding: 3px">
<center><b>data.kubernetes.ingresses[namespace][name]</b></center>
<pre><code>
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: prod
spec:
  rules:
  - host: initech.com
    http:
      paths:
      - path: /finance
        pathType: Prefix
        backend:
          service:
            name: banking
            port:
              number: 443
</code></pre></div>
<div style="float: left; width: 49%; padding 3px;">
<center><b>input</b></center>
<pre><code>
apiVersion: admission.k8s.io/v1
kind: AdmissionReview
request:
  kind:
    group: networking.k8s.io
    kind: Ingress
    version: v1
  operation: CREATE
  userInfo:
    groups:
    username: alice
  object:
    metadata:
      name: prod
    spec:
      rules:
      - host: initech.com
        http:
          paths:
          - path: /finance
            pathType: Prefix
            backend:
              service:
                name: banking
                port:
                  number: 443
</code></pre></div>
</div>

## Detailed Admission Control Flow

This section provides a detailed explanation of the admission control flow
introduced in the [Introduction](../kubernetes-introduction) page.

It starts with someone (or something) running `kubectl` (or sending a request to
the API server.) For example, a user might run `kubectl create -f pod.yaml`:

**pod.yaml**:

```yaml
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

When the request reaches the API server it's authenticated and authorized and
processed by the admission controllers. When the API server's Webhook admission
controller executes, the API server sends a webhook request to OPA containing an
**AdmissionReview** object.

**AdmissionReview**:

```yaml
apiVersion: admission.k8s.io/v1
kind: AdmissionReview
request:
  kind:
    group: ''
    kind: Pod
    version: v1
  namespace: opa
  object:
    metadata:
      creationTimestamp: '2018-10-27T02:12:20Z'
      labels:
        app: nginx
      name: nginx
      namespace: opa
      uid: bbfee96d-d98d-11e8-b280-080027868e77
    spec:
      containers:
      - image: nginx
        imagePullPolicy: Always
        name: nginx
        resources: {}
        terminationMessagePath: "/dev/termination-log"
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: "/var/run/secrets/kubernetes.io/serviceaccount"
          name: default-token-tm9v8
          readOnly: true
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: default
      serviceAccountName: default
      terminationGracePeriodSeconds: 30
      tolerations:
      - effect: NoExecute
        key: node.kubernetes.io/not-ready
        operator: Exists
        tolerationSeconds: 300
      - effect: NoExecute
        key: node.kubernetes.io/unreachable
        operator: Exists
        tolerationSeconds: 300
      volumes:
      - name: default-token-tm9v8
        secret:
          secretName: default-token-tm9v8
    status:
      phase: Pending
      qosClass: BestEffort
  oldObject:
  operation: CREATE
  resource:
    group: ''
    resource: pods
    version: v1
  uid: 8d836dfd-e0c0-4490-93ba-85ed4a04261e
  userInfo:
    groups:
    - system:masters
    - system:authenticated
    username: minikube-user
```

Typically the API server is configured (via `ValidatingWebhookConfiguration` or
`MutatingWebhookConfiguration` objects) to query OPA without providing the name
of a decision. For example:

```http
POST / HTTP/1.1
Content-Type: application/json
```

```json
{
  "apiVersion": "admission.k8s.io/v1",
  "kind": "AdmissionReview",
  "request": ...
}
```

When OPA receives the webhook request, it binds the payload to the `input`
document and generates the default decision: `system.main`. The `system.main`
decision is defined by a rule that evaluates all of the admission control
policies that have been loaded into OPA.

As the administrator responsible for deploying OPA, you have full control over
the `system.main` decision (i.e., it is just another Rego policy.) A basic
implementation of the `system.main` policy simply evaluates all deny rules that
have been loaded into OPA and unions the results:

```live:admission_main:module:read_only
package system

import data.kubernetes.admission

main := {
  "apiVersion": "admission.k8s.io/v1",
  "kind": "AdmissionReview",
  "response": response,
}

default uid := ""

uid := input.request.uid

response := {
    "allowed": false,
    "uid": uid,
    "status": {
        "message": reason,
    },
} {
    reason := concat(", ", admission.deny)
    reason != ""
}

else := {"allowed": true, "uid": uid}
```

The `system.main` policy MUST generate an **AdmissionReview** object containing
a response that the API server can interpret. If the request should be allowed,
the `response.allowed` field should be true. Otherwise, the `response.allowed`
field should be set to `false` and the `response.status.message` field should be
set to include an error message that indicates why the request is being
rejected. The error message will be returned to the API server caller (e.g., the
user running `kubectl`). Often the error message is the concatenation of all the
messages in the `deny` set defined above.

For example, with the input and Image Registry Safety examples above, the
response from OPA would be:

```yaml
apiVersion: admission.k8s.io/v1
kind: AdmissionReview
response:
  uid: 8d836dfd-e0c0-4490-93ba-85ed4a04261e
  allowed: false
  status:
    message: "image fails to come from trusted registry: nginx"
```

For more detail on how Kubernetes Admission Control works, see [this blog
post](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
on kubernetes.io.

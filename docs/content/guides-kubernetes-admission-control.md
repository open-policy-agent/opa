---
title: "Guides: Kubernetes Admission Control"
navtitle: Kubernetes Admission Control
kind: guides
weight: 2
---

In Kubernetes, Admission Controllers enforce semantic validation of objects during create, update, and delete operations. With OPA you can enforce custom policies on Kubernetes objects without recompiling or reconfiguring the Kubernetes API server or even Kubernetes Admission Controllers.

This primer assumes you, the Kubernetes administrator, have already installed OPA as a validating admission controller on Kubernetes as described in the [Kubernetes Admission Control Tutorial](../kubernetes-admission-control).  And now you are at the point where you want to write your own policies.

OPA was designed to write policies over arbitrary JSON/YAML.  It does NOT have built-in concepts like pods, deployments, or services.  OPA just sees the JSON/YAML sent by Kubernetes API server and allows you to write whatever policy you want to make a decision.  You as the policy-author know the semantics--what that JSON/YAML represents.

## Example Policy: Image Registry Safety

To get started, let's look at a common policy: ensure all images come from a trusted registry.

```
1: package kubernetes.admission
2: deny[msg] {
3:     input.request.kind.kind == "Pod"
4:     image := input.request.object.spec.containers[_].image
5:     not startswith(image, "hooli.com")
6:     msg := sprintf("image fails to come from trusted registry: %v", [image])
7: }
```
**Policies and Packages**.
In line 1 the `package kubernetes.admission` declaration gives the (hierarchical) name `kubernetes.admission` to the rules in the remainder of the policy.  The default installation of OPA as an admission controller assumes your rules are in the package `kubernetes.admission`.

**Deny Rules**.  For admission control, you write `deny` statements.  Order does not matter.  (OPA is far more flexible than this, but we recommend writing just `deny` statements to start.)  In line 2, the *head* of the rule `deny[msg]` says that the admission control request should be rejected and the user handed the error message `msg` if the conditions in the *body* (the statements between the `{}`) are true.

`deny` is the *set* of error messages that should be returned to the user.  Each rule you write adds to that set of error messages.

For example, suppose you tried to create the Pod below with nginx and mysql images.

```
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

`deny` evaluates to the following set of messages.

```
[
  "image fails to come from trusted registry: nginx",
  "image fails to come from trusted registry: mysql"
]
```

<!--
i = {
  "request": {
    "kind": {"kind": "Pod"},
    "object": {"spec": {"containers": [
      {"image": "nginx"},
      {"image": "mysql"}]}}}}

deny with input as i
-->

**Input**  In OPA, `input` is a reserved, global variable whose value is the  Kubernetes AdmissionReview object that the API server hands to any admission control webhook.

AdmissionReview objects have many fields.  The rule above uses `input.request.kind`, which includes the usual group/version/kind information.  The rule also uses `input.request.object`, which is the YAML that the user provided to `kubectl` (augmented with defaults, timestamps, etc.).  The full `input` object is 50+ lines of YAML, so below we show just the relevant parts.

```yaml
apiVersion: admission.k8s.io/v1beta1
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

**Dot notation**  In line 3 `input.request.kind.kind == "Pod"`, the expression `input.request.kind.kind` does the obvious thing: it descends through the YAML hierarchy.  The dot (.) operator never throws any errors; if the path does not exist the value of the expression is `undefined`.

<!--
{
  "apiVersion": "admission.k8s.io/v1beta1",
  "kind": "AdmissionReview",
  "request": {
    "kind": {
 "group": null,
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
},{
"image": "mysql",
"name": "mysql-backend"
}]}}}}
-->
You can see OPA's evaluation in the REPL.

```
> input.request.kind
{
  "group": null,
  "kind": "Pod",
  "version": "v1"
}
> input.request.kind.kind
"Pod"
> input.request.object.spec.containers
[
  {
    "image": "nginx",
    "name": "nginx-frontend"
  },
  {
    "image": "mysql",
    "name": "mysql-backend"
  }
]
```

**Equality**. Lines 3,4,6 all use a form of equality.  There are 3 forms of equality in OPA.

* `x := 7` declares a local variable `x` and assigns variable `x` to the value 7.  The compiler throws an error if `x` already has a value.
* `x == 7` returns true if `x`'s value is 7.  The compiler throws an error if `x` has no value.
* `x = 7` either assigns `x` to 7 if `x` has no value or compares `x`'s value to 7 if it has a value.  The compiler never throws an error.

The recommendation for rule-writing is to use `:=` and `==` wherever possible.  Rules written with `:=` and `==` are easier to write and to read.  `=` is invaluable in more advanced use cases, and outside of rules is the only supported form of equality.

**Arrays**.  Lines 4-5 find images in the Pod that don't come from the trusted registry.  To do that, they use the `[]` operator, which does what you expect: index into the array.

Continuing the example from earlier:

```
> input.request.object.spec.containers[0]
{
  "image": "nginx",
  "name": "nginx-frontend"
}
> input.request.object.spec.containers[0].image
"nginx"
```

The `[]` operators let you use variables to index into the array as well.

```
> i := 0
> input.request.object.spec.containers[i]
{
  "image": "nginx",
  "name": "nginx-frontend"
}
```

**Iteration** The containers array has an unknown number of elements, so to implement an image registry check you need to iterate over them.  Iteration in OPA requires no new syntax.  In fact, OPA is always iterating--it's always searching for all variable assignments that make the conditions in the rule true. It's just that sometimes the search is so easy people don't think of it as iteration/search.

To iterate over the indexes in the `input.request.object.spec.containers` array, you just put a variable that has no value in for the index.  OPA will do what it always does: find values for that variable that make the conditions true.

In the REPL, OPA detects when there will be multiple answers and displays all the results in a table.

```
> input.request.object.spec.containers[j]
+---+-------------------------------------------+
| j |  input.request.object.spec.containers[j]  |
+---+-------------------------------------------+
| 0 | {"image":"nginx","name":"nginx-frontend"} |
| 1 | {"image":"mysql","name":"mysql-backend"}  |
+---+-------------------------------------------+
```

Often you don't want to invent new variable names for iteration.  OPA provides the special anonymous variable `_` for exactly that reason.  So in line (4) `image := input.request.object.spec.containers[_].image` finds all the images in the containers array and assigns each to the `image` variable one at a time.

**Builtins**.  On line 5 the *builtin* `startswith` checks if one string is a prefix of the other.  The builtin `sprintf` on line 6 formats a string with arguments.  OPA has 50+ builtins detailed at [openpolicyagent.org/docs/language-reference](../language-reference).
Builtins let you analyze and manipulate:

* Numbers, Strings, Regexs, Networks
* Aggregates, Arrays, Sets
* Types
* Encodings (base64, YAML, JSON, URL, JWT)
* Time



## Unit Testing Policies
When you write policies, you should use the OPA unit-test framework *before* sending the policies out into the OPA that is running on your cluster.  The debugging process will be much quicker and effective.  Here's an example test for the policy from the last section.

```
 1: package kubernetes.test_admission
 2: import data.kubernetes.admission
 3:
 4: test_image_safety {
 5:   unsafe_image := {"request": {
 6:       "kind": {"kind": "Pod"},
 7:       "object": {"spec": {"containers": [
 8:           {"image": "hooli.com/nginx"},
 9:           {"image": "busybox"}]}}}}
10:   count(admission.deny) == 1 with input as unsafe_image
11: }
```

**Different Package**. On line 1 the `package` directive puts these tests in a different package than admission control policy itself.  This is the recommended best practice.

**Import**.  On line 2 `import data.kubernetes.admission` allows us to reference the admission control policy using the name `admission` everwhere in the test package.  `import` is not strictly necessary--it simply sets up an alias; you could instead reference `data.kubernetes.admission` inside the rules.

**Unit Test**.  On line 4 `test_image_safety` defines a unittest.  If the rule evaluates to true the test passes; otherwise it fails.  When you use the OPA test runner, anything in any package starting with `test` is treated as a test.

**Assignment**. On line 5 `unsafe_image` is the input we want to use for the test.  Ideally this would be a real AdmissionReview object, though those are so long that in this example we hand-rolled a partial input.

**Dot for packages**.  On line 11 we use the Dot operator on a package.  `admission.deny` runs (all) the `deny` rule(s) in package `admission` (and all other `deny` rules in the `admission` package).


**Test Input**.  Also on line 11 the stanza `with input as unsafe_image` sets the value of `input` to be `unsafe_image` while evaluating `count(admission.deny) == 1`.

**Running Tests**. If you've created the files *image-safety.rego* and *test-image-safety.rego* in the current directory then you run the tests by naming the files explicitly as shown below or by handing the `opa test` command the directory (and subdirectories) of files to load: `opa test .`

```
$ opa test image-safety.rego test-image-safety.rego
PASS: 1/1
```

## External Resources: Ingress Conflicts
The image-repository example shows an example where you can make a policy decision using just the one JSON/YAML file describing the resource in question. But sometimes you need to know what other resources exist in the cluster to make an allow/deny decision.

For example, it’s possible to accidentally configure two Kubernetes ingresses so that one steals traffic from the other. The policy that prevents conflicting ingresses needs to compare the ingress that’s being created/updated with all of the existing ingresses.  Just knowing the new/updated ingress isn't enough information to make an allow/deny decision.

Below is a partial example of the input OPA sees when someone creates an ingress.  To avoid conflicts, we want to prevent two ingresses from having the same `request.object.spec.rules.host`.  If OPA has only this one ingress configuration it doesn't have enough information to make an allow/deny decision; it also needs the configurations for all of the existing ingresses.

```yaml
apiVersion: admission.k8s.io/v1beta1
kind: AdmissionReview
request:
  kind:
    group: extensions
    kind: Ingress
    version: v1beta1
  object:
    metadata:
      name: prod
    spec:
      rules:
      - host: initech.com
        http:
          paths:
          - path: /finance
            backend:
              serviceName: banking
              servicePort: 443
```

To avoid conflicting ingresses, you write a policy like the one that follows.

```
1: package kubernetes.admission
2: deny[msg] {
3:     input.request.kind.kind == "Ingress"
4:     newhost := input.request.object.spec.rules[_].host
5:     oldhost := data.kubernetes.ingresses[namespace][name].spec.rules[_].host
6:     newhost == oldhost
7:     msg := sprintf("ingress host conflicts with ingress %v/%v", [namespace, name])
8: }
```
The first part of the rule you already understand:
* Line (3) checks if the `input` is an Ingress
* Line (4) iterates over all the rules in the `input` ingress and looks up the `host` field for each of its rules.

**Existing K8s Resources** Line (5) iterates over ingresses that already exist in Kubernetes. `data` is a global variable where (among other things) OPA has a record of the current resources inside Kubernetes.  The line `oldhost := data.kubernetes.ingresses[namespace][name].spec.rules[_].host` finds all ingresses in all namespaces, iterates over all the `rules` inside each of those and assigns the `host` field to the variable `oldhost`.  Whenever `newhost == oldhost`, there's a conflict, and the OPA rule includes an appropriate error message into the `deny` set.

In this case the rule uses explicit variable names `namespace` and `name` for iteration so that it can use those variables again when constructing the error message in line 7.

**Schema Differences**.  Both `input` and `data.kubernetes.ingresses[namespace][name]` represent ingresses, but they do it differently.

* `input` is a K8s AdmissionReview object.  It includes several fields in addition to the K8s Ingress object itself.
* `data.kubernetes.ingresses[namespace][name]` is a native Kubernetes Ingress object as returned by the API.

Here are two examples.

<div>
<div style="float: left; width: 49%; padding: 3px">
<center><b>data.kubernetes.ingresses[namespace][name]</b></center>
<pre><code>
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: prod
spec:
  rules:
  - host: initech.com
    http:
      paths:
      - path: /finance
        backend:
          serviceName: banking
          servicePort: 443
</code></pre></div>
<div style="float: left; width: 49%; padding 3px;">
<center><b>input</b></center>
<pre><code>
apiVersion: admission.k8s.io/v1beta1
kind: AdmissionReview
request:
  kind:
    group: extensions
    kind: Ingress
    version: v1beta1
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
            backend:
              serviceName: banking
              servicePort: 443
</code></pre></div>


<br>

## Admission Control Flow

Here is a sample of the flow of information from the user to the API server to
OPA and back.

It starts with someone (or something) running `kubectl` (or sending a request to
the API server.) For example, a user might run `kubkectl create -f pod.yaml`:

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
apiVersion: admission.k8s.io/v1beta1
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
  uid: bbfeef88-d98d-11e8-b280-080027868e77
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
  "apiVersion": "admission.k8s.io/v1beta1",
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

```ruby
package system

import data.kubernetes.admission

main = {
  "apiVersion": "admission.k8s.io/v1beta1",
  "kind": "AdmissionReview",
  "response": response,
}

default response = {"allowed": true}

response = {
    "allowed": false,
    "status": {
        "reason": reason,
    },
} {
    reason = concat(", ", admission.deny)
    reason != ""
}
```

The `system.main` policy MUST generate an **AdmissionReview** object containing
a response that the API server can interpret. If the request should be allowed,
the `response.allowed` field should be true. Otherwise, the `response.allowed`
field should be set to `false` and the `response.status.reason` field should be
set to include an error message that indicates why the request is being
rejected. The error message will be returned to the API server caller (e.g., the
user running `kubectl`). Often the error message is the concatenation of all the
messages in the `deny` set defined above.

For example, with the input and Image Registry Safety examples above, the
response from OPA would be:

```yaml
apiVersion: admission.k8s.io/v1beta1
kind: AdmissionReview
response:
  allowed: false
  status:
    reason: "image fails to come from trusted registry: nginx"
```

For more detail on how Kubernetes Admission Control works, see [this blog
post](https://kubernetes.io/blog/2019/03/21/a-guide-to-kubernetes-admission-controllers/)
on kubernetes.io.

## Debugging Tips

If you run into problems getting OPA to enforce admission control policies in
Kubernetes there are a few things you can check to make sure everything is
configured correctly. If none of these tips work, feel free to join
[slack.openpolicyagent.org](https://slack.openpolicyagent.org) and ask for help.

### Check for the `openpolicyagent.org/policy-status` annotation on ConfigMaps containing policies

If you are loading policies into OPA via
[kube-mgmt](https://github.com/open-policy-agent/kube-mgmt) you can check the
`openpolicyagent.org/policy-status` annotation on ConfigMaps that contain your
policies. The annotation should be set to `"ok"` if the policy was loaded
successfully. If errors occured during loading (e.g., because the policy
contained a syntax error) the cause will be reported here.

If the annotation is
missing entirely, check the `kube-mgmt` container logs for connection errors
between the container and the Kubernetes API server.

### Check the `kube-mgmt` container logs for error messages

When `kube-mgmt` is healthy, the container logs will be quiet/empty. If you are
trying to enforce policies based on Kubernetes context (e.g., to check for
ingress conflicts) then you need to make sure that `kube-mgmt` can replicate
Kubernetes objects into OPA. If `kube-mgmt` is unable to list/watch resources in
the Kubernetes API server, they will not be replicated into OPA and the policy
will not get enforced.

### Check the `opa` container logs for TLS errors

Communication between the Kubernetes API server and OPA is secured with TLS. If
the CA bundle specified in the webhook configuration is out-of-sync with the
server certificate that OPA is configured with, OPA will log errors indicating a
TLS issue. Verify that the CA bundle specified in the validating or mutating
webhook configurations matches the server certificate you configured OPA to use.

### Check for POST requests in the `opa` container logs

When the Kubernetes API server queries OPA for admission control decisions, it
sends HTTP `POST` requests. If there are no `POST` requests contained in the
`opa` container logs, it indicates that the webhook configuration is wrong or
there is a network connectivity problem between the Kubernetes API server and
OPA.

* If you have access to the Kubernetes API server logs, review them to see if
  they indicate the cause.
* If you are running on AWS EKS make sure your security group settings allow
  traffic from Kubernetes "master" nodes to the node(s) where OPA is running.

### Ensure the webhook is configured for the proper namespaces

When you create the webhook according to the installation instructions,
it includes a namespaceSelector so that you
can decide which namespaces to ignore.

```
    namespaceSelector:
      matchExpressions:
      - key: openpolicyagent.org/webhook
        operator: NotIn
        values:
        - ignore
```

If OPA seems to not be making the decisions you expect, check if the namespace
is using the label `openpolicyagent.org/webhook: ignore`.

If OPA is making decision on namespaces (like `kube-system`) that you would
prefer OPA would ignore, assign the namespace the label
`openpolicyagent.org/webhook: ignore`.

### Ensure mutating policies construct JSON Patches correctly

If you are using OPA to enforce mutating admission policies you must ensure the
JSON Patch objects you generate escape "/" characters in the JSON Pointer. For
example, if you are generating a JSON Patch that sets annotations like
`acmecorp.com/myannotation` you need to escape the "/" character in the
annotation name using `~1` (per [RFC
6901](https://tools.ietf.org/html/rfc6901#section-3)).

**Correct**:

```json
{
   "op": "add",
   "path": "/metadata/annotations/acmecorp.com~1myannotation",
   "value": "somevalue"
}
```

**Incorrect**:


```json
{
   "op": "add",
   "path": "/metadata/annotations/acmecorp.com/myannotation",
   "value": "somevalue"
}
```

In addition, when your policy generates the response for the Kubernetes API
server, you must use the `base64.encode` built-in function to encode the JSON
Patch objects. DO NOT use the `base64url.encode` function because the Kubernetes
API server will not process it:

**Correct**:

```ruby
main = {
	"apiVersion": "admission.k8s.io/v1beta1",
	"kind": "AdmissionReview",
	"response": response,
}

response = {
  "allowed": true,
  "patchType": "JSONPatch",
  "patch": base64.encode(json.marshal(patches))   # <-- GOOD: uses base64.encode
}

patches = [
  {
    "op": "add",
    "path": "/metadata/annotations/acmecorp.com~1myannotation",
    "value": "somevalue"
  }
]
```

**Incorrect**:

```ruby
main = {
	"apiVersion": "admission.k8s.io/v1beta1",
	"kind": "AdmissionReview",
	"response": response,
}

response = {
  "allowed": true,
  "patchType": "JSONPatch",
  "patch": base64url.encode(json.marshal(patches))   # <-- BAD: uses base64url.encode
}

patches = [
  {
    "op": "add",
    "path": "/metadata/annotations/acmecorp.com~1myannotation",
    "value": "somevalue"
  }
]
```

Also, for more examples of how to construct mutating policies and integrating
them with validating policies, see [these
examples](https://github.com/open-policy-agent/library/tree/master/kubernetes/mutating-admission)
in https://github.com/open-policy-agent/library.
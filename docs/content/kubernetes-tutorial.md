---
title: "Tutorial: Ingress Validation"
kind: kubernetes
weight: 10
---

This tutorial shows how to deploy OPA as an admission controller from scratch.
It covers the OPA-kubernetes version that uses kube-mgmt.
The [OPA Gatekeeper version](https://open-policy-agent.github.io/gatekeeper) has its own docs.
For the purpose of the tutorial we will deploy two policies that ensure:

- Ingress hostnames must be on allowlist on the Namespace containing the Ingress.
- Two ingresses in different namespaces must not have the same hostname.

> 💡 Kubernetes does not guarantee consistency across resources. If two
> ingresses are created in parallel, there is no guarantee that OPA (or any
> other admission controller) will observe the creation of one ingress before
> the other. This means that it's not possible to enforce these policies during
> admission control 100% of the time. There will be a small window of time
> (usually on the order of milliseconds) when the eventually consistent cache
> inside of OPA (or any other admission controller) is out-of-date. To catch
> these violations we recommend you periodically audit the state of the cluster
> against your policies. Offline auditing is one of the features provided by the
> [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper) project.

## Prerequisites

This tutorial requires Kubernetes 1.20 or later. To run the tutorial locally ensure you start a cluster with Kubernetes
version 1.20+, we recommend using [minikube](https://kubernetes.io/docs/getting-started-guides/minikube) or
[KIND](https://kind.sigs.k8s.io/).

## Steps

### 1. Enable recommended Kubernetes Admission Controllers

To implement admission control rules that validate Kubernetes resources during create, update, and delete operations,
you must enable the [ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook)
when the Kubernetes API server is started. The ValidatingAdmissionWebhook admission controller is included in
the [recommended set of admission controllers to enable](https://kubernetes.io/docs/admin/admission-controllers/#is-there-a-recommended-set-of-admission-controllers-to-use)

Start minikube:

```bash
minikube start
```

Make sure that the minikube ingress addon is enabled:

```bash
minikube addons enable ingress
```

### 2. Create a new Namespace to deploy OPA into

```bash
kubectl create namespace opa
```

Configure `kubectl` to use this namespace:

```bash
kubectl config set-context opa-tutorial --user minikube --cluster minikube --namespace opa
kubectl config use-context opa-tutorial
```

### 3. Create TLS credentials for OPA

Communication between Kubernetes and OPA must be secured using TLS. To configure TLS, use `openssl` to create a
certificate authority (CA) and certificate/key pair for OPA:

```bash
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -sha256 -key ca.key -days 100000 -out ca.crt -subj "/CN=admission_ca"
```

Generate the TLS key and certificate for OPA:

```bash
cat >server.conf <<EOF
[ req ]
prompt = no
req_extensions = v3_ext
distinguished_name = dn

[ dn ]
CN = opa.opa.svc

[ v3_ext ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = DNS:opa.opa.svc,DNS:opa.opa.svc.cluster,DNS:opa.opa.svc.cluster.local
EOF
```

```bash
openssl genrsa -out server.key 2048
openssl req -new -key server.key -sha256 -out server.csr -extensions v3_ext -config server.conf
openssl x509 -req -in server.csr -sha256 -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 100000 -extensions v3_ext -extfile server.conf
```

> Note: the Common Name value and Subject Alternative Name you give to openssl MUST match the name of the OPA service created below.

Create a Secret to store the TLS credentials for OPA:

```bash
kubectl create secret tls opa-server --cert=server.crt --key=server.key --namespace opa
```

### 4. Define OPA policy

Let's define a couple of policies to test admission control. First create a new folder to store our policies:

```bash
mkdir policies && cd policies
```

#### Policy 1: Restrict Hostnames

Create a policy that restricts the hostnames that an ingress can use. Only hostnames matching the specified regular
expressions will be allowed.

**ingress-allowlist.rego**:

```live:ingress_allowlist:module:read_only
package kubernetes.admission

import data.kubernetes.namespaces

operations := {"CREATE", "UPDATE"}

deny[msg] {
	input.request.kind.kind == "Ingress"
	operations[input.request.operation]
	host := input.request.object.spec.rules[_].host
	not fqdn_matches_any(host, valid_ingress_hosts)
	msg := sprintf("invalid ingress host %q", [host])
}

valid_ingress_hosts := {host |
	allowlist := namespaces[input.request.namespace].metadata.annotations["ingress-allowlist"]
	hosts := split(allowlist, ",")
	host := hosts[_]
}

fqdn_matches_any(str, patterns) {
	fqdn_matches(str, patterns[_])
}

fqdn_matches(str, pattern) {
	pattern_parts := split(pattern, ".")
	pattern_parts[0] == "*"
	suffix := trim(pattern, "*.")
	endswith(str, suffix)
}

fqdn_matches(str, pattern) {
    not contains(pattern, "*")
    str == pattern
}
```

#### Policy 2: Prohibit Hostname Conflicts

Now let's define another policy to test admission control. The following policy prevents Ingress objects in different
namespaces from sharing the same hostname.

**ingress-conflicts.rego**:

```live:ingress_conflicts:module:read_only
package kubernetes.admission

import data.kubernetes.ingresses

deny[msg] {
    some other_ns, other_ingress
    input.request.kind.kind == "Ingress"
    input.request.operation == "CREATE"
    host := input.request.object.spec.rules[_].host
    ingress := ingresses[other_ns][other_ingress]
    other_ns != input.request.namespace
    ingress.spec.rules[_].host == host
    msg := sprintf("invalid ingress host %q (conflicts with %v/%v)", [host, other_ns, other_ingress])
}
```

#### Combine Policies

Let's define a main policy that imports the [Restrict Hostnames](#policy-1-restrict-hostnames) and
[Prohibit Hostname Conflicts](#policy-2-prohibit-hostname-conflicts) policies and provides an overall policy decision.

**main.rego**:

```live:main:module:read_only
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
    reason = concat(", ", admission.deny)
    reason != ""
}

else := {"allowed": true, "uid": uid}
```

> ⚠️ When OPA receives a request, it executes a query against the document defined `data.system.main` by default.

### 5. Build and Publish OPA Bundle

Build an OPA bundle containing policies defined in the previous step. In our setup, OPA will download policies from the
bundle service and the `kube-mgmt` container will load Kubernetes resources into OPA. Since we load policy and data into
OPA from multiple sources, we need to scope the bundle to a subset of OPA’s policy and data cache by defining a manifest.
More information about this can be found [here](../management-bundles#multiple-sources-of-policy-and-data). Run the
following commands in the `policies` folder created in the previous step.

```bash
cat > .manifest <<EOF
{
    "roots": ["kubernetes/admission", "system"]
}
EOF
```

```bash
opa build -b .
```

We will now serve the OPA bundle using Nginx.

```bash
docker run --rm --name bundle-server -d -p 8888:80 -v ${PWD}:/usr/share/nginx/html:ro nginx:latest
```

### 6. Deploy OPA as an Admission Controller

Next, use the file below to deploy OPA as an admission controller.

**`admission-controller.yaml`**:

```
# Grant OPA/kube-mgmt read-only access to resources. This lets kube-mgmt
# replicate resources into OPA so they can be used in policies.
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: opa-viewer
roleRef:
  kind: ClusterRole
  name: view
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: Group
  name: system:serviceaccounts:opa
  apiGroup: rbac.authorization.k8s.io
---
# Define role for OPA/kube-mgmt to update configmaps with policy status.
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: opa
  name: configmap-modifier
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["update", "patch"]
---
# Grant OPA/kube-mgmt role defined above.
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: opa
  name: opa-configmap-modifier
roleRef:
  kind: Role
  name: configmap-modifier
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: Group
  name: system:serviceaccounts:opa
  apiGroup: rbac.authorization.k8s.io
---
kind: Service
apiVersion: v1
metadata:
  name: opa
  namespace: opa
spec:
  selector:
    app: opa
  ports:
  - name: https
    protocol: TCP
    port: 443
    targetPort: 8443
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: opa
  namespace: opa
  name: opa
spec:
  replicas: 1
  selector:
    matchLabels:
      app: opa
  template:
    metadata:
      labels:
        app: opa
      name: opa
    spec:
      containers:
        # WARNING: OPA is NOT running with an authorization policy configured. This
        # means that clients can read and write policies in OPA. If you are
        # deploying OPA in an insecure environment, be sure to configure
        # authentication and authorization on the daemon. See the Security page for
        # details: https://www.openpolicyagent.org/docs/security.html.
        - name: opa
          image: openpolicyagent/opa:{{< current_docker_version >}}
          args:
            - "run"
            - "--server"
            - "--tls-cert-file=/certs/tls.crt"
            - "--tls-private-key-file=/certs/tls.key"
            - "--addr=0.0.0.0:8443"
            - "--addr=http://127.0.0.1:8181"
            - "--set=services.default.url=http://host.minikube.internal:8888"
            - "--set=bundles.default.resource=bundle.tar.gz"
            - "--log-format=json-pretty"
            - "--set=status.console=true"
            - "--set=decision_logs.console=true"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-server
          readinessProbe:
            httpGet:
              path: /health?plugins&bundle
              scheme: HTTPS
              port: 8443
            initialDelaySeconds: 3
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /health
              scheme: HTTPS
              port: 8443
            initialDelaySeconds: 3
            periodSeconds: 5
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:2.0.1
          args:
            - "--replicate-cluster=v1/namespaces"
            - "--replicate=networking.k8s.io/v1/ingresses"
      volumes:
        - name: opa-server
          secret:
            secretName: opa-server
```

> ⚠️ If using `kind` to run a local Kubernetes cluster, the bundle service URL should be `http://host.docker.internal:8888`.

```bash
kubectl apply -f admission-controller.yaml
```

When OPA starts, the `kube-mgmt` container will load Kubernetes Namespace and Ingress objects into OPA. You can
configure the sidecar to load any kind of Kubernetes object into OPA. The sidecar establishes watches on the
Kubernetes API server so that OPA has access to an eventually consistent cache of Kubernetes objects.

Next, generate the manifest that will be used to register OPA as an admission controller.  This webhook will ignore
any namespace with the label `openpolicyagent.org/webhook=ignore`.

```bash
cat > webhook-configuration.yaml <<EOF
kind: ValidatingWebhookConfiguration
apiVersion: admissionregistration.k8s.io/v1
metadata:
  name: opa-validating-webhook
webhooks:
  - name: validating-webhook.openpolicyagent.org
    namespaceSelector:
      matchExpressions:
      - key: openpolicyagent.org/webhook
        operator: NotIn
        values:
        - ignore
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["*"]
        apiVersions: ["*"]
        resources: ["*"]
    clientConfig:
      caBundle: $(cat ca.crt | base64 | tr -d '\n')
      service:
        namespace: opa
        name: opa
    admissionReviewVersions: ["v1"]
    sideEffects: None
EOF
```

The generated configuration file includes a base64 encoded representation of the CA certificate created in [Step 3](#3-create-tls-credentials-for-opa)
so that TLS connections can be established between the Kubernetes API server and OPA.

Next label `kube-system` and the `opa` namespace so that OPA does not control the resources in those namespaces.

```bash
kubectl label ns kube-system openpolicyagent.org/webhook=ignore
kubectl label ns opa openpolicyagent.org/webhook=ignore
```

Finally, register OPA as an admission controller:

```bash
kubectl apply -f webhook-configuration.yaml
```

You can follow the OPA logs to see the webhook requests being issued by the Kubernetes API server:

```
# ctrl-c to exit
kubectl logs -l app=opa -c opa -f
```

### 7. Exercise Restrict Hostnames policy

Now let's exercise the [Restrict Hostnames](#policy-1-restrict-hostnames) policy by creating two new namespaces.

**qa-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-allowlist: "*.qa.acmecorp.com,*.internal.acmecorp.com"
  name: qa
```

**production-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-allowlist: "*.acmecorp.com"
  name: production
```

```bash
kubectl create -f qa-namespace.yaml
kubectl create -f production-namespace.yaml
```

Next, define two Ingress objects. One of the Ingress objects will be permitted
and the other will be rejected.

**ingress-ok.yaml**:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-ok
spec:
  rules:
  - host: signin.acmecorp.com
    http:
      paths:
      - pathType: ImplementationSpecific
        path: /
        backend:
          service:
            name: nginx
            port:
              number: 80
```

**ingress-bad.yaml**:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-bad
spec:
  rules:
  - host: acmecorp.com
    http:
      paths:
      - pathType: ImplementationSpecific
        path: /
        backend:
          service:
            name: nginx
            port:
              number: 80
```

Finally, try to create both Ingress objects:

```bash
kubectl create -f ingress-ok.yaml -n production
kubectl create -f ingress-bad.yaml -n qa
```

The second Ingress is rejected because its hostname does not match the allowlist in the `qa` namespace.

It will report an error as follows:

```
Error from server: error when creating "ingress-bad.yaml": admission webhook "validating-webhook.openpolicyagent.org"
denied the request: invalid ingress host "acmecorp.com"
```

### 8. Exercise Prohibit Hostname Conflicts policy

Test the [Prohibit Hostname Conflicts](#policy-2-prohibit-hostname-conflicts) policy by verifying that you cannot
create an Ingress in another namespace with the same hostname as the one created earlier.

**staging-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-allowlist: "*.acmecorp.com"
  name: staging
```

```bash
kubectl create -f staging-namespace.yaml
```

```bash
kubectl create -f ingress-ok.yaml -n staging
```

The above command will report an error as follows:

```
Error from server (BadRequest): error when creating "ingress-ok.yaml": admission webhook
"validate.nginx.ingress.kubernetes.io" denied the request: host "signin.acmecorp.com" and
path "/" is already defined in ingress production/ingress-ok
```

## Wrap Up

Congratulations for finishing the tutorial!

This tutorial showed how you can leverage OPA to enforce admission control
decisions in Kubernetes clusters without modifying or recompiling any
Kubernetes components. Furthermore, with OPA's [Bundle](../management-bundles) feature policies can be
periodically downloaded from remote servers to satisfy changing operational requirements.

For more information about deploying OPA on top of Kubernetes, see
[Deployments - Kubernetes](../deployments#kubernetes).

---
title: "Tutorial: Ingress Validation"
kind: kubernetes
weight: 10
---

This tutorial shows how to deploy OPA as an admission controller from scratch.
It covers the OPA-kubernetes version that uses kube-mgmt.  The [OPA Gatekeeper version](https://github.com/open-policy-agent/gatekeeper) has its own docs.
For the purpose of the tutorial we will deploy two policies that ensure:

- Ingress hostnames must be whitelisted on the Namespace containing the Ingress.
- Two ingresses in different namespaces must not have the same hostname.

## Prerequisites

This tutorial requires Kubernetes 1.13 or later. To run the tutorial locally ensure you start a cluster with Kubernetes version 1.13+, we recommend using [minikube](https://kubernetes.io/docs/getting-started-guides/minikube) or [KIND](https://kind.sigs.k8s.io/).

## Steps

### 1. Start Kubernetes recommended Admisson Controllers enabled

To implement admission control rules that validate Kubernetes resources during create, update, and delete operations, you must enable the [ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook) when the Kubernetes API server is started. The ValidatingAdmissionWebhook admission controller is included in the [recommended set of admission controllers to enable](https://kubernetes.io/docs/admin/admission-controllers/#is-there-a-recommended-set-of-admission-controllers-to-use)

Start minikube:

```bash
minikube start
```

Make sure that the minikube ingress addon is enabled:

```bash
minikube addons enable ingress
```

### 2. Create a new Namespace to deploy OPA into

When OPA is deployed on top of Kubernetes, policies are automatically loaded out of ConfigMaps in the `opa` namespace.

```bash
kubectl create namespace opa
```

Configure `kubectl` to use this namespace:

```bash
kubectl config set-context opa-tutorial --user minikube --cluster minikube --namespace opa
kubectl config use-context opa-tutorial
```

### 3. Deploy OPA on top of Kubernetes

Communication between Kubernetes and OPA must be secured using TLS. To configure TLS, use `openssl` to create a certificate authority (CA) and certificate/key pair for OPA:

```bash
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -days 100000 -out ca.crt -subj "/CN=admission_ca"
```

Generate the TLS key and certificate for OPA:

```bash
cat >server.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
EOF
```

```bash
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=opa.opa.svc" -config server.conf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 100000 -extensions v3_req -extfile server.conf
```

> Note: the Common Name value you give to openssl MUST match the name of the OPA service created below.

Create a Secret to store the TLS credentials for OPA:

```bash
kubectl create secret tls opa-server --cert=server.crt --key=server.key
```

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
    targetPort: 443
---
apiVersion: apiVersion: apps/v1
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
            - "--addr=0.0.0.0:443"
            - "--addr=http://127.0.0.1:8181"
            - "--log-format=json-pretty"
            - "--set=decision_logs.console=true"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-server
          readinessProbe:
            httpGet:
              path: /health
              scheme: HTTPS
              port: 443
            initialDelaySeconds: 3
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /health
              scheme: HTTPS
              port: 443
            initialDelaySeconds: 3
            periodSeconds: 5
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.8
          args:
            - "--replicate-cluster=v1/namespaces"
            - "--replicate=extensions/v1beta1/ingresses"
      volumes:
        - name: opa-server
          secret:
            secretName: opa-server
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: opa-default-system-main
  namespace: opa
data:
  main: |
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

```bash
kubectl apply -f admission-controller.yaml
```

When OPA starts, the `kube-mgmt` container will load Kubernetes Namespace and Ingress objects into OPA. You can configure the sidecar to load any kind of Kubernetes object into OPA. The sidecar establishes watches on the Kubernetes API server so that OPA has access to an eventually consistent cache of Kubernetes objects.

Next, generate the manifest that will be used to register OPA as an admission controller.  This webhook will ignore any namespace with the label `openpolicyagent.org/webhook=ignore`.

```bash
cat > webhook-configuration.yaml <<EOF
kind: ValidatingWebhookConfiguration
apiVersion: admissionregistration.k8s.io/v1beta1
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
EOF
```

The generated configuration file includes a base64 encoded representation of the CA certificate so that TLS connections can be established between the Kubernetes API server and OPA.

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

### 4. Define a policy and load it into OPA via Kubernetes

To test admission control, create a policy that restricts the hostnames that an ingress can use.

**ingress-whitelist.rego**:

```live:ingress_whitelist:module:read_only
package kubernetes.admission

import data.kubernetes.namespaces

operations = {"CREATE", "UPDATE"}

deny[msg] {
	input.request.kind.kind == "Ingress"
	operations[input.request.operation]
	host := input.request.object.spec.rules[_].host
	not fqdn_matches_any(host, valid_ingress_hosts)
	msg := sprintf("invalid ingress host %q", [host])
}

valid_ingress_hosts = {host |
	whitelist := namespaces[input.request.namespace].metadata.annotations["ingress-whitelist"]
	hosts := split(whitelist, ",")
	host := hosts[_]
}

fqdn_matches_any(str, patterns) {
	fqdn_matches(str, patterns[_])
}

fqdn_matches(str, pattern) {
	pattern_parts := split(pattern, ".")
	pattern_parts[0] == "*"
	str_parts := split(str, ".")
	n_pattern_parts := count(pattern_parts)
	n_str_parts := count(str_parts)
	suffix := trim(pattern, "*.")
	endswith(str, suffix)
}

fqdn_matches(str, pattern) {
    not contains(pattern, "*")
    str == pattern
}
```

Store the policy in Kubernetes as a ConfigMap. By default kube-mgmt will try to load policies out of configmaps in the opa namespace OR configmaps in other namespaces labelled openpolicyagent.org/policy=rego.

```bash
kubectl create configmap ingress-whitelist --from-file=ingress-whitelist.rego
```

The OPA sidecar will notice the ConfigMap and automatically load the policy into OPA.

### 5. Exercise the policy

Create two new namespaces to test the Ingress policy.

**qa-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-whitelist: "*.qa.acmecorp.com,*.internal.acmecorp.com"
  name: qa
```

**production-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-whitelist: "*.acmecorp.com"
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
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: ingress-ok
spec:
  rules:
  - host: signin.acmecorp.com
    http:
      paths:
      - backend:
          serviceName: nginx
          servicePort: 80
```

**ingress-bad.yaml**:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: ingress-bad
spec:
  rules:
  - host: acmecorp.com
    http:
      paths:
      - backend:
          serviceName: nginx
          servicePort: 80
```

Finally, try to create both Ingress objects:

```bash
kubectl create -f ingress-ok.yaml -n production
kubectl create -f ingress-bad.yaml -n qa
```

The second Ingress is rejected because its hostname does not match the whitelist in the `qa` namespace.

### 6. Modify the policy and exercise the changes

OPA allows you to modify policies on-the-fly without recompiling any of the services that offload policy decisions to it.

To enforce the second half of the policy from the start of this tutorial you can load another policy into OPA that prevents Ingress objects in different namespaces from sharing the same hostname.

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

```bash
kubectl create configmap ingress-conflicts --from-file=ingress-conflicts.rego
```

The OPA sidecar annotates ConfigMaps containing policies to indicate if they
were installed successfully. Verify that the ConfigMap was installed successfully:

```
kubectl get configmap ingress-conflicts -o yaml
```

Test that you cannot create an Ingress in another namespace with the same hostname as the one created earlier.

**staging-namespace.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    ingress-whitelist: "*.acmecorp.com"
  name: staging
```

```bash
kubectl create -f staging-namespace.yaml
```

```bash
kubectl create -f ingress-ok.yaml -n staging
```

## Wrap Up

Congratulations for finishing the tutorial!

This tutorial showed how you can leverage OPA to enforce admission control
decisions in Kubernetes clusters without modifying or recompiling any
Kubernetes components. Furthermore, once Kubernetes is configured to use OPA as
an External Admission Controller, policies can be modified on-the-fly to
satisfy changing operational requirements.

For more information about deploying OPA on top of Kubernetes, see
[Deployments - Kubernetes](../deployments#kubernetes).

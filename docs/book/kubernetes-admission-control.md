# Kubernetes Admission Control

In Kubernetes, [Admission Controllers](https://kubernetes.io/docs/admin/admission-controllers/) enforce semantic validation of objects during create, update, and delete operations. With OPA you can enforce custom policies on Kubernetes objects without recompiling or reconfiguring the Kubernetes API server.

## Goals

This tutorial shows how to enforce custom policies on Kubernetes objects using OPA. In this tutorial, you will define admission control rules that prevent users from creating Kubernetes Ingress objects that violate the following organization policy:

- Ingress hostnames must be whitelisted on the Namespace containing the Ingress.
- Two ingresses in different namespaces must not have the same hostname.

## Prerequisites

This tutorial requires Kubernetes 1.9 or later. To run the tutorial locally, we recommend using [minikube](https://kubernetes.io/docs/getting-started-guides/minikube) in version `v0.28+` with Kubernetes 1.10 (which is the default).

## Steps

### 1. Start Kubernetes recommended Admisson Controllers enabled

To implement admission control rules that validate Kubernetes resources during create, update, and delete operations, you must enable the [ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook) when the Kubernetes API server is started. the admission controller is included in the [recommended set of admission controllers to enable](https://kubernetes.io/docs/admin/admission-controllers/#is-there-a-recommended-set-of-admission-controllers-to-use)

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

**[admission-controller.yaml](https://github.com/open-policy-agent/opa/blob/master/docs/book/tutorials/kubernetes-admission-control-validation/admission-controller.yaml)**:
<pre><code class="lang-yaml">{% include "./tutorials/kubernetes-admission-control-validation/admission-controller.yaml" %}</code></pre>

```bash
kubectl apply -f admission-controller.yaml
```

When OPA starts, the `kube-mgmt` container will load Kubernetes Namespace and Ingress objects into OPA. You can configure the sidecar to load any kind of Kubernetes object into OPA. The sidecar establishes watches on the Kubernetes API server so that OPA has access to an eventually consistent cache of Kubernetes objects.

Next, generate the manifest that will be used to register OPA as an admission controller:

```bash
cat > webhook-configuration.yaml <<EOF
kind: ValidatingWebhookConfiguration
apiVersion: admissionregistration.k8s.io/v1beta1
metadata:
  name: opa-validating-webhook
webhooks:
  - name: validating-webhook.openpolicyagent.org
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

The generated configuration file inclues a base64 encoded representation of the CA certificate so that TLS connections can be established between the Kubernetes API server and OPA.

Finally, register OPA as and admission controller:

```bash
kubectl apply -f webhook-configuration.yaml
```

You can follow the OPA logs to see the webhook requests being issued by the Kubernetes API server:

```
kubectl logs -l app=opa -c opa
```

### 4. Define a policy and load it into OPA via Kubernetes

To test admission control, create a policy that restricts the hostnames that an ingress can use ([ingress-whitelist.rego](https://github.com/open-policy-agent/opa/blob/master/docs/book/tutorials/kubernetes-admission-control-validation/ingress-whitelist.rego)):

<pre><code class="lang-ruby">{% include "./tutorials/kubernetes-admission-control-validation/ingress-whitelist.rego" %}</code></pre>

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

The second Ingress is rejected because it's hostname does not match the whitelist in the `qa` namespace.

### 6. Modify the policy and exercise the changes

OPA allows you to modify policies on-the-fly without recompiling any of the services that offload policy decisions to it.

To enforce the second half of the policy from the start of this tutorial you can load another policy into OPA that rejects Ingress objects in different namespaces from sharing the same hostname.

[ingress-conflicts.rego](https://github.com/open-policy-agent/opa/blob/master/docs/book/tutorials/kubernetes-admission-control-validation/ingress-conflicts.rego):

<pre><code class="lang-ruby">{% include "./tutorials/kubernetes-admission-control-validation/ingress-conflicts.rego" %}</code></pre>

```bash
kubectl create configmap ingress-conflicts --from-file=ingress-conflicts.rego
```

The OPA sidecar annotates ConfigMaps containing policies to indicate if they
were installed successfully. Verify the ConfigMap was installed successfully.

```
kubectl get configmap ingress-conflicts -o yaml
```

Test that you cannot create an Ingress in another namespace with the same hostname as the one created earlier.

**staging-namespace.yaml**:

```
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
[Deployments - Kubernetes](/deployments.md#kubernetes).

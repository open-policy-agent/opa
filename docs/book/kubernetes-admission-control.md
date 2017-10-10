# Kubernetes Admission Control

Kubernetes [Admission Controllers](https://kubernetes.io/docs/admin/admission-controllers/) perform *semantic validation* of resources during create, update, and delete operations. In Kubernetes 1.7, you can use OPA to enforce custom policies without recompiling or reconfiguring the Kubernetes API server by leveraging [External Admission Webhooks](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks).

## Goals

This tutorial shows how to use OPA to enforce custom policies on resources in
Kubernetes. For the purpose of this tutorial, you will define a policy that
prevents users from running `kubectl exec` on "privileged" containers in the
"production" namespace.

## Prerequisites

This tutorial requires a Kubernetes 1.7 (or later) cluster. To test Kubernetes locally, we recommend using [minikube](https://kubernetes.io/docs/getting-started-guides/minikube/). Keep in mind that External Admission Webhook support in Kubernetes is currently in **alpha**.

## Steps

### 1. Start Kubernetes with External Admission Webhooks enabled

External Admission Controllers must be secured with TLS. See [Generating TLS Certificates](https://github.com/open-policy-agent/kube-mgmt#generating-tls-certificates) for steps to generate a CA and client/server credentials for test purposes.

Start minikube with the `GenericAdmissionWebhook` enabled and include the
client-side credentials via command line arguments.

```bash
# Set to directory containing generated TLS certificates.
CERT_DIR=<path/to/directory/containing/certificates>

# Set to admission controllers to include. This example uses the default set of
# admission controllers enabled in the Kubernetes API server plus the
# GenericAdmissionWebhook admission controller.
ADMISSION_CONTROLLERS=NamespaceLifecycle,LimitRanger,ServiceAccount,PersistentVolumeLabel,DefaultStorageClass,GenericAdmissionWebhook,ResourceQuota,DefaultTolerationSeconds

minikube start --kubernetes-version v1.7.0 \
    --extra-config=apiserver.Admission.PluginNames=$ADMISSION_CONTROLLERS
    --extra-config=apiserver.ProxyClientCertFile=$CERT_DIR/client.crt
    --extra-config=apiserver.ProxyClientKeyFile=$CERT_DIR/client.key
```

> `minikube` automatically mounts your home directory into the VM. Storing the
> TLS certificates under your home directory makes them easy to reference in the
> `--extra-config` arguments.

### 1. Deploy OPA on top of Kubernetes

First, create a namespace to deploy OPA into.

```bash
kubectl create namespace opa
```

Create a `kubectl` context for this namespace.

```bash
kubectl config set-context opa-tutorial --user minikube --cluster minikube --namespace opa
kubectl config use-context opa-tutorial
```

Create a Service to expose the OPA API. The Kubernetes API server will lookup
the Service and execute webhook requests against it.

**opa-admission-controller-service.yaml**:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: opa
spec:
  clusterIP: 10.0.0.222
  selector:
    app: opa
  ports:
  - name: https
    protocol: TCP
    port: 443
    targetPort: 443
```

```bash
kubectl create -f opa-admission-controller-service.yaml
```

The Service's `clusterIP` must match the subjectAltName in the server-side TLS
credentials.

Next, create Secrets containing the TLS credentials for OPA:

```bash
kubectl create secret generic opa-ca --from-file=ca.crt
kubectl create secret tls opa-server --cert=server.crt --key=server.key
```

Finally, create the Deployment to run OPA as an Admission Controller.

**opa-admission-controller-deployment.yaml**:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: opa
  name: opa
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: opa
      name: opa
    spec:
      containers:
        - name: opa
          image: openpolicyagent/opa:0.5.9
          args:
            - "run"
            - "--server"
            - "--tls-cert-file=/certs/tls.crt"
            - "--tls-private-key-file=/certs/tls.key"
            - "--addr=0.0.0.0:443"
            - "--insecure-addr=127.0.0.1:8181"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-server
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.4
          args:
            - "--replicate=v1/pods"
            - "--register-admission-controller"
            - "--admission-controller-ca-cert-file=/certs/ca.crt"
            - "--admission-controller-service-name=opa"
            - "--admission-controller-service-namespace=$(MY_POD_NAMESPACE)"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-ca
          env:
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
      volumes:
        - name: opa-server
          secret:
            secretName: opa-server
        - name: opa-ca
          secret:
            secretName: opa-ca
```

```bash
kubectl create -f opa-admission-controller-deployment.yaml
```

When OPA starts, the sidecar (`kube-mgmt`) will register it as an External
Admission Controller. To verify that registration succeeded, run `kubectl proxy`
in another terminal and then query the Kubernetes API for the list of External
Admission Controllers.

```bash
curl localhost:8001/apis/admissionregistration.k8s.io/v1alpha1/externaladmissionhookconfigurations
```

Finally, you can follow the OPA logs to see the webhook requests being issued
by the Kubernetes API server:

```
kubectl logs -l app=opa -c opa
```

### 2. Define a policy and load it into OPA via Kubernetes

To test admission control, create a policy that restricts exec access on
privileged pods:

**privileged-exec.rego**:

```ruby
package system

import data.kubernetes.break_glass

main = {
    "apiVersion": "admission.k8s.io/v1alpha1",
    "kind": "AdmissionReview",
    "status": status,
}

default status = {"allowed": true}

status = {
	"allowed": false,
	"status": {
		"reason": reason,
	},
} {
    concat(", ", blacklist, reason)
    reason != ""
}

blacklist["cannot exec into privileged container in production namespace"] {
    input.spec.operation = "CONNECT"
    input.spec.namespace = "production"
    is_privileged(input.spec.namespace, input.spec.name, true)
	not break_glass
}

is_privileged(namespace, name) {
    pod = data.kubernetes.pods[namespace][name]
    container = pod.spec.containers[_]
    container.securityContext.privileged
}
```

Store the policy in Kubernetes as a ConfigMap.

```bash
kubectl create configmap privileged-exec --from-file=privileged-exec.rego
```

The OPA sidecar will notice the ConfigMap and automatically load the contained
policy into OPA.

### 3. Exercise the policy

To verify that your policy is working, create separate test Pods in the `production` namespace.

**nginx-pod.yaml**:

```yaml
kind: Pod
version: v1
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  containers:
  - image: nginx
    name: nginx
```

**nginx-privileged-pod.yaml**:

```yaml
kind: Pod
version: v1
metadata:
  name: nginx-privileged
  labels:
    app: nginx
spec:
  containers:
  - image: nginx
    name: nginx
    securityContext:
      privileged: true
```

```bash
kubectl create namespace production
kubectl -n production create -f nginx-pod.yaml
kubectl -n production create -f nginx-privileged-pod.yaml
```

Verify that you can exec into non-privileged container:

```
kubectl -n production exec -i -t nginx bash
```

Verify that you cannot exec into the privileged container:

```
kubectl -n production exec -i -t nginx-privileged bash
```

### 4. Modify the policy and exercise the changes

OPA allows you to modify policies on-the-fly without recompiling any of the
services that offload policy decisions to it.

The original policy you created denies `kubectl exec` access unless the
`data.kubernetes.break_glass` value is *defined* (and not false). If you create
a new policy that defines this value, `kubectl exec` access will be granted.

**break-glass.rego**:

```ruby
package kubernetes

break_glass = true
```

```bash
kubectl create configmap break-glass --from-file=break-glass.rego
```

The OPA sidecar annotates ConfigMaps containing policies to indicate if they
were installed successfully. Verify the ConfigMap was installed successfully.

```
kubectl get configmap break-glass -o yaml
```

Test that `kubectl exec` access has been granted.

```bash
kubectl -n production exec -i -t nginx-privileged bash
```

Finally, delete the `break-glass` policy now that calm has been restored.

```bash
kubectl delete configmap break-glass
```

`kubectl exec` will no longer be able to access the privileged Pod in the production.

## Wrap Up

Congratulations for finishing the tutorial!

This tutorial showed how you can leverage OPA to enforce admission control
decisions in Kubernetes clusters without modifying or recompiling any
Kubernetes components. Furthermore, once Kubernetes is configured to use OPA as
an External Admission Controller, policies can be modified on-the-fly to
satisfy changing operational requirements.

For more information about deploying OPA on top of Kubernetes, see
[Deployments - Kubernetes](/deployments.md#kubernetes).

# Ceph Authorization

Ceph is a highly scalable distributed storage solution that uniquely delivers object, block, and file storage in one unified system. You can enforce fine-grained authorization over Ceph's Object Storage using OPA. Ceph's Object Storage essentially consists of a [Ceph Storage Cluster](http://docs.ceph.com/docs/nautilus/rados/#) and a [Ceph Object Gateway](http://docs.ceph.com/docs/nautilus/radosgw/).

The `Ceph Object Gateway` is an object storage interface built on top of [librados](http://docs.ceph.com/docs/nautilus/rados/api/librados-intro/) to provide applications with a RESTful gateway to Ceph Storage Clusters.

OPA is integrated with the `Ceph Object Gateway daemon (RGW)`, which is an HTTP server that interacts with a `Ceph Storage Cluster` and provides interfaces compatible with `OpenStack Swift` and `Amazon S3`.

When the `Ceph Object Gateway` gets a request, it checks with OPA whether the request should be allowed or not. OPA makes a decision (`allow` or `deny`) based on the policies and data it has access to and sends the decision back to the `Ceph Object Gateway` for enforcement.

## Goals

This tutorial shows how to enforce custom policies over the S3 API to the `Ceph Storage Cluster` which applications use to put and get data.

This tutorial uses [Rook](https://rook.io/) to run Ceph inside a Kubernetes cluster.

## Prerequisites

This tutorial requires Kubernetes 1.9 or later. To run the tutorial locally, we recommend using [minikube](https://kubernetes.io/docs/getting-started-guides/minikube) in version `v0.28+` with Kubernetes 1.10 (which is the default).

## Steps

### 1. Start Minikube

```bash
minikube start
```

### 2. Deploy the Rook Operator

Deploy the Rook system components, which include the `Rook agent` and `Rook operator` pods.

Save the operator spec as **operator.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rook-ceph-system
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cephclusters.ceph.rook.io
spec:
  group: ceph.rook.io
  names:
    kind: CephCluster
    listKind: CephClusterList
    plural: cephclusters
    singular: cephcluster
  scope: Namespaced
  version: v1
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            cephVersion:
              properties:
                allowUnsupported:
                  type: boolean
                image:
                  type: string
                name:
                  pattern: ^(luminous|mimic|nautilus)$
                  type: string
            dashboard:
              properties:
                enabled:
                  type: boolean
                urlPrefix:
                  type: string
                port:
                  type: integer
            dataDirHostPath:
              pattern: ^/(\S+)
              type: string
            mon:
              properties:
                allowMultiplePerNode:
                  type: boolean
                count:
                  maximum: 9
                  minimum: 1
                  type: integer
              required:
              - count
            network:
              properties:
                hostNetwork:
                  type: boolean
            storage:
              properties:
                nodes:
                  items: {}
                  type: array
                useAllDevices: {}
                useAllNodes:
                  type: boolean
          required:
          - mon
  additionalPrinterColumns:
    - name: DataDirHostPath
      type: string
      description: Directory used on the K8s nodes
      JSONPath: .spec.dataDirHostPath
    - name: MonCount
      type: string
      description: Number of MONs
      JSONPath: .spec.mon.count
    - name: Age
      type: date
      JSONPath: .metadata.creationTimestamp
    - name: State
      type: string
      description: Current State
      JSONPath: .status.state
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cephfilesystems.ceph.rook.io
spec:
  group: ceph.rook.io
  names:
    kind: CephFilesystem
    listKind: CephFilesystemList
    plural: cephfilesystems
    singular: cephfilesystem
  scope: Namespaced
  version: v1
  additionalPrinterColumns:
    - name: MdsCount
      type: string
      description: Number of MDSs
      JSONPath: .spec.metadataServer.activeCount
    - name: Age
      type: date
      JSONPath: .metadata.creationTimestamp
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cephobjectstores.ceph.rook.io
spec:
  group: ceph.rook.io
  names:
    kind: CephObjectStore
    listKind: CephObjectStoreList
    plural: cephobjectstores
    singular: cephobjectstore
  scope: Namespaced
  version: v1
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cephobjectstoreusers.ceph.rook.io
spec:
  group: ceph.rook.io
  names:
    kind: CephObjectStoreUser
    listKind: CephObjectStoreUserList
    plural: cephobjectstoreusers
    singular: cephobjectstoreuser
  scope: Namespaced
  version: v1
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cephblockpools.ceph.rook.io
spec:
  group: ceph.rook.io
  names:
    kind: CephBlockPool
    listKind: CephBlockPoolList
    plural: cephblockpools
    singular: cephblockpool
  scope: Namespaced
  version: v1
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: volumes.rook.io
spec:
  group: rook.io
  names:
    kind: Volume
    listKind: VolumeList
    plural: volumes
    singular: volume
    shortNames:
    - rv
  scope: Namespaced
  version: v1alpha2
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: rook-ceph-cluster-mgmt
  labels:
    operator: rook
    storage-backend: ceph
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  - pods
  - pods/log
  - services
  - configmaps
  verbs:
  - get
  - list
  - watch
  - patch
  - create
  - update
  - delete
- apiGroups:
  - extensions
  resources:
  - deployments
  - daemonsets
  - replicasets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: rook-ceph-system
  namespace: rook-ceph-system
  labels:
    operator: rook
    storage-backend: ceph
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - configmaps
  verbs:
  - get
  - list
  - watch
  - patch
  - create
  - update
  - delete
- apiGroups:
  - extensions
  resources:
  - daemonsets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: rook-ceph-global
  labels:
    operator: rook
    storage-backend: ceph
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  - nodes/proxy
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  - persistentvolumes
  - persistentvolumeclaims
  verbs:
  - get
  - list
  - watch
  - patch
  - create
  - update
  - delete
- apiGroups:
  - storage.k8s.io
  resources:
  - storageclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - delete
- apiGroups:
  - ceph.rook.io
  resources:
  - "*"
  verbs:
  - "*"
- apiGroups:
  - rook.io
  resources:
  - "*"
  verbs:
  - "*"
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr-cluster
  labels:
    operator: rook
    storage-backend: ceph
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - nodes
  - nodes/proxy
  verbs:
  - get
  - list
  - watch
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: rook-ceph-system
  namespace: rook-ceph-system
  labels:
    operator: rook
    storage-backend: ceph
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-system
  namespace: rook-ceph-system
  labels:
    operator: rook
    storage-backend: ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rook-ceph-system
subjects:
- kind: ServiceAccount
  name: rook-ceph-system
  namespace: rook-ceph-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-global
  namespace: rook-ceph-system
  labels:
    operator: rook
    storage-backend: ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: rook-ceph-global
subjects:
- kind: ServiceAccount
  name: rook-ceph-system
  namespace: rook-ceph-system
---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: rook-ceph-operator
  namespace: rook-ceph-system
  labels:
    operator: rook
    storage-backend: ceph
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: rook-ceph-operator
    spec:
      serviceAccountName: rook-ceph-system
      containers:
      - name: rook-ceph-operator
        image: openpolicyagent/rook-operator:latest
        args: ["ceph", "operator"]
        volumeMounts:
        - mountPath: /var/lib/rook
          name: rook-config
        - mountPath: /etc/ceph
          name: default-config-dir
        env:
        - name: ROOK_ALLOW_MULTIPLE_FILESYSTEMS
          value: "false"
        - name: ROOK_LOG_LEVEL
          value: "INFO"
        - name: ROOK_MON_HEALTHCHECK_INTERVAL
          value: "45s"
        - name: ROOK_MON_OUT_TIMEOUT
          value: "300s"
        - name: ROOK_DISCOVER_DEVICES_INTERVAL
          value: "60m"
        - name: ROOK_HOSTPATH_REQUIRES_PRIVILEGED
          value: "false"
        - name: ROOK_ENABLE_SELINUX_RELABELING
          value: "true"
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumes:
      - name: rook-config
        emptyDir: {}
      - name: default-config-dir
        emptyDir: {}
```

Create the operator:

```bash
kubectl create -f operator.yaml
```

Verify that `rook-ceph-operator`, `rook-ceph-agent`, and `rook-discover` pods are in the `Running` state.

```bash
kubectl -n rook-ceph-system get pod
```

> The Rook operator image `openpolicyagent/rook-operator:latest` used in the tutorial supports the latest version of Ceph (`nautilus`). Rook currently does not support the latest version of Ceph. See [this Github issue](https://github.com/rook/rook/issues/2475) for more details.

### 3. Deploy OPA on top of Kubernetes

Save the OPA spec as **opa.yaml**:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rook-ceph
---
kind: Service
apiVersion: v1
metadata:
  name: opa
  namespace: rook-ceph
  labels:
    app: opa
    rook_cluster: rook-ceph
spec:
  type: NodePort
  selector:
    app: opa
  ports:
  - name: http
    protocol: TCP
    port: 8181
    targetPort: 8181
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: opa
  namespace: rook-ceph
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
        - name: opa
          image: openpolicyagent/opa:0.10.5
          ports:
          - name: http
            containerPort: 8181
          args:
            - "run"
            - "--ignore=.*"
            - "--server"
            - "--log-level=debug"
            - "/policies/authz.rego"
          volumeMounts:
            - readOnly: true
              mountPath: /policies
              name: authz-policy
      volumes:
        - name: authz-policy
          configMap:
            name: authz-policy
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: authz-policy
  namespace: rook-ceph
data:
  authz.rego: |
    package ceph.authz

    default allow = false

    #-----------------------------------------------------------------------------
    # Data structures containing location info about users and buckets.
    # In real-world deployments, these data structures could be loaded into
    # OPA as raw JSON data. The JSON data could also be pulled from external
    # sources like AD, Git, etc.
    #-----------------------------------------------------------------------------

    # user-location information
    user_location = {
        "alice": "UK",
        "bob":   "USA"
    }

    # bucket-location information
    bucket_location = {
        "supersecretbucket": "USA"
    }

    allow {
        input.method = "HEAD"
        is_user_in_bucket_location(input.user_info.user_id, input.bucket_info.bucket.name)
    }

    allow {
        input.method = "GET"
    }

    allow {
        input.method = "PUT"
        input.user_info.display_name = "Bob"
    }

    allow {
        input.method = "DELETE"
        input.user_info.display_name = "Bob"
    }

    # Check if the user and the bucket being accessed belong to the same
    # location.
    is_user_in_bucket_location(user, bucket) {
        user_location[user] == bucket_location[bucket]
    }
```

```bash
kubectl apply -f opa.yaml
```

The OPA spec contains a ConfigMap where an OPA policy has been defined. This policy will be used to authorize requests received by the `Ceph Object Gateway`. More details on this policy will be covered later in the tutorial.

Verify that the OPA pod is `Running`.

```bash
kubectl -n rook-ceph get pod -l app=opa
```

### 4. Create a Ceph Cluster

For the cluster to survive reboots, make sure you set the `dataDirHostPath` property that is valid for your hosts. For minikube, `dataDirHostPath` is set to `/data/rook`.

Save the cluster spec as **cluster.yaml**:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: rook-ceph-osd
  namespace: rook-ceph
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: rook-ceph-mgr
  namespace: rook-ceph
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-osd
  namespace: rook-ceph
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: [ "get", "list", "watch", "create", "update", "delete" ]
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr-system
  namespace: rook-ceph
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr
  namespace: rook-ceph
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - services
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - delete
- apiGroups:
  - ceph.rook.io
  resources:
  - "*"
  verbs:
  - "*"
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-cluster-mgmt
  namespace: rook-ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: rook-ceph-cluster-mgmt
subjects:
- kind: ServiceAccount
  name: rook-ceph-system
  namespace: rook-ceph-system
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-osd
  namespace: rook-ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rook-ceph-osd
subjects:
- kind: ServiceAccount
  name: rook-ceph-osd
  namespace: rook-ceph
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr
  namespace: rook-ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rook-ceph-mgr
subjects:
- kind: ServiceAccount
  name: rook-ceph-mgr
  namespace: rook-ceph
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr-system
  namespace: rook-ceph-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rook-ceph-mgr-system
subjects:
- kind: ServiceAccount
  name: rook-ceph-mgr
  namespace: rook-ceph
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: rook-ceph-mgr-cluster
  namespace: rook-ceph
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: rook-ceph-mgr-cluster
subjects:
- kind: ServiceAccount
  name: rook-ceph-mgr
  namespace: rook-ceph
---
apiVersion: ceph.rook.io/v1
kind: CephCluster
metadata:
  name: rook-ceph
  namespace: rook-ceph
spec:
  cephVersion:
    image: ceph/daemon-base:latest-master
    allowUnsupported: true
  dataDirHostPath: /data/rook
  # set the amount of mons to be started
  mon:
    count: 3
    allowMultiplePerNode: true
  dashboard:
    enabled: true
  network:
    hostNetwork: false
  rbdMirroring:
    workers: 0
  resources:
  storage:
    useAllNodes: true
    useAllDevices: false
    deviceFilter:
    location:
    config:
      databaseSizeMB: "1024"
      journalSizeMB: "1024"
      osdsPerDevice: "1"
```

Create the cluster:

```bash
kubectl create -f cluster.yaml
```

Make sure the following pods are `Running`.

```bash
$ kubectl -n rook-ceph get pod
NAME                                   READY     STATUS      RESTARTS   AGE
opa-7458bf7dc6-g72tt                   1/1       Running     0          7m
rook-ceph-mgr-a-77c8bc845c-t4j9s       1/1       Running     0          4m
rook-ceph-mon-a-79f5886d9c-mgkpc       1/1       Running     0          5m
rook-ceph-mon-b-68dcffc7cb-xcdts       1/1       Running     0          4m
rook-ceph-mon-c-844f9d4fbd-tz9pn       1/1       Running     0          4m
rook-ceph-osd-0-7479c85878-mbrhd       1/1       Running     0          3m
rook-ceph-osd-prepare-minikube-4mm7c   0/2       Completed   0          4m
```

### 5. Configure Ceph to use OPA

The `Ceph Object Gateway` needs to be configured to use OPA for authorization decisions. The following configuration options are available for the OPA integration with the gateway:

```bash
rgw use opa authz = {use opa server to authorize client requests}
rgw opa url = {opa server url:opa server port}
rgw opa token = {opa bearer token}
rgw opa verify ssl = {verify opa server ssl certificate}
```

More information on the OPA - Ceph Object Gateway integration can be found in the Ceph [docs](http://docs.ceph.com/docs/nautilus/radosgw/opa/).

When the Rook Operator creates a cluster, a placeholder ConfigMap is created that can be used to override Ceph's configuration settings.

Update the ConfigMap to include the OPA-related options.

```bash
kubectl -n rook-ceph edit configmap rook-config-override
```

Modify the settings and save.

```bash
data:
  config: |
    [client.radosgw.gateway]
    rgw use opa authz = true
    rgw opa url = opa.rook-ceph:8181/v1/data/ceph/authz/allow
```

### 6. Create the Ceph Object Store

Save the object store spec as **object.yaml**:

```yaml
apiVersion: ceph.rook.io/v1
kind: CephObjectStore
metadata:
  name: my-store
  namespace: rook-ceph
spec:
  metadataPool:
    failureDomain: host
    replicated:
      size: 1
  dataPool:
    failureDomain: osd
    replicated:
      size: 1
  gateway:
    type: s3
    sslCertificateRef:
    port: 80
    securePort:
    instances: 1
    allNodes: false
    placement:
    resources:
```

```bash
kubectl create -f object.yaml
```

When the object store is created, the RGW service with the S3 API will be started in the cluster. The Rook operator will create all the pools and other resources necessary to start the service.

Check that the RGW pod is `Running`.

```bash
kubectl -n rook-ceph get pod -l app=rook-ceph-rgw
```

Rook sets up the object storage so pods will have access internal to the cluster. Create a new service for external access. We will need the external RGW service for exercising our OPA policy.

Save the external service as **rgw-external.yaml**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: rook-ceph-rgw-my-store-external
  namespace: rook-ceph
  labels:
    app: rook-ceph-rgw
    rook_cluster: rook-ceph
    rook_object_store: my-store
spec:
  ports:
  - name: rgw
    port: 80
    protocol: TCP
    targetPort: 80
  selector:
    app: rook-ceph-rgw
    rook_cluster: rook-ceph
    rook_object_store: my-store
  sessionAffinity: None
  type: NodePort
```

Create the external service.

```bash
kubectl create -f rgw-external.yaml
```

Check that both the `internal` and `external` RGW services are `Running`.

```bash
kubectl -n rook-ceph get service rook-ceph-rgw-my-store rook-ceph-rgw-my-store-external
```

### 7. Create Object Store Users

Create two object store users `Alice` and `Bob`.

**object-user-alice.yaml**

```yaml
apiVersion: ceph.rook.io/v1
kind: CephObjectStoreUser
metadata:
  name: alice
  namespace: rook-ceph
spec:
  store: my-store
  displayName: "Alice"
```

**object-user-bob.yaml**

```yaml
apiVersion: ceph.rook.io/v1
kind: CephObjectStoreUser
metadata:
  name: bob
  namespace: rook-ceph
spec:
  store: my-store
  displayName: "Bob"
```

Now create the users.

```bash
kubectl create -f object-user-alice.yaml
kubectl create -f object-user-bob.yaml
```

When the object store user is created the Rook operator will create the RGW user on the object store `my-store`, and store the user's Access Key and Secret Key in a Kubernetes secret in the namespace `rook-ceph`.

### 8. Understanding the OPA policy

As we saw earlier, the OPA spec contained a ConfigMap that defined the policy to be used to authorize requests received by the `Ceph Object Gateway`. Below is the policy:

**authz.rego**

{%ace lang='python'%}
package ceph.authz

default allow = false

#-----------------------------------------------------------------------------
# Data structures containing location info about users and buckets.
# In real-world deployments, these data structures could be loaded into
# OPA as raw JSON data. The JSON data could also be pulled from external
# sources like AD, Git, etc.
#-----------------------------------------------------------------------------

# user-location information
user_location = {
    "alice": "UK",
    "bob":   "USA"
}

# bucket-location information
bucket_location = {
    "supersecretbucket": "USA"
}

# Allow access to bucket in same location as user.
allow {
    input.method = "HEAD"
    is_user_in_bucket_location(input.user_info.user_id, input.bucket_info.bucket.name)
}

allow {
    input.method = "GET"
}

allow {
    input.method = "PUT"
    input.user_info.display_name = "Bob"
}

allow {
    input.method = "DELETE"
    input.user_info.display_name = "Bob"
}

# Check if the user and the bucket being accessed belong to the same
# location.
is_user_in_bucket_location(user, bucket) {
    user_location[user] == bucket_location[bucket]
}
{%endace%}

**The above policy will restrict a user from accessing a bucket whose location does not match the user's location.**. The user's and bucket's location is hardcoded in the policy for simplicity and in the real-world can be fetched from external sources or pushed into OPA using it's REST API.

In the above policy, `Bob's` location is `USA` while `Alice's` is `UK`. Since the bucket `supersecretbucket` is located in the `USA`, `Alice` should not be able to access it.

### 9. Create the S3 access test script

The below Python S3 access test script connects to the  `Ceph Object Store Gateway` to perform actions such as creating and deleting buckets. 

> You will need to install the `python-boto` package to run the test script.

Save the test script as **s3test.py**:

{%ace lang='python'%}
#!/usr/bin/env python

import sys
import boto.s3.connection
from boto.s3.key import Key
import os


def create_bucket(conn, bucket_name):
    try:
        bucket = conn.create_bucket(bucket_name)
    except Exception as e:
        print 'Unable to create bucket: Forbidden'


def delete_bucket(conn, bucket_name):
    try:
        bucket = conn.delete_bucket(bucket_name)
    except Exception as e:
        print 'Unable to delete bucket: Forbidden'


def list_bucket(conn):
    buckets = conn.get_all_buckets()
    if len(buckets) == 0:
        print 'No Buckets'
        return
    for bucket in buckets:
        print "{name} {created}".format(
            name=bucket.name,
            created=bucket.creation_date,
    )


def upload_data(conn, bucket_name, data):
    bucket = conn.get_bucket(bucket_name)
    k = Key(bucket)
    k.key = 'foobar'
    k.set_contents_from_string(data)


def download_data(conn, user, bucket_name):
    try:
        bucket = conn.get_bucket(bucket_name)
    except Exception as e:
        print 'Not allowed to access bucket "{}": User "{}" not in the same location as bucket "{}"'.format(bucket_name, user, bucket_name)
    else:
        k = Key(bucket)
        k.key = 'foobar'
        print k.get_contents_as_string()


if __name__ == '__main__':
    user = sys.argv[1]
    action = sys.argv[2]

    if len(sys.argv) == 4:
        bucket_name = sys.argv[3]

    if len(sys.argv) == 5:
        bucket_name = sys.argv[3]
        data = sys.argv[4]

    access_key_env_name = '{}_ACCESS_KEY'.format(user.upper())
    secret_key_env_name = '{}_SECRET_KEY'.format(user.upper())

    access_key = os.getenv(access_key_env_name)
    secret_key = os.getenv(secret_key_env_name)

    conn = boto.connect_s3(
            aws_access_key_id=access_key,
            aws_secret_access_key=secret_key,
            host=os.getenv("HOST"), port=int(os.getenv("PORT")),
            is_secure=False, calling_format=boto.s3.connection.OrdinaryCallingFormat(),
    )

    if action == 'create':
            create_bucket(conn, bucket_name)

    if action == 'list':
            list_bucket(conn)

    if action == 'delete':
            delete_bucket(conn, bucket_name)

    if action == 'upload_data':
        upload_data(conn, bucket_name, data)

    if action == 'download_data':
        download_data(conn, user, bucket_name)
{%endace%}

The script needs the following environment variables:

* `HOST` - Hostname of the machine running the RGW service in the cluster.
* `PORT` - Port of the RGW service.
* `<USER>_ACCESS_KEY` - USER's `ACCESS_KEY`
* `<USER>_SECRET_KEY` - USER's `SECRET_KEY`

We previously created a service to provide external access to the RGW.

```bash
$ kubectl -n rook-ceph get service rook-ceph-rgw-my-store-external
NAME                              TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
rook-ceph-rgw-my-store-external   NodePort   10.111.198.148   <none>        80:31765/TCP   2h
```
Internally the RGW service is running on port `80`. The external port in this case is `31765`.

Set the `HOST` and `PORT` environment variables:

```bash
export HOST=$(minikube ip)
export PORT=$(kubectl -n rook-ceph get service rook-ceph-rgw-my-store-external -o jsonpath='{.spec.ports[?(@.name=="rgw")].nodePort}')
```

Get Alice's and Bob's `ACCESS_KEY` and `SECRET_KEY` from the Kubernetes Secret and set the following environment variables:

```bash
export ALICE_ACCESS_KEY=$(kubectl get secret rook-ceph-object-user-my-store-alice -n rook-ceph -o yaml | grep AccessKey | awk '{print $2}' | base64 --decode)
export ALICE_SECRET_KEY=$(kubectl get secret rook-ceph-object-user-my-store-alice -n rook-ceph -o yaml | grep SecretKey | awk '{print $2}' | base64 --decode)
export BOB_ACCESS_KEY=$(kubectl get secret rook-ceph-object-user-my-store-bob -n rook-ceph -o yaml | grep AccessKey | awk '{print $2}' | base64 --decode)
export BOB_SECRET_KEY=$(kubectl get secret rook-ceph-object-user-my-store-bob -n rook-ceph -o yaml | grep SecretKey | awk '{print $2}' | base64 --decode)
```

Now let's create a bucket and add some data to it.

* First, `Bob` creates a bucket `supersecretbucket`

    ```bash
    python s3test.py Bob create supersecretbucket
    ```

* List the bucket just created

    ```bash
    python s3test.py Bob list
    ```

    The output will be something like:

    ```raw
    supersecretbucket 2019-01-14T21:18:03.872Z
    ```

* Add some data to the bucket `supersecretbucket`
  
    ```bash
    python s3test.py Bob upload_data supersecretbucket "This is some secret data"
    ```

### 10. Exercise the OPA policy

To recap, the policy we are going to test will **restrict a user from accessing a bucket whose location does not match the user's location.**.

Check that `Alice` cannot access the contents of the bucket `supersecretbucket`.

```bash
python s3test.py Alice download_data supersecretbucket
```

Since `Alice` is located in `UK` and and the bucket `supersecretbucket` in the `USA`, she would be denied access.

Check that `Bob` can access the contents of the bucket `supersecretbucket`.

```bash
python s3test.py Bob download_data supersecretbucket
```

Since `Bob` and the bucket `supersecretbucket` are both located in the `USA`, `Bob` is granted access to the contents in the bucket.

## Wrap Up

Congratulations for finishing the tutorial!

This tutorial showed how OPA can be used to enforce custom policies over the S3 API to the `Ceph Storage Cluster`. You can modify OPA's polices to get greater control over the actions performed on the `Ceph Object Storage` without making any changes to Ceph.

This tutorial also showed how OPA can seamlessly work with Rook without any modifications to Rook's components.
// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"context"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestScheduler(t *testing.T) {
	ctx := context.Background()
	rego := setup(ctx, t, "data_10nodes_30pods.json")

	rs, err := rego.Eval(ctx)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	ws := rs[0].Expressions[0].Value.(map[string]interface{})
	if len(ws) != 10 {
		t.Fatal("unexpected query result:", rs)
	}
	for n, w := range ws {
		if fmt.Sprint(w) != "5.0138888888888888886" {
			t.Fatalf("unexpected weight for: %v: %v\n\nDumping all weights:\n\n%v\n", n, w, rs)
		}
	}
}

func setup(ctx context.Context, t *testing.T, filename string) *rego.Rego {

	// policy compilation
	c := ast.NewCompiler()
	modules := map[string]*ast.Module{
		"test": ast.MustParseModule(policy),
	}

	if c.Compile(modules); c.Failed() {
		t.Fatal("unexpected error:", c.Errors)
	}

	// storage setup
	store := loadDataStore(filename)

	// parameter setup
	input := util.MustUnmarshalJSON([]byte(requestedPod))

	return rego.New(
		rego.Compiler(c),
		rego.Store(store),
		rego.Input(input),
		rego.Query("data.opa.test.scheduler.fit"),
	)
}

func loadDataStore(filename string) storage.Store {
	f, err := os.Open(getFilename(filename))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	return inmem.NewFromReader(f)
}

func getFilename(filename string) string {
	return filepath.Join("testdata", filename)
}

const (
	requestedPod = `{"pod": {
 "status": {
  "phase": "Pending"
 },
 "kind": "Pod",
 "spec": {
  "terminationGracePeriodSeconds": 30,
  "dnsPolicy": "ClusterFirst",
  "securityContext": {},
  "restartPolicy": "Always",
  "containers": [
   {
    "terminationMessagePath": "/dev/termination-log",
    "name": "nginx",
    "image": "nginx",
    "imagePullPolicy": "Always",
    "ports": [
     {
      "protocol": "TCP",
      "containerPort": 80
     }
    ],
    "resources": {}
   }
  ]
 },
 "apiVersion": "v1",
 "metadata": {
  "name": "nginx-mdj4s",
  "resourceVersion": "102515",
  "generateName": "nginx-",
  "namespace": "kubemark",
  "labels": {
   "app": "nginx30"
  },
  "creationTimestamp": "2016-07-09T22:01:27Z",
  "annotations": {
   "scheduler.alpha.kubernetes.io/name": "experimental",
   "kubernetes.io/created-by": "{\"kind\":\"SerializedReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"ReplicationController\",\"namespace\":\"kubemark\",\"name\":\"nginx\",\"uid\":\"af24f2bf-4620-11e6-bd6d-0800275521ee\",\"apiVersion\":\"v1\",\"resourceVersion\":\"102514\"}}\n"
  },
  "selfLink": "/api/v1/namespaces/kubemark/pods/nginx-mdj4s",
  "uid": "af25a765-4620-11e6-bd6d-0800275521ee"
 }
}}`

	policy = `
package opa.test.scheduler

import data.nodes
import data.pods
import data.pvs
import data.pvcs
import data.services
import data.replicationcontrollers as rcs
import input.pod as req

# Fit rule for all pods. Implements same filtering and
# prioritisation logic that is included by default in Kubernetes.
fit[node_name] = weight {
    scheduler_name[my_scheduler_name]
    filter[node_id]
    weight := prioritise[node_id]
    node_name := nodes[node_id].metadata.name
}

filter[node_id] {
    # Filtering for all pods except hollow node pods.
    not hollow_node
    not blacklisted[nodes[node_id].metadata.name]
    not port_conflicts[node_id]
    not disk_conflicts[node_id]
    resources_available[node_id]
} {
    # Filtering for hollow node pods. Force them all onto
    # localhost node for testing purposes.
    hollow_node
    nodes[node_id].metadata.name == "127.0.0.1"
}

port_conflicts[node_id] {
    node := nodes[node_id]
    pods[i].spec.nodeName == node.metadata.name
    container := pods[i].spec.containers[j]
    port := container.ports[k].hostPort
    req_container := req.spec.containers[l]
    req_port := req_container.ports[m].hostPort
    req_port == port
}

disk_conflicts[node_id] {
    gce_persistent_disk_conflicts[node_id]
    aws_ebs_conflicts[node_id]
    rbd_conflicts[node_id]
}

gce_persistent_disk_conflicts[node_id] {
    req_disk := req.spec.volumes[i].gcePersistentDisk
    not req_disk.readOnly
    node := nodes[node_id]
    pod := pods[j]
    pod.spec.nodeName == node.metadata.name
    disk := pod.volumes[k].gcePersistentDisk
    req_disk.pdName == disk.pdName
} {
    req_disk := req.spec.volumes[i].gcePersistentDisk
    req_disk.readOnly
    node := nodes[node_id]
    pod := pods[j]
    pod.spec.nodeName == node.metadata.name
    disk := pod.volumes[k].gcePersistentDisk
    req_disk.pdName == disk.pdName
    not disk.readOnly
}

aws_ebs_conflicts[node_id] {
    req_disk := req.spec.volumes[i].awsElasticBlockStore
    node := nodes[node_id]
    pod := pods[j]
    pod.spec.nodeName == node.metadata.name
    disk := pod.volumes[k].awsElasticBlockStore
    disk.volumeID == req_disk.volumeID
}

rbd_conflicts[node_id] {
    req_disk := req.spec.volumes[i].rbd
    node := nodes[node_id]
    pod := pods[j]
    pod.spec.nodeName == node.metadata.name
    disk = pod.volumes[k].rbd
    req_disk.image == disk.image
    req_disk.pool == disk.pool
    req_disk.monitors[l] == disk.monitors[m]
}

pv_zone_label_match[node_id] {
    req_volume := req.spec.volumes[i]
    req_claim_name := req_volume.persistentVolumeClaim.claimName
    req_namespace := req.metadata.namespace
    pvcs[j].metadata.namespace == req_namespace
    pvcs[j].metadata.name == req_claim_name
    pvs[k].metadata.name == pvcs[j].spec.volumeName
    label := zone_labels[l]
    value := pvs[k].metadata.labels[label]
    nodes[node_id].metadata.labels[label] == value
}

resources_available[node_id] {
    node := nodes[node_id]
    not pods_exceeded[node_id]
    not mem_exceeded[node_id]
    not cpu_exceeded[node_id]
}

pods_exceeded[node_id] {
    num_pods := count(pods_on_node[node_id])
    max_pods := to_number(nodes[node_id].status.allocatable.pods)
    num_pods >= max_pods
}

mem_exceeded[node_id] {
    alloc := allocatable_mem[node_id]
    total := mem_total[node_id]
    total >= alloc
}

cpu_exceeded[node_id] {
    alloc := allocatable_cpu[node_id]
    total := cpu_total[node_id]
    total >= alloc
}

cpu_total[node_id] = sum([cpu | cpu := req_cpu[_]]) + used_cpu[node_id]

mem_total[node_id] = sum([mem | mem := req_mem[_]]) + used_mem[node_id]

cpu_nonzero_total[node_id] = sum([cpu | cpu := req_cpu[_]]) + used_nonzero_cpu[node_id]

mem_nonzero_total[node_id] = sum([mem | mem := req_mem[_]]) + used_nonzero_mem[node_id]

req_cpu[name] = cpu {
    container := req.spec.containers[_]
    name := container.name
    cpu := container.resources.requests.cpu
} {
    container := req.spec.containers[i]
    name := container.name
    not container.resources.requests.cpu
    cpu := default_milli_cpu_req
}

req_mem[name] = mem {
    container := req.spec.containers[_]
    name := container.name
    mem := container.resources.requests.memory
} {
    container := req.spec.containers[_]
    name := container.name
    not container.resources.requests.memory
    mem := default_memory_req
}

allocatable_mem[node_id] = alloc {
    alloc := nodes[node_id].status.allocatable.memory
}

allocatable_cpu[node_id] = alloc {
    alloc := nodes[node_id].status.allocatable.cpu
}

used_mem[node_id] = used {
    node_pods := pods_on_node[node_id]
    mem := [m | pod := node_pods[_]
                container := pod.spec.containers[_]
                requested := container.resources.requests
                m := requested.memory]
    used := sum(mem)
}

used_cpu[node_id] = used {
    node_pods := pods_on_node[node_id]
    cpu := [c | pod := node_pods[_]
                container := pod.spec.containers[_]
                requested := container.resources.requests
                c := requested.cpu]
    used := sum(cpu)
}

used_nonzero_mem[node_id] = used {
    node_pods := pods_on_node[node_id]
    mem := [m | pod := node_pods[_]
                container := pod.spec.containers[_]
                requested := container.resources.requests
                m := requested.memory]
    def := [m | pod := node_pods[_]
                container := pod.spec.containers[_]
                not container.resources.requests.memory
                m := default_memory_req]
    used := sum(mem) + sum(def)
}

used_nonzero_cpu[node_id] = used {
    node_pods := pods_on_node[node_id]
    cpu = [c | pod := node_pods[_]
               container := pod.spec.containers[_]
               requested := container.resources.requests
               c := requested.cpu]
    def = [c | pod := node_pods[_]
               container := pod.spec.containers[_]
               not container.resources.requests.cpu
               c := default_milli_cpu_req]
    used = sum(cpu) + sum(def)
}

pods_on_node[node_id] = pds {
    node_name := nodes[node_id].metadata.name
    pds := [p | pods[i].spec.nodeName == node_name; p := pods[i]]
}

hollow_node {
    req.metadata.labels[i] == "hollow-node"
}

blacklisted[node_name] {
    node_names := [
        "127.0.0.1"
    ]
    node_name := node_names[i]
}

my_scheduler_name = "experimental"

# This scheduler is responsible for pods annotated with the following scheduler names.
scheduler_name[scheduler] {
    scheduler := req.metadata.annotations[k8s_scheduler_annotations]
}

# Scheduler annotation. This annotation indicates whether the scheduler is responsible
# for this pod.
k8s_scheduler_annotation = "scheduler.alpha.kubernetes.io/name"

# The maximum number of EBS volumes
# See http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/volume_limits.html#linux-specific-volume-limits
max_ebs_pd_volumes = 39

# The maximum number of GCE PersistentDisk volumes
# https://cloud.google.com/compute/docs/disks/#introduction
max_gce_pd_volumes = 16

zone_labels = [
    "failure-domain.beta.kubernetes.io/zone",
    "failure-domain.beta.kubernetes.io/region"
]

taint_annotation = "scheduler.alpha.kubernetes.io/taints"
toleration_annotation = "scheduler.alpha.kubernetes.io/tolerations"

default_milli_cpu_req = 100    # 0.1 cores
default_memory_req = 209715200 # 200MB

prioritise[node_id] = weight {
    weight := sum([
        selector_spreading[node_id],
        balanced_allocation[node_id],
        least_requested[node_id]]) / 3
}

least_requested[node_id] = weight {
    weight := (cpu_weight[node_id] + mem_weight[node_id]) / 2
}

cpu_weight[node_id] = weight {
    cpu_capacity := allocatable_cpu[node_id]
    weight := ((cpu_capacity - cpu_nonzero_total[node_id]) * 10) / cpu_capacity
}

mem_weight[node_id] = weight {
    mem_capacity := allocatable_mem[node_id]
    weight := ((mem_capacity - mem_nonzero_total[node_id]) * 10) / mem_capacity
}

balanced_allocation[node_id] = weight {
    mem_f := mem_fraction[node_id]
    cpu_f := cpu_fraction[node_id]
    mem_f < 1
    cpu_f < 1
    weight := (10 - (abs(cpu_f - mem_f) * 10))
} {
    mem_fraction[node_id] >= 1
    cpu_fraction[node_id] >= 1
    weight = 0
} {
    mem_fraction[node_id] < 1
    cpu_fraction[node_id] >= 1
    weight = 0
} {
    mem_fraction[node_id] >= 1
    cpu_fraction[node_id] < 1
    weight = 0
}

cpu_fraction[node_id] = f {
    f := cpu_nonzero_total[node_id] / allocatable_cpu[node_id]
}

mem_fraction[node_id] = f {
    f := mem_nonzero_total[node_id] / allocatable_mem[node_id]
}

selector_spreading[node_id] = weight {
    max_count := max_rc_match_count
    weight := ((max_count - rc_match_count[node_id]) / max_count) * 10
}

max_rc_match_count = max_count {
    max([1, max([c | c := rc_match_count[_]])], max_count)
}

rc_match_count[node_id] = cnt {
    nodes[node_id]
    rcs_req_matches[rc_id]
    cnt := count([1 | rcs_on_node[node_id][_] == rc_id])
}

rcs_on_node[node_id] = rc_ids {
    pods_on_node[node_id] = node_pods
    rc_ids = [ rc_id | pod := node_pods[_]
                       rc_id := rcs_for_pod[pod.metadata.uid][_]]
}

rcs_for_pod[pod_id] = rc_ids {
    pods[pod_id]
    rc_ids = [rc_id | rcs[rc_id]
                      selector_matches[[pod_id, rc_id]]]
}

selector_matches[[pod_id, rc_id]] {
    pods[pod_id]
    rcs[rc_id]
    x = [pod_id, rc_id]
    not selector_not_matches[x]
}

selector_not_matches[[pod_id, rc_id]] {
    pods[pod_id] = pod
    rc := rcs[rc_id]
    v := rc.spec.selector[k]
    not pod.metadata.labels[k] = v
}

rcs_req_matches[rc_id] {
    rcs[rc_id]
    not rcs_req_not_matches[rc_id]
}

rcs_req_not_matches[rc_id] {
    value := rcs[rc_id].spec.selector[label]
    not req.metadata.labels[label] = value
}

`
)

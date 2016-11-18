// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

func TestScheduler(t *testing.T) {
	params := setup(t, "data_10nodes_30pods.json")
	defer params.Store.Close(params.Transaction)

	qrs, err := topdown.Query(params)

	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	ws := qrs[0].Result.(map[string]interface{})
	if len(ws) != 10 {
		t.Fatal("unexpected query result:", qrs)
	}
	for n, w := range ws {
		if fmt.Sprintf("%.3f", w) != "5.014" {
			t.Fatalf("unexpected weight for: %v: %v\n\nDumping all weights:\n\n%v\n", n, w, qrs)
		}
	}
}

func setup(t *testing.T, filename string) *topdown.QueryParams {

	// policy compilation
	c := ast.NewCompiler()
	modules := map[string]*ast.Module{
		"test": ast.MustParseModule(policy),
	}

	if c.Compile(modules); c.Failed() {
		t.Fatal("unexpected error:", c.Errors)
	}

	// storage setup
	store := storage.New(storage.Config{
		Builtin: loadDataStore(filename),
	})

	// parameter setup
	globals := ast.NewValueMap()
	req := ast.MustParseTerm(requestedPod).Value
	globals.Put(ast.Var("requested_pod"), req)
	path := ast.MustParseRef("data.opa.test.scheduler.fit")
	txn := storage.NewTransactionOrDie(store)
	params := topdown.NewQueryParams(c, store, txn, globals, path)

	return params
}

func loadDataStore(filename string) *storage.DataStore {
	f, err := os.Open(getFilename(filename))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	return storage.NewDataStoreFromReader(f)
}

func getFilename(filename string) string {
	gopath := getGOPATH()
	return filepath.Join(gopath, path, filename)
}

func getGOPATH() string {
	for _, s := range os.Environ() {
		vs := strings.SplitN(s, "=", 2)
		if vs[0] == "GOPATH" {
			return vs[1]
		}
	}
	panic("cannot find GOPATH in environment")
}

const (
	path = "src/github.com/open-policy-agent/opa/test/scheduler"

	requestedPod = `{
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
}`

	policy = `
package opa.test.scheduler

import data.nodes
import data.pods
import data.pvs
import data.pvcs
import data.services
import data.replicationcontrollers as rcs
import requested_pod as req

# Fit rule for all pods. Implements same filtering and
# prioritisation logic that is included by default in Kubernetes.
fit[node_name] = weight :-
    scheduler_name[my_scheduler_name],
    filter[node_id],
    prioritise[node_id] = weight,
    node_name = nodes[node_id].metadata.name

# Filtering for all pods except hollow node pods.
filter[node_id] :-
    not hollow_node,
    not blacklisted[nodes[node_id].metadata.name],
    not port_conflicts[node_id],
    not disk_conflicts[node_id],
    resources_available[node_id]

# Filtering for hollow node pods. Force them all onto
# localhost node for testing purposes.
filter[node_id] :-
    hollow_node,
    nodes[node_id].metadata.name = "127.0.0.1"

port_conflicts[node_id] :-
    node = nodes[node_id],
    pods[i].spec.nodeName = node.metadata.name,
    container = pods[i].spec.containers[j],
    port = container.ports[k].hostPort,
    req_container = req.spec.containers[l],
    req_port = req_container.ports[m].hostPort,
    req_port = port

disk_conflicts[node_id] :-
    gce_persistent_disk_conflicts[node_id],
    aws_ebs_conflicts[node_id],
    rbd_conflicts[node_id]

gce_persistent_disk_conflicts[node_id] :-
    req_disk = req.spec.volumes[i].gcePersistentDisk,
    not req_disk.readOnly,
    node = nodes[node_id],
    pod = pods[j],
    pod.spec.nodeName = node.metadata.name,
    disk = pod.volumes[k].gcePersistentDisk,
    req_disk.pdName = disk.pdName

gce_persistent_disk_conflicts[node_id] :-
    req_disk = req.spec.volumes[i].gcePersistentDisk,
    req_disk.readOnly,
    node = nodes[node_id],
    pod = pods[j],
    pod.spec.nodeName = node.metadata.name,
    disk = pod.volumes[k].gcePersistentDisk,
    req_disk.pdName = disk.pdName,
    not disk.readOnly

aws_ebs_conflicts[node_id] :-
    req_disk = req.spec.volumes[i].awsElasticBlockStore,
    node = nodes[node_id],
    pod = pods[j],
    pod.spec.nodeName = node.metadata.name,
    disk = pod.volumes[k].awsElasticBlockStore,
    disk.volumeID = req_disk.volumeID

rbd_conflicts[node_id] :-
    req_disk = req.spec.volumes[i].rbd,
    node = nodes[node_id],
    pod = pods[j],
    pod.spec.nodeName = node.metadata.name,
    disk = pod.volumes[k].rbd,
    req_disk.image = disk.image,
    req_disk.pool = disk.pool,
    req_disk.monitors[l] = disk.monitors[m]

pv_zone_label_match[node_id] :-
    req_volume = req.spec.volumes[i],
    req_claim_name = req_volume.persistentVolumeClaim.claimName,
    req_namespace = req.metadata.namespace,
    pvcs[j].metadata.namespace = req_namespace,
    pvcs[j].metadata.name = req_claim_name,
    pvs[k].metadata.name = pvcs[j].spec.volumeName,
    label = zone_labels[l],
    pvs[k].metadata.labels[label] = value,
    nodes[node_id].metadata.labels[label] = value

resources_available[node_id] :-
    node = nodes[node_id],
    not pods_exceeded[node_id],
    not mem_exceeded[node_id],
    not cpu_exceeded[node_id]

pods_exceeded[node_id] :-
    count(pods_on_node[node_id], num_pods),
    to_number(nodes[node_id].status.allocatable.pods, max_pods),
    num_pods >= max_pods

mem_exceeded[node_id] :-
    allocatable_mem[node_id] = alloc,
    mem_total[node_id] = total,
    total >= alloc

cpu_exceeded[node_id] :-
    allocatable_cpu[node_id] = alloc,
    cpu_total[node_id] = total,
    total >= alloc

cpu_total[node_id] = cpu_t :-
    sum([cpu | cpu = req_cpu[_]], cpu_requested),
    plus(cpu_requested, used_cpu[node_id], cpu_t)

mem_total[node_id] = mem_t :-
    sum([mem | mem = req_mem[_]], mem_requested),
    plus(mem_requested, used_mem[node_id], mem_t)

cpu_nonzero_total[node_id] = cpu_t :-
    sum([cpu | cpu = req_cpu[_]], cpu_requested),
    plus(cpu_requested, used_nonzero_cpu[node_id], cpu_t)

mem_nonzero_total[node_id] = mem_t :-
    sum([mem | mem = req_mem[_]], mem_requested),
    plus(mem_requested, used_nonzero_mem[node_id], mem_t)

req_cpu[name] = cpu :-
    container = req.spec.containers[_],
    container.name = name,
    container.resources.requests.cpu = cpu

req_cpu[name] = cpu :-
    container = req.spec.containers[i],
    container.name = name,
    not container.resources.requests.cpu,
    cpu = default_milli_cpu_req

req_mem[name] = mem :-
    container = req.spec.containers[_],
    container.name = name,
    container.resources.requests.memory = mem

req_mem[name] = mem :-
    container = req.spec.containers[_],
    container.name = name,
    not container.resources.requests.memory,
    mem = default_memory_req

allocatable_mem[node_id] = alloc :-
    nodes[node_id].status.allocatable.memory = alloc

allocatable_cpu[node_id] = alloc :-
    nodes[node_id].status.allocatable.cpu = alloc

used_mem[node_id] = used :-
    pods_on_node[node_id] = node_pods,
    mem = [m | node_pods[_] = pod,
               pod.spec.containers[_] = container,
               requested = container.resources.requests,
               requested.memory = m],
    sum(mem, used)

used_cpu[node_id] = used :-
    pods_on_node[node_id] = node_pods,
    cpu = [c | node_pods[_] = pod,
               pod.spec.containers[_] = container,
               container.resources.requests = requested,
               requested.cpu = c],
    sum(cpu, used)

used_nonzero_mem[node_id] = used :-
    pods_on_node[node_id] = node_pods,
    mem = [m | node_pods[_] = pod,
               pod.spec.containers[_] = container,
               requested = container.resources.requests,
               requested.memory = m],
    default = [m | node_pods[_] = pod,
                   pod.spec.containers[_] = container,
                   not container.resources.requests.memory,
                   m = default_memory_req],
    sum(mem, used_nz),
    sum(default, used_default),
    plus(used_nz, used_default, used)

used_nonzero_cpu[node_id] = used :-
    pods_on_node[node_id] = node_pods,
    cpu = [c | node_pods[_] = pod,
               pod.spec.containers[_] = container,
               container.resources.requests = requested,
               requested.cpu = c],
    default = [c | node_pods[_] = pod,
                   pod.spec.containers[_] = container,
                   not container.resources.requests.cpu,
                   c = default_milli_cpu_req],
    sum(cpu, used_nz),
    sum(default, used_default),
    plus(used_nz, used_default, used)

pods_on_node[node_id] = pds :-
    node_name = nodes[node_id].metadata.name,
    pds = [p | pods[i].spec.nodeName = node_name, p = pods[i]]

hollow_node :-
    req.metadata.labels[i] = "hollow-node"

blacklisted[node_name] :-
    node_names = [
        "127.0.0.1"
    ],
    node_name = node_names[i]

my_scheduler_name = "experimental"

# This scheduler is responsible for pods annotated with the following scheduler names.
scheduler_name[scheduler] :-
    req.metadata.annotations[k8s_scheduler_annotations] = scheduler

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

prioritise[node_id] = weight :-
    least_requested[node_id] = lr_w,
    balanced_allocation[node_id] = ba_w,
    selector_spreading[node_id] = ss_w,
    sum([lr_w, ba_w, ss_w], sum_w),
    div(sum_w, 3, weight)

least_requested[node_id] = weight :-
    cpu_weight[node_id] = cpu_w,
    mem_weight[node_id] = mem_w,
    plus(cpu_w, mem_w, total_w),
    div(total_w, 2, weight)

cpu_weight[node_id] = weight :-
    allocatable_cpu[node_id] = cpu_capacity,
    minus(cpu_capacity, cpu_nonzero_total[node_id], cpu_delta),
    mul(cpu_delta, 10, cpu_scaled),
    div(cpu_scaled, cpu_capacity, weight)

mem_weight[node_id] = weight :-
    allocatable_mem[node_id] = mem_capacity,
    minus(mem_capacity, mem_nonzero_total[node_id], mem_delta),
    mul(mem_delta, 10, mem_scaled),
    div(mem_scaled, mem_capacity, weight)

balanced_allocation[node_id] = weight :-
    mem_fraction[node_id] = mem_f,
    cpu_fraction[node_id] = cpu_f,
    mem_f < 1,
    cpu_f < 1,
    minus(cpu_f, mem_f, usage),
    abs(usage, usage_pos),
    mul(usage_pos, 10, usage_scaled),
    minus(10, usage_scaled, weight)

balanced_allocation[node_id] = weight :-
    mem_fraction[node_id] = mem_f,
    cpu_fraction[node_id] = cpu_f,
    mem_f >= 1,
    cpu_f >= 1,
    weight = 0

balanced_allocation[node_id] = weight :-
    mem_fraction[node_id] = mem_f,
    cpu_fraction[node_id] = cpu_f,
    mem_f < 1,
    cpu_f >= 1,
    weight = 0

balanced_allocation[node_id] = weight :-
    mem_fraction[node_id] = mem_f,
    cpu_fraction[node_id] = cpu_f,
    mem_f >= 1,
    cpu_f < 1,
    weight = 0

cpu_fraction[node_id] = f :-
    cpu_nonzero_total[node_id] = cpu,
    allocatable_cpu[node_id] = cpu_capacity,
    div(cpu, cpu_capacity, f)

mem_fraction[node_id] = f :-
    mem_nonzero_total[node_id] = mem,
    allocatable_mem[node_id] = mem_capacity,
    div(mem, mem_capacity, f)

selector_spreading[node_id] = weight :-
    max_rc_match_count = max_count,
    minus(max_count, rc_match_count[node_id], delta),
    div(delta, max_count, ratio),
    mul(ratio, 10, weight)

max_rc_match_count = max_count :-
    max([c | rc_match_count[_] = c], max_c),
    max([1, max_c], max_count)

rc_match_count[node_id] = cnt :-
    nodes[node_id],
    rcs_req_matches[rc_id],
    count([1 | rcs_on_node[node_id][_] = rc_id], cnt)

rcs_on_node[node_id] = rc_ids :-
    pods_on_node[node_id] = node_pods,
    rc_ids = [ rc_id | node_pods[_] = pod,
                       rcs_for_pod[pod.metadata.uid][_] = rc_id]

rcs_for_pod[pod_id] = rc_ids :-
    pods[pod_id],
    rc_ids = [rc_id | rcs[rc_id],
                      x = [pod_id, rc_id],
                      selector_matches[x]]

selector_matches[[pod_id, rc_id]] :-
    pods[pod_id], rcs[rc_id],
    x = [pod_id, rc_id],
    not selector_not_matches[x]

selector_not_matches[[pod_id, rc_id]] :-
    pods[pod_id] = pod,
    rcs[rc_id] = rc,
    rc.spec.selector[k] = v,
    not pod.metadata.labels[k] = v

rcs_req_matches[rc_id] :-
    rcs[rc_id],
    not rcs_req_not_matches[rc_id]

rcs_req_not_matches[rc_id] :-
    rcs[rc_id].spec.selector[label] = value,
    not req.metadata.labels[label] = value
`
)

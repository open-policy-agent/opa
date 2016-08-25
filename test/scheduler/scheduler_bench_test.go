// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"text/template"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

func BenchmarkScheduler10x30(b *testing.B) {
	runSchedulerBenchmark(b, 10, 30)
}

func BenchmarkScheduler100x300(b *testing.B) {
	runSchedulerBenchmark(b, 100, 300)
}

func BenchmarkScheduler1000x3000(b *testing.B) {
	runSchedulerBenchmark(b, 1000, 3000)
}

func runSchedulerBenchmark(b *testing.B, nodes int, pods int) {
	params := setupBenchmark(nodes, pods)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := topdown.Query(params)
		if err != nil {
			b.Fatal("unexpected error:", err)
		}
		ws := r.(map[string]interface{})
		if len(ws) != nodes {
			b.Fatal("unexpected query result:", r)
		}
		for n, w := range ws {
			if fmt.Sprintf("%.3f", w) != "5.014" {
				b.Fatalf("unexpected weight for: %v: %v\n\nDumping all weights:\n\n%v\n", n, w, r)
			}
		}
	}
}

func setupBenchmark(nodes int, pods int) *topdown.QueryParams {

	// policy compilation
	c := ast.NewCompiler()
	modules := map[string]*ast.Module{
		"test": ast.MustParseModule(policy),
	}

	if c.Compile(modules); c.Failed() {
		panic(c.FlattenErrors())
	}

	// storage setup
	store := storage.New(storage.InMemoryConfig())
	insertPolicies(store, c.Modules)

	// parameter setup
	globals := storage.NewBindings()
	req := ast.MustParseTerm(requestedPod).Value
	globals.Put(ast.Var("requested_pod"), req)
	path := []interface{}{"opa", "test", "scheduler", "fit"}
	params := topdown.NewQueryParams(c, store, globals, path)

	// data setup
	txn := storage.NewTransactionOrDie(store)
	defer store.Close(txn)
	setupNodes(store, txn, nodes)
	setupRCs(store, txn, 1)
	setupPods(store, txn, pods, nodes)

	return params
}

type nodeTemplateInput struct {
	Name string
}

type podTemplateInput struct {
	Name     string
	NodeName string
}

type rcTemplateInput struct {
	Name string
}

func setupNodes(store *storage.Storage, txn storage.Transaction, n int) {
	tmpl, err := template.New("node").Parse(nodeTemplate)
	if err != nil {
		panic(err)
	}
	if err := store.Write(txn, storage.AddOp, ast.MustParseRef("data.nodes"), map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := nodeTemplateInput{
			Name: fmt.Sprintf("node%v", i),
		}
		v := runTemplate(tmpl, input)
		ref := ast.MustParseRef(fmt.Sprintf("data.nodes.%v", input.Name))
		if err := store.Write(txn, storage.AddOp, ref, v); err != nil {
			panic(err)
		}
	}
}

func setupRCs(store *storage.Storage, txn storage.Transaction, n int) {
	tmpl, err := template.New("rc").Parse(nodeTemplate)
	if err != nil {
		panic(err)
	}
	ref := ast.MustParseRef("data.replicationcontrollers")
	if err := store.Write(txn, storage.AddOp, ref, map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := nodeTemplateInput{
			Name: fmt.Sprintf("rc%v", i),
		}
		v := runTemplate(tmpl, input)
		ref = ast.MustParseRef(fmt.Sprintf("data.replicationcontrollers.%v", input.Name))
		if err := store.Write(txn, storage.AddOp, ref, v); err != nil {
			panic(err)
		}
	}
}

func setupPods(store *storage.Storage, txn storage.Transaction, n int, numNodes int) {
	tmpl, err := template.New("pod").Parse(podTemplate)
	if err != nil {
		panic(err)
	}
	ref := ast.MustParseRef("data.pods")
	if err := store.Write(txn, storage.AddOp, ref, map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := podTemplateInput{
			Name:     fmt.Sprintf("pod%v", i),
			NodeName: fmt.Sprintf("node%v", i%numNodes),
		}
		v := runTemplate(tmpl, input)
		ref = ast.MustParseRef(fmt.Sprintf("data.pods.%v", input.Name))
		if err := store.Write(txn, storage.AddOp, ref, v); err != nil {
			panic(err)
		}
	}
}

func runTemplate(tmpl *template.Template, input interface{}) interface{} {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		panic(err)
	}
	var v interface{}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		panic(err)
	}
	return v
}

const (
	nodeTemplate = `
    {"status": {
        "capacity": {
          "alpha.kubernetes.io/nvidia-gpu": "0",
          "pods": "200",
          "cpu": "1",
          "memory": "3840Mi"
        },
        "addresses": [
          {
            "type": "LegacyHostIP",
            "address": "172.17.0.5"
          },
          {
            "type": "InternalIP",
            "address": "172.17.0.5"
          }
        ],
        "nodeInfo": {
          "kernelVersion": "",
          "kubeletVersion": "v1.3.0-alpha.4.132+1cce15659750d9-dirty",
          "containerRuntimeVersion": "docker://1.8.1",
          "machineID": "",
          "kubeProxyVersion": "v1.3.0-alpha.4.132+1cce15659750d9-dirty",
          "bootID": "",
          "osImage": "",
          "architecture": "amd64",
          "systemUUID": "",
          "operatingSystem": "linux"
        },
        "allocatable": {
          "alpha.kubernetes.io/nvidia-gpu": "0",
          "pods": "200",
          "cpu": 1000,
          "memory": 4026531840
        },
        "daemonEndpoints": {
          "kubeletEndpoint": {
            "Port": 10250
          }
        },
        "conditions": [
          {
            "status": "False",
            "lastTransitionTime": "2016-07-08T19:09:41Z",
            "lastHeartbeatTime": "2016-07-09T20:38:22Z",
            "reason": "KubeletHasSufficientDisk",
            "message": "kubelet has sufficient disk space available",
            "type": "OutOfDisk"
          },
          {
            "status": "False",
            "lastTransitionTime": "2016-07-08T16:03:29Z",
            "lastHeartbeatTime": "2016-07-09T20:38:22Z",
            "reason": "KubeletHasSufficientMemory",
            "message": "kubelet has sufficient memory available",
            "type": "MemoryPressure"
          },
          {
            "status": "True",
            "lastTransitionTime": "2016-07-08T19:09:41Z",
            "lastHeartbeatTime": "2016-07-09T20:38:22Z",
            "reason": "KubeletReady",
            "message": "kubelet is posting ready status",
            "type": "Ready"
          }
        ]
      },
      "kind": "Node",
      "spec": {
        "externalID": "172.17.0.5"
      },
      "apiVersion": "v1",
      "metadata": {
        "uid": "{{ .Name }}",
        "labels": {
          "kubernetes.io/hostname": "172.17.0.5",
          "beta.kubernetes.io/os": "linux",
          "beta.kubernetes.io/arch": "amd64"
        },
        "resourceVersion": "96999",
        "creationTimestamp": "2016-07-08T16:03:29Z",
        "selfLink": "/api/v1/nodes/172.17.0.5",
        "name": "{{ .Name }}"
      }
    }`

	podTemplate = `
    {
      "status": {
        "containerStatuses": [
          {
            "restartCount": 0,
            "name": "nginx",
            "image": "nginx",
            "imageID": "docker://",
            "state": {
              "running": {
                "startedAt": "2016-07-09T20:37:05Z"
              }
            },
            "ready": true,
            "lastState": {},
            "containerID": "docker:///k8s_nginx.156efd59_nginx30-nm3wu_kubemark_e4b7acdc-4614-11e6-bd6d-0800275521ee_b63ce19a"
          }
        ],
        "podIP": "2.3.4.5",
        "startTime": "2016-07-09T20:37:04Z",
        "hostIP": "172.17.0.10",
        "phase": "Running",
        "conditions": [
          {
            "status": "True",
            "lastTransitionTime": "2016-07-09T20:37:04Z",
            "lastProbeTime": null,
            "type": "Initialized"
          },
          {
            "status": "True",
            "lastTransitionTime": "2016-07-09T20:37:06Z",
            "lastProbeTime": null,
            "type": "Ready"
          },
          {
            "status": "True",
            "lastTransitionTime": "2016-07-09T20:37:04Z",
            "lastProbeTime": null,
            "type": "PodScheduled"
          }
        ]
      },
      "kind": "Pod",
      "spec": {
        "dnsPolicy": "ClusterFirst",
        "securityContext": {},
        "nodeName": "{{ .NodeName }}",
        "terminationGracePeriodSeconds": 30,
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
        "name": "{{ .Name }}",
        "labels": {
          "app": "nginx30"
        },
        "namespace": "kubemark",
        "resourceVersion": "96837",
        "generateName": "nginx30-",
        "creationTimestamp": "2016-07-09T20:37:03Z",
        "annotations": {
          "scheduler.alpha.kubernetes.io/name": "experimental",
          "kubernetes.io/created-by": "{\"kind\":\"SerializedReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"ReplicationController\",\"namespace\":\"kubemark\",\"name\":\"nginx30\",\"uid\":\"e4b655d5-4614-11e6-bd6d-0800275521ee\",\"apiVersion\":\"v1\",\"resourceVersion\":\"96758\"}}\n"
        },
        "selfLink": "/api/v1/namespaces/kubemark/pods/nginx30-nm3wu",
        "uid": "{{ .Name }}"
      }
    }
    `

	rcTemplate = `
    {
      "status": {
        "observedGeneration": 1,
        "fullyLabeledReplicas": 30,
        "replicas": 30
      },
      "kind": "ReplicationController",
      "spec": {
        "selector": {
          "app": "nginx30"
        },
        "template": {
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
          "metadata": {
            "labels": {
              "app": "nginx30"
            },
            "creationTimestamp": null,
            "annotations": {
              "scheduler.alpha.kubernetes.io/name": "experimental"
            },
            "name": "nginx30"
          }
        },
        "replicas": 30
      },
      "apiVersion": "v1",
      "metadata": {
        "name": {{ .Name }},
        "generation": 1,
        "labels": {
          "app": "nginx30"
        },
        "namespace": "kubemark",
        "resourceVersion": "96796",
        "creationTimestamp": "2016-07-09T20:37:03Z",
        "selfLink": "/api/v1/namespaces/kubemark/replicationcontrollers/nginx30",
        "uid": {{ .Name }}
      }
    }
  }
    `
)

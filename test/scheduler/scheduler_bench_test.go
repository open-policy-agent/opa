// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package scheduler

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"context"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

// FIXME(tsandall): scheduling policy depends heavily on data indexing to
// provide adequate performance. Data indexing has been removed until it can be
// performed during compilation. Once data indexing is restored, the large
// benchmarks can be re-enabled.

func BenchmarkScheduler10x30(b *testing.B) {
	runSchedulerBenchmark(b, 10, 30)
}

type benchmarkParams struct {
	store    storage.Store
	compiler *ast.Compiler
	input    interface{}
}

func runSchedulerBenchmark(b *testing.B, nodes int, pods int) {
	ctx := context.Background()
	params := setupBenchmark(nodes, pods)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rego := rego.New(
			rego.Compiler(params.compiler),
			rego.Store(params.store),
			rego.Input(params.input),
			rego.Query("data.opa.test.scheduler.fit"),
		)
		rs, err := rego.Eval(ctx)
		if err != nil {
			b.Fatal("unexpected error:", err)
		}
		ws := rs[0].Expressions[0].Value.(map[string]interface{})
		if len(ws) != nodes {
			b.Fatal("unexpected query result:", rs)
		}
		for n, w := range ws {
			if fmt.Sprint(w) != "5.0138888888888888886" {
				b.Fatalf("unexpected weight for: %v: %v\n\nDumping all weights:\n\n%v\n", n, w, rs)
			}
		}
	}
}

func setupBenchmark(nodes int, pods int) benchmarkParams {

	// policy compilation
	c := ast.NewCompiler()
	modules := map[string]*ast.Module{
		"test": ast.MustParseModule(policy),
	}

	if c.Compile(modules); c.Failed() {
		panic(c.Errors)
	}

	// storage setup
	store := inmem.New()

	// parameter setup
	ctx := context.Background()
	input := util.MustUnmarshalJSON([]byte(requestedPod))

	// data setup
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	setupNodes(ctx, store, txn, nodes)
	setupRCs(ctx, store, txn, 1)
	setupPods(ctx, store, txn, pods, nodes)
	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	return benchmarkParams{
		store:    store,
		compiler: c,
		input:    input,
	}
}

type nodeTemplateInput struct {
	Name string
}

type podTemplateInput struct {
	Name     string
	NodeName string
}

func setupNodes(ctx context.Context, store storage.Store, txn storage.Transaction, n int) {
	tmpl, err := template.New("node").Parse(nodeTemplate)
	if err != nil {
		panic(err)
	}
	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/nodes"), map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := nodeTemplateInput{
			Name: fmt.Sprintf("node%v", i),
		}
		v := runTemplate(tmpl, input)
		path := storage.MustParsePath(fmt.Sprintf("/nodes/%v", input.Name))
		if err := store.Write(ctx, txn, storage.AddOp, path, v); err != nil {
			panic(err)
		}
	}
}

func setupRCs(ctx context.Context, store storage.Store, txn storage.Transaction, n int) {
	tmpl, err := template.New("rc").Parse(nodeTemplate)
	if err != nil {
		panic(err)
	}
	path := storage.MustParsePath("/replicationcontrollers")
	if err := store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := nodeTemplateInput{
			Name: fmt.Sprintf("rc%v", i),
		}
		v := runTemplate(tmpl, input)
		path = storage.MustParsePath(fmt.Sprintf("/replicationcontrollers/%v", input.Name))
		if err := store.Write(ctx, txn, storage.AddOp, path, v); err != nil {
			panic(err)
		}
	}
}

func setupPods(ctx context.Context, store storage.Store, txn storage.Transaction, n int, numNodes int) {
	tmpl, err := template.New("pod").Parse(podTemplate)
	if err != nil {
		panic(err)
	}
	path := storage.MustParsePath("/pods")
	if err := store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{}); err != nil {
		panic(err)
	}
	for i := 0; i < n; i++ {
		input := podTemplateInput{
			Name:     fmt.Sprintf("pod%v", i),
			NodeName: fmt.Sprintf("node%v", i%numNodes),
		}
		v := runTemplate(tmpl, input)
		path = storage.MustParsePath(fmt.Sprintf("/pods/%v", input.Name))
		if err := store.Write(ctx, txn, storage.AddOp, path, v); err != nil {
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
	if err := util.UnmarshalJSON(buf.Bytes(), &v); err != nil {
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
)

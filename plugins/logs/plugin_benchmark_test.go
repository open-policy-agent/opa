package logs

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
	"github.com/open-policy-agent/opa/util"
)

const largeEvent = `{
	"_id": "15596749567705615560",
	"decision_id": "0e67fda0-170b-454d-9f5e-29691073f97e",
	"input": {
	  "apiVersion": "admission.k8s.io/v1beta1",
	  "kind": "AdmissionReview",
	  "request": {
		"kind": {
		  "group": "",
		  "kind": "Pod",
		  "version": "v1"
		},
		"namespace": "demo",
		"object": {
		  "metadata": {
			"creationTimestamp": "2019-06-04T19:02:35Z",
			"labels": {
			  "run": "nginx"
			},
			"name": "nginx",
			"namespace": "demo",
			"uid": "507e4c3c-86fb-11e9-b289-42010a8000b2"
		  },
		  "spec": {
			"containers": [
			  {
				"image": "nginx",
				"imagePullPolicy": "Always",
				"name": "nginx",
				"resources": {},
				"terminationMessagePath": "/dev/termination-log",
				"terminationMessagePolicy": "File",
				"volumeMounts": [
				  {
					"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
					"name": "default-token-5vjbc",
					"readOnly": true
				  }
				]
			  }
			],
			"dnsPolicy": "ClusterFirst",
			"priority": 0,
			"restartPolicy": "Never",
			"schedulerName": "default-scheduler",
			"securityContext": {},
			"serviceAccount": "default",
			"serviceAccountName": "default",
			"terminationGracePeriodSeconds": 30,
			"tolerations": [
			  {
				"effect": "NoExecute",
				"key": "node.kubernetes.io/not-ready",
				"operator": "Exists",
				"tolerationSeconds": 300
			  },
			  {
				"effect": "NoExecute",
				"key": "node.kubernetes.io/unreachable",
				"operator": "Exists",
				"tolerationSeconds": 300
			  }
			],
			"volumes": [
			  {
				"name": "default-token-5vjbc",
				"secret": {
				  "secretName": "default-token-5vjbc"
				}
			  }
			]
		  },
		  "status": {
			"phase": "Pending",
			"qosClass": "BestEffort"
		  }
		},
		"oldObject": null,
		"operation": "CREATE",
		"resource": {
		  "group": "",
		  "resource": "pods",
		  "version": "v1"
		},
		"userInfo": {
		  "groups": [
			"system:serviceaccounts",
			"system:serviceaccounts:opa-system",
			"system:authenticated"
		  ],
		  "username": "system:serviceaccount:opa-system:default"
		}
	  }
	},
	"labels": {
	  "id": "462a43bd-6a5f-4530-9386-30b0f4e0c8af",
	  "policy-type": "kubernetes/admission_control",
	  "system-type": "kubernetes",
	  "version": "0.10.5"
	},
	"metrics": {
	  "timer_rego_module_compile_ns": 222,
	  "timer_rego_module_parse_ns": 313,
	  "timer_rego_query_compile_ns": 121360,
	  "timer_rego_query_eval_ns": 923279,
	  "timer_rego_query_parse_ns": 287152,
	  "timer_server_handler_ns": 2563846
	},
	"path": "admission_control/main",
	"requested_by": "10.52.0.1:53848",
	"result": {
	  "apiVersion": "admission.k8s.io/v1beta1",
	  "kind": "AdmissionReview",
	  "response": {
		"allowed": false,
		"status": {
		  "message": "Resource Pod/demo/nginx includes container image 'nginx' from prohibited registry"
		}
	  }
	},
	"revision": "jafsdkjfhaslkdfjlaksdjflaksjdflkajsdlkfjasldkfjlaksdjflkasdjflkasjdflkajsdflkjasdklfjalsdjf",
	"timestamp": "2019-06-04T19:02:35.692Z"
  }`

func BenchmarkMaskingNop(b *testing.B) {

	ctx := context.Background()
	store := inmem.New()

	manager, err := plugins.New(nil, "test", store)
	if err != nil {
		b.Fatal(err)
	} else if err := manager.Start(ctx); err != nil {
		b.Fatal(err)
	}

	cfg := &Config{Service: "svc"}
	t := plugins.DefaultTriggerMode
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t); err != nil {
		b.Fatal(err)
	}
	plugin := New(cfg, manager)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		var event EventV1

		if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		if err := plugin.maskEvent(ctx, nil, &event); err != nil {
			b.Fatal(err)
		}
	}

}

func BenchmarkMaskingRuleCountsNop(b *testing.B) {
	numRules := []int{1, 10, 100, 1000}

	ctx := context.Background()
	store := inmem.New()

	manager, err := plugins.New(nil, "test", store)
	if err != nil {
		b.Fatal(err)
	} else if err := manager.Start(ctx); err != nil {
		b.Fatal(err)
	}

	cfg := &Config{Service: "svc"}
	t := plugins.DefaultTriggerMode
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t); err != nil {
		b.Fatal(err)
	}
	plugin := New(cfg, manager)

	for _, ruleCount := range numRules {

		b.Run(fmt.Sprintf("%dRules", ruleCount), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				var event EventV1
				if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
					b.Fatal(err)
				}

				b.StartTimer()

				if err := plugin.maskEvent(ctx, nil, &event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMaskingErase(b *testing.B) {

	ctx := context.Background()
	store := inmem.New()

	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		return store.UpsertPolicy(ctx, txn, "test.rego", []byte(`
			package system.log

			mask["/input"] {
				input.input.request.kind.kind == "Pod"
			}
		`))
	})
	if err != nil {
		b.Fatal(err)
	}

	manager, err := plugins.New(nil, "test", store)
	if err != nil {
		b.Fatal(err)
	} else if err := manager.Start(ctx); err != nil {
		b.Fatal(err)
	}

	cfg := &Config{Service: "svc"}
	t := plugins.DefaultTriggerMode
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t); err != nil {
		b.Fatal(err)
	}
	plugin := New(cfg, manager)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		b.StopTimer()

		var event EventV1

		if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		if err := plugin.maskEvent(ctx, nil, &event); err != nil {
			b.Fatal(err)
		}

		if event.Input != nil {
			b.Fatal("Expected input to be erased")
		}
	}

}

package logs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/util"
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

// BenchmarkEventAST exercises EventV1.AST().
//
// With util.RoundTrip:      29402 ns/op   36476 B/op   637 allocs/op
// With util.RoundTripFast:  12226 ns/op   25416 B/op   407 allocs/op
func BenchmarkEventAST(b *testing.B) {
	var event EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		event.inputAST = nil // force AST() to redo the input conversion each iteration
		if _, err := event.AST(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChunkEncoderEncode exercises chunkEncoder.Encode() with the default upload size limit,
// one event per iteration. A chunk that grows past the limit is split again, and the
// time of the slowest call is reported as max-ns/op.
//
// With the chunk decoded and re-encoded when it grows past the limit:
//
//	small input:           3,300 ns/op    4,162 B/op       2 allocs/op
//	50000 element input: 970,000 ns/op  943,790 B/op  15,608 allocs/op   1,300,000,000 max-ns/op
//
// With the events of the chunk kept in memory:
//
//	small input:           3,300 ns/op    5,240 B/op       2 allocs/op
//	50000 element input: 153,000 ns/op   35,618 B/op       3 allocs/op      32,000,000 max-ns/op
//
// The max-ns/op of the small input is the first call, which sets up the encoder.
func BenchmarkChunkEncoderEncode(b *testing.B) {
	b.Run("small input", func(b *testing.B) {
		var event EventV1
		if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
			b.Fatal(err)
		}
		benchmarkChunkEncoderEncode(b, event)
	})

	b.Run("50000 element input", func(b *testing.B) {
		items := make([]any, 50000)
		for i := range items {
			items[i] = json.Number(strconv.Itoa(i % 100))
		}
		var input any = map[string]any{"items": items}
		benchmarkChunkEncoderEncode(b, EventV1{DecisionID: "abc", Path: "bench/sumlist", Input: &input})
	})
}

func benchmarkChunkEncoderEncode(b *testing.B, event EventV1) {
	eventBytes, err := json.Marshal(&event)
	if err != nil {
		b.Fatal(err)
	}

	enc := newChunkEncoder(defaultUploadSizeLimitBytes)

	var slowest time.Duration
	b.ReportAllocs()
	for b.Loop() {
		start := time.Now()
		if _, err := enc.Encode(event, eventBytes); err != nil {
			b.Fatal(err)
		}
		slowest = max(slowest, time.Since(start))
	}
	b.ReportMetric(float64(slowest.Nanoseconds()), "max-ns/op")
}

// BenchmarkEventToFields exercises eventToFields(), which prepares an event for the console decision logger.
//
// With JSON marshal and unmarshal:
//
//	small input:           20300 ns/op      19629 B/op      316 allocs/op
//	50000 element input: 7400000 ns/op    8128000 B/op   195095 allocs/op
//
// With util.RoundTripFast:
//
//	small input:            3480 ns/op       8544 B/op       71 allocs/op
//	50000 element input:  231000 ns/op     803600 B/op       11 allocs/op
func BenchmarkEventToFields(b *testing.B) {
	b.Run("small input", func(b *testing.B) {
		var event EventV1
		if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
			b.Fatal(err)
		}
		benchmarkEventToFields(b, event)
	})

	b.Run("50000 element input", func(b *testing.B) {
		items := make([]any, 50000)
		for i := range items {
			items[i] = json.Number(strconv.Itoa(i % 100))
		}
		var input any = map[string]any{"items": items}
		benchmarkEventToFields(b, EventV1{DecisionID: "abc", Path: "bench/sumlist", Input: &input})
	})
}

func benchmarkEventToFields(b *testing.B, event EventV1) {
	b.ReportAllocs()
	for b.Loop() {
		if fields := eventToFields(event); len(fields) == 0 {
			b.Fatal("expected fields")
		}
	}
}

func BenchmarkMaskingNop(b *testing.B) {

	ctx := b.Context()
	store := inmem.New()

	manager, err := plugins.New(nil, "test", store)
	if err != nil {
		b.Fatal(err)
	} else if err := manager.Start(ctx); err != nil {
		b.Fatal(err)
	}

	cfg := &Config{Service: "svc"}
	t := plugins.DefaultTriggerMode
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t, nil); err != nil {
		b.Fatal(err)
	}
	plugin := New(cfg, manager)

	var event EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
		b.Fatal(err)
	}
	input, err := event.AST()
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if err := plugin.maskEvent(ctx, nil, input, &event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMaskingRuleCountsNop(b *testing.B) {
	numRules := []int{1, 10, 100, 1000}

	var event EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
		b.Fatal(err)
	}
	input, err := event.AST()
	if err != nil {
		b.Fatal(err)
	}

	for _, ruleCount := range numRules {
		b.Run(fmt.Sprintf("%dRules", ruleCount), func(b *testing.B) {
			ctx := b.Context()
			store := inmem.New()

			// Rules that never fire. Cost is expected to stay flat as
			// ruleCount grows: rule indexing rejects the whole set on one
			// lookup. A linear result here means indexing stopped working.
			policy := strings.Builder{}
			policy.WriteString("package system.log\n")
			for i := range ruleCount {
				fmt.Fprintf(&policy, `
mask contains "/input/absent%d" if {
	input.input.request.kind.kind == "NoSuchKind%d"
}
`, i, i)
			}

			err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
				return store.UpsertPolicy(ctx, txn, "test.rego", []byte(policy.String()))
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
			if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t, nil); err != nil {
				b.Fatal(err)
			}
			plugin := New(cfg, manager)

			for b.Loop() {
				if err := plugin.maskEvent(ctx, nil, input, &event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMaskingErase(b *testing.B) {

	ctx := b.Context()
	store := inmem.New()

	err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
		return store.UpsertPolicy(ctx, txn, "test.rego", []byte(`
			package system.log

			mask contains "/input" if {
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
	if err := cfg.validateAndInjectDefaults([]string{"svc"}, nil, &t, nil); err != nil {
		b.Fatal(err)
	}
	plugin := New(cfg, manager)
	var event EventV1
	if err := util.UnmarshalJSON([]byte(largeEvent), &event); err != nil {
		b.Fatal(err)
	}
	input, err := event.AST()
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if err := plugin.maskEvent(ctx, nil, input, &event); err != nil {
			b.Fatal(err)
		}

		if event.Input != nil {
			b.Fatal("Expected input to be erased")
		}
	}
}

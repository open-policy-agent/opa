module github.com/open-policy-agent/opa

go 1.16

require (
	github.com/OneOfOne/xxhash v1.2.8
	github.com/bytecodealliance/wasmtime-go v0.36.0
	github.com/containerd/containerd v1.6.6
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/fortytw2/leaktest v1.3.0
	github.com/foxcpp/go-mockdns v0.0.0-20210729171921-fb145fc6f897
	github.com/fsnotify/fsnotify v1.5.4
	github.com/ghodss/yaml v1.0.0
	github.com/go-ini/ini v1.66.6
	github.com/go-logr/logr v1.2.3
	github.com/gobwas/glob v0.2.3
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/miekg/dns v1.1.43 // indirect
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799
	github.com/peterh/liner v0.0.0-20170211195444-bf27d3ba8e1d
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.2
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	github.com/vektah/gqlparser/v2 v2.4.7
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415
	github.com/yashtewari/glob-intersection v0.1.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.32.0
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.7.0
	go.opentelemetry.io/otel/sdk v1.7.0
	go.opentelemetry.io/otel/trace v1.7.0
	go.uber.org/automaxprocs v1.5.1
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/grpc v1.48.0
	gopkg.in/yaml.v2 v2.4.0
	oras.land/oras-go v1.2.0
)

// glog is only used through badger, and only for fatal log-and-exit. However, it comes
// at a cost, its init function will lookup the current user, and on Windows, that's much
// work.
replace github.com/golang/glog => ./build/replacements/github.com/golang/glog

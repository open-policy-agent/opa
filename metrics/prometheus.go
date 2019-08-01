package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// GlobalMetricsRegistry is the Prometheus metrics registry singleton
var GlobalMetricsRegistry *prometheus.Registry

func init() {
	ResetGlobalMetricsRegistry()
}

// ResetGlobalMetricsRegistry resets GlobalMetricsRegistry to it's default value.
// This is needed by the unit tests that create many server instances and would try to register duplicate collectors in the registry
func ResetGlobalMetricsRegistry() {
	GlobalMetricsRegistry = prometheus.NewRegistry()
	GlobalMetricsRegistry.MustRegister(prometheus.NewGoCollector())
}

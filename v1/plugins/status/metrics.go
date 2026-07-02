package status

import (
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/version"
	"github.com/prometheus/client_golang/prometheus"
)

var defaultBundleLoadStageBuckets = prometheus.ExponentialBuckets(1000, 2, 20)

type PrometheusConfig struct {
	Collectors *Collectors `json:"collectors,omitempty"`
}

type Collectors struct {
	BundleLoadDurationNanoseconds *BundleLoadDurationNanoseconds `json:"bundle_loading_duration_ns,omitempty"`
}

func injectDefaultDurationBuckets(p *PrometheusConfig) *PrometheusConfig {
	if p != nil && p.Collectors != nil && p.Collectors.BundleLoadDurationNanoseconds != nil && p.Collectors.BundleLoadDurationNanoseconds.Buckets != nil {
		return p
	}

	return &PrometheusConfig{
		Collectors: &Collectors{
			BundleLoadDurationNanoseconds: &BundleLoadDurationNanoseconds{
				Buckets: defaultBundleLoadStageBuckets,
			},
		},
	}
}

// collectors is a list of all collectors maintained by the status plugin.
// Note: when adding a new collector, make sure to also add it to this list,
// or it won't survive status plugin reconfigure events.
type collectors struct {
	opaInfo                  prometheus.Gauge
	pluginStatus             *prometheus.GaugeVec
	loaded                   *prometheus.CounterVec
	failLoad                 *prometheus.CounterVec
	lastRequest              *prometheus.GaugeVec
	lastSuccessfulActivation *prometheus.GaugeVec
	lastSuccessfulDownload   *prometheus.GaugeVec
	lastSuccessfulRequest    *prometheus.GaugeVec
	bundleLoadDuration       *prometheus.HistogramVec
}

func newCollectors(prometheusConfig *PrometheusConfig) *collectors {
	opaInfo := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "opa_info",
			Help:        "Information about the OPA environment.",
			ConstLabels: map[string]string{"version": version.Version},
		},
	)
	opaInfo.Set(1) // only publish once

	pluginStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "plugin_status_gauge",
			Help: "Gauge for the plugin by status.",
		},
		[]string{"name", "status"},
	)
	loaded := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bundle_loaded_counter",
			Help: "Counter for the bundle loaded.",
		},
		[]string{"name"},
	)
	failLoad := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bundle_failed_load_counter",
			Help: "Counter for the failed bundle load.",
		},
		[]string{"name", "code", "message"},
	)
	lastRequest := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_bundle_request",
			Help: "Gauge for the last bundle request.",
		},
		[]string{"name"},
	)
	lastSuccessfulActivation := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_activation",
			Help: "Gauge for the last success bundle activation.",
		},
		[]string{"name", "active_revision"},
	)
	lastSuccessfulDownload := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_download",
			Help: "Gauge for the last success bundle download.",
		},
		[]string{"name"},
	)
	lastSuccessfulRequest := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_request",
			Help: "Gauge for the last success bundle request.",
		},
		[]string{"name"},
	)

	bundleLoadDuration := newBundleLoadDurationCollector(prometheusConfig)

	return &collectors{
		opaInfo:                  opaInfo,
		pluginStatus:             pluginStatus,
		loaded:                   loaded,
		failLoad:                 failLoad,
		lastRequest:              lastRequest,
		lastSuccessfulActivation: lastSuccessfulActivation,
		lastSuccessfulDownload:   lastSuccessfulDownload,
		lastSuccessfulRequest:    lastSuccessfulRequest,
		bundleLoadDuration:       bundleLoadDuration,
	}
}

func newBundleLoadDurationCollector(prometheusConfig *PrometheusConfig) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bundle_loading_duration_ns",
		Help:    "Histogram for the bundle loading duration by stage.",
		Buckets: prometheusConfig.Collectors.BundleLoadDurationNanoseconds.Buckets,
	}, []string{"name", "stage"})
}

func (c *collectors) RegisterAll(register prometheus.Registerer, logger logging.Logger) {
	if register == nil {
		return
	}
	for _, collector := range c.toList() {
		if err := register.Register(collector); err != nil {
			logger.Error("Status metric failed to register on prometheus :%v.", err)
		}
	}
}

func (c *collectors) UnregisterAll(register prometheus.Registerer) {
	if register == nil {
		return
	}

	for _, collector := range c.toList() {
		register.Unregister(collector)
	}
}

func (c *collectors) ReregisterBundleLoadDuration(register prometheus.Registerer, config *PrometheusConfig, logger logging.Logger) {
	logger.Debug("Re-register bundleLoadDuration collector")
	register.Unregister(c.bundleLoadDuration)
	c.bundleLoadDuration = newBundleLoadDurationCollector(config)
	if err := register.Register(c.bundleLoadDuration); err != nil {
		logger.Error("Status metric failed to register bundleLoadDuration collector on prometheus :%v.", err)
	}
}

// helper function
func (c *collectors) toList() []prometheus.Collector {
	return []prometheus.Collector{
		c.opaInfo,
		c.pluginStatus,
		c.loaded,
		c.failLoad,
		c.lastRequest,
		c.lastSuccessfulActivation,
		c.lastSuccessfulDownload,
		c.lastSuccessfulRequest,
		c.bundleLoadDuration,
	}
}

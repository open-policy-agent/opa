package status

import (
	"sync"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/version"
	"github.com/prometheus/client_golang/prometheus"
)

var defaultBundleLoadStageBuckets = prometheus.ExponentialBuckets(1000, 2, 20)

// decisionLogsCounterNames lists the counters the decision logs plugin records
// on the global metrics provider when it uploads, drops or fails to encode
// events. They are defined in the logs plugin, which can't be imported here.
var decisionLogsCounterNames = []string{
	"decision_logs_dropped_rate_limit_exceeded",
	"decision_logs_dropped_buffer_size_limit_exceeded",
	"decision_logs_dropped_buffer_size_limit_bytes_exceeded",
	"decision_logs_encoding_failure",
	"decision_logs_nd_builtin_cache_dropped",
	"enc_log_exceeded_upload_size_limit_bytes",
	"decision_logs_chunks_uploaded",
	"decision_logs_chunks_upload_failed",
}

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
	decisionLogsStatus       *prometheus.GaugeVec
	decisionLogsCounters     *decisionLogsCounters
	decisionLogsBuffer       *decisionLogsBufferCollector
}

func newCollectors(prometheusConfig *PrometheusConfig, manager *plugins.Manager) *collectors {
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

	decisionLogsStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "decision_logs_status_gauge",
			Help: "Gauge for the last decision log upload by status.",
		},
		[]string{"code", "http_code"},
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
		decisionLogsStatus:       decisionLogsStatus,
		decisionLogsCounters:     newDecisionLogsCounters(),
		decisionLogsBuffer:       newDecisionLogsBufferCollector(manager),
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
		c.decisionLogsStatus,
		c.decisionLogsCounters,
		c.decisionLogsBuffer,
	}
}

// decisionLogsCounters exports the decision logs counters kept on the global
// metrics provider. The Prometheus provider doesn't forward counters to its
// registry, so the values are copied over on every status update. Exported
// names carry the conventional _total suffix for Prometheus counters.
type decisionLogsCounters struct {
	mtx    sync.Mutex
	descs  map[string]*prometheus.Desc
	values map[string]float64
}

func newDecisionLogsCounters() *decisionLogsCounters {
	descs := make(map[string]*prometheus.Desc, len(decisionLogsCounterNames))
	for _, name := range decisionLogsCounterNames {
		descs[name] = prometheus.NewDesc(name+"_total", "Counter for the decision logs metric "+name+".", nil, nil)
	}
	return &decisionLogsCounters{descs: descs, values: map[string]float64{}}
}

// update copies the known decision logs counters out of a metrics snapshot, as
// returned by metrics.Metrics.All.
func (c *decisionLogsCounters) update(all map[string]any) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for name := range c.descs {
		if v, ok := all["counter_"+name].(uint64); ok {
			c.values[name] = float64(v)
		}
	}
}

func (c *decisionLogsCounters) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *decisionLogsCounters) Collect(ch chan<- prometheus.Metric) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for name, v := range c.values {
		ch <- prometheus.MustNewConstMetric(c.descs[name], prometheus.CounterValue, v)
	}
}

// decisionLogsBuffer is implemented by the decision logs plugin, which is
// registered on the manager as "decision_logs".
type decisionLogsBuffer interface {
	BufferSize() (bufferType string, size int64)
}

// decisionLogsBufferCollector reports how much is waiting in the decision log
// buffer at scrape time. Only the gauge for the configured buffer type is
// reported, and nothing when the decision logs plugin isn't registered.
type decisionLogsBufferCollector struct {
	manager *plugins.Manager
	events  *prometheus.Desc
	bytes   *prometheus.Desc
}

func newDecisionLogsBufferCollector(manager *plugins.Manager) *decisionLogsBufferCollector {
	return &decisionLogsBufferCollector{
		manager: manager,
		events:  prometheus.NewDesc("decision_logs_buffer_size_events", "Number of items waiting in the decision log event buffer.", nil, nil),
		bytes:   prometheus.NewDesc("decision_logs_buffer_size_bytes", "Number of bytes waiting in the decision log size buffer.", nil, nil),
	}
}

func (c *decisionLogsBufferCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.events
	ch <- c.bytes
}

func (c *decisionLogsBufferCollector) Collect(ch chan<- prometheus.Metric) {
	dl, ok := c.manager.Plugin("decision_logs").(decisionLogsBuffer)
	if !ok {
		return
	}

	bufferType, size := dl.BufferSize()
	desc := c.events
	if bufferType == "size" {
		desc = c.bytes
	}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(size))
}

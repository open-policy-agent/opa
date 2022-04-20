package status

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pluginStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "plugin_status_gauge",
			Help: "Gauge for the plugin by status."},
		[]string{"name", "status"},
	)
	loaded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bundle_loaded_counter",
			Help: "Counter for the bundle loaded."},
		[]string{"name"},
	)
	failLoad = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bundle_failed_load_counter",
			Help: "Counter for the failed bundle load."},
		[]string{"name", "code", "message"},
	)
	lastRequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_bundle_request",
			Help: "Gauge for the last bundle request."},
		[]string{"name"},
	)
	lastSuccessfulActivation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_activation",
			Help: "Gauge for the last success bundle activation."},
		[]string{"name", "active_revision"},
	)
	lastSuccessfulDownload = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_download",
			Help: "Gauge for the last success bundle download."},
		[]string{"name"},
	)
	lastSuccessfulRequest = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "last_success_bundle_request",
			Help: "Gauge for the last success bundle request."},
		[]string{"name"},
	)
	bundleLoadDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bundle_loading_duration_ns",
		Help:    "Histogram for the bundle loading duration by stage.",
		Buckets: prometheus.ExponentialBuckets(1000, 2, 20),
	}, []string{"name", "stage"})
)

// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	bundleLastSuccessfulActivation = prometheus.NewDesc(
		"bundle_last_successful_activation",
		"The time of the last succesful activation",
		[]string{
			"name",
		}, nil,
	)

	bundleLastSuccessfulDownload = prometheus.NewDesc(
		"bundle_last_successful_download",
		"The time of the last succesful download",
		[]string{
			"name",
		}, nil,
	)
)

type bundleCollector struct {
	status Status
}

func NewBundleCollector(s Status) *bundleCollector {
	return &bundleCollector{status: s}
}

// Describe implements the prometheus.Collector interface.
func (c *bundleCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- bundleLastSuccessfulActivation
	ch <- bundleLastSuccessfulDownload
}

// Collect implements the prometheus.Collector interface.
func (c *bundleCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		bundleLastSuccessfulActivation, 
		prometheus.GaugeValue, 
		float64(c.status.LastSuccessfulActivation.Unix()),  
		c.status.Name)

	ch <- prometheus.MustNewConstMetric(
		bundleLastSuccessfulDownload, 
		prometheus.GaugeValue, 
		float64(c.status.LastSuccessfulDownload.Unix()),  
		c.status.Name)
}
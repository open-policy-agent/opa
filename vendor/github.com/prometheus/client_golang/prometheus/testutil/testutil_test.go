// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type untypedCollector struct{}

func (u untypedCollector) Describe(c chan<- *prometheus.Desc) {
	c <- prometheus.NewDesc("name", "help", nil, nil)
}

func (u untypedCollector) Collect(c chan<- prometheus.Metric) {
	c <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("name", "help", nil, nil),
		prometheus.UntypedValue,
		2001,
	)
}

func TestToFloat64(t *testing.T) {
	gaugeWithAValueSet := prometheus.NewGauge(prometheus.GaugeOpts{})
	gaugeWithAValueSet.Set(3.14)

	counterVecWithOneElement := prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"foo"})
	counterVecWithOneElement.WithLabelValues("bar").Inc()

	counterVecWithTwoElements := prometheus.NewCounterVec(prometheus.CounterOpts{}, []string{"foo"})
	counterVecWithTwoElements.WithLabelValues("bar").Add(42)
	counterVecWithTwoElements.WithLabelValues("baz").Inc()

	histogramVecWithOneElement := prometheus.NewHistogramVec(prometheus.HistogramOpts{}, []string{"foo"})
	histogramVecWithOneElement.WithLabelValues("bar").Observe(2.7)

	scenarios := map[string]struct {
		collector prometheus.Collector
		panics    bool
		want      float64
	}{
		"simple counter": {
			collector: prometheus.NewCounter(prometheus.CounterOpts{}),
			panics:    false,
			want:      0,
		},
		"simple gauge": {
			collector: prometheus.NewGauge(prometheus.GaugeOpts{}),
			panics:    false,
			want:      0,
		},
		"simple untyped": {
			collector: untypedCollector{},
			panics:    false,
			want:      2001,
		},
		"simple histogram": {
			collector: prometheus.NewHistogram(prometheus.HistogramOpts{}),
			panics:    true,
		},
		"simple summary": {
			collector: prometheus.NewSummary(prometheus.SummaryOpts{}),
			panics:    true,
		},
		"simple gauge with an actual value set": {
			collector: gaugeWithAValueSet,
			panics:    false,
			want:      3.14,
		},
		"counter vec with zero elements": {
			collector: prometheus.NewCounterVec(prometheus.CounterOpts{}, nil),
			panics:    true,
		},
		"counter vec with one element": {
			collector: counterVecWithOneElement,
			panics:    false,
			want:      1,
		},
		"counter vec with two elements": {
			collector: counterVecWithTwoElements,
			panics:    true,
		},
		"histogram vec with one element": {
			collector: histogramVecWithOneElement,
			panics:    true,
		},
	}

	for n, s := range scenarios {
		t.Run(n, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil && s.panics {
					t.Error("expected panic")
				} else if r != nil && !s.panics {
					t.Error("unexpected panic: ", r)
				}
				// Any other combination is the expected outcome.
			}()
			if got := ToFloat64(s.collector); got != s.want {
				t.Errorf("want %f, got %f", s.want, got)
			}
		})
	}
}

func TestCollectAndCompare(t *testing.T) {
	const metadata = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})
	c.Inc()

	expected := `

		some_total{ label1 = "value1" } 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected), "some_total"); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestNoMetricFilter(t *testing.T) {
	const metadata = `
		# HELP some_total A value that represents a counter.
		# TYPE some_total counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})
	c.Inc()

	expected := `
		some_total{label1="value1"} 1
	`

	if err := CollectAndCompare(c, strings.NewReader(metadata+expected)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestMetricNotFound(t *testing.T) {
	const metadata = `
		# HELP some_other_metric A value that represents a counter.
		# TYPE some_other_metric counter
	`

	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "some_total",
		Help: "A value that represents a counter.",
		ConstLabels: prometheus.Labels{
			"label1": "value1",
		},
	})
	c.Inc()

	expected := `
		some_other_metric{label1="value1"} 1
	`

	expectedError := `
metric output does not match expectation; want:

# HELP some_other_metric A value that represents a counter.
# TYPE some_other_metric counter
some_other_metric{label1="value1"} 1


got:

# HELP some_total A value that represents a counter.
# TYPE some_total counter
some_total{label1="value1"} 1

`

	err := CollectAndCompare(c, strings.NewReader(metadata+expected))
	if err == nil {
		t.Error("Expected error, got no error.")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected\n%#+v\nGot:\n%#+v\n", expectedError, err.Error())
	}
}

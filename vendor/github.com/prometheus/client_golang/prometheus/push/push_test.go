// Copyright 2016 The Prometheus Authors
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

package push

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/common/expfmt"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPush(t *testing.T) {

	var (
		lastMethod string
		lastBody   []byte
		lastPath   string
	)

	// Fake a Pushgateway that always responds with 202.
	pgwOK := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastMethod = r.Method
			var err error
			lastBody, err = ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			lastPath = r.URL.EscapedPath()
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusAccepted)
		}),
	)
	defer pgwOK.Close()

	// Fake a Pushgateway that always responds with 500.
	pgwErr := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "fake error", http.StatusInternalServerError)
		}),
	)
	defer pgwErr.Close()

	metric1 := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "testname1",
		Help: "testhelp1",
	})
	metric2 := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "testname2",
		Help:        "testhelp2",
		ConstLabels: prometheus.Labels{"foo": "bar", "dings": "bums"},
	})

	reg := prometheus.NewRegistry()
	reg.MustRegister(metric1)
	reg.MustRegister(metric2)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	enc := expfmt.NewEncoder(buf, expfmt.FmtProtoDelim)

	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
			t.Fatal(err)
		}
	}
	wantBody := buf.Bytes()

	// Push some Collectors, all good.
	if err := New(pgwOK.URL, "testjob").
		Collector(metric1).
		Collector(metric2).
		Push(); err != nil {
		t.Fatal(err)
	}
	if lastMethod != "PUT" {
		t.Error("want method PUT for Push, got", lastMethod)
	}
	if bytes.Compare(lastBody, wantBody) != 0 {
		t.Errorf("got body %v, want %v", lastBody, wantBody)
	}
	if lastPath != "/metrics/job/testjob" {
		t.Error("unexpected path:", lastPath)
	}

	// Add some Collectors, with nil grouping, all good.
	if err := New(pgwOK.URL, "testjob").
		Collector(metric1).
		Collector(metric2).
		Add(); err != nil {
		t.Fatal(err)
	}
	if lastMethod != "POST" {
		t.Error("want method POST for Add, got", lastMethod)
	}
	if bytes.Compare(lastBody, wantBody) != 0 {
		t.Errorf("got body %v, want %v", lastBody, wantBody)
	}
	if lastPath != "/metrics/job/testjob" {
		t.Error("unexpected path:", lastPath)
	}

	// Push some Collectors with a broken PGW.
	if err := New(pgwErr.URL, "testjob").
		Collector(metric1).
		Collector(metric2).
		Push(); err == nil {
		t.Error("push to broken Pushgateway succeeded")
	} else {
		if got, want := err.Error(), "unexpected status code 500 while pushing to "+pgwErr.URL+"/metrics/job/testjob: fake error\n"; got != want {
			t.Errorf("got error %q, want %q", got, want)
		}
	}

	// Push some Collectors with invalid grouping or job.
	if err := New(pgwOK.URL, "testjob").
		Grouping("foo", "bums").
		Collector(metric1).
		Collector(metric2).
		Push(); err == nil {
		t.Error("push with grouping contained in metrics succeeded")
	}
	if err := New(pgwOK.URL, "test/job").
		Collector(metric1).
		Collector(metric2).
		Push(); err == nil {
		t.Error("push with invalid job value succeeded")
	}
	if err := New(pgwOK.URL, "testjob").
		Grouping("foobar", "bu/ms").
		Collector(metric1).
		Collector(metric2).
		Push(); err == nil {
		t.Error("push with invalid grouping succeeded")
	}
	if err := New(pgwOK.URL, "testjob").
		Grouping("foo-bar", "bums").
		Collector(metric1).
		Collector(metric2).
		Push(); err == nil {
		t.Error("push with invalid grouping succeeded")
	}

	// Push registry, all good.
	if err := New(pgwOK.URL, "testjob").
		Gatherer(reg).
		Push(); err != nil {
		t.Fatal(err)
	}
	if lastMethod != "PUT" {
		t.Error("want method PUT for Push, got", lastMethod)
	}
	if bytes.Compare(lastBody, wantBody) != 0 {
		t.Errorf("got body %v, want %v", lastBody, wantBody)
	}

	// Add registry, all good.
	if err := New(pgwOK.URL, "testjob").
		Grouping("a", "x").
		Grouping("b", "y").
		Gatherer(reg).
		Add(); err != nil {
		t.Fatal(err)
	}
	if lastMethod != "POST" {
		t.Error("want method POST for Add, got", lastMethod)
	}
	if bytes.Compare(lastBody, wantBody) != 0 {
		t.Errorf("got body %v, want %v", lastBody, wantBody)
	}
	if lastPath != "/metrics/job/testjob/a/x/b/y" && lastPath != "/metrics/job/testjob/b/y/a/x" {
		t.Error("unexpected path:", lastPath)
	}
}

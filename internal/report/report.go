// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package report provides functions to report OPA's version information to an external service and process the response.
package report

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/internal/semver"
	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/version"

	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/util"
)

// ExternalServiceURL is the base HTTP URL for a github instance used
// to query for more recent version.
// If not otherwise specified, it will use the hard-coded default, api.github.com.
// GHRepo is the repository to use, and defaults to "open-policy-agent/opa"
//
// Override at build time via:
//
//	-ldflags "-X github.com/open-policy-agent/opa/internal/report.ExternalServiceURL=<url>"
//	-ldflags "-X github.com/open-policy-agent/opa/internal/report.GHRepo=<url>"
//
// ExternalServiceURL will be overridden if the OPA_TELEMETRY_SERVICE_URL environment variable
// is provided.
var ExternalServiceURL = "https://api.github.com"
var GHRepo = "open-policy-agent/opa"

// Reporter reports information such as the version, heap usage about the running OPA instance to an external service
type Reporter interface {
	SendReport(ctx context.Context) (*DataResponse, error)
	RegisterGatherer(key string, f Gatherer)
}

// Gatherer represents a mechanism to inject additional data in the telemetry report
type Gatherer func(ctx context.Context) (any, error)

// DataResponse represents the data returned by the external service
type DataResponse struct {
	Latest ReleaseDetails `json:"latest"`
}

// ReleaseDetails holds information about the latest OPA release
type ReleaseDetails struct {
	Download      string `json:"download,omitempty"`       // link to download the OPA release
	ReleaseNotes  string `json:"release_notes,omitempty"`  // link to the OPA release notes
	LatestRelease string `json:"latest_release,omitempty"` // latest OPA released version
	OPAUpToDate   bool   `json:"opa_up_to_date,omitempty"` // is running OPA version greater than or equal to the latest released
}

// Options supplies parameters to the reporter.
type Options struct {
	Logger logging.Logger
}

type GHVersionCollector struct {
	client rest.Client
}

type GHResponse struct {
	TagName      string `json:"tag_name,omitempty"`   // latest OPA release tag
	ReleaseNotes string `json:"html_url,omitempty"`   // link to the OPA release notes
	Download     string `json:"assets_url,omitempty"` // link to download the OPA release
}

// New returns an instance of the Reporter
func New(opts Options) (Reporter, error) {
	r := GHVersionCollector{}

	url := cmp.Or(os.Getenv("OPA_TELEMETRY_SERVICE_URL"), ExternalServiceURL)

	restConfig := fmt.Appendf(nil, `{
		"url": %q,
	}`, url)

	client, err := rest.New(restConfig, map[string]*keys.Config{}, rest.Logger(opts.Logger))
	if err != nil {
		return nil, err
	}
	r.client = client

	// heap_usage_bytes is always present, so register it unconditionally
	r.RegisterGatherer("heap_usage_bytes", readRuntimeMemStats)

	return &r, nil
}

// SendReport sends the telemetry report which includes information such as the OPA version, current memory usage to
// the external service
func (r *GHVersionCollector) SendReport(ctx context.Context) (*DataResponse, error) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := r.client.Do(rCtx, "GET", fmt.Sprintf("/repos/%s/releases/latest", GHRepo))
	if err != nil {
		return nil, err
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		if resp.Body != nil {
			var result GHResponse
			err := json.NewDecoder(resp.Body).Decode(&result)
			if err != nil {
				return nil, err
			}
			return createDataResponse(result)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("server replied with HTTP %v", resp.StatusCode)
	}
}

func createDataResponse(ghResp GHResponse) (*DataResponse, error) {
	if ghResp.TagName == "" {
		return nil, errors.New("server response does not contain tag_name")
	}

	v := strings.TrimPrefix(version.Version, "v")
	sv, err := semver.NewVersion(v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version %q: %w", v, err)
	}

	latestV := strings.TrimPrefix(ghResp.TagName, "v")
	latestSV, err := semver.NewVersion(latestV)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version %q: %w", latestV, err)
	}

	isLatest := sv.Compare(*latestSV) >= 0

	// Note: alternatively, we could look through the assets in the GH API response to find a matching asset,
	// and use its URL. However, this is not guaranteed to be more robust, and wouldn't use the 'openpolicyagent.org' domain.
	downloadLink := fmt.Sprintf("https://openpolicyagent.org/downloads/%v/opa_%v_%v",
		ghResp.TagName, runtime.GOOS, runtime.GOARCH)

	if runtime.GOARCH == "arm64" {
		downloadLink = fmt.Sprintf("%v_static", downloadLink)
	}

	if strings.HasPrefix(runtime.GOOS, "win") {
		downloadLink = fmt.Sprintf("%v.exe", downloadLink)
	}

	return &DataResponse{
		Latest: ReleaseDetails{
			Download:      downloadLink,
			ReleaseNotes:  ghResp.ReleaseNotes,
			LatestRelease: ghResp.TagName,
			OPAUpToDate:   isLatest,
		},
	}, nil
}

func (*GHVersionCollector) RegisterGatherer(_ string, _ Gatherer) {
	// no-op for this implementation
}

// IsSet returns true if dr is populated.
func (dr *DataResponse) IsSet() bool {
	return dr != nil && dr.Latest.LatestRelease != "" && dr.Latest.Download != "" && dr.Latest.ReleaseNotes != ""
}

// Slice returns the dr as a slice of key-value string pairs. If dr is nil, this function returns an empty slice.
func (dr *DataResponse) Slice() [][2]string {

	if !dr.IsSet() {
		return nil
	}

	return [][2]string{
		{"Latest Upstream Version", strings.TrimPrefix(dr.Latest.LatestRelease, "v")},
		{"Download", dr.Latest.Download},
		{"Release Notes", dr.Latest.ReleaseNotes},
	}
}

// Pretty returns OPA release information in a human-readable format.
func (dr *DataResponse) Pretty() string {
	if !dr.IsSet() {
		return ""
	}

	pairs := dr.Slice()
	lines := make([]string, 0, len(pairs))

	for _, pair := range pairs {
		lines = append(lines, fmt.Sprintf("%v: %v", pair[0], pair[1]))
	}

	return strings.Join(lines, "\n")
}

func readRuntimeMemStats(_ context.Context) (any, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return strconv.FormatUint(m.Alloc, 10), nil
}

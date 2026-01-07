// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package versioncheck provides functions to check for the latest OPA release version from GitHub.
package versioncheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
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
//	-ldflags "-X github.com/open-policy-agent/opa/internal/versioncheck.ExternalServiceURL=<url>"
//	-ldflags "-X github.com/open-policy-agent/opa/internal/versioncheck.GHRepo=<url>"
//
// ExternalServiceURL will be overridden if the OPA_VERSION_CHECK_SERVICE_URL environment variable
// is provided.
var ExternalServiceURL = "https://api.github.com"
var GHRepo = "open-policy-agent/opa"

// Checker checks for the latest OPA release version
type Checker interface {
	LatestVersion(ctx context.Context) (*DataResponse, error)
	RegisterGatherer(key string, f Gatherer)
}

// Gatherer represents a mechanism to inject additional data (currently unused for version checking)
type Gatherer func(ctx context.Context) (any, error)

// DataResponse represents the data returned by the version check
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

// Options supplies parameters to the version checker.
type Options struct {
	Logger logging.Logger
}

type GitHubVersionChecker struct {
	client rest.Client
}

type GitHubRelease struct {
	TagName      string `json:"tag_name,omitempty"`   // latest OPA release tag
	ReleaseNotes string `json:"html_url,omitempty"`   // link to the OPA release notes
	Download     string `json:"assets_url,omitempty"` // link to download the OPA release
}

// New returns an instance of the Checker
func New(opts Options) (Checker, error) {
	url := os.Getenv("OPA_VERSION_CHECK_SERVICE_URL")
	if url == "" {
		url = ExternalServiceURL
	}

	// Set a generic User-Agent to avoid sending version/platform information about the user's OPA instance.
	// This ensures we only retrieve version information without transmitting any identifying data.
	restConfig := fmt.Appendf(nil, `{
		"url": %q,
		"headers": {
			"User-Agent": "OPA-Version-Checker"
		}
	}`, url)

	client, err := rest.New(restConfig, map[string]*keys.Config{}, rest.Logger(opts.Logger))
	if err != nil {
		return nil, err
	}
	r := GitHubVersionChecker{client: client}

	return &r, nil
}

// LatestVersion queries the GitHub API to check for the latest OPA release version
func (r *GitHubVersionChecker) LatestVersion(ctx context.Context) (*DataResponse, error) {
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
			var result GitHubRelease
			err := json.NewDecoder(resp.Body).Decode(&result)
			if err != nil {
				return nil, err
			}
			return createReleaseInfo(result)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("server replied with HTTP %v", resp.StatusCode)
	}
}

func createReleaseInfo(ghResp GitHubRelease) (*DataResponse, error) {
	if ghResp.TagName == "" {
		return nil, errors.New("server response does not contain tag_name")
	}

	sv, err := semver.Parse(version.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version %q: %w", version.Version, err)
	}

	latestSV, err := semver.Parse(ghResp.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest version %q: %w", ghResp.TagName, err)
	}

	isLatest := sv.Compare(latestSV) >= 0

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

func (*GitHubVersionChecker) RegisterGatherer(_ string, _ Gatherer) {
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

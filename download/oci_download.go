//go:build !opa_no_oci

package download

import (
	v1 "github.com/open-policy-agent/opa/v1/download"

	"github.com/open-policy-agent/opa/plugins/rest"
)

// NewOCI returns a new Downloader that can be started.
func NewOCI(config Config, client rest.Client, path, storePath string) *OCIDownloader {
	return v1.NewOCI(config, client, path, storePath)
}

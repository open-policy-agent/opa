package download

import (
	"context"
	"sync"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/plugins/rest"
	"oras.land/oras-go/v2/content/oci"
)

type OCIDownloader struct {
	config         Config                        // downloader configuration for tuning polling and other downloader behaviour
	client         rest.Client                   // HTTP client to use for bundle downloading
	path           string                        // path for OCI image as <registry>/<org>/<repo>:<tag>
	localStorePath string                        // path for the local OCI storage
	trigger        chan chan struct{}            // channel to signal downloads when manual triggering is enabled
	stop           chan chan struct{}            // used to signal plugin to stop running
	f              func(context.Context, Update) // callback function invoked when download updates occur
	sizeLimitBytes *int64                        // max bundle file size in bytes (passed to reader)
	bvc            *bundle.VerificationConfig
	wg             sync.WaitGroup
	logger         logging.Logger
	mtx            sync.Mutex
	stopped        bool
	persist        bool
	store          *oci.Store
	etag           string
}

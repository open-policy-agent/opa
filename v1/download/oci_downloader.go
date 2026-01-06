package download

import (
	"context"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"oras.land/oras-go/v2/content/oci"
)

type OCIDownloader struct {
	config           Config
	logger           logging.Logger
	bvc              *bundle.VerificationConfig
	store            *oci.Store
	trigger          chan chan struct{}
	stop             chan chan struct{}
	f                func(context.Context, Update) error
	sizeLimitBytes   *int64
	path             string
	localStorePath   string
	etag             string
	client           rest.Client
	bundleParserOpts ast.ParserOptions
	wg               sync.WaitGroup
	mtx              sync.Mutex
	persist          bool
	stopped          bool
}

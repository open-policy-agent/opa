//go:build opa_no_oci

package download

import (
	"context"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
)

func NewOCI(Config, rest.Client, string, string) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) WithCallback(f func(context.Context, Update)) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) WithLogAttrs(map[string]any) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) WithBundleVerificationConfig(*bundle.VerificationConfig) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) WithSizeLimitBytes(int64) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) WithBundlePersistence(bool) *OCIDownloader {
	panic("built without OCI support")
}

func (d *OCIDownloader) ClearCache() {
	panic("built without OCI support")
}

func (d *OCIDownloader) SetCache(string) {
	panic("built without OCI support")
}

func (d *OCIDownloader) Trigger(context.Context) error {
	panic("built without OCI support")
}

func (d *OCIDownloader) Start(context.Context) {
	panic("built without OCI support")
}

func (d *OCIDownloader) Stop(context.Context) {
	panic("built without OCI support")
}

func (*OCIDownloader) WithBundleParserOpts(ast.ParserOptions) *OCIDownloader {
	panic("built without OCI support")
}

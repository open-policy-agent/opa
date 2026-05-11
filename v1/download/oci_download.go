//go:build !opa_no_oci

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/util"
)

// NewOCI returns a new Downloader that can be started.
func NewOCI(config Config, client rest.Client, path, storePath string) *OCIDownloader {
	localstore, err := oci.New(storePath)
	if err != nil {
		panic(err)
	}
	return &OCIDownloader{
		config:         config,
		path:           path,
		localStorePath: storePath,
		client:         client,
		trigger:        make(chan chan struct{}),
		stop:           make(chan chan struct{}),
		logger:         client.Logger(),
		store:          localstore,
	}
}

// WithCallback registers a function f to be called when download updates occur.
func (d *OCIDownloader) WithCallback(f func(context.Context, Update) error) *OCIDownloader {
	d.f = f
	return d
}

// WithLogAttrs sets an optional set of key/value pair attributes to include in
// log messages emitted by the downloader.
func (d *OCIDownloader) WithLogAttrs(attrs map[string]any) *OCIDownloader {
	d.logger = d.logger.WithFields(attrs)
	return d
}

// WithBundleVerificationConfig sets the key configuration used to verify a signed bundle
func (d *OCIDownloader) WithBundleVerificationConfig(config *bundle.VerificationConfig) *OCIDownloader {
	d.bvc = config
	return d
}

// WithSizeLimitBytes sets the file size limit for bundles read by this downloader.
func (d *OCIDownloader) WithSizeLimitBytes(n int64) *OCIDownloader {
	d.sizeLimitBytes = &n
	return d
}

// WithBundlePersistence specifies if the downloaded bundle will eventually be persisted to disk.
func (d *OCIDownloader) WithBundlePersistence(persist bool) *OCIDownloader {
	d.persist = persist
	return d
}

// WithBundleParserOpts specifies the parser options to use when parsing downloaded bundles.
func (d *OCIDownloader) WithBundleParserOpts(opts ast.ParserOptions) *OCIDownloader {
	d.bundleParserOpts = opts
	return d
}

// ClearCache is deprecated. Use SetCache instead.
func (*OCIDownloader) ClearCache() {
}

// SetCache sets the etag value to the SHA of the loaded bundle
func (d *OCIDownloader) SetCache(etag string) {
	d.etag = etag
}

// Trigger can be used to control when the downloader attempts to download
// a new bundle in manual triggering mode.
func (d *OCIDownloader) Trigger(ctx context.Context) error {
	done := make(chan error)

	go func() {
		err := d.oneShot(ctx)
		if err != nil {
			d.logger.Error("OCI - Bundle download failed: %v.", err)
			if ctx.Err() == nil {
				done <- err
			}
		}
		close(done)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start tells the Downloader to begin downloading bundles.
func (d *OCIDownloader) Start(ctx context.Context) {
	if *d.config.Trigger == plugins.TriggerPeriodic {
		go d.doStart(ctx)
	}
}

// Stop tells the Downloader to stop downloading bundles.
func (d *OCIDownloader) Stop(context.Context) {
	if *d.config.Trigger == plugins.TriggerManual {
		return
	}

	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.stopped {
		return
	}

	done := make(chan struct{})
	d.stop <- done
	<-done
}

func (d *OCIDownloader) doStart(context.Context) {
	// We'll revisit context passing/usage later.
	ctx, cancel := context.WithCancel(context.Background())

	d.wg.Add(1)
	go d.loop(ctx)

	done := <-d.stop // blocks until there's something to read
	cancel()
	d.wg.Wait()
	d.stopped = true
	close(done)
}

func (d *OCIDownloader) loop(ctx context.Context) {
	defer d.wg.Done()

	var retry int

	for {

		var delay time.Duration

		err := d.oneShot(ctx)

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*d.config.Polling.parsedMaxDelaySeconds), retry)
		} else {
			// revert the response header timeout value on the http client's transport
			min := float64(*d.config.Polling.parsedMinDelaySeconds)
			max := float64(*d.config.Polling.parsedMaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		}

		d.logger.Debug("OCI - Waiting %v before next download/retry.", delay)

		timer, timerCancel := util.TimerWithCancel(delay)
		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case <-ctx.Done():
			timerCancel() // explicitly cancel the timer.
			return
		}
	}
}

func (d *OCIDownloader) oneShot(ctx context.Context) error {
	m := metrics.New()
	resp, err := d.download(ctx, m)
	if err != nil {
		if d.f != nil {
			err = errors.Join(err, d.f(ctx, Update{ETag: "", Bundle: nil, Error: err, Metrics: m, Raw: nil}))
		}
		return err
	}
	d.SetCache(resp.etag) // set the current etag sha to the cache

	if d.f != nil {
		if err := d.f(ctx, Update{ETag: resp.etag, Bundle: resp.b, Error: nil, Metrics: m, Raw: resp.raw, Size: resp.size}); err != nil {
			return err
		}
	}
	return nil
}

func (d *OCIDownloader) download(ctx context.Context, m metrics.Metrics) (*downloaderResponse, error) {
	d.logger.Debug("OCI - Download starting.")
	var buf bytes.Buffer

	preferences := []string{fmt.Sprintf("modes=%v,%v", defaultBundleMode, deltaBundleMode)}

	preferValue := strings.Join(preferences, ";")
	d.client = d.client.WithHeader("Prefer", preferValue)

	m.Timer(metrics.BundleRequest).Start()
	desc, err := d.pull(ctx, d.path)
	if err != nil {
		return &downloaderResponse{}, fmt.Errorf("failed to pull %s: %w", d.path, err)
	}

	manifest, err := manifestFromDesc(ctx, d.store, desc)
	if err != nil {
		return nil, err
	}

	tarballDescriptor := ocispec.Descriptor{}
	for _, descriptor := range manifest.Layers {
		if descriptor.MediaType == "application/vnd.oci.image.layer.v1.tar+gzip" {
			tarballDescriptor = descriptor
			break
		}
	}
	if tarballDescriptor.MediaType == "" {
		return nil, errors.New("no tarball descriptor found in the layers")
	}
	etag := tarballDescriptor.Digest.Hex()
	bundleFilePath := filepath.Join(d.localStorePath, "blobs", "sha256", etag)
	// if the downloader etag sha is the same with digest of the tarball it was already loaded
	if d.etag == etag {
		return &downloaderResponse{
			b:        nil,
			raw:      nil,
			etag:     etag,
			longPoll: false,
		}, nil
	}
	fileReader, err := os.Open(bundleFilePath)

	cnt := &count{}
	r := io.TeeReader(fileReader, cnt)
	tee := io.TeeReader(r, &buf)

	if err != nil {
		return nil, err
	}
	loader := bundle.NewTarballLoaderWithBaseURL(tee, d.localStorePath)
	reader := bundle.NewCustomReader(loader).
		WithMetrics(m).
		WithBundleVerificationConfig(d.bvc).
		WithBundleEtag(etag).
		WithRegoVersion(d.bundleParserOpts.RegoVersion).
		WithProcessAnnotations(d.bundleParserOpts.ProcessAnnotation)
	bundleInfo, err := reader.Read()
	if err != nil {
		return &downloaderResponse{}, fmt.Errorf("unexpected error %w", err)
	}

	m.Timer(metrics.BundleRequest).Stop()

	return &downloaderResponse{
		b:        &bundleInfo,
		raw:      &buf,
		etag:     etag,
		longPoll: false,
		size:     cnt.Bytes(),
	}, nil
}

func (d *OCIDownloader) pull(ctx context.Context, ref string) (*ocispec.Descriptor, error) {
	lookup := d.client.AuthPluginLookup()

	plugin, err := d.client.Config().AuthPlugin(lookup)
	if err != nil {
		return nil, fmt.Errorf("failed to look up auth plugin: %w", err)
	}

	d.logger.Debug("OCIDownloader: using auth plugin: %T", plugin)

	target, err := newOCITarget(plugin, d.client.Config(), ref)
	if err != nil {
		return nil, fmt.Errorf("invalid host url %s: %w", d.client.Config().URL, err)
	}

	parsed, err := registry.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
	}

	manifestDescriptor, err := oraslib.Copy(ctx, target, parsed.Reference, d.store, "", oraslib.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("download for '%s' failed: %w", ref, err)
	}

	return &manifestDescriptor, nil
}

// ociTarget implements oraslib.ReadOnlyTarget using ORAS's auth.Client for HTTP
// authentication without the strict response validation of remote.Repository.
// Notably, it does NOT implement registry.ReferenceFetcher, forcing oraslib.Copy
// to use the HEAD (Resolve) + GET-by-digest (Fetch) path.
type ociTarget struct {
	client    *auth.Client
	registry  string
	repo      string
	plainHTTP bool
}

func newOCITarget(plugin rest.HTTPAuthPlugin, config *rest.Config, ref string) (*ociTarget, error) {
	httpClient, err := plugin.NewClient(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	urlInfo, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	parsed, err := registry.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	return &ociTarget{
		client: &auth.Client{
			Client: &http.Client{
				Transport: &pluginRoundTripper{
					base:   httpClient.Transport,
					plugin: plugin,
				},
			},
			Cache: auth.NewCache(),
		},
		registry:  urlInfo.Host,
		repo:      parsed.Repository,
		plainHTTP: urlInfo.Scheme == "http",
	}, nil
}

func (t *ociTarget) url(path string) string {
	scheme := "https"
	if t.plainHTTP {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/v2/%s/%s", scheme, t.registry, t.repo, path)
}

func (t *ociTarget) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	url := t.url("manifests/" + reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ocispec.Descriptor{}, fmt.Errorf("%s %s: %d %s", req.Method, url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	mediaType := resp.Header.Get("Content-Type")
	digestStr := resp.Header.Get("Docker-Content-Digest")
	if digestStr == "" {
		return ocispec.Descriptor{}, errors.New("missing Docker-Content-Digest header in response")
	}
	dgst, err := digest.Parse(digestStr)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("invalid digest %q: %w", digestStr, err)
	}

	return ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    dgst,
		Size:      resp.ContentLength,
	}, nil
}

func (t *ociTarget) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	// Use blobs endpoint for non-manifest content, manifests endpoint for manifests
	var url string
	switch target.MediaType {
	case "application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json":
		url = t.url("manifests/" + target.Digest.String())
	default:
		url = t.url("blobs/" + target.Digest.String())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s %s: %d %s", req.Method, url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return resp.Body, nil
}

func (t *ociTarget) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	rc, err := t.Fetch(ctx, target)
	if err != nil {
		return false, nil
	}
	rc.Close()
	return true, nil
}

// pluginRoundTripper injects authentication headers via the rest.HTTPAuthPlugin
// on requests that don't already carry an Authorization header. This allows ORAS's
// auth.Client to handle Docker token exchange challenges while still using the
// plugin's credentials for both direct auth and token-service authentication.
type pluginRoundTripper struct {
	base   http.RoundTripper
	plugin rest.HTTPAuthPlugin
}

func (t *pluginRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") == "" {
		if err := t.plugin.Prepare(req); err != nil {
			return nil, fmt.Errorf("failed to prepare request: %w", err)
		}
	}

	if t.base != nil {
		return t.base.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func manifestFromDesc(ctx context.Context, target oraslib.Target, desc *ocispec.Descriptor) (*ocispec.Manifest, error) {
	var manifest ocispec.Manifest

	descReader, err := target.Fetch(ctx, *desc)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch descriptor with digest %q: %w", desc.Digest, err)
	}
	defer descReader.Close()

	descBytes, err := io.ReadAll(descReader)
	if err != nil {
		return nil, fmt.Errorf("unable to read bytes from descriptor: %w", err)
	}

	if err = json.Unmarshal(descBytes, &manifest); err != nil {
		return nil, fmt.Errorf("unable to unmarshal manifest: %w", err)
	}

	if len(manifest.Layers) < 1 {
		return nil, errors.New("no layers in manifest")
	}

	return &manifest, nil
}

//go:build !opa_no_oci

package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oraslib "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/util"
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
func (d *OCIDownloader) WithCallback(f func(context.Context, Update)) *OCIDownloader {
	d.f = f
	return d
}

// WithLogAttrs sets an optional set of key/value pair attributes to include in
// log messages emitted by the downloader.
func (d *OCIDownloader) WithLogAttrs(attrs map[string]interface{}) *OCIDownloader {
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

// ClearCache is deprecated. Use SetCache instead.
func (d *OCIDownloader) ClearCache() {
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
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*d.config.Polling.MaxDelaySeconds), retry)
		} else {
			// revert the response header timeout value on the http client's transport
			min := float64(*d.config.Polling.MinDelaySeconds)
			max := float64(*d.config.Polling.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		}

		d.logger.Debug("OCI - Waiting %v before next download/retry.", delay)

		select {
		case <-time.After(delay):
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *OCIDownloader) oneShot(ctx context.Context) error {
	m := metrics.New()
	resp, err := d.download(ctx, m)
	if err != nil {
		if d.f != nil {
			d.f(ctx, Update{ETag: "", Bundle: nil, Error: err, Metrics: m, Raw: nil})
		}
		return err
	}
	d.SetCache(resp.etag) // set the current etag sha to the cache

	if d.f != nil {
		d.f(ctx, Update{ETag: resp.etag, Bundle: resp.b, Error: nil, Metrics: m, Raw: resp.raw})
	}
	return nil
}

func (d *OCIDownloader) download(ctx context.Context, m metrics.Metrics) (*downloaderResponse, error) {
	d.logger.Debug("OCI - Download starting.")

	preferences := []string{fmt.Sprintf("modes=%v,%v", defaultBundleMode, deltaBundleMode)}

	preferValue := fmt.Sprintf("%v", strings.Join(preferences, ";"))
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
		return nil, fmt.Errorf("no tarball descriptor found in the layers")
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
	if err != nil {
		return nil, err
	}
	loader := bundle.NewTarballLoaderWithBaseURL(fileReader, d.localStorePath)
	reader := bundle.NewCustomReader(loader).
		WithMetrics(m).
		WithBundleVerificationConfig(d.bvc).
		WithBundleEtag(etag)
	bundleInfo, err := reader.Read()
	if err != nil {
		return &downloaderResponse{}, fmt.Errorf("unexpected error %w", err)
	}

	m.Timer(metrics.BundleRequest).Stop()

	return &downloaderResponse{
		b:        &bundleInfo,
		raw:      fileReader,
		etag:     etag,
		longPoll: false,
	}, nil
}

func (d *OCIDownloader) pull(ctx context.Context, ref string) (*ocispec.Descriptor, error) {
	lookup := d.client.AuthPluginLookup()

	plugin, err := d.client.Config().AuthPlugin(lookup)
	if err != nil {
		return nil, fmt.Errorf("failed to look up auth plugin: %w", err)
	}

	d.logger.Debug("OCIDownloader: using auth plugin: %T", plugin)

	resolver, err := dockerResolver(plugin, d.client.Config(), d.logger)
	if err != nil {
		return nil, fmt.Errorf("invalid host url %s: %w", d.client.Config().URL, err)
	}

	target := remoteManager{
		resolver: resolver,
		srcRef:   ref,
	}

	manifestDescriptor, err := oraslib.Copy(ctx, &target, ref, d.store, "", oraslib.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("download for '%s' failed: %w", ref, err)
	}

	return &manifestDescriptor, nil
}

func dockerResolver(plugin rest.HTTPAuthPlugin, config *rest.Config, logger logging.Logger) (remotes.Resolver, error) {
	client, err := plugin.NewClient(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	urlInfo, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	authorizer := pluginAuthorizer{
		plugin: plugin,
		client: client,
		logger: logger,
	}

	registryHost := docker.RegistryHost{
		Host:         urlInfo.Host,
		Scheme:       urlInfo.Scheme,
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush,
		Client:       client,
		Path:         "/v2",
		Authorizer:   &authorizer,
	}

	opts := docker.ResolverOptions{
		Hosts: func(string) ([]docker.RegistryHost, error) {
			return []docker.RegistryHost{registryHost}, nil
		},
	}

	return docker.NewResolver(opts), nil
}

type pluginAuthorizer struct {
	plugin rest.HTTPAuthPlugin
	client *http.Client

	// authorizer will be populated by the first call to pluginAuthorizer.Prepare
	// since it requires a first pass through the plugin.Prepare method.
	authorizer docker.Authorizer

	logger logging.Logger
}

var _ docker.Authorizer = &pluginAuthorizer{}

func (a *pluginAuthorizer) AddResponses(ctx context.Context, responses []*http.Response) error {
	return a.authorizer.AddResponses(ctx, responses)
}

// Authorize uses a rest.HTTPAuthPlugin to Prepare a request before passing it on
// to the docker.Authorizer.
func (a *pluginAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	if err := a.plugin.Prepare(req); err != nil {
		err = fmt.Errorf("failed to prepare docker request: %w", err)

		// Make sure to log this before passing the error back to docker
		a.logger.Error(err.Error())

		return err
	}

	if a.authorizer == nil {
		// Some registry authentication implementations require a token fetch from
		// a separate authenticated token server. This flow is described in the
		// docker token auth spec:
		// https://docs.docker.com/registry/spec/auth/token/#requesting-a-token
		//
		// Unfortunately, the containerd implementation does not use the Prepare
		// mechanism to authenticate these token requests and we need to add
		// auth information in form of a static docker.WithAuthHeader.
		//
		// Since rest.HTTPAuthPlugins will set the auth header on the request
		// passed to HTTPAuthPlugin.Prepare, we can use it afterwards to build
		// our docker.Authorizer.
		a.authorizer = docker.NewDockerAuthorizer(
			docker.WithAuthHeader(req.Header),
			docker.WithAuthClient(a.client),
		)
	}

	return a.authorizer.Authorize(ctx, req)
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
		return nil, fmt.Errorf("no layers in manifest")
	}

	return &manifest, nil
}

type remoteManager struct {
	resolver remotes.Resolver
	srcRef   string
}

func (r *remoteManager) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	_, desc, err := r.resolver.Resolve(ctx, ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

func (r *remoteManager) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	fetcher, err := r.resolver.Fetcher(ctx, r.srcRef)
	if err != nil {
		return nil, err
	}
	return fetcher.Fetch(ctx, target)
}

func (r *remoteManager) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	_, err := r.Fetch(ctx, target)
	if err == nil {
		return true, nil
	}

	return !errdefs.IsNotFound(err), err
}

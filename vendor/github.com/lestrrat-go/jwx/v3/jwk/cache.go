package jwk

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/lestrrat-go/httprc/v3"
)

type HTTPClient = httprc.HTTPClient
type ErrorSink = httprc.ErrorSink
type TraceSink = httprc.TraceSink

// Cache is a container built on top of github.com/lestrrat-go/httprc/v3
// that keeps track of Set object by their source URLs.
// The Set objects are stored in memory, and are refreshed automatically
// behind the scenes.
//
// Before retrieving the Set objects, the user must pre-register the
// URLs they intend to use by calling `Register()`
//
//	c := jwk.NewCache(ctx, httprc.NewClient())
//	c.Register(ctx, url, options...)
//
// Once registered, you can call `Get()` to retrieve the Set object.
//
// All JWKS objects that are retrieved via this mechanism should be
// treated read-only, as they are shared among all consumers, as well
// as the `jwk.Cache` object.
//
// There are cases where `jwk.Cache` and `jwk.CachedSet` should and
// should not be used.
//
// First and foremost, do NOT use a cache for those JWKS objects that
// need constant checking. For example, unreliable or user-provided JWKS (i.e. those
// JWKS that are not from a well-known provider) should not be fetched
// through a `jwk.Cache` or `jwk.CachedSet`.
//
// For example, if you have a flaky JWKS server for development
// that can go down often, you should consider alternatives such as
// providing `http.Client` with a caching `http.RoundTripper` configured
// (see `jwk.WithHTTPClient`), setting up a reverse proxy, etc.
// These techniques allow you to set up a more robust way to both cache
// and report precise causes of the problems than using `jwk.Cache` or
// `jwk.CachedSet`. If you handle the caching at the HTTP level like this,
// you will be able to use a simple `jwk.Fetch` call and not worry about the cache.
//
// User-provided JWKS objects may also be problematic, as it may go down
// unexpectedly (and frequently!), and it will be hard to detect when
// the URLs or its contents are swapped.
//
// A good use-case for `jwk.Cache` and `jwk.CachedSet` are for "stable"
// JWKS objects.
//
// When we say "stable", we are thinking of JWKS that should mostly be
// ALWAYS available. A good example are those JWKS objects provided by
// major cloud providers such as Google Cloud, AWS, or Azure.
// Stable JWKS may still experience intermittent network connectivity problems,
// but you can expect that they will eventually recover in relatively
// short period of time. They rarely change URLs, and the contents are
// expected to be valid or otherwise it would cause havoc to those providers
//
// We also know that these stable JWKS objects are rotated periodically,
// which is a perfect use for `jwk.Cache` and `jwk.CachedSet`. The caches
// can be configured to periodically refresh the JWKS thereby keeping them
// fresh without extra intervention from the developer.
//
// Notice that for these recommended use-cases the requirement to check
// the validity or the availability of the JWKS objects are non-existent,
// as it is expected that they will be available and will be valid. The
// caching mechanism can hide intermittent connectivity problems as well
// as keep the objects mostly fresh.
type Cache struct {
	ctrl httprc.Controller
}

// Transformer is a specialized version of `httprc.Transformer` that implements
// conversion from a `http.Response` object to a `jwk.Set` object. Use this in
// conjection with `httprc.NewResource` to create a `httprc.Resource` object
// to auto-update `jwk.Set` objects.
type Transformer struct {
	parseOptions []ParseOption
}

func (t Transformer) Transform(_ context.Context, res *http.Response) (Set, error) {
	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(`failed to read response body status: %w`, err)
	}

	set, err := Parse(buf, t.parseOptions...)
	if err != nil {
		return nil, fmt.Errorf(`failed to parse JWK set at %q: %w`, res.Request.URL.String(), err)
	}

	return set, nil
}

// NewCache creates a new `jwk.Cache` object.
//
// Under the hood, `jwk.Cache` uses `httprc.Client` manage the
// fetching and caching of JWKS objects, and thus spawns multiple goroutines
// per `jwk.Cache` object.
//
// The provided `httprc.Client` object must NOT be started prior to
// passing it to `jwk.NewCache`. The `jwk.Cache` object will start
// the `httprc.Client` object on its own.
func NewCache(ctx context.Context, client *httprc.Client) (*Cache, error) {
	ctrl, err := client.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf(`failed to start httprc.Client: %w`, err)
	}

	return &Cache{
		ctrl: ctrl,
	}, nil
}

// Register registers a URL to be managed by the cache. URLs must
// be registered before issuing `Get`
//
// The `Register` method is a thin wrapper around `(httprc.Controller).Add`
func (c *Cache) Register(ctx context.Context, u string, options ...RegisterOption) error {
	var parseOptions []ParseOption
	var resourceOptions []httprc.NewResourceOption
	waitReady := true
	for _, option := range options {
		switch option := option.(type) {
		case ParseOption:
			parseOptions = append(parseOptions, option)
		case ResourceOption:
			var v httprc.NewResourceOption
			if err := option.Value(&v); err != nil {
				return fmt.Errorf(`failed to retrieve NewResourceOption option value: %w`, err)
			}
			resourceOptions = append(resourceOptions, v)
		default:
			switch option.Ident() {
			case identHTTPClient{}:
				var cli HTTPClient
				if err := option.Value(&cli); err != nil {
					return fmt.Errorf(`failed to retrieve HTTPClient option value: %w`, err)
				}
				resourceOptions = append(resourceOptions, httprc.WithHTTPClient(cli))
			case identWaitReady{}:
				if err := option.Value(&waitReady); err != nil {
					return fmt.Errorf(`failed to retrieve WaitReady option value: %w`, err)
				}
			}
		}
	}

	r, err := httprc.NewResource[Set](u, &Transformer{
		parseOptions: parseOptions,
	}, resourceOptions...)
	if err != nil {
		return fmt.Errorf(`failed to create httprc.Resource: %w`, err)
	}
	if err := c.ctrl.Add(ctx, r, httprc.WithWaitReady(waitReady)); err != nil {
		return fmt.Errorf(`failed to add resource to httprc.Client: %w`, err)
	}

	return nil
}

// LookupResource returns the `httprc.Resource` object associated with the
// given URL `u`. If the URL has not been registered, an error is returned.
func (c *Cache) LookupResource(ctx context.Context, u string) (*httprc.ResourceBase[Set], error) {
	r, err := c.ctrl.Lookup(ctx, u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	//nolint:forcetypeassert
	return r.(*httprc.ResourceBase[Set]), nil
}

func (c *Cache) Lookup(ctx context.Context, u string) (Set, error) {
	r, err := c.LookupResource(ctx, u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	set := r.Resource()
	if set == nil {
		return nil, fmt.Errorf(`resource %q is not ready`, u)
	}
	return set, nil
}

func (c *Cache) Ready(ctx context.Context, u string) bool {
	r, err := c.LookupResource(ctx, u)
	if err != nil {
		return false
	}
	if err := r.Ready(ctx); err != nil {
		return false
	}
	return true
}

// Refresh is identical to Get(), except it always fetches the
// specified resource anew, and updates the cached content
//
// Please refer to the documentation for `(httprc.Cache).Refresh` for
// more details
func (c *Cache) Refresh(ctx context.Context, u string) (Set, error) {
	if err := c.ctrl.Refresh(ctx, u); err != nil {
		return nil, fmt.Errorf(`failed to refresh resource %q: %w`, u, err)
	}
	return c.Lookup(ctx, u)
}

// IsRegistered returns true if the given URL `u` has already been registered
// in the cache.
func (c *Cache) IsRegistered(ctx context.Context, u string) bool {
	_, err := c.LookupResource(ctx, u)
	return err == nil
}

// Unregister removes the given URL `u` from the cache.
func (c *Cache) Unregister(ctx context.Context, u string) error {
	return c.ctrl.Remove(ctx, u)
}

// CachedSet is a thin shim over jwk.Cache that allows the user to cloak
// jwk.Cache as if it's a `jwk.Set`. Behind the scenes, the `jwk.Set` is
// retrieved from the `jwk.Cache` for every operation.
//
// Since `jwk.CachedSet` always deals with a cached version of the `jwk.Set`,
// all operations that mutate the object (such as AddKey(), RemoveKey(), et. al)
// are no-ops and return an error.
//
// Note that since this is a utility shim over `jwk.Cache`, you _will_ lose
// the ability to control the finer details (such as controlling how long to
// wait for in case of a fetch failure using `context.Context`)
//
// Make sure that you read the documentation for `jwk.Cache` as well.
type CachedSet interface {
	Set
	cached() (Set, error) // used as a marker
}

type cachedSet struct {
	r *httprc.ResourceBase[Set]
}

func (c *Cache) CachedSet(u string) (CachedSet, error) {
	r, err := c.LookupResource(context.Background(), u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	return NewCachedSet(r), nil
}

func NewCachedSet(r *httprc.ResourceBase[Set]) CachedSet {
	return &cachedSet{
		r: r,
	}
}

func (cs *cachedSet) cached() (Set, error) {
	if err := cs.r.Ready(context.Background()); err != nil {
		return nil, fmt.Errorf(`failed to fetch resource: %w`, err)
	}
	return cs.r.Resource(), nil
}

// Add is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*cachedSet) AddKey(_ Key) error {
	return fmt.Errorf(`(jwk.Cachedset).AddKey: jwk.CachedSet is immutable`)
}

// Clear is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*cachedSet) Clear() error {
	return fmt.Errorf(`(jwk.cachedSet).Clear: jwk.CachedSet is immutable`)
}

// Set is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*cachedSet) Set(_ string, _ any) error {
	return fmt.Errorf(`(jwk.cachedSet).Set: jwk.CachedSet is immutable`)
}

// Remove is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*cachedSet) Remove(_ string) error {
	// TODO: Remove() should be renamed to Remove(string) error
	return fmt.Errorf(`(jwk.cachedSet).Remove: jwk.CachedSet is immutable`)
}

// RemoveKey is a no-op for `jwk.CachedSet`, as the `jwk.Set` should be treated read-only
func (*cachedSet) RemoveKey(_ Key) error {
	return fmt.Errorf(`(jwk.cachedSet).RemoveKey: jwk.CachedSet is immutable`)
}

func (cs *cachedSet) Clone() (Set, error) {
	set, err := cs.cached()
	if err != nil {
		return nil, fmt.Errorf(`failed to get cached jwk.Set: %w`, err)
	}

	return set.Clone()
}

// Get returns the value of non-Key field stored in the jwk.Set
func (cs *cachedSet) Get(name string, dst any) error {
	set, err := cs.cached()
	if err != nil {
		return err
	}

	return set.Get(name, dst)
}

// Key returns the Key at the specified index
func (cs *cachedSet) Key(idx int) (Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}

	return set.Key(idx)
}

func (cs *cachedSet) Index(key Key) int {
	set, err := cs.cached()
	if err != nil {
		return -1
	}

	return set.Index(key)
}

func (cs *cachedSet) Keys() []string {
	set, err := cs.cached()
	if err != nil {
		return nil
	}

	return set.Keys()
}

func (cs *cachedSet) Len() int {
	set, err := cs.cached()
	if err != nil {
		return -1
	}

	return set.Len()
}

func (cs *cachedSet) LookupKeyID(kid string) (Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}

	return set.LookupKeyID(kid)
}

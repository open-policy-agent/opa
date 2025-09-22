package jwk

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Fetcher is an interface that represents an object that fetches a JWKS.
// Currently this is only used in the `jws.WithVerifyAuto` option.
//
// Particularly, do not confuse this as the backend to `jwk.Fetch()` function.
// If you need to control how `jwk.Fetch()` implements HTTP requests look into
// providing a custom `http.Client` object via `jwk.WithHTTPClient` option
type Fetcher interface {
	Fetch(context.Context, string, ...FetchOption) (Set, error)
}

// FetchFunc describes a type of Fetcher that is represented as a function.
//
// You can use this to wrap functions (e.g. `jwk.Fetch“) as a Fetcher object.
type FetchFunc func(context.Context, string, ...FetchOption) (Set, error)

func (ff FetchFunc) Fetch(ctx context.Context, u string, options ...FetchOption) (Set, error) {
	return ff(ctx, u, options...)
}

// CachedFetcher wraps `jwk.Cache` so that it can be used as a `jwk.Fetcher`.
//
// One notable diffence from a general use fetcher is that `jwk.CachedFetcher`
// can only be used with JWKS URLs that have been registered with the cache.
// Please read the documentation fo `(jwk.CachedFetcher).Fetch` for more details.
//
// This object is intended to be used with `jws.WithVerifyAuto` option, specifically
// for a scenario where there is a very small number of JWKS URLs that are trusted
// and used to verify JWS messages. It is NOT meant to be used as a general purpose
// caching fetcher object.
type CachedFetcher struct {
	cache *Cache
}

// Creates a new `jwk.CachedFetcher` object.
func NewCachedFetcher(cache *Cache) *CachedFetcher {
	return &CachedFetcher{cache}
}

// Fetch fetches a JWKS from the cache. If the JWKS URL has not been registered with
// the cache, an error is returned.
func (f *CachedFetcher) Fetch(ctx context.Context, u string, _ ...FetchOption) (Set, error) {
	if !f.cache.IsRegistered(ctx, u) {
		return nil, fmt.Errorf(`jwk.CachedFetcher: url %q has not been registered`, u)
	}
	return f.cache.Lookup(ctx, u)
}

// Fetch fetches a JWK resource specified by a URL. The url must be
// pointing to a resource that is supported by `net/http`.
//
// This function is just a wrapper around `net/http` and `jwk.Parse`.
// There is nothing special here, so you are safe to use your own
// mechanism to fetch the JWKS.
//
// If you are using the same `jwk.Set` for long periods of time during
// the lifecycle of your program, and would like to periodically refresh the
// contents of the object with the data at the remote resource,
// consider using `jwk.Cache`, which automatically refreshes
// jwk.Set objects asynchronously.
func Fetch(ctx context.Context, u string, options ...FetchOption) (Set, error) {
	var parseOptions []ParseOption
	//nolint:revive // I want to keep the type of `wl` as `Whitelist` instead of `InsecureWhitelist`
	var wl Whitelist = InsecureWhitelist{}
	var client HTTPClient = http.DefaultClient
	for _, option := range options {
		if parseOpt, ok := option.(ParseOption); ok {
			parseOptions = append(parseOptions, parseOpt)
			continue
		}

		switch option.Ident() {
		case identHTTPClient{}:
			if err := option.Value(&client); err != nil {
				return nil, fmt.Errorf(`failed to retrieve HTTPClient option value: %w`, err)
			}
		case identFetchWhitelist{}:
			if err := option.Value(&wl); err != nil {
				return nil, fmt.Errorf(`failed to retrieve fetch whitelist option value: %w`, err)
			}
		}
	}

	if !wl.IsAllowed(u) {
		return nil, fmt.Errorf(`jwk.Fetch: url %q has been rejected by whitelist`, u)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf(`jwk.Fetch: failed to create new request: %w`, err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(`jwk.Fetch: request failed: %w`, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(`jwk.Fetch: request returned status %d, expected 200`, res.StatusCode)
	}

	buf, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf(`jwk.Fetch: failed to read response body for %q: %w`, u, err)
	}

	return Parse(buf, parseOptions...)
}

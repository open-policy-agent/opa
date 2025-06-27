package httprc

import (
	"regexp"
	"sync"
)

// Whitelist is an interface that allows you to determine if a given URL is allowed
// or not. Implementations of this interface can be used to restrict the URLs that
// the client can access.
//
// By default all URLs are allowed, but this may not be ideal in production environments
// for security reasons.
//
// This exists because you might use this module to store resources provided by
// user of your application, in which case you cannot necessarily trust that the
// URLs are safe.
//
// You will HAVE to provide some sort of whitelist.
type Whitelist interface {
	IsAllowed(string) bool
}

// WhitelistFunc is a function type that implements the Whitelist interface.
type WhitelistFunc func(string) bool

func (f WhitelistFunc) IsAllowed(u string) bool { return f(u) }

// BlockAllWhitelist is a Whitelist implementation that blocks all URLs.
type BlockAllWhitelist struct{}

// NewBlockAllWhitelist creates a new BlockAllWhitelist instance. It is safe to
// use the zero value of this type; this constructor is provided for consistency.
func NewBlockAllWhitelist() BlockAllWhitelist { return BlockAllWhitelist{} }

func (BlockAllWhitelist) IsAllowed(_ string) bool { return false }

// InsecureWhitelist is a Whitelist implementation that allows all URLs. Be careful
// when using this in your production code: make sure you do not blindly register
// URLs from untrusted sources.
type InsecureWhitelist struct{}

// NewInsecureWhitelist creates a new InsecureWhitelist instance. It is safe to
// use the zero value of this type; this constructor is provided for consistency.
func NewInsecureWhitelist() InsecureWhitelist { return InsecureWhitelist{} }

func (InsecureWhitelist) IsAllowed(_ string) bool { return true }

// RegexpWhitelist is a jwk.Whitelist object comprised of a list of *regexp.Regexp
// objects. All entries in the list are tried until one matches. If none of the
// *regexp.Regexp objects match, then the URL is deemed unallowed.
type RegexpWhitelist struct {
	mu       sync.RWMutex
	patterns []*regexp.Regexp
}

// NewRegexpWhitelist creates a new RegexpWhitelist instance. It is safe to use the
// zero value of this type; this constructor is provided for consistency.
func NewRegexpWhitelist() *RegexpWhitelist {
	return &RegexpWhitelist{}
}

// Add adds a new regular expression to the list of expressions to match against.
func (w *RegexpWhitelist) Add(pat *regexp.Regexp) *RegexpWhitelist {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.patterns = append(w.patterns, pat)
	return w
}

// IsAllowed returns true if any of the patterns in the whitelist
// returns true.
func (w *RegexpWhitelist) IsAllowed(u string) bool {
	w.mu.RLock()
	patterns := w.patterns
	w.mu.RUnlock()
	for _, pat := range patterns {
		if pat.MatchString(u) {
			return true
		}
	}
	return false
}

// MapWhitelist is a jwk.Whitelist object comprised of a map of strings.
// If the URL exists in the map, then the URL is allowed to be fetched.
type MapWhitelist interface {
	Whitelist
	Add(string) MapWhitelist
}

type mapWhitelist struct {
	mu    sync.RWMutex
	store map[string]struct{}
}

func NewMapWhitelist() MapWhitelist {
	return &mapWhitelist{store: make(map[string]struct{})}
}

func (w *mapWhitelist) Add(pat string) MapWhitelist {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.store[pat] = struct{}{}
	return w
}

func (w *mapWhitelist) IsAllowed(u string) bool {
	w.mu.RLock()
	_, b := w.store[u]
	w.mu.RUnlock()
	return b
}

package gh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// A release run makes ~3 requests per commit — hundreds of sequential calls over
// several minutes, so a transient blip is likely and losing the run to one is
// expensive.
const (
	maxAttempts = 4 // including the first try
	baseBackoff = time.Second
	// maxBackoff also caps server-supplied wait hints, so a rate-limit reset an
	// hour out can't park the run.
	maxBackoff     = 30 * time.Second
	requestTimeout = 30 * time.Second
)

// retryTransport backs both the REST and GraphQL clients, so there is one retry
// policy rather than two. Bearer auth lives here for the same reason.
type retryTransport struct {
	base  http.RoundTripper
	token string
	sleep func(time.Duration)
	logf  func(format string, args ...any)
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffered so it can be replayed; every request here is bodiless or a small
	// GraphQL query.
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
	}

	var lastErr error
	// wait is the server's requested delay; zero means use exponential backoff.
	var wait time.Duration

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			if wait == 0 {
				wait = backoff(attempt)
			}
			t.log("retrying %s %s in %s (attempt %d/%d): %v",
				req.Method, req.URL.Path, wait, attempt, maxAttempts, lastErr)
			t.sleep(wait)
		}

		resp, err := t.attempt(req, body)
		if err != nil {
			if !isRetryableErr(err) {
				return nil, err
			}
			lastErr, wait = err, 0
			continue
		}

		hint, retry := retryAfter(resp)
		if !retry {
			return resp, nil
		}
		lastErr, wait = fmt.Errorf("HTTP %d", resp.StatusCode), hint
		// Drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return nil, fmt.Errorf("giving up after %d attempts: %w", maxAttempts, lastErr)
}

// attempt performs one request under its own timeout.
//
// The timeout has to live here rather than on http.Client.Timeout, which bounds
// the whole Do — retries and backoff sleeps included. A hung socket would then
// consume the entire budget and leave nothing for the retry that might succeed.
//
// The per-attempt context must outlive attempt() when the response is returned
// to the caller, or the body would be cancelled before it is read; cancel is
// therefore deferred to Body.Close().
func (t *retryTransport) attempt(req *http.Request, body []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	resp, err := t.base.RoundTrip(t.prepare(req, body, ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// prepare clones because RoundTrip must not mutate its argument.
func (t *retryTransport) prepare(req *http.Request, body []byte, ctx context.Context) *http.Request {
	out := req.Clone(ctx)
	if body != nil {
		out.Body = io.NopCloser(bytes.NewReader(body))
		out.ContentLength = int64(len(body))
	}
	if t.token != "" && out.Header.Get("Authorization") == "" {
		out.Header.Set("Authorization", "Bearer "+t.token)
	}
	return out
}

func (t *retryTransport) log(format string, args ...any) {
	if t.logf != nil {
		t.logf(format, args...)
	}
}

func backoff(attempt int) time.Duration {
	if attempt < 2 {
		return 0
	}
	d := baseBackoff << (attempt - 2)
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// isRetryableErr excludes things retrying cannot fix, like a malformed URL.
func isRetryableErr(err error) bool {
	if err == nil {
		return false
	}
	_, isNetErr := errors.AsType[net.Error](err)
	// A reset or EOF mid-response arrives wrapped in *url.Error, not as net.Error.
	return isNetErr || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed)
}

// retryAfter retries 5xx, 429, and 403-with-exhausted-rate-limit (GitHub's
// secondary limit, meaning "slow down" rather than "not allowed"). 401/404/422
// are real answers — 422 in particular is how GitHub reports an unpushed commit.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	switch {
	case resp.StatusCode >= 500:
	case resp.StatusCode == http.StatusTooManyRequests:
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("x-ratelimit-remaining") == "0":
	default:
		return 0, false
	}
	return waitHint(resp.Header), true
}

// waitHint prefers Retry-After (seconds) over x-ratelimit-reset (Unix seconds),
// and returns 0 when neither is usable.
func waitHint(h http.Header) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return min(time.Duration(secs)*time.Second, maxBackoff)
		}
	}
	if v := h.Get("x-ratelimit-reset"); v != "" {
		if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
			if d := time.Until(time.Unix(unix, 0)); d > 0 {
				return min(d, maxBackoff)
			}
		}
	}
	return 0
}

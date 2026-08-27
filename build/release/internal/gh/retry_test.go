package gh

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stubTransport answers from a scripted list of outcomes, recording what it was
// asked. Stubbing at the RoundTripper seam rather than standing up an
// httptest.Server keeps these tests hermetic — no sockets, no ports, and the
// unit under test is exactly retryTransport.
type stubTransport struct {
	// outcomes is consumed one per attempt. The last entry repeats once
	// exhausted, so "always fails" needs a single entry.
	outcomes []outcome
	attempts int
	// Recorded per attempt.
	bodies  []string
	headers []http.Header
}

type outcome struct {
	status int
	header http.Header
	err    error
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.attempts++
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	s.bodies = append(s.bodies, body)
	s.headers = append(s.headers, req.Header.Clone())

	o := s.outcomes[min(s.attempts, len(s.outcomes))-1]
	if o.err != nil {
		return nil, o.err
	}
	h := o.header
	if h == nil {
		h = http.Header{}
	}
	return &http.Response{
		StatusCode: o.status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader("payload")),
		Request:    req,
	}, nil
}

// newTestTransport wraps stub in a retryTransport whose sleeps are recorded
// rather than performed, so the backoff schedule is asserted without waiting.
func newTestTransport(stub http.RoundTripper, token string) (*retryTransport, *[]time.Duration) {
	var slept []time.Duration
	return &retryTransport{
		base:  stub,
		token: token,
		sleep: func(d time.Duration) { slept = append(slept, d) },
	}, &slept
}

func get(t *testing.T, rt *retryTransport) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/commits/abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if resp != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return resp, err
}

func rateLimited() http.Header {
	return http.Header{"X-Ratelimit-Remaining": {"0"}}
}

func TestRetryTransport_StatusPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		// failing is the outcome returned until the run gives up or succeeds.
		failing outcome
		// transient, when true, means the stub succeeds after one failure.
		transient bool

		wantAttempts int
		// wantStatus is the status the caller should observe; 0 means the call
		// must return an error instead of a response.
		wantStatus int
	}{
		{name: "500 retried then succeeds", failing: outcome{status: 500}, transient: true, wantAttempts: 2, wantStatus: 200},
		{name: "502 retried then succeeds", failing: outcome{status: 502}, transient: true, wantAttempts: 2, wantStatus: 200},
		{name: "503 retried then succeeds", failing: outcome{status: 503}, transient: true, wantAttempts: 2, wantStatus: 200},
		{name: "429 retried then succeeds", failing: outcome{status: http.StatusTooManyRequests, header: http.Header{"Retry-After": {"1"}}}, transient: true, wantAttempts: 2, wantStatus: 200},
		{
			name:         "403 with exhausted rate limit is retried",
			failing:      outcome{status: http.StatusForbidden, header: rateLimited()},
			transient:    true,
			wantAttempts: 2,
			wantStatus:   200,
		},
		{
			name:         "403 without rate-limit header is a real denial",
			failing:      outcome{status: http.StatusForbidden},
			wantAttempts: 1,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "401 is not retried",
			failing:      outcome{status: http.StatusUnauthorized},
			wantAttempts: 1,
			wantStatus:   http.StatusUnauthorized,
		},
		{
			name:         "404 is not retried",
			failing:      outcome{status: http.StatusNotFound},
			wantAttempts: 1,
			wantStatus:   http.StatusNotFound,
		},
		{
			// 422 is how GitHub says "no commit found for SHA", which Commit
			// turns into the local-only fallback. Retrying it would delay a
			// correct answer three times per unpushed commit.
			name:         "422 is not retried",
			failing:      outcome{status: http.StatusUnprocessableEntity},
			wantAttempts: 1,
			wantStatus:   http.StatusUnprocessableEntity,
		},
		{
			name:         "persistent 500 exhausts retries",
			failing:      outcome{status: 500},
			wantAttempts: maxAttempts,
			wantStatus:   0,
		},
		{
			name:         "persistent rate limiting exhausts retries",
			failing:      outcome{status: http.StatusForbidden, header: rateLimited()},
			wantAttempts: maxAttempts,
			wantStatus:   0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcomes := []outcome{tc.failing}
			if tc.transient {
				outcomes = append(outcomes, outcome{status: 200})
			}
			stub := &stubTransport{outcomes: outcomes}
			rt, _ := newTestTransport(stub, "")

			resp, err := get(t, rt)

			if stub.attempts != tc.wantAttempts {
				t.Errorf("attempts = %d, want %d", stub.attempts, tc.wantAttempts)
			}
			if tc.wantStatus == 0 {
				if err == nil {
					t.Fatal("expected an error after exhausting retries")
				}
				if !strings.Contains(err.Error(), "giving up after") {
					t.Errorf("error = %v, want it to mention giving up", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

// timeoutErr mimics a net.Error timeout — the failure the v1.19.0 release run
// actually hit: "read tcp ...: read: operation timed out".
type timeoutErr struct{}

func (timeoutErr) Error() string {
	return "read tcp 10.0.0.1:1->10.0.0.2:443: read: operation timed out"
}
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRetryTransport_ErrorPolicy(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err          error
		transient    bool
		wantAttempts int
		wantOK       bool
	}{
		{name: "net timeout retried", err: timeoutErr{}, transient: true, wantAttempts: 2, wantOK: true},
		{name: "EOF retried", err: io.EOF, transient: true, wantAttempts: 2, wantOK: true},
		{name: "unexpected EOF retried", err: io.ErrUnexpectedEOF, transient: true, wantAttempts: 2, wantOK: true},
		{name: "closed connection retried", err: net.ErrClosed, transient: true, wantAttempts: 2, wantOK: true},
		{name: "persistent timeout exhausts retries", err: timeoutErr{}, wantAttempts: maxAttempts, wantOK: false},
		{
			// Retrying a bad URL or an unknown host cannot change the outcome,
			// so it fails immediately with the original error.
			name:         "non-network error not retried",
			err:          errors.New("unsupported protocol scheme"),
			wantAttempts: 1,
			wantOK:       false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcomes := []outcome{{err: tc.err}}
			if tc.transient {
				outcomes = append(outcomes, outcome{status: 200})
			}
			stub := &stubTransport{outcomes: outcomes}
			rt, _ := newTestTransport(stub, "")

			_, err := get(t, rt)

			if stub.attempts != tc.wantAttempts {
				t.Errorf("attempts = %d, want %d", stub.attempts, tc.wantAttempts)
			}
			if tc.wantOK && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestRetryTransport_BackoffSchedule(t *testing.T) {
	// Three failures, then success: the caller should have waited 1s, 2s, 4s.
	stub := &stubTransport{outcomes: []outcome{
		{status: 500}, {status: 500}, {status: 500}, {status: 200},
	}}
	rt, slept := newTestTransport(stub, "")

	if _, err := get(t, rt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(*slept) != len(want) {
		t.Fatalf("slept %v, want %v", *slept, want)
	}
	for i, d := range *slept {
		if d != want[i] {
			t.Errorf("sleep %d = %s, want %s", i, d, want[i])
		}
	}
}

func TestRetryTransport_ServerWaitHints(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header http.Header
		status int
		want   time.Duration
	}{
		{
			name:   "Retry-After honoured over exponential backoff",
			status: http.StatusTooManyRequests,
			header: http.Header{"Retry-After": {"5"}},
			want:   5 * time.Second,
		},
		{
			name:   "hour-away ratelimit-reset is capped, not obeyed",
			status: http.StatusForbidden,
			header: http.Header{
				"X-Ratelimit-Remaining": {"0"},
				"X-Ratelimit-Reset":     {strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)},
			},
			want: maxBackoff,
		},
		{
			name:   "no hint falls back to exponential backoff",
			status: 500,
			want:   time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{outcomes: []outcome{
				{status: tc.status, header: tc.header},
				{status: 200},
			}}
			rt, slept := newTestTransport(stub, "")
			if _, err := get(t, rt); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(*slept) != 1 || (*slept)[0] != tc.want {
				t.Errorf("slept %v, want [%s]", *slept, tc.want)
			}
		})
	}
}

// TestRetryTransport_ReplaysRequestBody guards the GraphQL path: the query body
// must survive a retry, or the second attempt posts an empty document and the
// PR→issue links come back empty instead of erroring.
func TestRetryTransport_ReplaysRequestBody(t *testing.T) {
	const query = `{"query":"query{viewer{login}}"}`
	stub := &stubTransport{outcomes: []outcome{{status: 500}, {status: 200}}}
	rt, _ := newTestTransport(stub, "")

	req, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", strings.NewReader(query))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if len(stub.bodies) != 2 {
		t.Fatalf("saw %d attempts, want 2", len(stub.bodies))
	}
	for i, b := range stub.bodies {
		if b != query {
			t.Errorf("attempt %d body = %q, want %q", i+1, b, query)
		}
	}
}

func TestRetryTransport_Token(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
		want  string
	}{
		{name: "token attached to every attempt", token: "sekrit", want: "Bearer sekrit"},
		{name: "no token sends no header", token: "", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubTransport{outcomes: []outcome{{status: 500}, {status: 200}}}
			rt, _ := newTestTransport(stub, tc.token)
			if _, err := get(t, rt); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(stub.headers) != 2 {
				t.Fatalf("saw %d attempts, want 2", len(stub.headers))
			}
			for i, h := range stub.headers {
				if got := h.Get("Authorization"); got != tc.want {
					t.Errorf("attempt %d Authorization = %q, want %q", i+1, got, tc.want)
				}
			}
		})
	}
}

// TestRetryTransport_DoesNotMutateRequest asserts the http.RoundTripper contract:
// the caller's request must come back unchanged, since retries clone it.
func TestRetryTransport_DoesNotMutateRequest(t *testing.T) {
	stub := &stubTransport{outcomes: []outcome{{status: 200}}}
	rt, _ := newTestTransport(stub, "sekrit")

	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("caller's request was mutated: Authorization = %q", got)
	}
}

func TestWaitHint(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header http.Header
		want   time.Duration
	}{
		{name: "no hints", header: http.Header{}, want: 0},
		{name: "Retry-After seconds", header: http.Header{"Retry-After": {"7"}}, want: 7 * time.Second},
		{name: "Retry-After capped at maxBackoff", header: http.Header{"Retry-After": {"3600"}}, want: maxBackoff},
		{name: "Retry-After zero ignored", header: http.Header{"Retry-After": {"0"}}, want: 0},
		{name: "Retry-After garbage ignored", header: http.Header{"Retry-After": {"soon"}}, want: 0},
		{
			name:   "past ratelimit-reset ignored",
			header: http.Header{"X-Ratelimit-Reset": {strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)}},
			want:   0,
		},
		{
			name: "Retry-After wins over ratelimit-reset",
			header: http.Header{
				"Retry-After":       {"3"},
				"X-Ratelimit-Reset": {strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)},
			},
			want: 3 * time.Second,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := waitHint(tc.header); got != tc.want {
				t.Errorf("waitHint() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 0},
		{attempt: 2, want: time.Second},
		{attempt: 3, want: 2 * time.Second},
		{attempt: 4, want: 4 * time.Second},
		{attempt: 20, want: maxBackoff},
	} {
		if got := backoff(tc.attempt); got != tc.want {
			t.Errorf("backoff(%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

// deadlineStub records each attempt's context deadline and whether the body was
// still readable when handed back.
type deadlineStub struct {
	outcomes    []outcome
	attempts    int
	deadlines   []time.Time
	hadDeadline []bool
}

func (d *deadlineStub) RoundTrip(req *http.Request) (*http.Response, error) {
	d.attempts++
	dl, ok := req.Context().Deadline()
	d.deadlines = append(d.deadlines, dl)
	d.hadDeadline = append(d.hadDeadline, ok)

	o := d.outcomes[min(d.attempts, len(d.outcomes))-1]
	if o.err != nil {
		return nil, o.err
	}
	return &http.Response{
		StatusCode: o.status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("payload")),
		Request:    req,
	}, nil
}

// TestRetryTransport_PerAttemptTimeout asserts each attempt gets its own
// deadline. A single shared deadline (which is what http.Client.Timeout gives
// you) would be consumed by the first hung socket, leaving no budget for the
// retry that might succeed.
func TestRetryTransport_PerAttemptTimeout(t *testing.T) {
	stub := &deadlineStub{outcomes: []outcome{{status: 500}, {status: 500}, {status: 200}}}
	rt, _ := newTestTransport(stub, "")

	before := time.Now()
	resp, err := get(t, rt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp

	if stub.attempts != 3 {
		t.Fatalf("attempts = %d, want 3", stub.attempts)
	}
	for i, ok := range stub.hadDeadline {
		if !ok {
			t.Errorf("attempt %d had no deadline", i+1)
		}
	}
	// Each deadline should sit roughly requestTimeout out from when its attempt
	// started, so later attempts have strictly later deadlines.
	for i := 1; i < len(stub.deadlines); i++ {
		if !stub.deadlines[i].After(stub.deadlines[i-1]) {
			t.Errorf("deadline %d (%s) is not after deadline %d (%s) — the deadline is shared, not per attempt",
				i+1, stub.deadlines[i], i, stub.deadlines[i-1])
		}
	}
	if d := stub.deadlines[0].Sub(before); d > requestTimeout+time.Second || d < requestTimeout-time.Second {
		t.Errorf("first deadline is %s out, want ~%s", d, requestTimeout)
	}
}

// TestRetryTransport_BodyReadableAfterReturn guards the cancelOnClose wiring:
// the per-attempt context must not be cancelled until the caller closes the
// body, or every successful response would come back truncated.
func TestRetryTransport_BodyReadableAfterReturn(t *testing.T) {
	stub := &stubTransport{outcomes: []outcome{{status: 500}, {status: 200}}}
	rt, _ := newTestTransport(stub, "")

	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body after RoundTrip returned: %v", err)
	}
	if string(body) != "payload" {
		t.Errorf("body = %q, want %q", body, "payload")
	}
	if err := resp.Body.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	// Closing twice must not panic — cancel is idempotent.
	_ = resp.Body.Close()
}

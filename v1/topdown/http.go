// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/internal/version"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/tracing"
	"github.com/open-policy-agent/opa/v1/util"
)

const (
	defaultHTTPRequestTimeoutEnv             = "HTTP_SEND_TIMEOUT"
	defaultCachingMode           cachingMode = "serialized"
	cachingModeDeserialized      cachingMode = "deserialized"
	httpSendLatencyMetricKey                 = "rego_builtin_http_send"
	httpSendInterQueryCacheHits              = httpSendLatencyMetricKey + "_interquery_cache_hits"
	httpSendNetworkRequests                  = httpSendLatencyMetricKey + "_network_requests"

	// httpSendBuiltinCacheKey is the key in the builtin context cache that
	// points to the http.send() specific cache resides at.
	httpSendBuiltinCacheKey httpSendKey = "HTTP_SEND_CACHE_KEY"

	// HTTPSendInternalErr represents a runtime evaluation error.
	HTTPSendInternalErr string = "eval_http_send_internal_error"

	// HTTPSendNetworkErr represents a network error.
	HTTPSendNetworkErr string = "eval_http_send_network_error"

	// minRetryDelay is amount of time to backoff after the first failure.
	minRetryDelay = time.Millisecond * 100

	// maxRetryDelay is the upper bound of backoff delay.
	maxRetryDelay = time.Second * 60
)

var (
	defaultHTTPRequestTimeout = time.Second * 5
	allowedKeyNames           = [...]string{
		"method",
		"url",
		"body",
		"enable_redirect",
		"force_json_decode",
		"force_yaml_decode",
		"headers",
		"raw_body",
		"tls_use_system_certs",
		"tls_ca_cert",
		"tls_ca_cert_file",
		"tls_ca_cert_env_variable",
		"tls_client_cert",
		"tls_client_cert_file",
		"tls_client_cert_env_variable",
		"tls_client_key",
		"tls_client_key_file",
		"tls_client_key_env_variable",
		"tls_insecure_skip_verify",
		"tls_server_name",
		"timeout",
		"cache",
		"force_cache",
		"force_cache_duration_seconds",
		"raise_error",
		"caching_mode",
		"max_retry_attempts",
		"cache_ignored_headers",
	}
	// ref: https://www.rfc-editor.org/rfc/rfc7231#section-6.1
	cacheableHTTPStatusCodes = [...]int{
		http.StatusOK,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusPartialContent,
		http.StatusMultipleChoices,
		http.StatusMovedPermanently,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusGone,
		http.StatusRequestURITooLong,
		http.StatusNotImplemented,
	}

	allowedKeys    = ast.NewSet()
	cacheableCodes = ast.NewSet()
	requiredKeys   = ast.NewSet(ast.InternedTerm("method"), ast.InternedTerm("url"))
)

type (
	// CustomizeRoundTripper allows customizing an existing http.Transport,
	// to the returned value, which could be the same Transport or a new one.
	CustomizeRoundTripper func(*http.Transport) http.RoundTripper
	cachingMode           string
	httpSendKey           string
	interQueryCacheValue  struct {
		Data []byte
	}
	// httpRequestExecutor defines an interface for the http send cache
	httpRequestExecutor interface {
		CheckCache() (ast.Value, error)
		InsertIntoCache(value *http.Response) (ast.Value, error)
		InsertErrorIntoCache(err error)
		ExecuteHTTPRequest() (*http.Response, error)
	}
	// The httpSendCache is used for intra-query caching of http.send results.
	httpSendCache struct {
		entries *util.HasherMap[ast.Value, httpSendCacheEntry]
	}
	httpSendCacheEntry struct {
		response *ast.Value
		error    error
	}
	interQueryCache struct {
		bctx             BuiltinContext
		req              ast.Object
		key              ast.Object
		httpReq          *http.Request
		httpClient       *http.Client
		forceJSONDecode  bool
		forceYAMLDecode  bool
		forceCacheParams forceCacheParams
	}
	interQueryCacheData struct {
		RespBody   []byte
		Status     string
		StatusCode int
		Headers    http.Header
		ExpiresAt  time.Time
	}
	intraQueryCache struct {
		bctx BuiltinContext
		req  ast.Object
		key  ast.Object
	}
	forceCacheParams struct {
		forceDurationSeconds int32
	}
	responseHeaders struct {
		etag         string // identifier for a specific version of the response
		lastModified string // date and time response was last modified as per origin server
	}
	// deltaSeconds specifies a non-negative integer, representing
	// time in seconds: http://tools.ietf.org/html/rfc7234#section-1.2.1
	deltaSeconds int32
)

func builtinHTTPSend(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return handleBuiltinErr(ast.HTTPSend.Name, bctx.Location, err)
	}

	raiseError, err := getRaiseErrorValue(obj)
	if err != nil {
		return handleBuiltinErr(ast.HTTPSend.Name, bctx.Location, err)
	}

	req, err := validateHTTPRequestOperand(operands[0], 1)
	if err != nil {
		if raiseError {
			return handleHTTPSendErr(bctx.Context, bctx.Location, err)
		}

		return iter(generateRaiseErrorResult(handleBuiltinErr(ast.HTTPSend.Name, bctx.Location, err)))
	}

	result, err := getHTTPResponse(bctx, req)
	if err != nil {
		if raiseError {
			return handleHTTPSendErr(bctx.Context, bctx.Location, err)
		}
		result = generateRaiseErrorResult(err)
	}
	return iter(result)
}

func generateRaiseErrorResult(err error) *ast.Term {
	var errObj ast.Object
	switch err.(type) {
	case *url.Error:
		errObj = ast.NewObject(
			ast.Item(ast.InternedTerm("code"), ast.InternedTerm(HTTPSendNetworkErr)),
			ast.Item(ast.InternedTerm("message"), ast.StringTerm(err.Error())),
		)
	default:
		errObj = ast.NewObject(
			ast.Item(ast.InternedTerm("code"), ast.InternedTerm(HTTPSendInternalErr)),
			ast.Item(ast.InternedTerm("message"), ast.StringTerm(err.Error())),
		)
	}

	return ast.ObjectTerm(
		ast.Item(ast.InternedTerm("status_code"), ast.InternedTerm(0)),
		ast.Item(ast.InternedTerm("error"), ast.NewTerm(errObj)),
	)
}

func getHTTPResponse(bctx BuiltinContext, req ast.Object) (*ast.Term, error) {
	bctx.Metrics.Timer(httpSendLatencyMetricKey).Start()
	defer bctx.Metrics.Timer(httpSendLatencyMetricKey).Stop()

	key, err := getKeyFromRequest(req)
	if err != nil {
		return nil, err
	}

	reqExecutor, err := newHTTPRequestExecutor(bctx, req, key)
	if err != nil {
		return nil, err
	}
	// Check if cache already has a response for this query
	// set headers to exclude cache_ignored_headers
	resp, err := reqExecutor.CheckCache()
	if err != nil {
		return nil, err
	}

	if resp == nil {
		httpResp, err := reqExecutor.ExecuteHTTPRequest()
		defer util.Close(httpResp)

		if err != nil {
			reqExecutor.InsertErrorIntoCache(err)
			return nil, err
		}
		// Add result to intra/inter-query cache.
		resp, err = reqExecutor.InsertIntoCache(httpResp)
		if err != nil {
			return nil, err
		}
	}

	return ast.NewTerm(resp), nil
}

// getKeyFromRequest returns a key to be used for caching HTTP responses
// deletes headers from request object mentioned in cache_ignored_headers
func getKeyFromRequest(req ast.Object) (ast.Object, error) {
	// deep copy so changes to key do not reflect in the request object
	key := req.Copy()
	cacheIgnoredHeadersTerm := req.Get(ast.InternedTerm("cache_ignored_headers"))
	allHeadersTerm := req.Get(ast.InternedTerm("headers"))
	// skip because no headers to delete
	if cacheIgnoredHeadersTerm == nil || allHeadersTerm == nil {
		// need to explicitly set cache_ignored_headers to null
		// equivalent requests might have different sets of exclusion lists
		key.Insert(ast.InternedTerm("cache_ignored_headers"), ast.InternedNullTerm)
		return key, nil
	}
	cacheIgnoredHeaders, ok := cacheIgnoredHeadersTerm.Value.(*ast.Array)
	if !ok || cacheIgnoredHeaders.Until(util.Not(ast.TermValueIs[ast.String])) {
		return nil, errors.New("cache_ignored_headers must be an array of strings")
	}
	allHeaders := allHeadersTerm.Value.(ast.Object)
	filteredHeaders := ast.NewObjectWithCapacity(allHeaders.Len())

	allHeaders.Foreach(func(key, val *ast.Term) {
		if !cacheIgnoredHeaders.Until(key.Equal) {
			filteredHeaders.Insert(key, val)
		}
	})

	key.Insert(ast.InternedTerm("headers"), ast.NewTerm(filteredHeaders))
	// remove cache_ignored_headers key
	key.Insert(ast.InternedTerm("cache_ignored_headers"), ast.InternedNullTerm)
	return key, nil
}

func init() {
	ast.InternStringTerm(HTTPSendNetworkErr, HTTPSendInternalErr)
	ast.InternStringTerm(allowedKeyNames[:]...)
	for _, element := range allowedKeyNames {
		allowedKeys.Add(ast.InternedTerm(element))
	}

	createCacheableHTTPStatusCodes()
	initDefaults()
	RegisterBuiltinFunc(ast.HTTPSend.Name, builtinHTTPSend)
}

func handleHTTPSendErr(ctx context.Context, loc *ast.Location, err error) error {
	// Return HTTP client timeout errors in a generic error message to avoid confusion about what happened.
	// Do not do this if the builtin context was cancelled and is what caused the request to stop.
	if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() && ctx.Err() == nil {
		err = fmt.Errorf("%s %s: request timed out", urlErr.Op, urlErr.URL)
	}
	if err := ctx.Err(); err != nil {
		return Halt{Err: &Error{Code: CancelErr, Message: fmt.Sprintf("http.send: timed out (%s)", err.Error())}}
	}
	return handleBuiltinErr(ast.HTTPSend.Name, loc, err)
}

func initDefaults() {
	if timeoutDuration := os.Getenv(defaultHTTPRequestTimeoutEnv); timeoutDuration != "" {
		var err error
		defaultHTTPRequestTimeout, err = time.ParseDuration(timeoutDuration)
		if err != nil {
			// If it is set to something not valid don't let the process continue in a state
			// that will almost definitely give unexpected results by having it set at 0
			// which means no timeout..
			// This environment variable isn't considered part of the public API.
			// TODO(patrick-east): Remove the environment variable
			panic(fmt.Sprintf("invalid value for HTTP_SEND_TIMEOUT: %s", err))
		}
	}
}

func validateHTTPRequestOperand(term *ast.Term, pos int) (ast.Object, error) {
	obj, err := builtins.ObjectOperand(term.Value, pos)
	if err != nil {
		return nil, err
	}

	requestKeys := ast.NewSet(obj.Keys()...)

	invalidKeys := requestKeys.Diff(allowedKeys)
	if invalidKeys.Len() != 0 {
		return nil, builtins.NewOperandErr(pos, "invalid request parameters(s): %v", invalidKeys)
	}

	missingKeys := requiredKeys.Diff(requestKeys)
	if missingKeys.Len() != 0 {
		return nil, builtins.NewOperandErr(pos, "missing required request parameters(s): %v", missingKeys)
	}

	return obj, nil
}

// canonicalizeHeaders returns a copy of the headers where the keys are in
// canonical HTTP form.
func canonicalizeHeaders(headers map[string]any) map[string]any {
	canonicalized := make(map[string]any, len(headers))
	for k, v := range headers {
		canonicalized[http.CanonicalHeaderKey(k)] = v
	}

	return canonicalized
}

// useSocket examines the url for "unix://" and returns a *http.Transport with
// a DialContext that opens a socket (specified in the http call).
// The url is expected to contain socket=/path/to/socket (url encoded)
// Ex. "unix://localhost/end/point?socket=%2Ftmp%2Fhttp.sock"
func useSocket(rawURL string) (bool, string, *http.Transport) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, "", nil
	}

	if u.Scheme != "unix" || u.RawQuery == "" {
		return false, rawURL, nil
	}

	v, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false, rawURL, nil
	}

	// Rewrite URL targeting the UNIX domain socket.
	u.Scheme = "http"

	// Extract the path to the socket.
	// Only retrieve the first value. Subsequent values are ignored and removed
	// to prevent HTTP parameter pollution.
	socket := v.Get("socket")
	v.Del("socket")
	u.RawQuery = v.Encode()

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return http.DefaultTransport.(*http.Transport).DialContext(ctx, "unix", socket)
	}
	tr.DisableKeepAlives = true

	return true, u.String(), tr
}

func verifyHost(caps *ast.Capabilities, host string) error {
	if caps == nil || caps.AllowNet == nil || slices.Contains(caps.AllowNet, host) {
		return nil
	}
	return fmt.Errorf("disallowed host: %s", host)
}

func verifyURLHost(caps *ast.Capabilities, unverifiedURL string) error {
	// Eager return to avoid unnecessary URL parsing
	if caps == nil || caps.AllowNet == nil {
		return nil
	}

	parsedURL, err := url.Parse(unverifiedURL)
	if err != nil {
		return err
	}

	host, _, _ := strings.Cut(parsedURL.Host, ":")

	return verifyHost(caps, host)
}

func createHTTPRequest(bctx BuiltinContext, obj ast.Object) (*http.Request, *http.Client, error) {
	var (
		url, method string
		// CA and client certificates loading options. Each input source comes in a matched pair.
		tlsCaCertEnvVar, tlsCaCertFile                                               string
		tlsCaCert, tlsClientCert, tlsClientKey                                       []byte
		tlsClientCertEnvVar, tlsClientKeyEnvVar, tlsClientCertFile, tlsClientKeyFile string

		body                                  *bytes.Buffer
		enableRedirect, tlsUseSystemCerts, ok bool
		customHeaders                         map[string]any
	)

	tlsConfig := &tls.Config{}
	timeout := defaultHTTPRequestTimeout

	for _, key := range obj.Keys() {
		keyAny, err := ast.JSON(key.Value)
		if err != nil {
			return nil, nil, err
		}
		val := obj.Get(key)

		var strVal string
		if s, ok := val.Value.(ast.String); ok {
			strVal = strings.Trim(string(s), "\"")
		} else {
			// Most parameters are strings, so consolidate the type checking.
			switch keyAny {
			case "method",
				"url",
				"raw_body",
				"tls_ca_cert",
				"tls_ca_cert_file",
				"tls_ca_cert_env_variable",
				"tls_client_cert",
				"tls_client_cert_file",
				"tls_client_cert_env_variable",
				"tls_client_key",
				"tls_client_key_file",
				"tls_client_key_env_variable",
				"tls_server_name":
				return nil, nil, fmt.Errorf("%q must be a string", keyAny)
			}
		}

		switch keyAny {
		case "method":
			method = strings.ToUpper(strVal)
		case "url":
			if err := verifyURLHost(bctx.Capabilities, strVal); err != nil {
				return nil, nil, err
			}
			url = strVal
		case "enable_redirect":
			if enableRedirect, err = strconv.ParseBool(val.String()); err != nil {
				return nil, nil, err
			}
		case "body":
			if body != nil {
				break // raw_body takes precedence
			}
			bodyVal, err := ast.JSON(val.Value)
			if err != nil {
				return nil, nil, err
			}
			bodyValBytes, err := json.Marshal(bodyVal)
			if err != nil {
				return nil, nil, err
			}
			body = bytes.NewBuffer(bodyValBytes)
		case "raw_body":
			body = bytes.NewBufferString(strVal)
		case "tls_use_system_certs":
			tlsUseSystemCerts, err = strconv.ParseBool(val.String())
			if err != nil {
				return nil, nil, err
			}
		case "tls_ca_cert":
			tlsCaCert = util.StringToByteSlice(strVal)
		case "tls_ca_cert_file":
			tlsCaCertFile = strVal
		case "tls_ca_cert_env_variable":
			tlsCaCertEnvVar = strVal
		case "tls_client_cert":
			tlsClientCert = util.StringToByteSlice(strVal)
		case "tls_client_cert_file":
			tlsClientCertFile = strVal
		case "tls_client_cert_env_variable":
			tlsClientCertEnvVar = strVal
		case "tls_client_key":
			tlsClientKey = util.StringToByteSlice(strVal)
		case "tls_client_key_file":
			tlsClientKeyFile = strVal
		case "tls_client_key_env_variable":
			tlsClientKeyEnvVar = strVal
		case "tls_server_name":
			tlsConfig.ServerName = strVal
		case "headers":
			headersValInterface, err := ast.JSON(val.Value)
			if err != nil {
				return nil, nil, err
			}
			customHeaders, ok = headersValInterface.(map[string]any)
			if !ok {
				return nil, nil, errors.New("invalid type for headers key")
			}
		case "tls_insecure_skip_verify":
			tlsConfig.InsecureSkipVerify, err = strconv.ParseBool(val.String())
			if err != nil {
				return nil, nil, err
			}
		case "timeout":
			if timeout, err = parseTimeout(val.Value); err != nil {
				return nil, nil, err
			}
		case "cache", "caching_mode",
			"force_cache", "force_cache_duration_seconds",
			"force_json_decode", "force_yaml_decode",
			"raise_error", "max_retry_attempts", "cache_ignored_headers": // no-op
		default:
			return nil, nil, fmt.Errorf("invalid parameter %q", keyAny)
		}
	}

	if len(customHeaders) != 0 {
		customHeaders = canonicalizeHeaders(customHeaders)
	}

	client := &http.Client{Timeout: timeout, CheckRedirect: useLastResponseRedirect}

	if len(tlsClientCert) > 0 && len(tlsClientKey) > 0 {
		cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}

	if tlsClientCertFile != "" && tlsClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsClientCertFile, tlsClientKeyFile)
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}

	if tlsClientCertEnvVar != "" && tlsClientKeyEnvVar != "" {
		cert, err := tls.X509KeyPair(
			[]byte(os.Getenv(tlsClientCertEnvVar)),
			[]byte(os.Getenv(tlsClientKeyEnvVar)))
		if err != nil {
			return nil, nil, fmt.Errorf("cannot extract public/private key pair from envvars %q, %q: %w",
				tlsClientCertEnvVar, tlsClientKeyEnvVar, err)
		}
		tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
	}

	// Check the system certificates config first so that we
	// load additional certificated into the correct pool.
	if tlsUseSystemCerts && runtime.GOOS != "windows" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.RootCAs = pool
	}

	if len(tlsCaCert) != 0 {
		tlsCaCert = bytes.ReplaceAll(tlsCaCert, []byte("\\n"), []byte("\n"))
		pool, err := addCACertsFromBytes(tlsConfig.RootCAs, tlsCaCert)
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.RootCAs = pool
	}

	if tlsCaCertFile != "" {
		pool, err := addCACertsFromFile(tlsConfig.RootCAs, tlsCaCertFile)
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.RootCAs = pool
	}

	if tlsCaCertEnvVar != "" {
		pool, err := addCACertsFromEnv(tlsConfig.RootCAs, tlsCaCertEnvVar)
		if err != nil {
			return nil, nil, err
		}
		tlsConfig.RootCAs = pool
	}

	// If Host header is set, use it for TLS server name, unless set with tls_server_name.
	if host, hasHost := customHeaders["Host"]; tlsConfig.ServerName == "" && hasHost {
		// Only default the ServerName if the caller has
		// specified the host. If we don't specify anything,
		// Go will default to the target hostname. This name
		// is not the same as the default that Go populates
		// `req.Host` with, which is why we don't just set
		// this unconditionally.
		tlsConfig.ServerName, _ = host.(string)
	}

	var transport *http.Transport
	if ok, parsedURL, tr := useSocket(url); ok {
		transport = tr
		url = parsedURL
	} else if hasTLSConfig(tlsConfig) {
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = tlsConfig
		transport.DisableKeepAlives = true
	}

	if bctx.RoundTripper != nil {
		client.Transport = bctx.RoundTripper(transport)
	} else if transport != nil {
		client.Transport = transport
	}

	// check if redirects are enabled
	if enableRedirect {
		client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
			return verifyURLHost(bctx.Capabilities, req.URL.String())
		}
	}

	// create the http request, use the builtin context's context to ensure
	// the request is cancelled if evaluation is cancelled.
	req, err := http.NewRequestWithContext(bctx.Context, method, url, util.Or(body, emptyBytesBuffer))
	if err != nil {
		return nil, nil, err
	}

	// Add custom headers
	if len(customHeaders) != 0 {
		for k, v := range customHeaders {
			header, ok := v.(string)
			if !ok {
				return nil, nil, fmt.Errorf("invalid type for headers value %q", v)
			}
			req.Header.Add(k, header)
		}

		// Don't overwrite or append to one that was set in the custom headers
		if _, hasUA := customHeaders["User-Agent"]; !hasUA {
			req.Header.Add("User-Agent", version.UserAgent)
		}

		// If the caller specifies the Host header, use it for the HTTP
		// request host and the TLS server name.
		if host, hasHost := customHeaders["Host"]; hasHost {
			req.Host = host.(string) // We already checked that it's a string.
		}
	}

	if len(bctx.DistributedTracingOpts) > 0 {
		client.Transport = tracing.NewTransport(client.Transport, bctx.DistributedTracingOpts)
	}

	return req, client, nil
}

func hasTLSConfig(tlsConfig *tls.Config) bool {
	return tlsConfig.InsecureSkipVerify ||
		tlsConfig.ServerName != "" ||
		tlsConfig.RootCAs != nil ||
		len(tlsConfig.Certificates) > 0
}

func useLastResponseRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

func executeHTTPRequest(req *http.Request, client *http.Client, inputReqObj ast.Object) (*http.Response, error) {
	var err error
	var retry int

	retry, err = getNumberValFromReqObj(inputReqObj, ast.InternedTerm("max_retry_attempts"))
	if err != nil {
		return nil, err
	}

	for i := 0; true; i++ {

		var resp *http.Response
		resp, err = client.Do(req)
		if err == nil {
			return resp, nil
		}

		// final attempt
		if i == retry {
			break
		}

		if err == context.Canceled {
			return nil, err
		}

		delay := util.DefaultBackoff(float64(minRetryDelay), float64(maxRetryDelay), i)
		timer, timerCancel := util.TimerWithCancel(delay)
		select {
		case <-timer.C:
		case <-req.Context().Done():
			timerCancel() // explicitly cancel the timer.
			return nil, context.Canceled
		}
	}
	return nil, err
}

func emptyBytesBuffer() *bytes.Buffer {
	return bytes.NewBuffer([]byte{})
}

func isJSONType(header http.Header) bool {
	t, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return false
	}

	mediaType := strings.Split(t, "/")
	if len(mediaType) != 2 {
		return false
	}

	if mediaType[0] == "application" {
		if mediaType[1] == "json" || strings.HasSuffix(mediaType[1], "+json") {
			return true
		}
	}

	return false
}

func isContentType(header http.Header, typ ...string) bool {
	for _, t := range typ {
		if strings.Contains(header.Get("Content-Type"), t) {
			return true
		}
	}
	return false
}

func newHTTPSendCache() *httpSendCache {
	return &httpSendCache{
		entries: util.NewHasherMap[ast.Value, httpSendCacheEntry](ast.ValueEqual),
	}
}

func (cache *httpSendCache) get(k ast.Value) *httpSendCacheEntry {
	if v, ok := cache.entries.Get(k); ok {
		return &v
	}
	return nil
}

func (cache *httpSendCache) putResponse(k ast.Value, v *ast.Value) {
	cache.entries.Put(k, httpSendCacheEntry{response: v})
}

func (cache *httpSendCache) putError(k ast.Value, v error) {
	cache.entries.Put(k, httpSendCacheEntry{error: v})
}

// In the BuiltinContext cache we only store a single entry that points to
// our ValueMap which is the "real" http.send() cache.
func getHTTPSendCache(bctx BuiltinContext) *httpSendCache {
	raw, ok := bctx.Cache.Get(httpSendBuiltinCacheKey)
	if !ok {
		// Initialize if it isn't there
		c := newHTTPSendCache()
		bctx.Cache.Put(httpSendBuiltinCacheKey, c)
		return c
	}

	c, ok := raw.(*httpSendCache)
	if !ok {
		return nil
	}
	return c
}

// checkHTTPSendCache checks for the given key's value in the cache
func checkHTTPSendCache(bctx BuiltinContext, key ast.Object) (ast.Value, error) {
	requestCache := getHTTPSendCache(bctx)
	if requestCache == nil {
		return nil, nil
	}

	v := requestCache.get(key)
	if v != nil {
		if v.error != nil {
			return nil, v.error
		}
		if v.response != nil {
			return *v.response, nil
		}
		// This should never happen
	}

	return nil, nil
}

func insertIntoHTTPSendCache(bctx BuiltinContext, key ast.Object, value ast.Value) {
	requestCache := getHTTPSendCache(bctx)
	if requestCache == nil {
		// Should never happen.. if it does just skip caching the value
		// FIXME: return error instead, to prevent inconsistencies?
		return
	}
	requestCache.putResponse(key, &value)
}

func insertErrorIntoHTTPSendCache(bctx BuiltinContext, key ast.Object, err error) {
	requestCache := getHTTPSendCache(bctx)
	if requestCache == nil {
		// Should never happen.. if it does just skip caching the value
		// FIXME: return error instead, to prevent inconsistencies?
		return
	}
	requestCache.putError(key, err)
}

// checkHTTPSendInterQueryCache checks for the given key's value in the inter-query cache
func (c *interQueryCache) checkHTTPSendInterQueryCache() (ast.Value, error) {
	requestCache := c.bctx.InterQueryBuiltinCache

	cachedValue, found := requestCache.Get(c.key)
	if !found {
		return nil, nil
	}

	value, cerr := requestCache.Clone(cachedValue)
	if cerr != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, cerr)
	}

	c.bctx.Metrics.Counter(httpSendInterQueryCacheHits).Incr()
	var cachedRespData *interQueryCacheData

	switch v := value.(type) {
	case *interQueryCacheValue:
		var err error
		cachedRespData, err = v.copyCacheData()
		if err != nil {
			return nil, err
		}
	case *interQueryCacheData:
		cachedRespData = v
	default:
		return nil, nil
	}

	if getCurrentTime(c.bctx.Time).Before(cachedRespData.ExpiresAt) {
		return cachedRespData.formatToAST(c.forceJSONDecode, c.forceYAMLDecode)
	}

	var err error
	c.httpReq, c.httpClient, err = createHTTPRequest(c.bctx, c.key)
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	headers := &responseHeaders{
		etag:         cachedRespData.Headers.Get("etag"),
		lastModified: cachedRespData.Headers.Get("last-modified"),
	}

	// check with the server if the stale response is still up-to-date.
	// If server returns a new response (ie. status_code=200), update the cache with the new response
	// If server returns an unmodified response (ie. status_code=304), update the headers for the existing response
	result, modified, err := revalidateCachedResponse(c.httpReq, c.httpClient, c.key, headers)
	requestCache.Delete(c.key)
	if err != nil || result == nil {
		return nil, err
	}

	defer result.Body.Close()

	if !modified {
		// update the headers in the cached response with their corresponding values from the 304 (Not Modified) response
		for headerName, values := range result.Header {
			cachedRespData.Headers.Del(headerName)
			for _, v := range values {
				cachedRespData.Headers.Add(headerName, v)
			}
		}

		if forceCaching(c.forceCacheParams) {
			createdAt := getCurrentTime(c.bctx.Time)
			cachedRespData.ExpiresAt = createdAt.Add(time.Second * time.Duration(c.forceCacheParams.forceDurationSeconds))
		} else {
			expiresAt, err := expiryFromHeaders(result.Header)
			if err != nil {
				return nil, err
			}
			cachedRespData.ExpiresAt = expiresAt
		}

		cachingMode, err := getCachingMode(c.key)
		if err != nil {
			return nil, err
		}

		var pcv cache.InterQueryCacheValue

		if cachingMode == defaultCachingMode {
			pcv, err = cachedRespData.toCacheValue()
			if err != nil {
				return nil, err
			}
		} else {
			pcv = cachedRespData
		}

		c.bctx.InterQueryBuiltinCache.InsertWithExpiry(c.key, pcv, cachedRespData.ExpiresAt)

		return cachedRespData.formatToAST(c.forceJSONDecode, c.forceYAMLDecode)
	}

	newValue, respBody, err := formatHTTPResponseToAST(result, c.forceJSONDecode, c.forceYAMLDecode)
	if err != nil {
		return nil, err
	}

	if err := insertIntoHTTPSendInterQueryCache(c.bctx, c.key, result, respBody, c.forceCacheParams); err != nil {
		return nil, err
	}

	return newValue, nil
}

// insertIntoHTTPSendInterQueryCache inserts given key and value in the inter-query cache
func insertIntoHTTPSendInterQueryCache(
	bctx BuiltinContext,
	key ast.Value,
	resp *http.Response,
	respBody []byte,
	cacheParams forceCacheParams,
) error {
	if resp == nil || (!forceCaching(cacheParams) && !canStore(resp.Header)) || !cacheableCodes.Contains(ast.InternedTerm(resp.StatusCode)) {
		return nil
	}

	obj, ok := key.(ast.Object)
	if !ok {
		return errors.New("interface conversion error")
	}

	cachingMode, err := getCachingMode(obj)
	if err != nil {
		return err
	}

	var pcv cache.InterQueryCacheValue
	var pcvData *interQueryCacheData
	if cachingMode == defaultCachingMode {
		pcv, pcvData, err = newInterQueryCacheValue(bctx.Time, resp, respBody, cacheParams)
	} else {
		pcvData, err = newInterQueryCacheData(bctx.Time, resp, respBody, cacheParams)
		pcv = pcvData
	}

	if err != nil {
		return err
	}

	bctx.InterQueryBuiltinCache.InsertWithExpiry(key, pcv, pcvData.ExpiresAt)
	return nil
}

func createCacheableHTTPStatusCodes() {
	for _, element := range cacheableHTTPStatusCodes {
		cacheableCodes.Add(ast.InternedTerm(element))
	}
}

func parseTimeout(timeoutVal ast.Value) (timeout time.Duration, err error) {
	switch t := timeoutVal.(type) {
	case ast.Number:
		if timeoutInt, ok := t.Int64(); ok {
			return time.Duration(timeoutInt), nil
		}
		err = fmt.Errorf("invalid timeout number value %v, must be int64", timeoutVal)
	case ast.String:
		// Support strings without a unit, treat them the same as just a number value (ns)
		if timeoutInt, ok := util.Atoi64(string(t)); ok {
			return time.Duration(timeoutInt), nil
		}
		// Try parsing it as a duration (requires a supported units suffix)
		if timeout, err = time.ParseDuration(string(t)); err != nil {
			err = fmt.Errorf("invalid timeout value %v: %s", timeoutVal, err)
		}
	default:
		err = builtins.NewOperandErr(1, "'timeout' must be one of {string, number} but got %s", ast.ValueName(t))
	}
	return timeout, err
}

func getBoolValFromReqObj(req ast.Object, key *ast.Term) (bool, error) {
	var b ast.Boolean
	var ok bool
	if v := req.Get(key); v != nil {
		if b, ok = v.Value.(ast.Boolean); !ok {
			return false, fmt.Errorf("invalid value for %v field", key.String())
		}
	}
	return bool(b), nil
}

func getNumberValFromReqObj(req ast.Object, key *ast.Term) (int, error) {
	term := req.Get(key)
	if term == nil {
		return 0, nil
	}

	if t, ok := term.Value.(ast.Number); ok {
		num, ok := t.Int()
		if !ok || num < 0 {
			return 0, fmt.Errorf("invalid value %v for field %v", t.String(), key.String())
		}
		return num, nil
	}

	return 0, fmt.Errorf("invalid value %v for field %v", term.String(), key.String())
}

func getCachingMode(req ast.Object) (cachingMode, error) {
	key := ast.InternedTerm("caching_mode")
	var s ast.String
	var ok bool
	if v := req.Get(key); v != nil {
		if s, ok = v.Value.(ast.String); !ok {
			return "", fmt.Errorf("invalid value for %v field", key.String())
		}

		switch cachingMode(s) {
		case defaultCachingMode, cachingModeDeserialized:
			return cachingMode(s), nil
		default:
			return "", fmt.Errorf("invalid value specified for %v field: %v", key.String(), string(s))
		}
	}
	return defaultCachingMode, nil
}

func newInterQueryCacheValue(
	now *ast.Term,
	resp *http.Response,
	respBody []byte,
	cacheParams forceCacheParams,
) (*interQueryCacheValue, *interQueryCacheData, error) {
	data, err := newInterQueryCacheData(now, resp, respBody, cacheParams)
	if err != nil {
		return nil, nil, err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}
	return &interQueryCacheValue{Data: b}, data, nil
}

func (cb interQueryCacheValue) Clone() (cache.InterQueryCacheValue, error) {
	return &interQueryCacheValue{Data: slices.Clone(cb.Data)}, nil
}

func (cb interQueryCacheValue) SizeInBytes() int64 {
	return int64(len(cb.Data))
}

func (cb *interQueryCacheValue) copyCacheData() (res *interQueryCacheData, err error) {
	err = util.UnmarshalJSON(cb.Data, &res)
	return res, err
}

func forceCaching(cacheParams forceCacheParams) bool {
	return cacheParams.forceDurationSeconds > 0
}

func expiryFromHeaders(headers http.Header) (time.Time, error) {
	var expiresAt time.Time
	maxAge, err := parseMaxAgeCacheDirective(parseCacheControlHeader(headers))
	if err != nil {
		return time.Time{}, err
	}
	if maxAge != -1 {
		createdAt, err := getResponseHeaderDate(headers)
		if err != nil {
			return time.Time{}, err
		}
		expiresAt = createdAt.Add(time.Second * time.Duration(maxAge))
	} else {
		expiresAt = getResponseHeaderExpires(headers)
	}
	return expiresAt, nil
}

func newInterQueryCacheData(
	now *ast.Term,
	resp *http.Response,
	respBody []byte,
	cacheParams forceCacheParams,
) (data *interQueryCacheData, err error) {
	data = &interQueryCacheData{
		RespBody:   respBody,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}
	if forceCaching(cacheParams) {
		data.ExpiresAt = getCurrentTime(now).Add(time.Second * time.Duration(cacheParams.forceDurationSeconds))
	} else {
		data.ExpiresAt, err = expiryFromHeaders(resp.Header)
	}

	return data, err
}

func (c *interQueryCacheData) formatToAST(forceJSONDecode, forceYAMLDecode bool) (ast.Value, error) {
	return prepareASTResult(c.Headers, forceJSONDecode, forceYAMLDecode, c.RespBody, c.Status, c.StatusCode)
}

func (c *interQueryCacheData) toCacheValue() (*interQueryCacheValue, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return &interQueryCacheValue{Data: b}, nil
}

func (*interQueryCacheData) SizeInBytes() int64 {
	return 0
}

func (c *interQueryCacheData) Clone() (cache.InterQueryCacheValue, error) {
	return &interQueryCacheData{
		ExpiresAt:  c.ExpiresAt,
		RespBody:   slices.Clone(c.RespBody),
		Status:     c.Status,
		StatusCode: c.StatusCode,
		Headers:    c.Headers.Clone(),
	}, nil
}

func revalidateCachedResponse(
	req *http.Request,
	client *http.Client,
	inputReqObj ast.Object,
	headers *responseHeaders,
) (*http.Response, bool, error) {
	if headers.etag == "" && headers.lastModified == "" {
		return nil, false, nil
	}

	cloneReq := req.Clone(req.Context())
	if headers.etag != "" {
		cloneReq.Header.Set("if-none-match", headers.etag)
	}
	if headers.lastModified != "" {
		cloneReq.Header.Set("if-modified-since", headers.lastModified)
	}

	response, err := executeHTTPRequest(cloneReq, client, inputReqObj)
	if err != nil {
		return nil, false, err
	}

	if isOK := response.StatusCode == http.StatusOK; isOK || response.StatusCode == http.StatusNotModified {
		return response, isOK, nil
	}

	util.Close(response)
	return nil, false, nil
}

func canStore(headers http.Header) bool {
	// Check "no-store" cache directive
	// The "no-store" response directive indicates that a cache MUST NOT
	// store any part of either the immediate request or response.
	_, ok := parseCacheControlHeader(headers)["no-store"]
	return !ok
}

func getCurrentTime(now *ast.Term) time.Time {
	if valueNum, ok := now.Value.(ast.Number); ok {
		if valueNumInt, ok := valueNum.Int64(); ok {
			return time.Unix(0, valueNumInt).UTC()
		}
	}
	return time.Now().UTC()
}

func parseCacheControlHeader(headers http.Header) map[string]string {
	ccDirectives := map[string]string{}
	ccHeader := headers.Get("cache-control")

	for part := range strings.SplitSeq(ccHeader, ",") {
		part = strings.Trim(part, " ")
		if part == "" {
			continue
		}
		if strings.ContainsRune(part, '=') {
			if strings.Count(part, "=") == 1 {
				left, right, _ := strings.Cut(part, "=")
				ccDirectives[strings.Trim(left, " ")] = strings.Trim(right, ",")
			}
		} else {
			ccDirectives[part] = ""
		}
	}

	return ccDirectives
}

func getResponseHeaderDate(headers http.Header) (date time.Time, err error) {
	if dateHeader := headers.Get("date"); dateHeader != "" {
		return http.ParseTime(dateHeader)
	}
	return date, errors.New("no date header")
}

func getResponseHeaderExpires(headers http.Header) (exp time.Time) {
	if expiresHeader := headers.Get("expires"); expiresHeader != "" {
		exp, _ = http.ParseTime(expiresHeader)
	}
	return exp
}

// parseMaxAgeCacheDirective parses the max-age directive expressed in delta-seconds as per
// https://tools.ietf.org/html/rfc7234#section-1.2.1
func parseMaxAgeCacheDirective(cc map[string]string) (deltaSeconds, error) {
	maxAge, ok := cc["max-age"]
	if !ok {
		return deltaSeconds(-1), nil
	}

	val, err := strconv.ParseUint(maxAge, 10, 32)
	if err != nil {
		if numError, ok := err.(*strconv.NumError); ok && numError.Err == strconv.ErrRange {
			return deltaSeconds(math.MaxInt32), nil
		}
		return deltaSeconds(-1), err
	}

	return deltaSeconds(min(val, math.MaxInt32)), nil
}

func formatHTTPResponseToAST(resp *http.Response, forceJSONDecode, forceYAMLDecode bool) (ast.Value, []byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	resultObj, err := prepareASTResult(resp.Header, forceJSONDecode, forceYAMLDecode, raw, resp.Status, resp.StatusCode)

	return resultObj, raw, err
}

func prepareASTResult(
	headers http.Header,
	forceJSONDecode, forceYAMLDecode bool,
	body []byte,
	status string,
	statusCode int,
) (ast.Value, error) {
	var resultBody any

	// If the response body cannot be JSON/YAML decoded,
	// an error will not be returned. Instead, the "body" field
	// in the result will be null.
	switch {
	case forceJSONDecode || isJSONType(headers):
		_ = util.UnmarshalJSON(body, &resultBody)
	case forceYAMLDecode || isContentType(headers, "application/yaml", "application/x-yaml"):
		_ = util.Unmarshal(body, &resultBody)
	}

	bodyValue, err := ast.InterfaceToValue(resultBody)
	if err != nil {
		return nil, err
	}

	return ast.NewObject(
		ast.Item(ast.InternedTerm("status"), ast.InternedTerm(status)),
		ast.Item(ast.InternedTerm("status_code"), ast.InternedTerm(statusCode)),
		ast.Item(ast.InternedTerm("body"), ast.NewTerm(bodyValue)),
		ast.Item(ast.InternedTerm("raw_body"), ast.InternedTerm(util.ByteSliceToString(body))),
		ast.Item(ast.InternedTerm("headers"), getResponseHeaders(headers)),
	), nil
}

func getResponseHeaders(headers http.Header) *ast.Term {
	if len(headers) == 0 {
		return ast.InternedEmptyObject
	}
	return ast.NewTerm(ast.MapToObject(headers, strings.ToLower, arrayFromStringSlice))
}

// newHTTPRequestExecutor returns a new HTTP request executor that wraps either an inter-query or
// intra-query cache implementation
func newHTTPRequestExecutor(bctx BuiltinContext, req ast.Object, key ast.Object) (httpRequestExecutor, error) {
	useInterQueryCache, forceCacheParams, err := useInterQueryCache(req)
	if err != nil {
		return nil, handleHTTPSendErr(bctx.Context, bctx.Location, err)
	}

	if useInterQueryCache && bctx.InterQueryBuiltinCache != nil {
		return newInterQueryCache(bctx, req, key, forceCacheParams)
	}
	return newIntraQueryCache(bctx, req, key)
}

func newInterQueryCache(bctx BuiltinContext, req ast.Object, key ast.Object, forceCacheParams forceCacheParams) (*interQueryCache, error) {
	return &interQueryCache{bctx: bctx, req: req, key: key, forceCacheParams: forceCacheParams}, nil
}

// CheckCache checks the cache for the value of the key set on this object
func (c *interQueryCache) CheckCache() (resp ast.Value, err error) {
	// Checking the intra-query cache first ensures consistency of errors and HTTP responses within a query.
	if resp, err = checkHTTPSendCache(c.bctx, c.key); err != nil || resp != nil {
		return resp, err
	}

	if c.forceJSONDecode, err = getBoolValFromReqObj(c.key, ast.InternedTerm("force_json_decode")); err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}
	if c.forceYAMLDecode, err = getBoolValFromReqObj(c.key, ast.InternedTerm("force_yaml_decode")); err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	resp, err = c.checkHTTPSendInterQueryCache()
	// Always insert the result of the inter-query cache into the intra-query cache, to maintain consistency within the same query.
	if err != nil {
		insertErrorIntoHTTPSendCache(c.bctx, c.key, err)
	}
	if resp != nil {
		insertIntoHTTPSendCache(c.bctx, c.key, resp)
	}
	return resp, err
}

// InsertIntoCache inserts the key set on this object into the cache with the given value
func (c *interQueryCache) InsertIntoCache(value *http.Response) (ast.Value, error) {
	result, respBody, err := formatHTTPResponseToAST(value, c.forceJSONDecode, c.forceYAMLDecode)
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	// Always insert into the intra-query cache, to maintain consistency within the same query.
	insertIntoHTTPSendCache(c.bctx, c.key, result)

	// We ignore errors when populating the inter-query cache, because we've already populated the intra-cache,
	// and query consistency is our primary concern.
	_ = insertIntoHTTPSendInterQueryCache(c.bctx, c.key, value, respBody, c.forceCacheParams)
	return result, nil
}

func (c *interQueryCache) InsertErrorIntoCache(err error) {
	insertErrorIntoHTTPSendCache(c.bctx, c.key, err)
}

// ExecuteHTTPRequest executes a HTTP request
func (c *interQueryCache) ExecuteHTTPRequest() (*http.Response, error) {
	var err error
	c.httpReq, c.httpClient, err = createHTTPRequest(c.bctx, c.req)
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	// Increment counter for actual network requests
	c.bctx.Metrics.Counter(httpSendNetworkRequests).Incr()

	return executeHTTPRequest(c.httpReq, c.httpClient, c.req)
}

func newIntraQueryCache(bctx BuiltinContext, req ast.Object, key ast.Object) (*intraQueryCache, error) {
	return &intraQueryCache{bctx: bctx, req: req, key: key}, nil
}

// CheckCache checks the cache for the value of the key set on this object
func (c *intraQueryCache) CheckCache() (ast.Value, error) {
	return checkHTTPSendCache(c.bctx, c.key)
}

// InsertIntoCache inserts the key set on this object into the cache with the given value
func (c *intraQueryCache) InsertIntoCache(value *http.Response) (ast.Value, error) {
	forceJSONDecode, err := getBoolValFromReqObj(c.key, ast.InternedTerm("force_json_decode"))
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}
	forceYAMLDecode, err := getBoolValFromReqObj(c.key, ast.InternedTerm("force_yaml_decode"))
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	result, _, err := formatHTTPResponseToAST(value, forceJSONDecode, forceYAMLDecode)
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	if cacheableCodes.Contains(ast.InternedTerm(value.StatusCode)) {
		insertIntoHTTPSendCache(c.bctx, c.key, result)
	}

	return result, nil
}

func (c *intraQueryCache) InsertErrorIntoCache(err error) {
	insertErrorIntoHTTPSendCache(c.bctx, c.key, err)
}

// ExecuteHTTPRequest executes a HTTP request
func (c *intraQueryCache) ExecuteHTTPRequest() (*http.Response, error) {
	httpReq, httpClient, err := createHTTPRequest(c.bctx, c.req)
	if err != nil {
		return nil, handleHTTPSendErr(c.bctx.Context, c.bctx.Location, err)
	}

	// Increment counter for actual network requests
	c.bctx.Metrics.Counter(httpSendNetworkRequests).Incr()

	return executeHTTPRequest(httpReq, httpClient, c.req)
}

func useInterQueryCache(req ast.Object) (bool, forceCacheParams, error) {
	value, err := getBoolValFromReqObj(req, ast.InternedTerm("cache"))
	if err != nil {
		return false, forceCacheParams{}, err
	}

	valueForceCache, err := getBoolValFromReqObj(req, ast.InternedTerm("force_cache"))
	if err != nil {
		return false, forceCacheParams{}, err
	}

	if valueForceCache {
		forceCacheParams, err := newForceCacheParams(req)
		return true, forceCacheParams, err
	}

	return value, forceCacheParams{}, nil
}

func newForceCacheParams(req ast.Object) (p forceCacheParams, err error) {
	term := req.Get(ast.InternedTerm("force_cache_duration_seconds"))
	if term == nil {
		return p, errors.New("'force_cache' set but 'force_cache_duration_seconds' parameter is missing")
	}

	forceCacheDurationSeconds := term.String()

	value, err := strconv.ParseInt(forceCacheDurationSeconds, 10, 32)
	if err != nil {
		return p, err
	}

	return forceCacheParams{forceDurationSeconds: int32(value)}, nil
}

func getRaiseErrorValue(req ast.Object) (bool, error) {
	result := ast.Boolean(true)
	var ok bool
	if v := req.Get(ast.InternedTerm("raise_error")); v != nil {
		if result, ok = v.Value.(ast.Boolean); !ok {
			return false, errors.New("invalid value for raise_error field")
		}
	}
	return bool(result), nil
}

func arrayFromStringSlice(slice []string) *ast.Term {
	return ast.ArrayTerm(util.Map(slice, ast.InternedTerm)...)
}

//go:build usegorillamux

// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"time"

	"github.com/gorilla/mux"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/storage/disk"
	"github.com/open-policy-agent/opa/tracing"
)

// Params stores the configuration for an OPA instance.
type Params struct {

	// Globally unique identifier for this OPA instance. If an ID is not specified,
	// the runtime will generate one.
	ID string

	// Addrs are the listening addresses that the OPA server will bind to.
	Addrs *[]string

	// DiagnosticAddrs are the listening addresses that the OPA server will bind to
	// for read-only diagnostic API's (/health, /metrics, etc)
	DiagnosticAddrs *[]string

	// H2CEnabled flag controls whether OPA will allow H2C (HTTP/2 cleartext) on
	// HTTP listeners.
	H2CEnabled bool

	// Authentication is the type of authentication scheme to use.
	Authentication server.AuthenticationScheme

	// Authorization is the type of authorization scheme to use.
	Authorization server.AuthorizationScheme

	// Certificate is the certificate to use in server-mode. If the certificate
	// is nil, the server will NOT use TLS.
	Certificate *tls.Certificate

	// CertificateFile and CertificateKeyFile are the paths to the cert and its
	// keyfile. It'll be used to periodically reload the files from disk if they
	// have changed. The server will attempt to refresh every 5 minutes, unless
	// a different CertificateRefresh time.Duration is provided
	CertificateFile    string
	CertificateKeyFile string
	CertificateRefresh time.Duration

	// CertPool holds the CA certs trusted by the OPA server.
	CertPool *x509.CertPool

	// MinVersion contains the minimum TLS version that is acceptable.
	// If zero, TLS 1.2 is currently taken as the minimum.
	MinTLSVersion uint16

	// HistoryPath is the filename to store the interactive shell user
	// input history.
	HistoryPath string

	// Output format controls how the REPL will print query results.
	// Default: "pretty".
	OutputFormat string

	// Paths contains filenames of base documents and policy modules to load on
	// startup. Data files may be prefixed with "<dotted-path>:" to indicate
	// where the contained document should be loaded.
	Paths []string

	// Optional filter that will be passed to the file loader.
	Filter loader.Filter

	// BundleMode will enable treating the Paths provided as bundles rather than
	// loading all data & policy files.
	BundleMode bool

	// Watch flag controls whether OPA will watch the Paths files for changes.
	// If this flag is true, OPA will watch the Paths files for changes and
	// reload the storage layer each time they change. This is useful for
	// interactive development.
	Watch bool

	// ErrorLimit is the number of errors the compiler will allow to occur before
	// exiting early.
	ErrorLimit int

	// PprofEnabled flag controls whether pprof endpoints are enabled
	PprofEnabled bool

	// DecisionIDFactory generates decision IDs to include in API responses
	// sent by the server (in response to Data API queries.)
	DecisionIDFactory func() string

	// Logging configures the logging behaviour.
	Logging LoggingConfig

	// Logger sets the logger implementation to use for debug logs.
	Logger logging.Logger

	// ConsoleLogger sets the logger implementation to use for console logs.
	ConsoleLogger logging.Logger

	// ConfigFile refers to the OPA configuration to load on startup.
	ConfigFile string

	// ConfigOverrides are overrides for the OPA configuration that are applied
	// over top the config file They are in a list of key=value syntax that
	// conform to the syntax defined in the `strval` package
	ConfigOverrides []string

	// ConfigOverrideFiles Similar to `ConfigOverrides` except they are in the
	// form of `key=path/to/file`where the file contains the value to be used.
	ConfigOverrideFiles []string

	// Output is the output stream used when run as an interactive shell. This
	// is mostly for test purposes.
	Output io.Writer

	// GracefulShutdownPeriod is the time (in seconds) to wait for the http
	// server to shutdown gracefully.
	GracefulShutdownPeriod int

	// ShutdownWaitPeriod is the time (in seconds) to wait before initiating shutdown.
	ShutdownWaitPeriod int

	// EnableVersionCheck flag controls whether OPA will report its version to an external service.
	// If this flag is true, OPA will report its version to the external service
	EnableVersionCheck bool

	// BundleVerificationConfig sets the key configuration used to verify a signed bundle
	BundleVerificationConfig *bundle.VerificationConfig

	// SkipBundleVerification flag controls whether OPA will verify a signed bundle
	SkipBundleVerification bool

	// ReadyTimeout flag controls if and for how long OPA server will wait (in seconds) for
	// configured bundles and plugins to be activated/ready before listening for traffic.
	// A value of 0 or less means no wait is exercised.
	ReadyTimeout int

	// Router is the router to which handlers for the REST API are added.
	// Router uses a first-matching-route-wins strategy, so no existing routes are overridden
	// If it is nil, a new mux.Router will be created
	Router *mux.Router

	// DiskStorage, if set, will make the runtime instantiate a disk-backed storage
	// implementation (instead of the default, in-memory store).
	// It can also be enabled via config, and this runtime field takes precedence.
	DiskStorage *disk.Options

	DistributedTracingOpts tracing.Options
}

func newRouter() *mux.Router {
	return mux.NewRouter()
}

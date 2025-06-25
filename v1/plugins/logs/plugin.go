// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package logs implements decision log buffering and uploading.
package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	lstat "github.com/open-policy-agent/opa/v1/plugins/logs/status"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/server"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/util"
)

// Logger defines the interface for decision logging plugins.
type Logger interface {
	plugins.Plugin

	Log(context.Context, EventV1) error
}

// EventV1 represents a decision log event.
// WARNING: The AST() function for EventV1 must be kept in sync with
// the struct. Any changes here MUST be reflected in the AST()
// implementation below.
type EventV1 struct {
	Labels         map[string]string       `json:"labels"`
	DecisionID     string                  `json:"decision_id"`
	TraceID        string                  `json:"trace_id,omitempty"`
	SpanID         string                  `json:"span_id,omitempty"`
	Revision       string                  `json:"revision,omitempty"` // Deprecated: Use Bundles instead
	Bundles        map[string]BundleInfoV1 `json:"bundles,omitempty"`
	Path           string                  `json:"path,omitempty"`
	Query          string                  `json:"query,omitempty"`
	Input          *any                    `json:"input,omitempty"`
	Result         *any                    `json:"result,omitempty"`
	MappedResult   *any                    `json:"mapped_result,omitempty"`
	NDBuiltinCache *any                    `json:"nd_builtin_cache,omitempty"`
	Erased         []string                `json:"erased,omitempty"`
	Masked         []string                `json:"masked,omitempty"`
	Error          error                   `json:"error,omitempty"`
	RequestedBy    string                  `json:"requested_by,omitempty"`
	Timestamp      time.Time               `json:"timestamp"`
	Metrics        map[string]any          `json:"metrics,omitempty"`
	RequestID      uint64                  `json:"req_id,omitempty"`
	RequestContext *RequestContext         `json:"request_context,omitempty"`

	inputAST ast.Value
}

// BundleInfoV1 describes a bundle associated with a decision log event.
type BundleInfoV1 struct {
	Revision string `json:"revision,omitempty"`
}

type RequestContext struct {
	HTTPRequest *HTTPRequestContext `json:"http,omitempty"`
}

type HTTPRequestContext struct {
	Headers map[string][]string `json:"headers,omitempty"`
}

// AST returns the BundleInfoV1 as an AST value
func (b *BundleInfoV1) AST() ast.Value {
	result := ast.NewObject()
	if len(b.Revision) > 0 {
		result.Insert(ast.InternedTerm("revision"), ast.StringTerm(b.Revision))
	}
	return result
}

// AST returns the Rego AST representation for a given EventV1 object.
// This avoids having to round trip through JSON while applying a decision log
// mask policy to the event.
func (e *EventV1) AST() (ast.Value, error) {
	var err error
	event := ast.NewObject(
		ast.Item(ast.InternedTerm("decision_id"), ast.StringTerm(e.DecisionID)),
	)

	if e.Labels != nil {
		labelsObj := ast.NewObject()
		for k, v := range e.Labels {
			labelsObj.Insert(ast.StringTerm(k), ast.StringTerm(v))
		}
		event.Insert(ast.InternedTerm("labels"), ast.NewTerm(labelsObj))
	} else {
		event.Insert(ast.InternedTerm("labels"), ast.NullTerm())
	}

	if len(e.Revision) > 0 {
		event.Insert(ast.InternedTerm("revision"), ast.StringTerm(e.Revision))
	}

	if len(e.Bundles) > 0 {
		bundlesObj := ast.NewObject()
		for k, v := range e.Bundles {
			bundlesObj.Insert(ast.StringTerm(k), ast.NewTerm(v.AST()))
		}
		event.Insert(ast.InternedTerm("bundles"), ast.NewTerm(bundlesObj))
	}

	if len(e.Path) > 0 {
		event.Insert(ast.InternedTerm("path"), ast.StringTerm(e.Path))
	}

	if len(e.Query) > 0 {
		event.Insert(ast.InternedTerm("query"), ast.StringTerm(e.Query))
	}

	if e.inputAST != nil {
		event.Insert(ast.InternedTerm("input"), ast.NewTerm(e.inputAST))
	} else if e.Input != nil {
		e.inputAST, err = roundtripJSONToAST(e.Input)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("input"), ast.NewTerm(e.inputAST))
	}

	if e.Result != nil {
		results, err := roundtripJSONToAST(e.Result)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("result"), ast.NewTerm(results))
	}

	if e.MappedResult != nil {
		mResults, err := roundtripJSONToAST(e.MappedResult)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("mapped_result"), ast.NewTerm(mResults))
	}

	if e.NDBuiltinCache != nil {
		ndbCache, err := roundtripJSONToAST(e.NDBuiltinCache)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("nd_builtin_cache"), ast.NewTerm(ndbCache))
	}

	if len(e.Erased) > 0 {
		erased := make([]*ast.Term, len(e.Erased))
		for i, v := range e.Erased {
			erased[i] = ast.StringTerm(v)
		}
		event.Insert(ast.InternedTerm("erased"), ast.ArrayTerm(erased...))
	}

	if len(e.Masked) > 0 {
		masked := make([]*ast.Term, len(e.Masked))
		for i, v := range e.Masked {
			masked[i] = ast.StringTerm(v)
		}
		event.Insert(ast.InternedTerm("masked"), ast.ArrayTerm(masked...))
	}

	if e.Error != nil {
		evalErr, err := roundtripJSONToAST(e.Error)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("error"), ast.NewTerm(evalErr))
	}

	if len(e.RequestedBy) > 0 {
		event.Insert(ast.InternedTerm("requested_by"), ast.StringTerm(e.RequestedBy))
	}

	// Use the timestamp JSON marshaller to ensure the format is the same as
	// round tripping through JSON.
	timeBytes, err := e.Timestamp.MarshalJSON()
	if err != nil {
		return nil, err
	}
	event.Insert(ast.InternedTerm("timestamp"), ast.StringTerm(strings.Trim(string(timeBytes), "\"")))

	if e.Metrics != nil {
		m, err := ast.InterfaceToValue(e.Metrics)
		if err != nil {
			return nil, err
		}
		event.Insert(ast.InternedTerm("metrics"), ast.NewTerm(m))
	}

	if e.RequestID > 0 {
		event.Insert(ast.InternedTerm("req_id"), ast.UIntNumberTerm(e.RequestID))
	}

	return event, nil
}

func roundtripJSONToAST(x any) (ast.Value, error) {
	rawPtr := util.Reference(x)
	// roundtrip through json: this turns slices (e.g. []string, []bool) into
	// []any, the only array type ast.InterfaceToValue can work with
	if err := util.RoundTrip(rawPtr); err != nil {
		return nil, err
	}

	return ast.InterfaceToValue(*rawPtr)
}

const (
	// min amount of time to wait following a failure
	minRetryDelay                       = time.Millisecond * 100
	defaultMinDelaySeconds              = int64(300)
	defaultMaxDelaySeconds              = int64(600)
	defaultBufferSizeLimitEvents        = int64(10000)
	defaultUploadSizeLimitBytes         = int64(32768)      // 32KB limit
	minUploadSizeLimitBytes             = int64(90)         // A single event with a decision ID (69 bytes) + empty gzip file (21 bytes)
	maxUploadSizeLimitBytes             = int64(4294967296) // about 4GB
	defaultBufferSizeLimitBytes         = int64(0)          // unlimited
	defaultMaskDecisionPath             = "/system/log/mask"
	defaultDropDecisionPath             = "/system/log/drop"
	logRateLimitExDropCounterName       = "decision_logs_dropped_rate_limit_exceeded"
	logBufferEventDropCounterName       = "decision_logs_dropped_buffer_size_limit_exceeded"
	logBufferSizeLimitExDropCounterName = "decision_logs_dropped_buffer_size_limit_bytes_exceeded"
	logEncodingFailureCounterName       = "decision_logs_encoding_failure"
	defaultResourcePath                 = "/logs"
	sizeBufferType                      = "size"
	eventBufferType                     = "event"
)

// ReportingConfig represents configuration for the plugin's reporting behaviour.
type ReportingConfig struct {
	BufferType            string               `json:"buffer_type,omitempty"`              // toggles how the buffer stores events, defaults to using bytes
	BufferSizeLimitBytes  *int64               `json:"buffer_size_limit_bytes,omitempty"`  // max size of in-memory size buffer
	BufferSizeLimitEvents *int64               `json:"buffer_size_limit_events,omitempty"` // max size of in-memory event channel buffer
	UploadSizeLimitBytes  *int64               `json:"upload_size_limit_bytes,omitempty"`  // max size of upload payload
	MinDelaySeconds       *int64               `json:"min_delay_seconds,omitempty"`        // min amount of time to wait between successful poll attempts
	MaxDelaySeconds       *int64               `json:"max_delay_seconds,omitempty"`        // max amount of time to wait between poll attempts
	MaxDecisionsPerSecond *float64             `json:"max_decisions_per_second,omitempty"` // max number of decision logs to buffer per second
	Trigger               *plugins.TriggerMode `json:"trigger,omitempty"`                  // trigger mode
}

type RequestContextConfig struct {
	HTTPRequest *HTTPRequestContextConfig `json:"http,omitempty"`
}

type HTTPRequestContextConfig struct {
	Headers []string `json:"headers,omitempty"`
}

// Config represents the plugin configuration.
type Config struct {
	Plugin          *string              `json:"plugin"`
	Service         string               `json:"service"`
	PartitionName   string               `json:"partition_name,omitempty"`
	Reporting       ReportingConfig      `json:"reporting"`
	RequestContext  RequestContextConfig `json:"request_context"`
	MaskDecision    *string              `json:"mask_decision"`
	DropDecision    *string              `json:"drop_decision"`
	ConsoleLogs     bool                 `json:"console"`
	Resource        *string              `json:"resource"`
	NDBuiltinCache  bool                 `json:"nd_builtin_cache,omitempty"`
	maskDecisionRef ast.Ref
	dropDecisionRef ast.Ref
}

func (c *Config) validateAndInjectDefaults(services []string, pluginsList []string, trigger *plugins.TriggerMode, l logging.Logger) error {

	if c.Plugin != nil {
		var found bool
		if slices.Contains(pluginsList, *c.Plugin) {
			found = true
		}
		if !found {
			return fmt.Errorf("invalid plugin name %q in decision_logs", *c.Plugin)
		}
	} else if c.Service == "" && len(services) != 0 && !c.ConsoleLogs {
		// For backwards compatibility allow defaulting to the first
		// service listed, but only if console logging is disabled. If enabled
		// we can't tell if the deployer wanted to use only console logs or
		// both console logs and the default service option.
		c.Service = services[0]
	} else if c.Service != "" {
		found := slices.Contains(services, c.Service)

		if !found {
			return fmt.Errorf("invalid service name %q in decision_logs", c.Service)
		}
	}

	t, err := plugins.ValidateAndInjectDefaultsForTriggerMode(trigger, c.Reporting.Trigger)
	if err != nil {
		return fmt.Errorf("invalid decision_log config: %w", err)
	}
	c.Reporting.Trigger = t

	min := defaultMinDelaySeconds
	max := defaultMaxDelaySeconds

	// reject bad min/max values
	if c.Reporting.MaxDelaySeconds != nil && c.Reporting.MinDelaySeconds != nil {
		if *c.Reporting.MaxDelaySeconds < *c.Reporting.MinDelaySeconds {
			return errors.New("max reporting delay must be >= min reporting delay in decision_logs")
		}
		min = *c.Reporting.MinDelaySeconds
		max = *c.Reporting.MaxDelaySeconds
	} else if c.Reporting.MaxDelaySeconds == nil && c.Reporting.MinDelaySeconds != nil {
		return errors.New("reporting configuration missing 'max_delay_seconds' in decision_logs")
	} else if c.Reporting.MinDelaySeconds == nil && c.Reporting.MaxDelaySeconds != nil {
		return errors.New("reporting configuration missing 'min_delay_seconds' in decision_logs")
	}

	// scale to seconds
	minSeconds := int64(time.Duration(min) * time.Second)
	c.Reporting.MinDelaySeconds = &minSeconds

	maxSeconds := int64(time.Duration(max) * time.Second)
	c.Reporting.MaxDelaySeconds = &maxSeconds

	// default the upload size limit
	uploadLimit := defaultUploadSizeLimitBytes
	if c.Reporting.UploadSizeLimitBytes != nil {
		uploadLimit = *c.Reporting.UploadSizeLimitBytes
	}

	switch {
	case uploadLimit > maxUploadSizeLimitBytes:
		maxUploadLimit := maxUploadSizeLimitBytes
		c.Reporting.UploadSizeLimitBytes = &maxUploadLimit
		if l != nil {
			l.Warn("the configured `upload_size_limit_bytes` (%d) has been set to the maximum limit (%d)", uploadLimit, maxUploadLimit)
		}
	case uploadLimit < minUploadSizeLimitBytes:
		minUploadLimit := minUploadSizeLimitBytes
		c.Reporting.UploadSizeLimitBytes = &minUploadLimit
		if l != nil {
			l.Warn("the configured `upload_size_limit_bytes` (%d) has been set to the minimum limit (%d)", uploadLimit, minUploadLimit)
		}
	default:
		c.Reporting.UploadSizeLimitBytes = &uploadLimit
	}

	if c.Reporting.BufferType == "" {
		c.Reporting.BufferType = sizeBufferType
	} else if c.Reporting.BufferType != eventBufferType && c.Reporting.BufferType != sizeBufferType {
		return fmt.Errorf("invalid buffer type %q, expected %q or %q", c.Reporting.BufferType, eventBufferType, sizeBufferType)
	}

	if c.Reporting.BufferType == eventBufferType && c.Reporting.BufferSizeLimitBytes != nil {
		return fmt.Errorf("invalid decision_log config, 'buffer_size_limit_bytes' isn't supported for the %v buffer type", eventBufferType)
	}
	if c.Reporting.BufferType == sizeBufferType && c.Reporting.BufferSizeLimitEvents != nil {
		return fmt.Errorf("invalid decision_log config, 'buffer_size_limit_events' isn't supported for the %v buffer type", sizeBufferType)
	}

	if c.Reporting.BufferSizeLimitBytes != nil && c.Reporting.MaxDecisionsPerSecond != nil {
		return errors.New("invalid decision_log config, specify either 'buffer_size_limit_bytes' or 'max_decisions_per_second'")
	}

	// default the buffer size limit
	sizeBufferLimit := defaultBufferSizeLimitBytes
	if c.Reporting.BufferSizeLimitBytes != nil {
		if *c.Reporting.BufferSizeLimitBytes <= int64(0) {
			return errors.New("invalid decision_log config, 'buffer_size_limit_bytes' must be higher than 0")
		}
		sizeBufferLimit = *c.Reporting.BufferSizeLimitBytes
	}
	c.Reporting.BufferSizeLimitBytes = &sizeBufferLimit

	eventBufferLimit := defaultBufferSizeLimitEvents
	if c.Reporting.BufferSizeLimitEvents != nil {
		if *c.Reporting.BufferSizeLimitEvents <= int64(0) {
			return errors.New("invalid decision_log config, 'buffer_size_limit_entries' must be higher than 0")
		}
		eventBufferLimit = *c.Reporting.BufferSizeLimitEvents
	}
	c.Reporting.BufferSizeLimitEvents = &eventBufferLimit

	if c.MaskDecision == nil {
		maskDecision := defaultMaskDecisionPath
		c.MaskDecision = &maskDecision
	}

	c.maskDecisionRef, err = ref.ParseDataPath(*c.MaskDecision)
	if err != nil {
		return fmt.Errorf("invalid mask_decision in decision_logs: %w", err)
	}

	if c.DropDecision == nil {
		dropDecision := defaultDropDecisionPath
		c.DropDecision = &dropDecision
	}

	c.dropDecisionRef, err = ref.ParseDataPath(*c.DropDecision)
	if err != nil {
		return fmt.Errorf("invalid drop_decision in decision_logs: %w", err)
	}

	if c.PartitionName != "" {
		resourcePath := fmt.Sprintf("/logs/%v", c.PartitionName)
		c.Resource = &resourcePath
	} else if c.Resource == nil {
		resourcePath := defaultResourcePath
		c.Resource = &resourcePath
	} else {
		if _, err := url.Parse(*c.Resource); err != nil {
			return fmt.Errorf("invalid resource path %q: %w", *c.Resource, err)
		}
	}

	return nil
}

// Plugin implements decision log buffering and uploading.
type Plugin struct {
	manager       *plugins.Manager
	config        Config
	runningBuffer string
	reconfigMtx   sync.RWMutex // reconfigMtx blocks reads/writes on buffer reconfiguration
	eventBuffer   *eventBuffer
	buffer        *logBuffer
	enc           *chunkEncoder
	mtx           sync.Mutex
	statusMtx     sync.Mutex
	stop          chan chan struct{}
	reconfig      chan reconfigure
	preparedMask  prepareOnce
	preparedDrop  prepareOnce
	limiter       *rate.Limiter
	metrics       metrics.Metrics
	logger        logging.Logger
	status        *lstat.Status
}

type prepareOnce struct {
	once          *sync.Once
	preparedQuery *rego.PreparedEvalQuery
	err           error
}

func newPrepareOnce() *prepareOnce {
	return &prepareOnce{
		once: new(sync.Once),
	}
}

func (po *prepareOnce) drop() {
	po.once = new(sync.Once)
}

func (po *prepareOnce) prepareOnce(f func() (*rego.PreparedEvalQuery, error)) (*rego.PreparedEvalQuery, error) {
	po.once.Do(func() {
		po.preparedQuery, po.err = f()
	})
	return po.preparedQuery, po.err
}

type reconfigure struct {
	config any
	done   chan struct{}
}

// ParseConfig validates the config and injects default values.
func ParseConfig(config []byte, services []string, pluginList []string) (*Config, error) {
	t := plugins.DefaultTriggerMode
	return NewConfigBuilder().
		WithBytes(config).
		WithServices(services).
		WithPlugins(pluginList).
		WithTriggerMode(&t).
		Parse()
}

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder struct {
	raw      []byte
	services []string
	plugins  []string
	trigger  *plugins.TriggerMode
	logger   logging.Logger
}

// NewConfigBuilder returns a new ConfigBuilder to build and parse the plugin config.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

func (b *ConfigBuilder) WithLogger(l logging.Logger) *ConfigBuilder {
	b.logger = l
	return b
}

// WithBytes sets the raw plugin config.
func (b *ConfigBuilder) WithBytes(config []byte) *ConfigBuilder {
	b.raw = config
	return b
}

// WithServices sets the services that implement control plane APIs.
func (b *ConfigBuilder) WithServices(services []string) *ConfigBuilder {
	b.services = services
	return b
}

// WithPlugins sets the list of named plugins for decision logging.
func (b *ConfigBuilder) WithPlugins(plugins []string) *ConfigBuilder {
	b.plugins = plugins
	return b
}

// WithTriggerMode sets the plugin trigger mode.
func (b *ConfigBuilder) WithTriggerMode(trigger *plugins.TriggerMode) *ConfigBuilder {
	b.trigger = trigger
	return b
}

// Parse validates the config and injects default values.
func (b *ConfigBuilder) Parse() (*Config, error) {
	if b.raw == nil {
		return nil, nil
	}

	var parsedConfig Config

	if err := util.Unmarshal(b.raw, &parsedConfig); err != nil {
		return nil, err
	}

	if parsedConfig.Plugin == nil && parsedConfig.Service == "" && len(b.services) == 0 && !parsedConfig.ConsoleLogs {
		// Nothing to validate or inject
		return nil, nil
	}

	if err := parsedConfig.validateAndInjectDefaults(b.services, b.plugins, b.trigger, b.logger); err != nil {
		return nil, err
	}

	return &parsedConfig, nil
}

// New returns a new Plugin with the given config.
func New(parsedConfig *Config, manager *plugins.Manager) *Plugin {

	plugin := &Plugin{
		manager:      manager,
		config:       *parsedConfig,
		stop:         make(chan chan struct{}),
		enc:          newChunkEncoder(*parsedConfig.Reporting.UploadSizeLimitBytes),
		reconfig:     make(chan reconfigure),
		logger:       manager.Logger().WithFields(map[string]any{"plugin": Name}),
		status:       &lstat.Status{},
		preparedDrop: *newPrepareOnce(),
		preparedMask: *newPrepareOnce(),
	}

	plugin.enc.WithLogger(plugin.logger)

	switch parsedConfig.Reporting.BufferType {
	case eventBufferType:
		plugin.eventBuffer = newEventBuffer(
			*parsedConfig.Reporting.BufferSizeLimitEvents,
			plugin.manager.Client(plugin.config.Service),
			*parsedConfig.Resource,
			*parsedConfig.Reporting.UploadSizeLimitBytes,
		).WithLogger(plugin.logger).WithMetrics(plugin.metrics)
		plugin.runningBuffer = eventBufferType
	case sizeBufferType:
		plugin.buffer = newLogBuffer(*parsedConfig.Reporting.BufferSizeLimitBytes)
		plugin.runningBuffer = sizeBufferType
	}

	if parsedConfig.Reporting.MaxDecisionsPerSecond != nil {
		limit := *parsedConfig.Reporting.MaxDecisionsPerSecond
		plugin.limiter = rate.NewLimiter(rate.Limit(limit), int(math.Max(1, limit)))
	}

	manager.RegisterCompilerTrigger(plugin.compilerUpdated)

	manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})

	return plugin
}

// WithMetrics sets the global metrics provider to be used by the plugin.
func (p *Plugin) WithMetrics(m metrics.Metrics) *Plugin {
	p.metrics = m
	p.enc.WithMetrics(m)
	return p
}

// Name identifies the plugin on manager.
const Name = "decision_logs"

// Lookup returns the decision logs plugin registered with the manager.
func Lookup(manager *plugins.Manager) *Plugin {
	if p := manager.Plugin(Name); p != nil {
		return p.(*Plugin)
	}
	return nil
}

// Start starts the plugin.
func (p *Plugin) Start(_ context.Context) error {
	p.logger.Info("Starting decision logger.")
	go p.loop()
	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	p.logger.Info("Stopping decision logger.")

	if *p.config.Reporting.Trigger == plugins.TriggerPeriodic {
		if _, ok := ctx.Deadline(); ok && p.config.Service != "" {
			p.flushDecisions(ctx)
		}
	}

	done := make(chan struct{})
	p.stop <- done
	<-done
	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
}

// Config returns the plugin's current configuration
func (p *Plugin) Config() *Config {
	return &p.config
}

func (p *Plugin) flushDecisions(ctx context.Context) {
	p.logger.Info("Flushing decision logs.")

	done := make(chan bool)

	go func(ctx context.Context, done chan bool) {
		for ctx.Err() == nil {
			if err := p.oneShot(ctx); err != nil && !errors.Is(err, &bufferEmpty{}) {
				p.logger.Error("Error flushing decisions: %s", err)
				// Wait some before retrying, but skip incrementing interval since we are shutting down
				time.Sleep(1 * time.Second)
			} else {
				done <- true
				return
			}
		}
	}(ctx, done)

	select {
	case <-done:
		p.logger.Info("All decisions in buffer uploaded.")
	case <-ctx.Done():
		switch ctx.Err() {
		case context.DeadlineExceeded, context.Canceled:
			p.logger.Error("Plugin stopped with decisions possibly still in buffer.")
		}
	}
}

// Log appends a decision log event to the buffer for uploading.
func (p *Plugin) Log(ctx context.Context, decision *server.Info) error {
	bundles := map[string]BundleInfoV1{}
	for name, info := range decision.Bundles {
		bundles[name] = BundleInfoV1{Revision: info.Revision}
	}

	event := EventV1{
		Labels:         p.manager.Labels(),
		DecisionID:     decision.DecisionID,
		TraceID:        decision.TraceID,
		SpanID:         decision.SpanID,
		Revision:       decision.Revision,
		Bundles:        bundles,
		Path:           decision.Path,
		Query:          decision.Query,
		Input:          decision.Input,
		Result:         decision.Results,
		MappedResult:   decision.MappedResults,
		NDBuiltinCache: decision.NDBuiltinCache,
		RequestedBy:    decision.RemoteAddr,
		Timestamp:      decision.Timestamp,
		RequestID:      decision.RequestID,
		inputAST:       decision.InputAST,
	}

	headers := map[string][]string{}
	rctx := p.config.RequestContext

	if rctx.HTTPRequest != nil && len(rctx.HTTPRequest.Headers) > 0 && decision.HTTPRequestContext.Header != nil {
		for _, h := range rctx.HTTPRequest.Headers {
			values := decision.HTTPRequestContext.Header.Values(h)
			if len(values) > 0 {
				headers[h] = decision.HTTPRequestContext.Header.Values(h)
			}
		}
	}

	if len(headers) > 0 {
		event.RequestContext = &RequestContext{HTTPRequest: &HTTPRequestContext{Headers: headers}}
	}

	input, err := event.AST()
	if err != nil {
		return err
	}

	drop, err := p.dropEvent(ctx, decision.Txn, input)
	if err != nil {
		p.logger.Error("Log drop decision failed: %v.", err)
		return nil
	}

	if drop {
		p.logger.Debug("Decision log event to path %v dropped", event.Path)
		return nil
	}

	if decision.Metrics != nil {
		event.Metrics = decision.Metrics.All()
	}

	if decision.Error != nil {
		event.Error = decision.Error
	}

	if err := p.maskEvent(ctx, decision.Txn, input, &event); err != nil {
		// TODO(tsandall): see note below about error handling.
		p.logger.Error("Log event masking failed: %v.", err)
		return nil
	}

	if p.config.ConsoleLogs {
		if err := p.logEvent(event); err != nil {
			p.logger.Error("Failed to log to console: %v.", err)
		}
	}

	if p.config.Service != "" {
		p.encodeAndBufferEvent(event)
	}

	if p.config.Plugin != nil {
		proxy, ok := p.manager.Plugin(*p.config.Plugin).(Logger)
		if !ok {
			return errors.New("plugin does not implement Logger interface")
		}
		return proxy.Log(ctx, event)
	}

	return nil
}

// Reconfigure notifies the plugin with a new configuration.
func (p *Plugin) Reconfigure(_ context.Context, config any) {

	done := make(chan struct{})
	p.reconfig <- reconfigure{config: config, done: done}

	p.preparedMask.drop()
	p.preparedDrop.drop()

	<-done
}

// Trigger can be used to control when the plugin attempts to upload
// a new decision log in manual triggering mode.
func (p *Plugin) Trigger(ctx context.Context) error {
	done := make(chan error)

	go func() {
		if p.config.Service != "" {
			err := p.doOneShot(ctx)
			if err != nil {
				if ctx.Err() == nil {
					done <- err
				}
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

// compilerUpdated is called when a compiler trigger on the plugin manager
// fires. This indicates a new compiler instance is available. The decision
// logger needs to prepare a new masking query.
func (p *Plugin) compilerUpdated(storage.Transaction) {
	p.preparedMask.drop()
	p.preparedDrop.drop()
}

func (p *Plugin) loop() {
	ctx, cancel := context.WithCancel(context.Background())

	var retry int

	for {
		var waitC chan struct{}

		if *p.config.Reporting.Trigger == plugins.TriggerPeriodic && p.config.Service != "" {
			err := p.doOneShot(ctx)

			var delay time.Duration

			if err == nil {
				min := float64(*p.config.Reporting.MinDelaySeconds)
				max := float64(*p.config.Reporting.MaxDelaySeconds)
				delay = time.Duration(((max - min) * rand.Float64()) + min)
			} else {
				delay = util.DefaultBackoff(float64(minRetryDelay), float64(*p.config.Reporting.MaxDelaySeconds), retry)
			}

			p.logger.Debug("Waiting %v before next upload/retry.", delay)

			waitC = make(chan struct{})
			go func() {
				timer, timerCancel := util.TimerWithCancel(delay)
				select {
				case <-timer.C:
					if err != nil {
						retry++
					} else {
						retry = 0
					}
					close(waitC)
				case <-ctx.Done():
					timerCancel() // explicitly cancel the timer.
				}
			}()
		}

		select {
		case <-waitC:
		case update := <-p.reconfig:
			p.reconfigure(ctx, update.config)
			update.done <- struct{}{}
		case done := <-p.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

type bufferEmpty struct{}

func (*bufferEmpty) Error() string {
	return "buffer is empty"
}

func (p *Plugin) doOneShot(ctx context.Context) error {
	err := p.oneShot(ctx)

	if err != nil {
		if errors.Is(err, &bufferEmpty{}) {
			p.logger.Debug("Log upload queue was empty.")
			err = nil
		} else {
			p.logger.Error("%v.", err)
		}
	} else {
		p.logger.Info("Logs uploaded successfully.")
	}

	p.setStatus(err)
	return err
}

func (p *Plugin) oneShot(ctx context.Context) error {
	if p.runningBuffer == eventBufferType {
		return p.eventBuffer.Upload(ctx)
	}

	// Make a local copy of the plugin's encoder and buffer and create
	// a new encoder and buffer. This is needed as locking the buffer for
	// the upload duration will block policy evaluation and result in
	// increased latency for OPA clients
	p.mtx.Lock()
	oldChunkEnc := p.enc
	oldBuffer := p.buffer
	p.buffer = newLogBuffer(*p.config.Reporting.BufferSizeLimitBytes)
	p.enc = newChunkEncoder(*p.config.Reporting.UploadSizeLimitBytes).WithMetrics(p.metrics).WithLogger(p.logger).
		WithUncompressedLimit(oldChunkEnc.uncompressedLimit, oldChunkEnc.uncompressedLimitScaleDownExponent, oldChunkEnc.uncompressedLimitScaleUpExponent)
	p.mtx.Unlock()

	// Along with uploading the compressed events in the buffer
	// to the remote server, flush any pending compressed data to the
	// underlying writer and add to the buffer.
	chunk, err := oldChunkEnc.Flush()
	if err != nil {
		return err
	}

	for _, ch := range chunk {
		p.bufferChunk(oldBuffer, ch)
	}

	if oldBuffer.Len() == 0 {
		return &bufferEmpty{}
	}

	for bs := oldBuffer.Pop(); bs != nil; bs = oldBuffer.Pop() {
		if err == nil {
			err = uploadChunk(ctx, p.manager.Client(p.config.Service), *p.config.Resource, bs)
		}
		if err != nil {
			if p.limiter != nil {
				events, decErr := newChunkDecoder(bs).decode()
				if decErr != nil {
					continue
				}

				for i := range events {
					p.encodeAndBufferEvent(events[i])
				}
			} else {
				// requeue the chunk
				p.mtx.Lock()
				p.bufferChunk(p.buffer, bs)
				p.mtx.Unlock()
			}
		}
	}

	return err
}

func (p *Plugin) reconfigure(ctx context.Context, config any) {
	newConfig := config.(*Config)

	if reflect.DeepEqual(p.config, *newConfig) {
		p.logger.Debug("Decision log uploader configuration unchanged.")
		return
	}

	p.logger.Info("Decision log uploader configuration changed.")
	p.config = *newConfig

	p.reconfigMtx.Lock()
	defer p.reconfigMtx.Unlock()

	switch newConfig.Reporting.BufferType {
	case eventBufferType:
		if p.eventBuffer == nil {
			p.eventBuffer = newEventBuffer(
				*p.config.Reporting.BufferSizeLimitEvents,
				p.manager.Client(p.config.Service),
				*p.config.Resource,
				*p.config.Reporting.UploadSizeLimitBytes).WithLogger(p.logger).WithMetrics(p.metrics)
		} else {
			p.eventBuffer.Reconfigure(
				*p.config.Reporting.BufferSizeLimitEvents,
				p.manager.Client(p.config.Service),
				*p.config.Resource,
				*p.config.Reporting.UploadSizeLimitBytes)
		}

		if p.runningBuffer == sizeBufferType {
			if err := p.oneShot(ctx); err != nil && !errors.Is(err, &bufferEmpty{}) {
				p.setStatus(err)
			}
		}

		p.runningBuffer = eventBufferType
	case sizeBufferType:
		if p.runningBuffer == eventBufferType {
			if err := p.eventBuffer.Upload(ctx); err != nil && !errors.Is(err, &bufferEmpty{}) {
				p.setStatus(err)
			}
		}

		if p.buffer == nil {
			p.buffer = newLogBuffer(*p.config.Reporting.BufferSizeLimitBytes)
		}

		p.runningBuffer = sizeBufferType
	}
}

// NOTE(philipc): Because ND builtins caching can cause unbounded growth in
// decision log entry size, we do best-effort event encoding here, and when we
// run out of space, we drop the ND builtins cache, and try encoding again.
func (p *Plugin) encodeAndBufferEvent(event EventV1) {
	if p.limiter != nil && !p.limiter.Allow() {
		p.incrMetric(logRateLimitExDropCounterName)
		p.logger.Error("Decision log dropped as rate limit exceeded. Reduce reporting interval or increase rate limit.")
		return
	}

	// only blocks when the buffer is being reconfigured
	p.reconfigMtx.RLock()
	defer p.reconfigMtx.RUnlock()

	if p.runningBuffer == eventBufferType {
		p.eventBuffer.Push(&event)
		return
	}

	eventBytes, err := json.Marshal(&event)
	if err != nil {
		p.logger.Error("Decision log dropped due to error serializing event to JSON: %v", err)
		return
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()
	result, err := p.enc.Encode(event, eventBytes)
	if err != nil {
		return
	}
	for _, chunk := range result {
		p.bufferChunk(p.buffer, chunk)
	}
}

func (p *Plugin) bufferChunk(buffer *logBuffer, bs []byte) {
	dropped := buffer.Push(bs)
	if dropped > 0 {
		p.incrMetric(logBufferEventDropCounterName)
		p.incrMetric(logBufferSizeLimitExDropCounterName)
		p.logger.Error("Dropped %v chunks from buffer. Reduce reporting interval or increase buffer size.", dropped)
	}
}

func (p *Plugin) maskEvent(ctx context.Context, txn storage.Transaction, input ast.Value, event *EventV1) error {
	pq, err := p.preparedMask.prepareOnce(func() (*rego.PreparedEvalQuery, error) {
		var pq rego.PreparedEvalQuery

		query := ast.NewBody(ast.NewExpr(ast.NewTerm(p.config.maskDecisionRef)))

		r := rego.New(
			rego.ParsedQuery(query),
			rego.Compiler(p.manager.GetCompiler()),
			rego.Store(p.manager.Store),
			rego.Transaction(txn),
			rego.Runtime(p.manager.Info),
			rego.EnablePrintStatements(p.manager.EnablePrintStatements()),
			rego.PrintHook(p.manager.PrintHook()),
		)

		pq, err := r.PrepareForEval(context.Background())
		if err != nil {
			return nil, err
		}
		return &pq, nil
	})

	if err != nil {
		return err
	}

	rs, err := pq.Eval(
		ctx,
		rego.EvalParsedInput(input),
		rego.EvalTransaction(txn),
	)

	if err != nil {
		return err
	} else if len(rs) == 0 {
		return nil
	}

	mRuleSet, err := newMaskRuleSet(
		rs[0].Expressions[0].Value,
		func(mRule *maskRule, err error) {
			p.logger.Error("mask rule skipped: %s: %s", mRule.String(), err.Error())
		},
	)
	if err != nil {
		return err
	}

	mRuleSet.Mask(event)

	return nil
}

func (p *Plugin) dropEvent(ctx context.Context, txn storage.Transaction, input ast.Value) (bool, error) {
	var err error

	pq, err := p.preparedDrop.prepareOnce(func() (*rego.PreparedEvalQuery, error) {
		var pq rego.PreparedEvalQuery

		query := ast.NewBody(ast.NewExpr(ast.NewTerm(p.config.dropDecisionRef)))
		r := rego.New(
			rego.ParsedQuery(query),
			rego.Compiler(p.manager.GetCompiler()),
			rego.Store(p.manager.Store),
			rego.Transaction(txn),
			rego.Runtime(p.manager.Info),
			rego.EnablePrintStatements(p.manager.EnablePrintStatements()),
			rego.PrintHook(p.manager.PrintHook()),
		)

		pq, err := r.PrepareForEval(context.Background())
		if err != nil {
			return nil, err
		}
		return &pq, nil
	})

	if err != nil {
		return false, err
	}

	rs, err := pq.Eval(
		ctx,
		rego.EvalParsedInput(input),
		rego.EvalTransaction(txn),
	)

	if err != nil {
		return false, err
	}

	return rs.Allowed(), nil
}

func uploadChunk(ctx context.Context, client rest.Client, uploadPath string, data []byte) error {

	resp, err := client.
		WithHeader("Content-Type", "application/json").
		WithHeader("Content-Encoding", "gzip").
		WithBytes(data).
		Do(ctx, "POST", uploadPath)

	if err != nil {
		return fmt.Errorf("log upload failed: %w", err)
	}

	defer util.Close(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return lstat.HTTPError{StatusCode: resp.StatusCode}
	}

	return nil
}

func (p *Plugin) logEvent(event EventV1) error {
	eventBuf, err := json.Marshal(&event)
	if err != nil {
		return err
	}
	fields := map[string]any{}
	err = util.UnmarshalJSON(eventBuf, &fields)
	if err != nil {
		return err
	}
	p.manager.ConsoleLogger().WithFields(fields).WithFields(map[string]any{
		"type": "openpolicyagent.org/decision_logs",
	}).Info("Decision Log")
	return nil
}

func (p *Plugin) incrMetric(name string) {
	if p.metrics != nil {
		p.metrics.Counter(name).Incr()
	}
}

func (p *Plugin) setStatus(err error) {
	p.statusMtx.Lock()
	p.status.SetError(err)
	oldStatus := p.status
	p.statusMtx.Unlock()

	if s := status.Lookup(p.manager); s != nil {
		s.UpdateDecisionLogsStatus(*oldStatus)
	}
}

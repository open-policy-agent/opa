// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package metrics contains helpers for performance metric management inside the policy engine.
package metrics

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	go_metrics "github.com/rcrowley/go-metrics"
)

type stringBuilderPool struct{ pool sync.Pool }

func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

func (p *stringBuilderPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

var sbPool = &stringBuilderPool{
	pool: sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	},
}

// Fast integer to string conversion helpers using strconv for better performance
func writeInt64(buf *strings.Builder, v int64) {
	// Use a small stack buffer for common case
	var b [20]byte // enough for 64-bit int
	s := b[:]
	if v < 0 {
		buf.WriteByte('-')
		v = -v
	}
	// Convert to string in reverse
	i := len(s)
	for v >= 10 {
		i--
		s[i] = byte('0' + v%10)
		v /= 10
	}
	i--
	s[i] = byte('0' + v)
	buf.Write(s[i:])
}

func writeUint64(buf *strings.Builder, v uint64) {
	// Use a small stack buffer
	var b [20]byte // enough for 64-bit uint
	s := b[:]
	// Convert to string in reverse
	i := len(s)
	for v >= 10 {
		i--
		s[i] = byte('0' + v%10)
		v /= 10
	}
	i--
	s[i] = byte('0' + v)
	buf.Write(s[i:])
}

// Well-known metric names.
const (
	BundleRequest                     = "bundle_request"
	ServerHandler                     = "server_handler"
	ServerQueryCacheHit               = "server_query_cache_hit"
	SDKDecisionEval                   = "sdk_decision_eval"
	RegoQueryCompile                  = "rego_query_compile"
	RegoQueryEval                     = "rego_query_eval"
	RegoQueryParse                    = "rego_query_parse"
	RegoModuleParse                   = "rego_module_parse"
	RegoDataParse                     = "rego_data_parse"
	RegoModuleCompile                 = "rego_module_compile"
	RegoPartialEval                   = "rego_partial_eval"
	RegoInputParse                    = "rego_input_parse"
	RegoLoadFiles                     = "rego_load_files"
	RegoLoadBundles                   = "rego_load_bundles"
	RegoExternalResolve               = "rego_external_resolve"
	CompilePrepPartial                = "compile_prep_partial"
	CompileEvalConstraints            = "compile_eval_constraints"
	CompileTranslateQueries           = "compile_translate_queries"
	CompileExtractAnnotationsUnknowns = "compile_extract_annotations_unknowns"
	CompileExtractAnnotationsMask     = "compile_extract_annotations_mask"
	CompileEvalMaskRule               = "compile_eval_mask_rule"
)

// Shared percentiles slice for all histograms to avoid repeated allocations
var sharedPercentiles = []float64{
	0.5,
	0.75,
	0.9,
	0.95,
	0.99,
	0.999,
	0.9999,
}

// Histogram field names - interned to avoid repeated string allocations
const (
	histogramCount  = "count"
	histogramMin    = "min"
	histogramMax    = "max"
	histogramMean   = "mean"
	histogramStddev = "stddev"
	histogramMedian = "median"
	histogram75     = "75%"
	histogram90     = "90%"
	histogram95     = "95%"
	histogram99     = "99%"
	histogram999    = "99.9%"
	histogram9999   = "99.99%"
)

// Info contains attributes describing the underlying metrics provider.
type Info struct {
	Name string `json:"name"` // name is a unique human-readable identifier for the provider.
}

// Metrics defines the interface for a collection of performance metrics in the
// policy engine.
type Metrics interface {
	Info() Info
	Timer(name string) Timer
	Histogram(name string) Histogram
	Counter(name string) Counter
	All() map[string]any
	Clear()
	json.Marshaler
}

type TimerMetrics interface {
	Timers() map[string]any
}

// metricsState holds the actual metrics maps and is replaced atomically
type metricsState struct {
	// Direct string-keyed maps for fast access path
	timers     map[string]Timer
	histograms map[string]Histogram
	counters   map[string]Counter
	// Pre-formatted keys cache to avoid string allocations during marshaling
	timerKeys     map[string]string
	histogramKeys map[string]string
	counterKeys   map[string]string
}

type metrics struct {
	state atomic.Pointer[metricsState]
	mtx   sync.Mutex // Only for writes to ensure consistency
}

// New returns a new Metrics object.
func New() Metrics {
	m := &metrics{}
	initialState := &metricsState{
		timers:        make(map[string]Timer),
		histograms:    make(map[string]Histogram),
		counters:      make(map[string]Counter),
		timerKeys:     make(map[string]string),
		histogramKeys: make(map[string]string),
		counterKeys:   make(map[string]string),
	}
	m.state.Store(initialState)
	return m
}

// NoOp returns a Metrics implementation that does nothing and costs nothing.
// Used when metrics are expected, but not of interest.
func NoOp() Metrics {
	return noOpMetricsInstance
}

type metric struct {
	Key   string
	Value any
}

func (*metrics) Info() Info {
	return Info{
		Name: "<built-in>",
	}
}

func (m *metrics) String() string {
	all := m.All()
	sorted := make([]metric, 0, len(all))

	for key, value := range all {
		sorted = append(sorted, metric{
			Key:   key,
			Value: value,
		})
	}

	slices.SortFunc(sorted, func(a, b metric) int {
		return strings.Compare(a.Key, b.Key)
	})

	buf := sbPool.Get()
	defer sbPool.Put(buf)

	totalLen := 0
	for i := range sorted {
		totalLen += len(sorted[i].Key) + 20 // estimate for value and separators
	}
	buf.Grow(totalLen)

	for i := range sorted {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(sorted[i].Key)
		buf.WriteByte(':')
		// Avoid fmt.Fprintf overhead by type-switching on common types
		switch v := sorted[i].Value.(type) {
		case int64:
			writeInt64(buf, v)
		case uint64:
			writeUint64(buf, v)
		default:
			fmt.Fprintf(buf, "%v", v)
		}
	}

	return buf.String()
}

func (m *metrics) MarshalJSON() ([]byte, error) {
	state := m.state.Load()

	// Estimate buffer size
	estimatedSize := len(state.timers)*50 + len(state.histograms)*200 + len(state.counters)*30 + 100
	buf := sbPool.Get()
	defer sbPool.Put(buf)
	buf.Grow(estimatedSize)

	buf.WriteByte('{')
	first := true

	// Write timers
	for name, timer := range state.timers {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteByte('"')
		buf.WriteString(state.timerKeys[name])
		buf.WriteString("\":")
		writeInt64(buf, timer.Int64())
	}

	// Write counters
	for name, cntr := range state.counters {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteByte('"')
		buf.WriteString(state.counterKeys[name])
		buf.WriteString("\":")
		writeUint64(buf, cntr.Value().(uint64))
	}

	// Write histograms (these need standard JSON marshaling for the nested map)
	for name, hist := range state.histograms {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteByte('"')
		buf.WriteString(state.histogramKeys[name])
		buf.WriteString("\":")
		// Use standard JSON for histogram values since they're complex
		histJSON, err := json.Marshal(hist.Value())
		if err != nil {
			return nil, err
		}
		buf.Write(histJSON)
	}

	buf.WriteByte('}')

	// Return a copy since buf will be reused
	result := make([]byte, buf.Len())
	copy(result, buf.String())
	return result, nil
}

func (m *metrics) Timer(name string) Timer {
	// Fast path: single map lookup
	state := m.state.Load()
	if t, ok := state.timers[name]; ok {
		return t
	}

	// Slow path: need to add new timer
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Double-check after acquiring lock
	state = m.state.Load()
	if t, ok := state.timers[name]; ok {
		return t
	}

	// Create new state with added timer
	newState := &metricsState{
		timers:        make(map[string]Timer, len(state.timers)+1),
		histograms:    state.histograms,
		counters:      state.counters,
		timerKeys:     make(map[string]string, len(state.timerKeys)+1),
		histogramKeys: state.histogramKeys,
		counterKeys:   state.counterKeys,
	}
	maps.Copy(newState.timers, state.timers)
	maps.Copy(newState.timerKeys, state.timerKeys)

	// Pre-format and cache the key
	formattedKey := formatTimerKey(name)
	newState.timerKeys[name] = formattedKey

	t := &timer{}
	newState.timers[name] = t
	m.state.Store(newState)

	return t
}

func (m *metrics) Histogram(name string) Histogram {
	// Fast path: single map lookup
	state := m.state.Load()
	if h, ok := state.histograms[name]; ok {
		return h
	}

	// Slow path: need to add new histogram
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Double-check after acquiring lock
	state = m.state.Load()
	if h, ok := state.histograms[name]; ok {
		return h
	}

	// Create new state with added histogram
	newState := &metricsState{
		timers:        state.timers,
		histograms:    make(map[string]Histogram, len(state.histograms)+1),
		counters:      state.counters,
		timerKeys:     state.timerKeys,
		histogramKeys: make(map[string]string, len(state.histogramKeys)+1),
		counterKeys:   state.counterKeys,
	}
	maps.Copy(newState.histograms, state.histograms)
	maps.Copy(newState.histogramKeys, state.histogramKeys)

	// Pre-format and cache the key
	formattedKey := formatHistogramKey(name)
	newState.histogramKeys[name] = formattedKey

	h := newHistogram()
	newState.histograms[name] = h
	m.state.Store(newState)

	return h
}

func (m *metrics) Counter(name string) Counter {
	// Fast path: single map lookup
	state := m.state.Load()
	if c, ok := state.counters[name]; ok {
		return c
	}

	// Slow path: need to add new counter
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Double-check after acquiring lock
	state = m.state.Load()
	if c, ok := state.counters[name]; ok {
		return c
	}

	// Create new state with added counter
	newState := &metricsState{
		timers:        state.timers,
		histograms:    state.histograms,
		counters:      make(map[string]Counter, len(state.counters)+1),
		timerKeys:     state.timerKeys,
		histogramKeys: state.histogramKeys,
		counterKeys:   make(map[string]string, len(state.counterKeys)+1),
	}
	maps.Copy(newState.counters, state.counters)
	maps.Copy(newState.counterKeys, state.counterKeys)

	// Pre-format and cache the key
	formattedKey := formatCounterKey(name)
	newState.counterKeys[name] = formattedKey

	c := &counter{}
	newState.counters[name] = c
	m.state.Store(newState)

	return c
}

func (m *metrics) All() map[string]any {
	state := m.state.Load()
	result := make(map[string]any, len(state.timers)+len(state.histograms)+len(state.counters))

	for name, timer := range state.timers {
		key := state.timerKeys[name]
		result[key] = timer.Value()
	}
	for name, hist := range state.histograms {
		key := state.histogramKeys[name]
		result[key] = hist.Value()
	}
	for name, cntr := range state.counters {
		key := state.counterKeys[name]
		result[key] = cntr.Value()
	}

	return result
}

func (m *metrics) Timers() map[string]any {
	state := m.state.Load()
	ts := make(map[string]any, len(state.timers))

	for name, t := range state.timers {
		key := state.timerKeys[name]
		ts[key] = t.Value()
	}

	return ts
}

func (m *metrics) Clear() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	newState := &metricsState{
		timers:        make(map[string]Timer),
		histograms:    make(map[string]Histogram),
		counters:      make(map[string]Counter),
		timerKeys:     make(map[string]string),
		histogramKeys: make(map[string]string),
		counterKeys:   make(map[string]string),
	}
	m.state.Store(newState)
}

// Optimized key formatters that avoid allocations by using strings.Builder
func formatTimerKey(name string) string {
	// timer_ + name + _ns
	buf := sbPool.Get()
	defer sbPool.Put(buf)
	buf.Grow(len(name) + 9) // "timer_" (6) + "_ns" (3)
	buf.WriteString("timer_")
	buf.WriteString(name)
	buf.WriteString("_ns")
	return buf.String()
}

func formatHistogramKey(name string) string {
	// histogram_ + name
	buf := sbPool.Get()
	defer sbPool.Put(buf)
	buf.Grow(len(name) + 10) // "histogram_" (10)
	buf.WriteString("histogram_")
	buf.WriteString(name)
	return buf.String()
}

func formatCounterKey(name string) string {
	// counter_ + name
	buf := sbPool.Get()
	defer sbPool.Put(buf)
	buf.Grow(len(name) + 8) // "counter_" (8)
	buf.WriteString("counter_")
	buf.WriteString(name)
	return buf.String()
}

// Timer defines the interface for a restartable timer that accumulates elapsed
// time.
type Timer interface {
	Value() any
	Int64() int64
	// Start or resume a timer's time tracking.
	Start()
	// Stop a timer, and accumulate the delta (in nanoseconds) since it was last
	// started.
	Stop() int64
}

type timer struct {
	start atomic.Int64 // nanoseconds since epoch, 0 means not started
	value atomic.Int64 // accumulated nanoseconds
}

func (t *timer) Start() {
	t.start.Store(time.Now().UnixNano())
}

func (t *timer) Stop() int64 {
	startNs := t.start.Swap(0) // Reset to 0 atomically
	if startNs == 0 {
		return 0
	}

	delta := max(time.Now().UnixNano()-startNs,
		// Clock skew protection
		0)

	t.value.Add(delta)
	return delta
}

func (t *timer) Value() any {
	return t.Int64()
}

func (t *timer) Int64() int64 {
	return t.value.Load()
}

// Histogram defines the interface for a histogram with hardcoded percentiles.
type Histogram interface {
	Value() any
	Update(int64)
}

type histogram struct {
	hist go_metrics.Histogram // is thread-safe because of the underlying ExpDecaySample
}

func newHistogram() Histogram {
	// NOTE(tsandall): the reservoir size and alpha factor are taken from
	// https://github.com/rcrowley/go-metrics. They may need to be tweaked in
	// the future.
	sample := go_metrics.NewExpDecaySample(1028, 0.015)
	hist := go_metrics.NewHistogram(sample)
	return &histogram{
		hist: hist,
	}
}

func (h *histogram) Update(v int64) {
	h.hist.Update(v)
}

func (h *histogram) Value() any {
	snap := h.hist.Snapshot()
	// Use shared percentiles slice to avoid allocation
	percentiles := snap.Percentiles(sharedPercentiles)

	// Preallocate map with exact size and use interned string keys
	values := make(map[string]any, 12)
	values[histogramCount] = snap.Count()
	values[histogramMin] = snap.Min()
	values[histogramMax] = snap.Max()
	values[histogramMean] = snap.Mean()
	values[histogramStddev] = snap.StdDev()
	values[histogramMedian] = percentiles[0]
	values[histogram75] = percentiles[1]
	values[histogram90] = percentiles[2]
	values[histogram95] = percentiles[3]
	values[histogram99] = percentiles[4]
	values[histogram999] = percentiles[5]
	values[histogram9999] = percentiles[6]

	return values
}

// Counter defines the interface for a monotonic increasing counter.
type Counter interface {
	Value() any
	Incr()
	Add(n uint64)
}

type counter struct {
	c uint64
}

func (c *counter) Incr() {
	atomic.AddUint64(&c.c, 1)
}

func (c *counter) Add(n uint64) {
	atomic.AddUint64(&c.c, n)
}

func (c *counter) Value() any {
	return atomic.LoadUint64(&c.c)
}

func Statistics(num ...int64) any {
	t := newHistogram()
	for _, n := range num {
		t.Update(n)
	}
	return t.Value()
}

type noOpMetrics struct{}
type noOpTimer struct{}
type noOpHistogram struct{}
type noOpCounter struct{}

var (
	noOpMetricsInstance   = &noOpMetrics{}
	noOpTimerInstance     = &noOpTimer{}
	noOpHistogramInstance = &noOpHistogram{}
	noOpCounterInstance   = &noOpCounter{}
)

func (*noOpMetrics) Info() Info                      { return Info{Name: "<built-in no-op>"} }
func (*noOpMetrics) Timer(name string) Timer         { return noOpTimerInstance }
func (*noOpMetrics) Histogram(name string) Histogram { return noOpHistogramInstance }
func (*noOpMetrics) Counter(name string) Counter     { return noOpCounterInstance }
func (*noOpMetrics) All() map[string]any             { return nil }
func (*noOpMetrics) Clear()                          {}
func (*noOpMetrics) MarshalJSON() ([]byte, error) {
	return []byte(`{"name": "<built-in no-op>"}`), nil
}

func (*noOpTimer) Start()       {}
func (*noOpTimer) Stop() int64  { return 0 }
func (*noOpTimer) Value() any   { return 0 }
func (*noOpTimer) Int64() int64 { return 0 }

func (*noOpHistogram) Update(v int64) {}
func (*noOpHistogram) Value() any     { return nil }

func (*noOpCounter) Incr()        {}
func (*noOpCounter) Add(_ uint64) {}
func (*noOpCounter) Value() any   { return 0 }
func (*noOpCounter) Int64() int64 { return 0 }

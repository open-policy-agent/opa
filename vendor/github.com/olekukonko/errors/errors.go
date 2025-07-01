package errors

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	ctxTimeout = "[error] timeout" // Context key for marking timeout errors
	ctxRetry   = "[error] retry"   // Context key for marking retryable errors

	contextSize = 4   // Default size of fixed-size context array
	bufferSize  = 256 // Initial buffer size for JSON marshaling
	warmUpSize  = 100 // Number of errors to pre-warm the pool
	stackDepth  = 32  // Default maximum stack trace depth
)

type ErrorCategory string

// ErrorOpts provides options for customizing error creation.
type ErrorOpts struct {
	SkipStack int // Number of stack frames to skip when capturing the stack trace
}

// Config defines the configuration for the errors package.
type Config struct {
	StackDepth     int  // Maximum depth of the stack trace; 0 uses default
	ContextSize    int  // Initial size of the context map; 0 uses default
	DisablePooling bool // Disables object pooling for errors if true
	FilterInternal bool // Filters internal package frames from stack traces if true
	AutoFree       bool // Automatically frees errors to pool if true
}

// cachedConfig holds the current configuration, updated only on Configure().
type cachedConfig struct {
	stackDepth     int
	contextSize    int
	disablePooling bool
	filterInternal bool
	autoFree       bool
}

var (
	currentConfig cachedConfig
	configMu      sync.RWMutex
	errorPool     = NewErrorPool() // Custom pool for Error instances
	stackPool     = sync.Pool{     // Pool for stack trace slices
		New: func() interface{} {
			return make([]uintptr, currentConfig.stackDepth)
		},
	}
	emptyError = &Error{
		smallContext: [contextSize]contextItem{},
		msg:          "",
		name:         "",
		template:     "",
		cause:        nil,
	}
)

//var bufferPool = sync.Pool{
//	New: func() interface{} {
//		return bytes.NewBuffer(make([]byte, 0, bufferSize))
//	},
//}

// contextItem represents a single key-value pair in the smallContext array.
type contextItem struct {
	key   string
	value interface{}
}

// Error represents a custom error with enhanced features like context, stack traces, and wrapping.
type Error struct {
	// Primary error information (most frequently accessed)
	msg   string    // Error message
	name  string    // Error name/type
	stack []uintptr // Stack trace

	// Secondary error metadata
	template   string // Message template used if msg is empty
	category   string // Error category (e.g., "network", "validation")
	count      uint64 // Occurrence count for tracking frequency
	code       int32  // HTTP-like error code
	smallCount int32  // Number of items in smallContext

	// Context and chaining
	context      map[string]interface{}   // Additional context as key-value pairs
	cause        error                    // Wrapped underlying error
	callback     func()                   // Optional callback executed on Error()
	smallContext [contextSize]contextItem // Fixed-size context storage for efficiency

	// Synchronization
	mu sync.RWMutex // Protects concurrent access to mutable fields
}

// init initializes the package with default configuration and pre-warms the error pool.
func init() {
	currentConfig = cachedConfig{
		stackDepth:     stackDepth,
		contextSize:    contextSize,
		disablePooling: false,
		filterInternal: true,
		autoFree:       true,
	}
	WarmPool(warmUpSize) // Pre-warm pool with initial errors
}

// Configure updates the global configuration for the errors package.
// Thread-safe; should be called before heavy usage for optimal performance.
// Changes apply immediately to all subsequent error operations.
func Configure(cfg Config) {
	configMu.Lock()
	defer configMu.Unlock()

	if cfg.StackDepth != 0 {
		currentConfig.stackDepth = cfg.StackDepth
	}
	if cfg.ContextSize != 0 {
		currentConfig.contextSize = cfg.ContextSize
	}
	currentConfig.disablePooling = cfg.DisablePooling
	currentConfig.filterInternal = cfg.FilterInternal
	currentConfig.autoFree = cfg.AutoFree
}

// newError creates a new Error instance, using the pool if enabled.
// Initializes smallContext and stack appropriately.
func newError() *Error {
	if currentConfig.disablePooling {
		return &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
	}
	return errorPool.Get()
}

// Empty creates a new empty error with no stack trace.
// Useful as a base for building errors incrementally.
func Empty() *Error {
	return emptyError
}

// Named creates a new error with a specific name and stack trace.
// The name is used as the error message if no other message is set.
func Named(name string) *Error {
	e := newError()
	e.name = name
	return e.WithStack()
}

// New creates a fast, lightweight error without stack tracing.
// Use instead of Trace() when stack traces aren't needed for better performance.
func New(text string) *Error {
	if text == "" {
		return emptyError.Copy() // Global pre-allocated empty error
	}
	err := newError()
	err.msg = text
	return err
}

// Newf is an alias to Errorf for fmt.Errorf compatibility.
// Creates a formatted error without stack traces.
func Newf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// Std creates a standard error using errors.New, provided for backward compatibility.
// This function serves as a lightweight wrapper around the standard library's error creation,
// allowing users to opt into basic error handling without adopting the full features of this package.
func Std(text string) error {
	return errors.New(text)
}

// Stdf creates a formatted standard error using fmt.Errorf, provided for backward compatibility.
// This function wraps the standard library's formatted error creation, offering a simple alternative
// to the package's enhanced error handling while maintaining compatibility with existing codebases.
func Stdf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// Trace creates an error with stack trace capture enabled.
// Use when call stacks are needed for debugging; has performance overhead.
func Trace(text string) *Error {
	e := New(text)
	return e.WithStack()
}

// Tracef creates a formatted error with stack trace.
// Combines Errorf and WithStack for convenience.
func Tracef(format string, args ...interface{}) *Error {
	e := Newf(format, args...)
	return e.WithStack()
}

// As attempts to assign the error or one in its chain to the target interface.
// It only assigns when the target is a **Error and the current error node has a non-empty name.
// If the current node has an empty name, it delegates to its wrapped cause.
func (e *Error) As(target interface{}) bool {
	if e == nil {
		return false
	}
	// Handle *Error target (for stderrors.As compatibility)
	if targetPtr, ok := target.(*Error); ok {
		current := e
		for current != nil {
			if current.name != "" {
				*targetPtr = *current
				return true
			}
			if next, ok := current.cause.(*Error); ok {
				current = next
			} else if current.cause != nil {
				return errors.As(current.cause, target)
			} else {
				return false
			}
		}
		return false
	}
	// Handle *error target - unwrap to innermost error
	if targetErr, ok := target.(*error); ok {
		innermost := error(e)
		current := error(e)
		for current != nil {
			if err, ok := current.(*Error); ok && err.cause != nil {
				current = err.cause
				innermost = current
			} else {
				break
			}
		}
		*targetErr = innermost
		return true
	}

	// Delegate to cause for other types
	if e.cause != nil {
		return errors.As(e.cause, target)
	}
	return false
}

// Callback sets a function to be called when Error() is invoked.
// Useful for logging or side effects; returns the error for chaining.
func (e *Error) Callback(fn func()) *Error {
	e.callback = fn
	return e
}

// Category returns the error's category, if set.
// Returns an empty string if no category is defined.
func (e *Error) Category() string {
	return e.category
}

// Code returns the error's status code, if set.
// Returns 0 if no code is defined.
func (e *Error) Code() int {
	return int(e.code)
}

// Context returns the error's context as a map.
// Converts smallContext to a map if needed; returns nil if empty.
func (e *Error) Context() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 && e.context == nil {
		e.context = make(map[string]interface{}, e.smallCount)
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}
	return e.context
}

// Copy creates a deep copy of the error, preserving all fields except stack.
// The new error does not capture a new stack trace unless explicitly added.
func (e *Error) Copy() *Error {
	if e == emptyError {
		return &Error{
			smallContext: [contextSize]contextItem{},
		}
	}

	newErr := newError()

	newErr.msg = e.msg
	newErr.name = e.name
	newErr.template = e.template
	newErr.cause = e.cause
	newErr.code = e.code
	newErr.category = e.category
	newErr.count = e.count

	if e.smallCount > 0 {
		newErr.smallCount = e.smallCount
		for i := int32(0); i < e.smallCount; i++ {
			newErr.smallContext[i] = e.smallContext[i]
		}
	} else if e.context != nil {
		newErr.context = make(map[string]interface{}, len(e.context))
		for k, v := range e.context {
			newErr.context[k] = v
		}
	}

	if e.stack != nil && len(e.stack) > 0 {
		if newErr.stack == nil {
			newErr.stack = stackPool.Get().([]uintptr)
		}
		newErr.stack = append(newErr.stack[:0], e.stack...)
	}

	return newErr
}

// Count returns the number of times the error has been incremented.
// Useful for tracking occurrence frequency.
func (e *Error) Count() uint64 {
	return e.count
}

// Err returns the error as an error interface.
// Provided for compatibility; simply returns the error itself.
func (e *Error) Err() error {
	return e
}

// Error returns the string representation of the error.
// Prioritizes msg, then template, then name, falling back to "unknown error".
// Executes callback if set before returning the message.
func (e *Error) Error() string {
	if e.callback != nil {
		e.callback()
	}
	var msg string
	switch {
	case e.msg != "":
		msg = e.msg
	case e.template != "":
		msg = e.template
	case e.name != "":
		msg = e.name
	default:
		msg = "unknown error"
	}
	if e.cause != nil {
		causeMsg := e.cause.Error()
		if msg != "" && causeMsg != "" {
			msg = msg + ": " + causeMsg
		} else if causeMsg != "" {
			msg = causeMsg
		}
	}
	return msg
}

// Errorf creates a formatted error without stack traces.
// Compatible with fmt.Errorf; does not capture stack trace for performance.
func Errorf(format string, args ...interface{}) *Error {
	err := newError()
	err.msg = fmt.Sprintf(format, args...)
	return err
}

// FastStack returns a lightweight stack trace without function names.
// Filters internal frames if FilterInternal is enabled; returns nil if no stack.
func (e *Error) FastStack() []string {
	if e.stack == nil {
		return nil
	}
	configMu.RLock()
	filter := currentConfig.filterInternal
	configMu.RUnlock()

	pcs := e.stack
	frames := make([]string, 0, len(pcs))
	for _, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			frames = append(frames, "unknown")
			continue
		}
		file, line := fn.FileLine(pc)
		if filter && isInternalFrame(runtime.Frame{File: file, Function: fn.Name()}) {
			continue
		}
		frames = append(frames, fmt.Sprintf("%s:%d", file, line))
	}
	return frames
}

// Find searches the error chain for the first error matching pred.
// Starts with the current error and follows Unwrap() and Cause() chains.
func (e *Error) Find(pred func(error) bool) error {
	if e == nil || pred == nil {
		return nil
	}
	return Find(e, pred)
}

// Format returns a formatted string representation of the error.
// Includes message, code, context, and stack trace if present.
func (e *Error) Format() string {
	var sb strings.Builder

	// Error message
	sb.WriteString("Error: " + e.Error() + "\n")

	// Metadata
	if e.code != 0 {
		sb.WriteString(fmt.Sprintf("Code: %d\n", e.code))
	}

	// Context (only show context added at this level)
	if ctx := e.contextAtThisLevel(); len(ctx) > 0 {
		sb.WriteString("Context:\n")
		for k, v := range ctx {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}

	// Stack trace
	if e.stack != nil {
		sb.WriteString("Stack:\n")
		for i, frame := range e.Stack() {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, frame))
		}
	}

	return sb.String()
}

// contextAtThisLevel returns context specific to this error level, excluding inherited context.
// Combines smallContext and context map into a single map; returns nil if empty.
func (e *Error) contextAtThisLevel() map[string]interface{} {
	if e.context == nil && e.smallCount == 0 {
		return nil
	}

	ctx := make(map[string]interface{})
	// Add smallContext items
	for i := 0; i < int(e.smallCount); i++ {
		ctx[e.smallContext[i].key] = e.smallContext[i].value
	}
	// Add map context items
	if e.context != nil {
		for k, v := range e.context {
			ctx[k] = v
		}
	}
	return ctx
}

// Free resets the error and returns it to the pool if pooling is enabled.
// Does nothing beyond reset if pooling is disabled.
func (e *Error) Free() {
	if currentConfig.disablePooling {
		return
	}

	e.Reset()

	if e.stack != nil {
		stackPool.Put(e.stack[:cap(e.stack)])
		e.stack = nil
	}
	errorPool.Put(e)
}

// Has checks if the error contains meaningful content.
// Returns true if msg, template, name, or cause is non-empty/nil.
func (e *Error) Has() bool {
	return e != nil && (e.msg != "" || e.template != "" || e.name != "" || e.cause != nil)
}

// HasContextKey checks if the specified key exists in the error's context.
// Searches both smallContext and context map; thread-safe.
func (e *Error) HasContextKey(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.smallCount > 0 {
		for i := int32(0); i < e.smallCount; i++ {
			if e.smallContext[i].key == key {
				return true
			}
		}
	}
	if e.context != nil {
		_, exists := e.context[key]
		return exists
	}
	return false
}

// Increment increases the error's count by 1 and returns the error.
// Uses atomic operation for thread safety.
func (e *Error) Increment() *Error {
	atomic.AddUint64(&e.count, 1)
	return e
}

// Is checks if the error matches a target error by pointer equality, name, or wrapped cause.
// Ensures compatibility with stderrors.Is by prioritizing chain traversal.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return e == target
	}
	if e == target {
		return true
	}
	if e.name != "" {
		if te, ok := target.(*Error); ok && te.name != "" && e.name == te.name {
			return true
		}
	}
	// Add string comparison for standard errors
	if stdErr, ok := target.(error); ok && e.Error() == stdErr.Error() {
		return true
	}
	if e.cause != nil {
		return errors.Is(e.cause, target)
	}
	return false
}

// IsEmpty checks if the error has no meaningful content (empty message, no name/template/cause).
// Returns true for nil errors or errors with no data.
func (e *Error) IsEmpty() bool {
	if e == nil {
		return true
	}
	return e.msg == "" && e.template == "" && e.name == "" && e.cause == nil
}

// IsNull checks if an error is nil or represents a SQL NULL value.
// Considers both the error itself and any context values; returns true if all context is null.
func (e *Error) IsNull() bool {
	if e == nil || e == emptyError {
		return true
	}
	// If no context or cause, and no content, it's not null
	if e.smallCount == 0 && e.context == nil && e.cause == nil {
		return false
	}

	// Check cause first - if it’s null, the whole error is null
	if e.cause != nil {
		var isNull bool
		if ce, ok := e.cause.(*Error); ok {
			isNull = ce.IsNull()
		} else {
			isNull = sqlNull(e.cause)
		}
		if isNull {
			return true
		}
		// If cause isn’t null, continue checking this error’s context
	}

	// Check small context
	if e.smallCount > 0 {
		allNull := true
		for i := 0; i < int(e.smallCount); i++ {
			isNull := sqlNull(e.smallContext[i].value)
			if !isNull {
				allNull = false
				break
			}
		}
		if !allNull {
			return false
		}
	}

	// Check regular context
	if e.context != nil {
		allNull := true
		for _, v := range e.context {
			isNull := sqlNull(v)
			if !isNull {
				allNull = false
				break
			}
		}
		if !allNull {
			return false
		}
	}

	// Null if we have context and it’s all null
	return e.smallCount > 0 || e.context != nil
}

var (
	jsonBufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, bufferSize))
		},
	}
)

// MarshalJSON serializes the error to JSON, including name, message, context, cause, and stack.
// Handles nested *Error causes and custom marshalers efficiently.
func (e *Error) MarshalJSON() ([]byte, error) {
	// Get buffer from pool
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	defer jsonBufferPool.Put(buf)
	buf.Reset()

	// Create new encoder each time (no Reset available)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	// Prepare error data
	je := struct {
		Name    string                 `json:"name,omitempty"`
		Message string                 `json:"message,omitempty"`
		Context map[string]interface{} `json:"context,omitempty"`
		Cause   interface{}            `json:"cause,omitempty"`
		Stack   []string               `json:"stack,omitempty"`
		Code    int                    `json:"code,omitempty"`
	}{
		Name:    e.name,
		Message: e.msg,
		Code:    e.Code(),
	}

	// Handle context
	if ctx := e.Context(); len(ctx) > 0 {
		je.Context = ctx
	}

	// Handle stack
	if e.stack != nil {
		je.Stack = e.Stack()
	}

	// Handle cause
	if e.cause != nil {
		switch c := e.cause.(type) {
		case *Error:
			je.Cause = c
		case json.Marshaler:
			je.Cause = c
		default:
			je.Cause = c.Error()
		}
	}

	// Encode
	if err := enc.Encode(je); err != nil {
		return nil, err
	}

	// Return bytes without trailing newline
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// Msgf sets the error message using a formatted string.
// Overwrites any existing message; returns the error for chaining.
func (e *Error) Msgf(format string, args ...interface{}) *Error {
	e.msg = fmt.Sprintf(format, args...)
	return e
}

// Name returns the error's name, if set.
// Returns an empty string if no name is defined.
func (e *Error) Name() string {
	return e.name
}

// Reset clears all fields of the error, preparing it for reuse.
// Does not free the stack; use Free() to return to pool.
func (e *Error) Reset() {
	e.msg = ""
	e.name = ""
	e.template = ""
	e.category = ""
	e.code = 0
	e.count = 0
	e.cause = nil
	e.callback = nil

	if e.context != nil {
		for k := range e.context {
			delete(e.context, k)
		}
	}
	e.smallCount = 0

	if e.stack != nil {
		e.stack = e.stack[:0]
	}
}

// Stack returns a detailed stack trace as a slice of strings.
// Filters internal frames if FilterInternal is enabled; returns nil if no stack.
func (e *Error) Stack() []string {
	if e.stack == nil {
		return nil
	}

	frames := runtime.CallersFrames(e.stack)
	var trace []string
	for {
		frame, more := frames.Next()
		if frame == (runtime.Frame{}) {
			break
		}

		if currentConfig.filterInternal && isInternalFrame(frame) {
			continue
		}

		trace = append(trace, fmt.Sprintf("%s %s:%d",
			frame.Function,
			frame.File,
			frame.Line))

		if !more {
			break
		}
	}
	return trace
}

// Trace ensures the error has a stack trace, capturing it if missing.
// Skips capture if stack already exists; returns the error for chaining.
func (e *Error) Trace() *Error {
	if e.stack == nil {
		e.stack = captureStack(2)
	}
	return e
}

// Transform applies transformations to a copy of the error.
// Returns the transformed copy or the original if no changes are needed.
func (e *Error) Transform(fn func(*Error)) *Error {
	if e == nil || fn == nil {
		return e
	}
	newErr := e.Copy()
	fn(newErr)
	return newErr
}

// Unwrap returns the underlying cause of the error, if any.
// Implements the errors.Unwrap interface for unwrapping chains.
func (e *Error) Unwrap() error {
	return e.cause
}

// UnwrapAll returns a slice of all errors in the chain, starting with this error.
// Traverses the cause chain, creating isolated copies of each *Error.
func (e *Error) UnwrapAll() []error {
	if e == nil {
		return nil
	}
	var chain []error
	current := error(e)
	for current != nil {
		if err, ok := current.(*Error); ok {
			isolated := newError()
			isolated.msg = err.msg
			isolated.name = err.name
			isolated.template = err.template
			isolated.code = err.code
			isolated.category = err.category
			if err.smallCount > 0 {
				isolated.smallCount = err.smallCount
				for i := int32(0); i < err.smallCount; i++ {
					isolated.smallContext[i] = err.smallContext[i]
				}
			}
			if err.context != nil {
				isolated.context = make(map[string]interface{}, len(err.context))
				for k, v := range err.context {
					isolated.context[k] = v
				}
			}
			if err.stack != nil {
				isolated.stack = append([]uintptr(nil), err.stack...)
			}
			chain = append(chain, isolated)
		} else {
			chain = append(chain, current)
		}
		if unwrapper, ok := current.(interface{ Unwrap() error }); ok {
			current = unwrapper.Unwrap()
		} else {
			break
		}
	}
	return chain
}

// Walk traverses the error chain, applying fn to each error.
// Starts with the current error and follows the cause chain.
func (e *Error) Walk(fn func(error)) {
	if e == nil || fn == nil {
		return
	}
	current := error(e)
	for current != nil {
		fn(current)
		if unwrappable, ok := current.(interface{ Unwrap() error }); ok {
			current = unwrappable.Unwrap()
		} else {
			break
		}
	}
}

// With adds a key-value pair to the error's context.
// Uses smallContext for efficiency until full, then switches to map; thread-safe.
func (e *Error) With(key string, value interface{}) *Error {
	// Fast path for small context (no map needed)
	if e.smallCount < contextSize && e.context == nil {
		e.mu.Lock()
		// Double-check after acquiring lock
		if e.smallCount < contextSize && e.context == nil {
			e.smallContext[e.smallCount] = contextItem{key, value}
			e.smallCount++
			e.mu.Unlock()
			return e
		}
		e.mu.Unlock()
	}

	// Slow path - requires map
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.context == nil {
		e.context = make(map[string]interface{}, currentConfig.contextSize)
		// Migrate existing items if any
		for i := int32(0); i < e.smallCount; i++ {
			e.context[e.smallContext[i].key] = e.smallContext[i].value
		}
	}

	e.context[key] = value
	return e
}

// WithCategory sets a category for the error and returns the error.
// Useful for classifying errors (e.g., "network", "validation").
func (e *Error) WithCategory(category ErrorCategory) *Error {
	e.category = string(category)
	return e
}

// WithCode sets an HTTP-like status code for the error and returns the error.
// Overwrites any existing code.
func (e *Error) WithCode(code int) *Error {
	e.code = int32(code)
	return e
}

// WithName sets the error's name and returns the error.
// Overwrites any existing name.
func (e *Error) WithName(name string) *Error {
	e.name = name
	return e
}

// WithRetryable marks the error as retryable in its context.
// Adds a "retry" key with value true; returns the error.
func (e *Error) WithRetryable() *Error {
	return e.With(ctxRetry, true)
}

// WithStack captures the stack trace at call time and returns the error.
// Skips capturing if stack already exists or depth is 0.
func (e *Error) WithStack() *Error {
	if e.stack == nil {
		e.stack = captureStack(1) // Skip WithStack
	}
	return e
}

// WithTemplate sets a template string for the error and returns the error.
// Used as the error message if no explicit message is set.
func (e *Error) WithTemplate(template string) *Error {
	e.template = template
	return e
}

// WithTimeout marks the error as a timeout error in its context.
// Adds a "timeout" key with value true; returns the error.
func (e *Error) WithTimeout() *Error {
	return e.With(ctxTimeout, true)
}

// Wrap associates a cause error with this error, creating an error chain.
// Returns the error for method chaining.
func (e *Error) Wrap(cause error) *Error {
	if cause == nil {
		return e
	}
	e.cause = cause
	return e
}

// WrapNotNil wraps a cause error only if it is non-nil.
// Returns the error for method chaining; no-op if cause is nil.
func (e *Error) WrapNotNil(cause error) *Error {
	if cause != nil {
		e.cause = cause
	}
	return e
}

// WarmPool pre-populates the error pool with a specified number of instances.
// Reduces allocation overhead during initial usage; no effect if pooling is disabled.
func WarmPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		e := &Error{
			smallContext: [contextSize]contextItem{},
			stack:        nil,
		}
		errorPool.Put(e)
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}

// WarmStackPool pre-populates the stack pool with a specified number of slices.
// Reduces allocation overhead for stack traces; no effect if pooling is disabled.
func WarmStackPool(count int) {
	if currentConfig.disablePooling {
		return
	}
	for i := 0; i < count; i++ {
		stackPool.Put(make([]uintptr, 0, currentConfig.stackDepth))
	}
}

package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/api"
)

// FunctionListenerFactoryKey is a context.Context Value key. Its associated value should be a FunctionListenerFactory.
//
// Note: This is interpreter-only for now!
//
// See https://github.com/tetratelabs/wazero/issues/451
type FunctionListenerFactoryKey struct{}

// FunctionListenerFactory returns FunctionListeners to be notified when a function is called.
type FunctionListenerFactory interface {
	// NewListener returns a FunctionListener for a defined function. If nil is returned, no
	// listener will be notified.
	NewListener(FunctionDefinition) FunctionListener
}

// FunctionListener can be registered for any function via FunctionListenerFactory to
// be notified when the function is called.
type FunctionListener interface {
	// Before is invoked before a function is called. ctx is the context of the caller function.
	// The returned context will be used as the context of this function call. To add context
	// information for this function call, add it to ctx and return the updated context. If
	// no context information is needed, return ctx as is.
	Before(ctx context.Context, paramValues []uint64) context.Context

	// After is invoked after a function is called. ctx is the context of this function call.
	// The err parameter is nil on success.
	After(ctx context.Context, err error, resultValues []uint64)
}

// TODO: We need to add tests to enginetest to ensure contexts nest. A good test can use a combination of call and call
// indirect in terms of depth and breadth. The test could show a tree 3 calls deep where the there are a couple calls at
// each depth under the root. The main thing this can help prevent is accidentally swapping the context internally.

// TODO: Errors aren't handled, and the After hook should accept one along with the result values.

// TODO: The context parameter of the After hook is not the same as the Before hook. This means interceptor patterns
// are awkward. Ex. something like timing is difficult as it requires propagating a stack. Otherwise, nested calls will
// overwrite each other's "since" time. Propagating a stack is further awkward as the After hook needs to know the
// position to read from which might be subtle.

// FunctionDefinition includes information about a function available pre-instantiation.
type FunctionDefinition interface {
	// ModuleName is the possibly empty name of the module defining this function.
	ModuleName() string

	// Index is the position in the module's function index namespace, imports first.
	Index() uint32

	// Name is the module-defined name of the function, which is not necessarily the same as its export name.
	Name() string

	// ExportNames include all exported names for the given function.
	ExportNames() []string

	// ParamTypes are the parameters of the function.
	ParamTypes() []api.ValueType

	// ParamNames are index-correlated with ParamTypes or nil if not available for one or more parameters.
	ParamNames() []string

	// ResultTypes are the results of the function.
	ResultTypes() []api.ValueType
}

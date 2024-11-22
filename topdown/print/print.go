package print

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown/print"
)

// Context provides the Hook implementation context about the print() call.
type Context = v1.Context

// Hook defines the interface that callers can implement to receive print
// statement outputs. If the hook returns an error, it will be surfaced if
// strict builtin error checking is enabled (otherwise, it will not halt
// execution.)
type Hook = v1.Hook

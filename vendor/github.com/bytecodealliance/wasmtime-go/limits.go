package wasmtime

// #include <wasm.h>
import "C"
import "runtime"

// LimitsMaxNone is the value for the Max field in Limits
const LimitsMaxNone = 0xffffffff

// Limits is the resource limits specified for a TableType and MemoryType
type Limits struct {
	// The minimum size of this resource, in units specified by the resource
	// itself.
	Min uint32
	// The maximum size of this resource, in units specified by the resource
	// itself.
	//
	// A value of LimitsMaxNone will mean that there is no maximum.
	Max uint32
}

// NewLimits creates a new resource limits specified for a TableType and MemoryType,
// in which min and max are the minimum and maximum size of this resource.
func NewLimits(min, max uint32) *Limits {
	return &Limits{
		Min: min,
		Max: max,
	}
}

func (limits Limits) ffi() C.wasm_limits_t {
	return C.wasm_limits_t{
		min: C.uint32_t(limits.Min),
		max: C.uint32_t(limits.Max),
	}
}

func mkLimits(ptr *C.wasm_limits_t, owner interface{}) Limits {
	ret := Limits{
		Min: uint32(ptr.min),
		Max: uint32(ptr.max),
	}
	runtime.KeepAlive(owner)
	return ret
}

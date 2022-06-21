package moremath

import "math"

// WasmCompatMin is the Wasm spec compatible variant of math.Min
//
// This returns math.NaN if either parameter is math.NaN, even if the other is -math.Inf.
//
// See https://github.com/golang/go/blob/1d20a362d0ca4898d77865e314ef6f73582daef0/src/math/dim.go#L74-L91
func WasmCompatMin(x, y float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y): // NaN cannot be compared with themselves, so we have to use IsNaN
		return math.NaN()
	case math.IsInf(x, -1) || math.IsInf(y, -1):
		return math.Inf(-1)
	case x == 0 && x == y:
		if math.Signbit(x) {
			return x
		}
		return y
	}
	if x < y {
		return x
	}
	return y
}

// WasmCompatMax is the Wasm spec compatible variant of math.Max
//
// This returns math.NaN if either parameter is math.NaN, even if the other is math.Inf.
//
// See https://github.com/golang/go/blob/1d20a362d0ca4898d77865e314ef6f73582daef0/src/math/dim.go#L42-L59
func WasmCompatMax(x, y float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y): // NaN cannot be compared with themselves, so we have to use IsNaN
		return math.NaN()
	case math.IsInf(x, 1) || math.IsInf(y, 1):
		return math.Inf(1)

	case x == 0 && x == y:
		if math.Signbit(x) {
			return y
		}
		return x
	}
	if x > y {
		return x
	}
	return y
}

// WasmCompatNearestF32 is the Wasm spec compatible variant of math.Round, used for Nearest instruction.
// For example, this converts 1.9 to 2.0, and this has the semantics of LLVM's rint intrinsic.
//
// Ex. math.Round(-4.5) results in -5 while this results in -4.
//
// See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
func WasmCompatNearestF32(f float32) float32 {
	// TODO: look at https://github.com/bytecodealliance/wasmtime/pull/2171 and reconsider this algorithm
	if f != 0 {
		ceil := float32(math.Ceil(float64(f)))
		floor := float32(math.Floor(float64(f)))
		distToCeil := math.Abs(float64(f - ceil))
		distToFloor := math.Abs(float64(f - floor))
		h := ceil / 2.0
		if distToCeil < distToFloor {
			f = ceil
		} else if distToCeil == distToFloor && float32(math.Floor(float64(h))) == h {
			f = ceil
		} else {
			f = floor
		}
	}
	return f
}

// WasmCompatNearestF64 is the Wasm spec compatible variant of math.Round, used for Nearest instruction.
// For example, this converts 1.9 to 2.0, and this has the semantics of LLVM's rint intrinsic.
//
// Ex. math.Round(-4.5) results in -5 while this results in -4.
//
// See https://llvm.org/docs/LangRef.html#llvm-rint-intrinsic.
func WasmCompatNearestF64(f float64) float64 {
	// TODO: look at https://github.com/bytecodealliance/wasmtime/pull/2171 and reconsider this algorithm
	if f != 0 {
		ceil := math.Ceil(f)
		floor := math.Floor(f)
		distToCeil := math.Abs(f - ceil)
		distToFloor := math.Abs(f - floor)
		h := ceil / 2.0
		if distToCeil < distToFloor {
			f = ceil
		} else if distToCeil == distToFloor && math.Floor(float64(h)) == h {
			f = ceil
		} else {
			f = floor
		}
	}
	return f
}

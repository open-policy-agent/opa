package pool

var errorSlicePool = New[[]error](allocErrorSlice, freeErrorSlice)

func allocErrorSlice() []error {
	return make([]error, 0, 1)
}

func freeErrorSlice(s []error) []error {
	// Reset the slice to its zero value
	return s[:0]
}

func ErrorSlice() *Pool[[]error] {
	return errorSlicePool
}

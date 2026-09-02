package util

type (
	Number interface {
		Integer | Float
	}
	Integer interface {
		SignedInteger | UnsignedInteger
	}
	SignedInteger interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}
	UnsignedInteger interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
	}
	Float interface {
		~float32 | ~float64
	}
)

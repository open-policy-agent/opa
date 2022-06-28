//go:build !cgo

package platform

func nanotime() int64 {
	return nanotimePortable()
}

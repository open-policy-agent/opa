//go:build !(darwin || linux || windows)

package platform

import (
	"fmt"
	"runtime"
)

var errUnsupported = fmt.Errorf("mmap unsupported on GOOS=%s. Use interpreter instead.", runtime.GOOS)

func munmapCodeSegment(code []byte) error {
	panic(errUnsupported)
}

func mmapCodeSegmentAMD64(code []byte) ([]byte, error) {
	panic(errUnsupported)
}

func mmapCodeSegmentARM64(code []byte) ([]byte, error) {
	panic(errUnsupported)
}

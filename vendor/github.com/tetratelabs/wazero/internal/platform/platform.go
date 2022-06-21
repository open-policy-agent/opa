// Package platform includes runtime-specific code needed for the compiler or otherwise.
//
// Note: This is a dependency-free alternative to depending on parts of Go's x/sys.
// See /RATIONALE.md for more context.
package platform

import (
	"errors"
	"runtime"
)

// CompilerSupported is exported for tests and includes constraints here and also the assembler.
func CompilerSupported() bool {
	switch runtime.GOOS {
	case "darwin", "windows", "linux":
	default:
		return false
	}

	switch runtime.GOARCH {
	case "amd64", "arm64":
	default:
		return false
	}

	return true
}

// MmapCodeSegment copies the code into the executable region and returns the byte slice of the region.
//
// See https://man7.org/linux/man-pages/man2/mmap.2.html for mmap API and flags.
func MmapCodeSegment(code []byte) ([]byte, error) {
	if len(code) == 0 {
		panic(errors.New("BUG: MmapCodeSegment with zero length"))
	}
	if runtime.GOARCH == "amd64" {
		return mmapCodeSegmentAMD64(code)
	} else {
		return mmapCodeSegmentARM64(code)
	}
}

// MunmapCodeSegment unmaps the given memory region.
func MunmapCodeSegment(code []byte) error {
	if len(code) == 0 {
		panic(errors.New("BUG: MunmapCodeSegment with zero length"))
	}
	return munmapCodeSegment(code)
}

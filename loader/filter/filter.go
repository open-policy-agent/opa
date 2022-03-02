package filter

import "os"

// Filter defines the interface for filtering files during loading. If the
// filter returns true, the file should be excluded from the result.
type LoaderFilter func(abspath string, info os.FileInfo, depth int) bool

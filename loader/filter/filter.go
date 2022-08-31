package filter

import "os"

type LoaderFilter func(abspath string, info os.FileInfo, depth int) bool

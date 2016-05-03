// Copyright 2013 Apcera Inc. All rights reserved.

package term

import (
	"os"
	"strconv"
)

// GetEnvWindowSize returns the window Size, as determined by process
// environment; if either LINES or COLUMNS is present, and whichever is
// present is also numeric, the Size will be non-nil.  If Size is nil,
// there's insufficient data in environ.  If one entry is 0, that means
// that the environment does not include that data.  If a value is
// negative, we treat that as an error.
func GetEnvWindowSize() *Size {
	lines := os.Getenv("LINES")
	columns := os.Getenv("COLUMNS")
	if lines == "" && columns == "" {
		return nil
	}

	nLines := 0
	nColumns := 0
	var err error
	if lines != "" {
		nLines, err = strconv.Atoi(lines)
		if err != nil || nLines < 0 {
			return nil
		}
	}
	if columns != "" {
		nColumns, err = strconv.Atoi(columns)
		if err != nil || nColumns < 0 {
			return nil
		}
	}

	return &Size{
		Lines:   nLines,
		Columns: nColumns,
	}
}

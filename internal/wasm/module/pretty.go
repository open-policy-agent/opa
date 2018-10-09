// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package module

import (
	"encoding/hex"
	"fmt"
	"io"
)

// PrettyOption defines options for controlling pretty printing.
type PrettyOption struct {
	Contents bool // show raw byte content of data+code sections.
}

// Pretty writes a human-readable representation of m to w.
func Pretty(w io.Writer, m *Module, opts ...PrettyOption) {
	fmt.Println("version:", m.Version)
	fmt.Println("types:")
	for _, fn := range m.Type.Functions {
		fmt.Println("  -", fn)
	}
	fmt.Println("imports:")
	for i, imp := range m.Import.Imports {
		if imp.Descriptor.Kind() == FunctionImportType {
			fmt.Printf("  - [%d] %v\n", i, imp)
		} else {
			fmt.Println("  -", imp)
		}
	}
	fmt.Println("functions:")
	for _, fn := range m.Function.TypeIndices {
		if fn >= uint32(len(m.Type.Functions)) {
			fmt.Println("  -", "???")
		} else {
			fmt.Println("  -", m.Type.Functions[fn])
		}
	}
	fmt.Println("exports:")
	for _, exp := range m.Export.Exports {
		fmt.Println("  -", exp)
	}
	fmt.Println("code:")
	for _, seg := range m.Code.Segments {
		fmt.Println("  -", seg)
	}
	fmt.Println("data:")
	for _, seg := range m.Data.Segments {
		fmt.Println("  -", seg)
	}
	if len(opts) == 0 {
		return
	}
	fmt.Println()
	for _, opt := range opts {
		if opt.Contents {
			newline := false
			if len(m.Data.Segments) > 0 {
				fmt.Println("data section:")
				for _, seg := range m.Data.Segments {
					if newline {
						fmt.Println()
					}
					fmt.Println(hex.Dump(seg.Init))
					newline = true
				}
				newline = false
			}
			if len(m.Code.Segments) > 0 {
				fmt.Println("code section:")
				for _, seg := range m.Code.Segments {
					if newline {
						fmt.Println()
					}
					fmt.Println(hex.Dump(seg.Code))
					newline = true
				}
				newline = false
			}
		}
	}
}

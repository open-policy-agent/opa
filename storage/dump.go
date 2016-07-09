// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"encoding/json"
	"io"
)

// Dump writes the content of the DataStore ds to the io.Writer w.
func Dump(ds *DataStore, w io.Writer) error {
	e := json.NewEncoder(w)
	return e.Encode(ds.data)
}

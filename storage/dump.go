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

// Load reads the content of a serialized DataStore from the io.Reader r.
func Load(r io.Reader) (*DataStore, error) {
	d := json.NewDecoder(r)
	var data map[string]interface{}
	if err := d.Decode(&data); err != nil {
		return nil, err
	}
	return NewDataStoreFromJSONObject(data), nil
}

// LoadOrDie reads the content of a serialized DataStore from the io.Reader r.
// If the load fails for any reason, this function will panic.
func LoadOrDie(r io.Reader) *DataStore {
	ds, err := Load(r)
	if err != nil {
		panic(err)
	}
	return ds
}

// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package maps

import (
	"bytes"
	"encoding/gob"
)

func CopyMap(m map[string]interface{}) (map[string]interface{}, error) {
	var cpy map[string]interface{}
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		return nil, err
	}

	if err := gob.NewDecoder(&buf).Decode(&cpy); err != nil {
		return nil, err
	}

	return cpy, nil
}

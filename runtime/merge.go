// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"

	"github.com/pkg/errors"
)

// mergeDocs returns the result of merging a and b. If a and b cannot be merged
// because of conflicting key-value pairs, an error is returned.
func mergeDocs(a map[string]interface{}, b map[string]interface{}) (map[string]interface{}, error) {

	merged := map[string]interface{}{}
	for k := range a {
		merged[k] = a[k]
	}

	for k := range b {

		add := b[k]
		exist, ok := merged[k]
		if !ok {
			merged[k] = add
			continue
		}

		existObj, existOk := exist.(map[string]interface{})
		addObj, addOk := add.(map[string]interface{})
		if !existOk || !addOk {
			return nil, fmt.Errorf("%v: merge error: %T cannot merge into %T", k, add, exist)
		}

		mergedObj, err := mergeDocs(existObj, addObj)
		if err != nil {
			return nil, errors.Wrapf(err, k)
		}

		merged[k] = mergedObj
	}

	return merged, nil
}

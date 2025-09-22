// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

// +build go1.12

package clone

import (
	"reflect"
)

func mapIter(m reflect.Value) *reflect.MapIter {
	return m.MapRange()
}

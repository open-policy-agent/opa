// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

// +build !go1.12

package clone

import (
	"reflect"
)

type iter struct {
	m    reflect.Value
	k    reflect.Value
	keys []reflect.Value
}

func mapIter(m reflect.Value) *iter {
	return &iter{
		m:    m,
		keys: m.MapKeys(),
	}
}

func (it *iter) Next() bool {
	if len(it.keys) == 0 {
		return false
	}

	it.k = it.keys[0]
	it.keys = it.keys[1:]
	return true
}

func (it *iter) Key() reflect.Value {
	return it.k
}

func (it *iter) Value() reflect.Value {
	return it.m.MapIndex(it.k)
}

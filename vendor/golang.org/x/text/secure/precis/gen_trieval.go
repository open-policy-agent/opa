// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

type property int

const (
	pValid property = 1 << iota
	contextO
	contextJ
	disallowed
	unassigned
	freePVal
	idDis
)

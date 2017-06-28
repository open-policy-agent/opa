// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package watch provides the ability to set a watch on a Rego query. A watch will
// monitor the query and determine when any of it's dependencies change, notifying the
// client of the new results of the query evaluation whenever this occurs.
package watch

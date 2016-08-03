// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package topdown provides query evaluation support.
//
// The topdown implementation is a (slightly modified) version of the standard "top-down evaluation" algorithm used in query languages such as Datalog. The main difference in the implementation is in the handling of Rego references.
//
// At a high level, the topdown implementation consists of (1) term evaluation and (2) expression evaluation. Term evaluation involves handling expression terms in isolation whereas expression evaluation involves evaluating the terms in relation to each other.
//
// During the term evaluation phase, the topdown implementation will evaluate references found in the expression to produce bindings for:
//
//  1. Variables appearing in references
//  2. References to Virtual Documents
//  3. Comprehensions
//
// Once terms have been evaluated in isolation, the overall expression can be evaluated. If the expression is simply a term (e.g., string, reference, variable, etc.) then it is compared against the boolean "false" value. If the value IS NOT "false", evaluation continues. On the other hand, if the expression is defined by a built-in operator (e.g., =, !=, min, max, etc.) then the appropriate built-in function is invoked to determine if evaluation continues and bind variables accordingly.
package topdown

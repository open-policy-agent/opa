// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

// DefaultIncludeRuleMetadata controls whether custom metadata from rule annotations
// is included in decision logs by default. Embedding projects can set this to true
// via an init() function or linker flag to change the default.
var DefaultIncludeRuleMetadata bool

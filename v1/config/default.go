// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

// DefaultIncludeRuleMetadata controls whether custom metadata from rule annotations
// is included in decision logs by default. When true, this also enables annotation
// processing during policy parsing. Embedding projects can set this to true via an
// init() function to change the default.
var DefaultIncludeRuleMetadata bool

// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package clone

// maxByteSize is a large enough value to cheat Go compiler
// when converting unsafe address to []byte.
// It's not actually used in runtime.
//
// The value 2^30 is the max value AFAIK to make Go compiler happy on all archs.
const maxByteSize = 1 << 30

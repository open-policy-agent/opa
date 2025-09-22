// Copyright 2019 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package clone

import "reflect"

// As golint reports warning on possible misuse of these headers,
// avoid to use these header types directly to silience golint.

type sliceHeader reflect.SliceHeader
type stringHeader reflect.StringHeader

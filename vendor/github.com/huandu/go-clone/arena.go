// Copyright 2023 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

//go:build !(go1.20 && goexperiment.arenas)
// +build !go1.20 !goexperiment.arenas

package clone

const arenaIsEnabled = false

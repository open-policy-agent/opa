// Copyright 2023 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package clone

// Cloner implements clone API with given allocator.
type Cloner struct {
	allocator *Allocator
}

// MakeCloner creates a cloner with given allocator.
func MakeCloner(allocator *Allocator) Cloner {
	return Cloner{
		allocator: allocator,
	}
}

// Clone clones v with given allocator.
func (c Cloner) Clone(v interface{}) interface{} {
	return clone(c.allocator, v)
}

// CloneSlowly clones v with given allocator.
// It can clone v with cycle pointer.
func (c Cloner) CloneSlowly(v interface{}) interface{} {
	return cloneSlowly(c.allocator, v)
}

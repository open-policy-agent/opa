/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errdef

import "errors"

// Common errors used in ORAS
var (
	ErrAlreadyExists      = errors.New("already exists")
	ErrUnsupported        = errors.New("unsupported")
	ErrInvalidReference   = errors.New("invalid reference")
	ErrInvalidDigest      = errors.New("invalid digest")
	ErrNotFound           = errors.New("not found")
	ErrUnsupportedVersion = errors.New("unsupported version")
	ErrMissingReference   = errors.New("missing reference")
	ErrSizeExceedsLimit   = errors.New("size exceeds limit")
)

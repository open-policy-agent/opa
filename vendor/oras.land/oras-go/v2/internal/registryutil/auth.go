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

package registryutil

import (
	"context"

	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// WithScopeHint adds a hinted scope to the context.
func WithScopeHint(ctx context.Context, ref registry.Reference, actions ...string) context.Context {
	scope := auth.ScopeRepository(ref.Repository, actions...)
	return auth.AppendScopes(ctx, scope)
}

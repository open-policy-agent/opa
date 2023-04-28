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

package registry

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/errdef"
)

// regular expressions for components.
var (
	// repositoryRegexp is adapted from the distribution implementation. The
	// repository name set under OCI distribution spec is a subset of the docker
	// spec. For maximum compatability, the docker spec is verified client-side.
	// Further checks are left to the server-side.
	// References:
	// - https://github.com/distribution/distribution/blob/v2.7.1/reference/regexp.go#L53
	// - https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	repositoryRegexp = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|[-]*)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|[-]*)[a-z0-9]+)*)*$`)

	// tagRegexp checks the tag name.
	// The docker and OCI spec have the same regular expression.
	// Reference: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	tagRegexp = regexp.MustCompile(`^[\w][\w.-]{0,127}$`)
)

// Reference references to a descriptor in the registry.
type Reference struct {
	// Registry is the name of the registry.
	// It is usually the domain name of the registry optionally with a port.
	Registry string

	// Repository is the name of the repository.
	Repository string

	// Reference is the reference of the object in the repository.
	// A reference can be a tag or a digest.
	Reference string
}

// ParseReference parses a string (artifact) into an `artifact reference`.
//
// Note: An "image" is an "artifact", however, an "artifact" is not necessarily
// an "image".
//
// The parameter `artifact` is an opaque polymorphic string, composed of two
// other opaque tokens; namely `socketaddr` and `path`: These can in turn take
// on their own polymorphic forms.  To do this, the Backus–Naur Form (BNF)
// notation has been adopted, with the exception that *lowercase* labels here
// imply opaque values (composed of other components), and *uppercase* labels
// imply resolute/final values (can not be expanded further into subcomponents).
//
//	  <artifact> ::= <socketaddr> "/" <path>
//
//	<socketaddr> ::= <host> | <host> ":" <PORT>
//	      <host> ::= <ip> | <FQDN>
//	        <ip> ::= <IPV4-ADDR> | <IPV6-ADDR>
//
//	      <path> ::= <REPOSITORY> | <REPOSITORY> <reference>
//	 <reference> ::= "@" <digest> | ":" <TAG> "@" <DIGEST> | ":" <TAG>
//	    <digest> ::= <ALGO> ":" <HASH>
//
// That is, of all the possible forms that `path` can take, there are exactly 4
// that are considered valid.  Expanding the BNF notation for `path`, the 4 are:
//
//	<---------------------- path ----------------------> |  - Decode `path`
//	<=== REPOSITORY ===><---------- reference ---------> |    - Decode `reference`
//	<=== REPOSITORY ===><=================== digest ===> |      - Valid Form A
//	<=== REPOSITORY ===><!!! TAG !!!> @ <=== digest ===> |      - Valid Form B
//	<=== REPOSITORY ===><=== TAG ======================> |      - Valid Form C
//	<=== REPOSITORY ===================================> |    - Valid Form D
//
// Note: In the case of Valid Form B, TAG is dropped without any validation or
// further consideration.
func ParseReference(artifact string) (Reference, error) {
	parts := strings.SplitN(artifact, "/", 2)
	if len(parts) == 1 {
		// Invalid Form
		return Reference{}, fmt.Errorf("%w: missing repository", errdef.ErrInvalidReference)
	}
	registry, path := parts[0], parts[1]

	var repository string
	var reference string
	if index := strings.Index(path, "@"); index != -1 {
		// `digest` found; Valid Form A (if not B)
		repository = path[:index]
		reference = path[index+1:]

		if index := strings.Index(repository, ":"); index != -1 {
			// `tag` found (and now dropped without validation) since `the
			// `digest` already present; Valid Form B
			repository = repository[:index]
		}
	} else if index := strings.Index(path, ":"); index != -1 {
		// `tag` found; Valid Form C
		repository = path[:index]
		reference = path[index+1:]
	} else {
		// empty `reference`; Valid Form D
		repository = path
	}
	res := Reference{
		Registry:   registry,
		Repository: repository,
		Reference:  reference,
	}
	if err := res.Validate(); err != nil {
		return Reference{}, err
	}
	return res, nil
}

// Validate validates the entire reference.
func (r Reference) Validate() error {
	err := r.ValidateRegistry()
	if err != nil {
		return err
	}
	err = r.ValidateRepository()
	if err != nil {
		return err
	}
	return r.ValidateReference()
}

// ValidateRegistry validates the registry.
func (r Reference) ValidateRegistry() error {
	uri, err := url.ParseRequestURI("dummy://" + r.Registry)
	if err != nil || uri.Host != r.Registry {
		return fmt.Errorf("%w: invalid registry", errdef.ErrInvalidReference)
	}
	return nil
}

// ValidateRepository validates the repository.
func (r Reference) ValidateRepository() error {
	if !repositoryRegexp.MatchString(r.Repository) {
		return fmt.Errorf("%w: invalid repository", errdef.ErrInvalidReference)
	}
	return nil
}

// ValidateReference validates the reference.
func (r Reference) ValidateReference() error {
	if r.Reference == "" {
		return nil
	}
	if _, err := r.Digest(); err == nil {
		return nil
	}
	if !tagRegexp.MatchString(r.Reference) {
		return fmt.Errorf("%w: invalid tag", errdef.ErrInvalidReference)
	}
	return nil
}

// Host returns the host name of the registry.
func (r Reference) Host() string {
	if r.Registry == "docker.io" {
		return "registry-1.docker.io"
	}
	return r.Registry
}

// ReferenceOrDefault returns the reference or the default reference if empty.
func (r Reference) ReferenceOrDefault() string {
	if r.Reference == "" {
		return "latest"
	}
	return r.Reference
}

// Digest returns the reference as a digest.
func (r Reference) Digest() (digest.Digest, error) {
	return digest.Parse(r.Reference)
}

// String implements `fmt.Stringer` and returns the reference string.
// The resulted string is meaningful only if the reference is valid.
func (r Reference) String() string {
	if r.Repository == "" {
		return r.Registry
	}
	ref := r.Registry + "/" + r.Repository
	if r.Reference == "" {
		return ref
	}
	if d, err := r.Digest(); err == nil {
		return ref + "@" + d.String()
	}
	return ref + ":" + r.Reference
}

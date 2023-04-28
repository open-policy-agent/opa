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

package oras

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	artifactspec "github.com/oras-project/artifacts-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
)

// MediaTypeUnknownConfig is the default mediaType used when no
// config media type is specified.
const MediaTypeUnknownConfig = "application/vnd.unknown.config.v1+json"

var (
	// ErrMissingArtifactType is returned by PackArtifact() when no artifact
	// type is specified.
	ErrMissingArtifactType = errors.New("missing artifact type")
	// ErrInvalidDateTimeFormat is returned by PackArtifact() when
	// AnnotationArtifactCreated is provided, but its value is not in RFC 3339
	// format.
	// Reference: https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
	ErrInvalidDateTimeFormat = errors.New("invalid date and time format")
)

// PackOptions contains parameters for oras.Pack.
type PackOptions struct {
	// ConfigDescriptor is a pointer to the descriptor of the config blob.
	ConfigDescriptor *ocispec.Descriptor
	// ConfigMediaType is the media type of the config blob.
	// If not specified, MediaTypeUnknownConfig will be used.
	ConfigMediaType string
	// ConfigAnnotations is the annotation map of the config descriptor.
	ConfigAnnotations map[string]string
	// ManifestAnnotations is the annotation map of the manifest.
	ManifestAnnotations map[string]string
}

// PackArtifactOptions contains parameters for oras.PackArtifact.
type PackArtifactOptions struct {
	// Subject is the subject of the ORAS Artifact Manifest.
	Subject *artifactspec.Descriptor
	// ManifestAnnotations is the annotation map of the manifest.
	ManifestAnnotations map[string]string
}

// Pack packs the given layers, generates a manifest for the pack,
// and pushes it to a content storage.
// If succeeded, returns a descriptor of the manifest.
func Pack(ctx context.Context, pusher content.Pusher, layers []ocispec.Descriptor, opts PackOptions) (ocispec.Descriptor, error) {
	if opts.ConfigMediaType == "" {
		opts.ConfigMediaType = MediaTypeUnknownConfig
	}

	var configDesc ocispec.Descriptor
	if opts.ConfigDescriptor != nil {
		configDesc = *opts.ConfigDescriptor
	} else {
		// Use an empty JSON object here, because some registries may not accept
		// empty config blob.
		// As of September 2022, GAR is known to return 400 on empty blob upload.
		// See https://github.com/oras-project/oras-go/issues/294 for details.
		configBytes := []byte("{}")
		configDesc = ocispec.Descriptor{
			MediaType:   opts.ConfigMediaType,
			Digest:      digest.FromBytes(configBytes),
			Size:        int64(len(configBytes)),
			Annotations: opts.ConfigAnnotations,
		}

		// push config
		if err := pusher.Push(ctx, configDesc, bytes.NewReader(configBytes)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			return ocispec.Descriptor{}, fmt.Errorf("failed to push config: %w", err)
		}
	}

	if layers == nil {
		layers = []ocispec.Descriptor{} // make it an empty array to prevent potential server-side bugs
	}

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      configDesc,
		MediaType:   ocispec.MediaTypeImageManifest,
		Layers:      layers,
		Annotations: opts.ManifestAnnotations,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	manifestDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageManifest,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}

	// push manifest
	if err := pusher.Push(ctx, manifestDesc, bytes.NewReader(manifestBytes)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}

	return manifestDesc, nil
}

// PackArtifact packs the given blobs, generates an ORAS Artifact Manifest for
// the pack, and pushes it to a content storage.
// If succeeded, returns a descriptor of the manifest.
// Returns ErrMissingArtifactType if artifactType is empty.
// Reference: https://github.com/oras-project/artifacts-spec/blob/main/artifact-manifest.md
func PackArtifact(ctx context.Context, pusher content.Pusher, artifactType string, blobs []artifactspec.Descriptor, opts PackArtifactOptions) (ocispec.Descriptor, error) {
	if artifactType == "" {
		// artifactType is required for ORAS Artifact Manifest
		return ocispec.Descriptor{}, ErrMissingArtifactType
	}

	if createdTime, ok := opts.ManifestAnnotations[artifactspec.AnnotationArtifactCreated]; ok {
		// if AnnotationArtifactCreated is provided, validate its format
		if _, err := time.Parse(time.RFC3339, createdTime); err != nil {
			return ocispec.Descriptor{}, fmt.Errorf("%w: %v", ErrInvalidDateTimeFormat, err)
		}
	} else {
		// copy the original annotation map
		annotations := make(map[string]string, len(opts.ManifestAnnotations)+1)
		for k, v := range opts.ManifestAnnotations {
			annotations[k] = v
		}

		// set creation time in RFC 3339 format
		// reference: https://github.com/oras-project/artifacts-spec/blob/main/artifact-manifest.md#oras-artifact-manifest-properties
		now := time.Now().UTC()
		annotations[artifactspec.AnnotationArtifactCreated] = now.Format(time.RFC3339)
		opts.ManifestAnnotations = annotations
	}

	if blobs == nil {
		blobs = []artifactspec.Descriptor{} // make it an empty array to prevent potential server-side bugs
	}

	manifest := artifactspec.Manifest{
		MediaType:    artifactspec.MediaTypeArtifactManifest,
		ArtifactType: artifactType,
		Blobs:        blobs,
		Subject:      opts.Subject,
		Annotations:  opts.ManifestAnnotations,
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestDesc := ocispec.Descriptor{
		MediaType: artifactspec.MediaTypeArtifactManifest,
		Digest:    digest.FromBytes(manifestBytes),
		Size:      int64(len(manifestBytes)),
	}

	// push manifest
	if err := pusher.Push(ctx, manifestDesc, bytes.NewReader(manifestBytes)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}

	return manifestDesc, nil
}

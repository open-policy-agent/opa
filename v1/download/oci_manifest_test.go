//go:build !opa_no_oci

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
)

func pushJSON(t *testing.T, store *memory.Store, mediaType string, v any) ocispec.Descriptor {
	t.Helper()

	bs, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	desc := content.NewDescriptorFromBytes(mediaType, bs)
	if err := store.Push(t.Context(), desc, bytes.NewReader(bs)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		t.Fatal(err)
	}

	return desc
}

func bundleManifest(t *testing.T, store *memory.Store, digestHex string) ocispec.Descriptor {
	t.Helper()

	return pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
		Layers: []ocispec.Descriptor{{
			MediaType: ocispec.MediaTypeImageLayerGzip,
			Digest:    digest.Digest("sha256:" + strings.Repeat(digestHex, 64)),
			Size:      42,
		}},
	})
}

func TestManifestFromDescImageIndex(t *testing.T) {
	store := memory.New()

	bundle := bundleManifest(t, store, "a")

	// An attestation manifest as `docker buildx` attaches it: an "unknown"
	// platform and an in-toto layer rather than a bundle.
	attestation := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
		Layers: []ocispec.Descriptor{{
			MediaType: "application/vnd.in-toto+json",
			Digest:    digest.Digest("sha256:" + strings.Repeat("b", 64)),
			Size:      7,
		}},
	})
	attestation.Platform = &ocispec.Platform{OS: "unknown", Architecture: "unknown"}

	// Same "unknown" platform, but carrying a gzip layer, so the platform check
	// is the only thing that can keep it from being chosen.
	decoy := bundleManifest(t, store, "d")
	decoy.Platform = &ocispec.Platform{OS: "unknown", Architecture: "unknown"}

	noBundle := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
		Layers: []ocispec.Descriptor{{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    digest.Digest("sha256:" + strings.Repeat("c", 64)),
			Size:      7,
		}},
	})

	empty := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
	})

	index := func(mediaType string, entries ...ocispec.Descriptor) ocispec.Descriptor {
		return pushJSON(t, store, mediaType, ocispec.Index{
			MediaType: mediaType,
			Manifests: entries,
		})
	}

	nested := index(ocispec.MediaTypeImageIndex, bundle)
	for range maxIndexDepth {
		nested = index(ocispec.MediaTypeImageIndex, nested)
	}

	tests := []struct {
		note   string
		desc   ocispec.Descriptor
		expErr string
	}{
		{
			note: "image manifest",
			desc: bundle,
		},
		{
			note: "image index with a single manifest",
			desc: index(ocispec.MediaTypeImageIndex, bundle),
		},
		{
			note: "docker manifest list",
			desc: index("application/vnd.docker.distribution.manifest.list.v2+json", bundle),
		},
		{
			note: "image index with buildx attestation manifest",
			desc: index(ocispec.MediaTypeImageIndex, attestation, bundle),
		},
		{
			note: "image index with an unknown-platform entry that has a gzip layer",
			desc: index(ocispec.MediaTypeImageIndex, decoy, bundle),
		},
		{
			note: "image index with the bundle manifest last",
			desc: index(ocispec.MediaTypeImageIndex, noBundle, bundle),
		},
		{
			note: "image index nested in an image index",
			desc: index(ocispec.MediaTypeImageIndex, index(ocispec.MediaTypeImageIndex, bundle)),
		},
		{
			note:   "image manifest without layers",
			desc:   empty,
			expErr: "no layers in manifest",
		},
		{
			note:   "image index without a bundle layer",
			desc:   index(ocispec.MediaTypeImageIndex, noBundle),
			expErr: "contains a application/vnd.oci.image.layer.v1.tar+gzip layer",
		},
		{
			note:   "empty image index",
			desc:   index(ocispec.MediaTypeImageIndex),
			expErr: "no layers in manifest",
		},
		{
			note:   "image index nested beyond the depth limit",
			desc:   nested,
			expErr: "nested more than 4 levels deep",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			manifest, err := manifestFromDesc(t.Context(), store, &tc.desc)

			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q but got manifest %+v", tc.expErr, manifest)
				}
				if !strings.Contains(err.Error(), tc.expErr) {
					t.Fatalf("expected error:\n\n%v\n\nbut got:\n\n%v", tc.expErr, err)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}

			layer, ok := bundleLayer(manifest)
			if !ok {
				t.Fatalf("expected a bundle layer in %+v", manifest)
			}
			if exp := "sha256:" + strings.Repeat("a", 64); layer.Digest.String() != exp {
				t.Fatalf("expected layer digest %v but got %v", exp, layer.Digest)
			}
		})
	}
}

type countingTarget struct {
	oras.Target
	fetches int
}

func (t *countingTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	t.fetches++
	return t.Target.Fetch(ctx, desc)
}

func TestManifestFromDescIndexFanOut(t *testing.T) {
	store := memory.New()

	// Every entry at a level repeats the same digest, so walking the index
	// naively costs fanOut^maxIndexDepth fetches.
	const fanOut = 8

	repeat := func(desc ocispec.Descriptor) ocispec.Descriptor {
		entries := make([]ocispec.Descriptor, fanOut)
		for i := range entries {
			entries[i] = desc
		}
		return pushJSON(t, store, ocispec.MediaTypeImageIndex, ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: entries,
		})
	}

	// no bundle layer anywhere, so the walk can't exit early
	desc := pushJSON(t, store, ocispec.MediaTypeImageManifest, ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig},
		Layers: []ocispec.Descriptor{{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    digest.Digest("sha256:" + strings.Repeat("c", 64)),
			Size:      7,
		}},
	})
	for range maxIndexDepth - 1 {
		desc = repeat(desc)
	}

	target := &countingTarget{Target: store}

	if _, err := manifestFromDesc(t.Context(), target, &desc); err == nil {
		t.Fatal("expected an error")
	}

	// one fetch per distinct digest: the indexes plus the leaf manifest
	if target.fetches > maxIndexDepth {
		t.Fatalf("expected at most %d fetches but got %d", maxIndexDepth, target.fetches)
	}
}

// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package download

import (
	"testing"
	"time"
)

func newSupersedingDownloader(ttlSeconds *int64) *Downloader {
	return &Downloader{config: Config{Polling: PollingConfig{SupersededETagTTLSeconds: ttlSeconds}}}
}

// activate mimics a successful download of a new bundle.
func activate(d *Downloader, etag string) {
	prev := d.etag
	d.etag = etag
	d.supersede(prev, etag)
}

func TestSupersededETags(t *testing.T) {
	t.Parallel()

	ttl := int64(60)
	d := newSupersedingDownloader(&ttl)

	activate(d, `"a"`)
	if exp, act := `"a"`, d.ifNoneMatch(); exp != act {
		t.Fatalf("expected %s, got %s", exp, act)
	}

	activate(d, `"b"`)
	activate(d, `"c"`)
	if exp, act := `"c", "b", "a"`, d.ifNoneMatch(); exp != act {
		t.Fatalf("expected %s, got %s", exp, act)
	}

	// a expires, b and c are kept
	d.superseded[0].at = time.Now().Add(-2 * time.Minute)
	if exp, act := `"c", "b"`, d.ifNoneMatch(); exp != act {
		t.Fatalf("expected %s, got %s", exp, act)
	}

	// rollback to an expired etag makes it current again
	activate(d, `"a"`)
	if exp, act := `"a", "c", "b"`, d.ifNoneMatch(); exp != act {
		t.Fatalf("expected %s, got %s", exp, act)
	}

	// the current etag never appears twice
	activate(d, `"b"`)
	if exp, act := `"b", "a", "c"`, d.ifNoneMatch(); exp != act {
		t.Fatalf("expected %s, got %s", exp, act)
	}
}

func TestSupersededETagsDisabled(t *testing.T) {
	t.Parallel()

	zero := int64(0)
	for _, ttl := range []*int64{nil, &zero} {
		d := newSupersedingDownloader(ttl)
		activate(d, `"a"`)
		activate(d, `"b"`)
		if exp, act := `"b"`, d.ifNoneMatch(); exp != act {
			t.Fatalf("expected %s, got %s", exp, act)
		}
	}
}

func TestSupersededETagsBounded(t *testing.T) {
	t.Parallel()

	ttl := int64(60)
	d := newSupersedingDownloader(&ttl)
	for i := range 3 * maxSupersededETags {
		activate(d, string(rune('a'+i)))
	}
	if exp, act := maxSupersededETags, len(d.superseded); exp != act {
		t.Fatalf("expected %d superseded etags, got %d", exp, act)
	}
}

func TestSupersededETagsNoCurrent(t *testing.T) {
	t.Parallel()

	ttl := int64(60)
	d := newSupersedingDownloader(&ttl)
	activate(d, `"a"`)
	activate(d, `"b"`)

	d.etag = ""
	if act := d.ifNoneMatch(); act != "" {
		t.Fatalf("expected no If-None-Match without a current etag, got %s", act)
	}
}

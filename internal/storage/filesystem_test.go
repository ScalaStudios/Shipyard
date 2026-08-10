package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDigest(t *testing.T) {
	t.Parallel()

	hex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, err := normalizeDigest("sha256:" + hex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hex {
		t.Fatalf("got %q want %q", got, hex)
	}

	if _, err := normalizeDigest("md5:deadbeef"); err == nil {
		t.Fatal("expected error for non-sha256 digest")
	}
}

func TestNewFilesystemStoreRejectsUnwritableRoot(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "blobs")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := NewFilesystemStore(dir); err == nil {
		t.Fatal("expected error for read-only storage root")
	}
}

func TestFilesystemPutGetRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := NewFilesystemStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	payload := []byte("shipyard-phase-0")
	info, err := store.Put(t.Context(), "", bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if info.Size != int64(len(payload)) {
		t.Fatalf("size=%d want=%d", info.Size, len(payload))
	}

	exists, err := store.Exists(t.Context(), info.Digest)
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}

	rc, gotInfo, err := store.Get(t.Context(), info.Digest)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer rc.Close()
	if gotInfo.Digest != info.Digest {
		t.Fatalf("digest mismatch")
	}

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: %q", got)
	}
}

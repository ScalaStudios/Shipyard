package httpapi

import "testing"

func TestParseOCIRest(t *testing.T) {
	t.Parallel()
	kind, name, arg := parseOCIRest("app/api/blobs/sha256:deadbeef")
	if kind != "blob" || name != "app/api" || arg != "sha256:deadbeef" {
		t.Fatalf("blob got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("app/api/blobs/uploads/")
	if kind != "upload-start" || name != "app/api" || arg != "" {
		t.Fatalf("upload-start got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("app/api/blobs/uploads/abc")
	if kind != "upload-complete" || name != "app/api" || arg != "abc" {
		t.Fatalf("upload-complete got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("app/api/manifests/latest")
	if kind != "manifest" || name != "app/api" || arg != "latest" {
		t.Fatalf("manifest got %q %q %q", kind, name, arg)
	}
}

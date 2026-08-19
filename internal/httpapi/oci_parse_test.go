package httpapi

import "testing"

func TestParseOCIRest(t *testing.T) {
	t.Parallel()
	kind, name, arg := parseOCIRest("acme/web/api/blobs/sha256:deadbeef")
	if kind != "blob" || name != "acme/web/api" || arg != "sha256:deadbeef" {
		t.Fatalf("blob got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("acme/web/api/blobs/uploads/")
	if kind != "upload-start" || name != "acme/web/api" || arg != "" {
		t.Fatalf("upload-start got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("acme/web/api/blobs/uploads/abc")
	if kind != "upload-complete" || name != "acme/web/api" || arg != "abc" {
		t.Fatalf("upload-complete got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("acme/web/api/manifests/latest")
	if kind != "manifest" || name != "acme/web/api" || arg != "latest" {
		t.Fatalf("manifest got %q %q %q", kind, name, arg)
	}
	kind, name, arg = parseOCIRest("acme/web/api/tags/list")
	if kind != "tags" || name != "acme/web/api" || arg != "" {
		t.Fatalf("tags got %q %q %q", kind, name, arg)
	}
}

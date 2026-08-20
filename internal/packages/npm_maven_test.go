package packages

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseNPMPublish(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"name":      "demo",
		"dist-tags": map[string]string{"latest": "1.2.3"},
		"versions": map[string]any{
			"1.2.3": map[string]any{"name": "demo", "version": "1.2.3"},
		},
		"_attachments": map[string]any{
			"demo-1.2.3.tgz": map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString([]byte("tarball-bytes")),
				"length":       13,
			},
		},
	}
	raw, _ := json.Marshal(payload)
	got, err := ParseNPMPublish(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "demo" || got.Version != "1.2.3" || string(got.Data) != "tarball-bytes" {
		t.Fatalf("unexpected %#v", got)
	}
	if len(got.Manifest) == 0 {
		t.Fatalf("missing manifest")
	}
}

func TestMavenMetadata(t *testing.T) {
	t.Parallel()
	group, artifact, ok := MavenMetadataCoords("com/example/lib/maven-metadata.xml")
	if !ok || group != "com.example" || artifact != "lib" {
		t.Fatalf("coords %q %q %v", group, artifact, ok)
	}
	xmlBytes, err := BuildMavenMetadata(group, artifact, []string{"1.0.0", "1.1.0"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(xmlBytes)
	if !strings.Contains(s, "<groupId>com.example</groupId>") || !strings.Contains(s, "<version>1.1.0</version>") {
		t.Fatalf("bad metadata: %s", s)
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	if CompareVersions("1.9.0", "1.10.0") >= 0 {
		t.Fatalf("1.9.0 should be < 1.10.0")
	}
	if CompareVersions("1.0-SNAPSHOT", "1.0") >= 0 {
		t.Fatalf("1.0-SNAPSHOT should be < 1.0")
	}
	if CompareVersions("1.0", "1.0.1") >= 0 {
		t.Fatalf("1.0 should be < 1.0.1")
	}
}

func TestBuildMavenMetadataRelease(t *testing.T) {
	t.Parallel()
	xmlBytes, err := BuildMavenMetadata("com.example", "lib", []string{"1.0.0", "1.1.0-SNAPSHOT"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(xmlBytes)
	if !strings.Contains(s, "<release>1.0.0</release>") {
		t.Fatalf("release should be 1.0.0: %s", s)
	}
	if strings.Contains(s, "<release>1.1.0-SNAPSHOT</release>") {
		t.Fatalf("release must exclude SNAPSHOT: %s", s)
	}
}

func TestNPMIntegrity(t *testing.T) {
	t.Parallel()
	got := NPMIntegrity("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	if got != "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=" {
		t.Fatalf("unexpected integrity %q", got)
	}
}

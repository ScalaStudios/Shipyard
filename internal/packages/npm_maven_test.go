package packages

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseNPMPublish(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"name": "demo",
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
}

func TestMavenMetadata(t *testing.T) {
	t.Parallel()
	group, artifact, ok := MavenMetadataCoords("com/example/lib/maven-metadata.xml")
	if !ok || group != "com.example" || artifact != "lib" {
		t.Fatalf("coords %q %q %v", group, artifact, ok)
	}
	xmlBytes, err := BuildMavenMetadata(group, artifact, []string{"1.0.0", "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(xmlBytes)
	if !strings.Contains(s, "<groupId>com.example</groupId>") || !strings.Contains(s, "<version>1.1.0</version>") {
		t.Fatalf("bad metadata: %s", s)
	}
}

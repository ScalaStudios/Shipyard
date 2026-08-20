package httpapi

import (
	"encoding/json"
	"testing"
)

func TestOpenAPIEmbeddedParses(t *testing.T) {
	doc, err := parsedOpenAPI()
	if err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("expected mapping at root, got %T", doc)
	}
	if m["openapi"] == nil {
		t.Fatal("missing openapi key")
	}
	if m["paths"] == nil {
		t.Fatal("missing paths key")
	}
	if _, err := json.Marshal(doc); err != nil {
		t.Fatalf("marshal openapi to json: %v", err)
	}
}

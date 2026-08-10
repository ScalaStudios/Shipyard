package discord

import "testing"

func TestEmbedColors(t *testing.T) {
	t.Parallel()
	s := &Service{publicURL: "http://localhost:5173"}
	ok := s.embed(Event{Kind: "run.succeeded", Title: "ok", Status: "succeeded"})
	if ok["color"].(int) != 0x177A49 {
		t.Fatalf("success color: %#v", ok["color"])
	}
	fail := s.embed(Event{Kind: "run.failed", Title: "bad", Status: "failed", Href: "/pipelines/runs/1"})
	if fail["url"] != "http://localhost:5173/pipelines/runs/1" {
		t.Fatalf("url: %#v", fail["url"])
	}
}

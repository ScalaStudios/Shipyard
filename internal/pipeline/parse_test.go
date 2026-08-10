package pipeline

import "testing"

func TestParseValidDAG(t *testing.T) {
	t.Parallel()
	src := `
pipeline:
  name: demo
jobs:
  test:
    runner:
      os: linux
    steps:
      - name: unit
        run: go test ./...
  build:
    needs: [test]
    steps:
      - run: go build ./...
`
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Jobs) != 2 {
		t.Fatalf("jobs=%d", len(doc.Jobs))
	}
}

func TestParseRejectsCycle(t *testing.T) {
	t.Parallel()
	src := `
jobs:
  a:
    needs: [b]
    steps: [{run: echo a}]
  b:
    needs: [a]
    steps: [{run: echo b}]
`
	if _, err := Parse(src); err == nil {
		t.Fatal("expected cycle error")
	}
}

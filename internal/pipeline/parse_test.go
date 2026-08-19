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

func TestMatchesEmptyOn(t *testing.T) {
	t.Parallel()
	doc := Document{}
	if !doc.Matches("push", "refs/heads/anything") {
		t.Fatal("expected empty on to match")
	}
	if !doc.Matches("pull_request", "feat") {
		t.Fatal("expected empty on to match pull_request")
	}
}

func TestMatchesPushBranches(t *testing.T) {
	t.Parallel()
	doc := Document{On: map[string]any{"push": map[string]any{"branches": []any{"main"}}}}
	if !doc.Matches("push", "refs/heads/main") {
		t.Fatal("expected main to match")
	}
	if doc.Matches("push", "refs/heads/dev") {
		t.Fatal("expected dev not to match")
	}
	if doc.Matches("pull_request", "feat") {
		t.Fatal("expected pull_request not to match")
	}
}

func TestMatchesPushGlob(t *testing.T) {
	t.Parallel()
	doc := Document{On: map[string]any{"push": map[string]any{"branches": []any{"release/*"}}}}
	if !doc.Matches("push", "refs/heads/release/1.2") {
		t.Fatal("expected release/1.2 to match")
	}
	if doc.Matches("push", "refs/heads/main") {
		t.Fatal("expected main not to match")
	}
}

func TestMatchesPullRequestPresence(t *testing.T) {
	t.Parallel()
	doc := Document{On: map[string]any{"pull_request": nil}}
	if !doc.Matches("pull_request", "feat") {
		t.Fatal("expected pull_request to match")
	}
	if doc.Matches("push", "refs/heads/main") {
		t.Fatal("expected push not to match")
	}
}

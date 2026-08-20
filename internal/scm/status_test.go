package scm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostCommitStatusGitHub(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	err := PostCommitStatus(context.Background(), StatusInput{
		Connection: Connection{
			Provider:    ProviderGitHub,
			BaseURL:     srv.URL + "/api/v3",
			RepoOwner:   "o",
			RepoName:    "r",
			AccessToken: "tok",
		},
		GitSHA:      "deadbeef",
		State:       "pending",
		TargetURL:   "https://shipyard.example/runs/1",
		Description: "Run #1 started",
	})
	if err != nil {
		t.Fatalf("PostCommitStatus: %v", err)
	}
	if gotPath != "/api/v3/repos/o/r/statuses/deadbeef" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["state"] != "pending" || gotBody["context"] != "shipyard" ||
		gotBody["target_url"] != "https://shipyard.example/runs/1" || gotBody["description"] != "Run #1 started" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestStatusStateForRun(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"started":   "pending",
		"queued":    "pending",
		"running":   "pending",
		"succeeded": "success",
		"failed":    "failure",
		"canceled":  "error",
		"skipped":   "",
	}
	for in, want := range cases {
		if got := StatusStateForRun(in); got != want {
			t.Fatalf("StatusStateForRun(%q) = %q, want %q", in, got, want)
		}
	}
}

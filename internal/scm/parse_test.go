package scm

import (
	"net/http"
	"testing"
)

func TestParseGitHubPush(t *testing.T) {
	t.Parallel()
	body := []byte(`{"ref":"refs/heads/main","after":"abcdef0123456789abcdef0123456789abcdef01","repository":{"name":"shipyard","owner":{"login":"Shipyard"}}}`)
	ev := ParseEvent("github", "push", body)
	if !ev.ShouldBuild || ev.GitRef != "refs/heads/main" || ev.RepoOwner != "Shipyard" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestParseGitLabMR(t *testing.T) {
	t.Parallel()
	body := []byte(`{"object_kind":"merge_request","project":{"path_with_namespace":"org/repo","name":"repo"},"object_attributes":{"iid":12,"action":"open","source_branch":"feat","title":"hi","last_commit":{"id":"deadbeef"}}}`)
	ev := ParseEvent("gitlab", "Merge Request Hook", body)
	if !ev.ShouldBuild || ev.PRNumber != 12 || ev.RepoOwner != "org" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestVerifyGitLabToken(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("X-Gitlab-Token", "s3cret")
	if !VerifyRequest("gitlab", "s3cret", h, nil) {
		t.Fatal("expected gitlab token ok")
	}
	h.Set("X-Gitlab-Token", "nope")
	if VerifyRequest("gitlab", "s3cret", h, nil) {
		t.Fatal("expected gitlab token fail")
	}
}

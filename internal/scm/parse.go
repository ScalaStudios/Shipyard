package scm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ParsedEvent is a normalized forge webhook event.
type ParsedEvent struct {
	Provider     string
	EventType    string
	DeliveryID   string
	Action       string
	GitRef       string
	GitSHA       string
	PRNumber     int
	RepoOwner    string
	RepoName     string
	Title        string
	ShouldBuild  bool
	IgnoreReason string
}

func VerifyRequest(provider, secret string, headers http.Header, body []byte) bool {
	if secret == "" {
		return true
	}
	family := Family(provider)
	switch family {
	case "github", "gitea":
		sig := headers.Get("X-Hub-Signature-256")
		if sig == "" {
			sig = headers.Get("X-Gitea-Signature")
		}
		if sig == "" {
			sig = headers.Get("X-Forgejo-Signature")
		}
		if VerifyHMACSHA256(secret, sig, body) {
			return true
		}
		// Gitea sometimes sends raw hex without sha256= prefix.
		if sig != "" && !strings.HasPrefix(sig, "sha256=") {
			return VerifyHMACSHA256(secret, "sha256="+sig, body)
		}
		return false
	case "gitlab":
		token := headers.Get("X-Gitlab-Token")
		return token != "" && hmac.Equal([]byte(token), []byte(secret))
	default:
		shared := headers.Get("X-Shipyard-Webhook-Secret")
		if shared != "" && hmac.Equal([]byte(shared), []byte(secret)) {
			return true
		}
		sig := headers.Get("X-Hub-Signature-256")
		return VerifyHMACSHA256(secret, sig, body)
	}
}

func VerifyHMACSHA256(secret, signatureHeader string, body []byte) bool {
	if secret == "" || signatureHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func EventTypeFromHeaders(provider string, headers http.Header) string {
	family := Family(provider)
	switch family {
	case "github":
		if v := headers.Get("X-GitHub-Event"); v != "" {
			return v
		}
	case "gitea":
		if v := headers.Get("X-Gitea-Event"); v != "" {
			return v
		}
		if v := headers.Get("X-Forgejo-Event"); v != "" {
			return v
		}
	case "gitlab":
		if v := headers.Get("X-Gitlab-Event"); v != "" {
			return v
		}
	}
	for _, h := range []string{"X-GitHub-Event", "X-Gitea-Event", "X-Forgejo-Event", "X-Gitlab-Event", "X-Gogs-Event"} {
		if v := headers.Get(h); v != "" {
			return v
		}
	}
	return "unknown"
}

func DeliveryIDFromHeaders(headers http.Header) string {
	for _, h := range []string{"X-GitHub-Delivery", "X-Gitea-Delivery", "X-Forgejo-Delivery", "X-Gitlab-Event-UUID", "X-Request-Id"} {
		if v := headers.Get(h); v != "" {
			return v
		}
	}
	return ""
}

func ParseEvent(provider, eventType string, body []byte) ParsedEvent {
	provider = NormalizeProvider(provider)
	out := ParsedEvent{Provider: provider, EventType: eventType}
	family := Family(provider)

	var raw map[string]any
	_ = json.Unmarshal(body, &raw)

	switch family {
	case "gitlab":
		return parseGitLab(out, raw)
	case "github", "gitea":
		return parseGitHubFamily(out, raw, eventType)
	default:
		// Best-effort: treat like GitHub-family if fields exist.
		if eventType == "unknown" || eventType == "" {
			if _, ok := raw["object_kind"]; ok {
				return parseGitLab(out, raw)
			}
			eventType = stringField(raw, "action")
			out.EventType = eventType
		}
		return parseGitHubFamily(out, raw, eventType)
	}
}

func parseGitHubFamily(out ParsedEvent, raw map[string]any, eventType string) ParsedEvent {
	repo, _ := raw["repository"].(map[string]any)
	out.RepoOwner = stringField(repo, "owner", "login")
	if out.RepoOwner == "" {
		if owner, ok := repo["owner"].(map[string]any); ok {
			out.RepoOwner = stringField(owner, "login")
			if out.RepoOwner == "" {
				out.RepoOwner = stringField(owner, "username")
			}
		}
	}
	out.RepoName = stringField(repo, "name")
	out.Action = stringField(raw, "action")

	switch eventType {
	case "push":
		out.GitRef = stringField(raw, "ref")
		out.GitSHA = stringField(raw, "after")
		out.ShouldBuild = out.GitSHA != "" && out.GitSHA != strings.Repeat("0", 40)
		out.Title = "push " + out.GitRef
	case "pull_request", "PullRequestHook", "pull_request_sync":
		pr, _ := raw["pull_request"].(map[string]any)
		out.PRNumber = intField(pr, "number")
		if out.PRNumber == 0 {
			out.PRNumber = intField(raw, "number")
		}
		out.GitRef = stringField(pr, "head", "ref")
		if head, ok := pr["head"].(map[string]any); ok {
			out.GitRef = stringField(head, "ref")
			out.GitSHA = stringField(head, "sha")
		}
		out.Title = stringField(pr, "title")
		action := out.Action
		out.ShouldBuild = action == "" || action == "opened" || action == "synchronize" || action == "synchronized" || action == "reopened"
		if action == "closed" || action == "labeled" || action == "unlabeled" || action == "assigned" {
			out.ShouldBuild = false
			out.IgnoreReason = "pull_request action " + action
		}
	case "merge_request": // some Gitea variants
		out = parseGitHubFamily(out, raw, "pull_request")
	default:
		out.IgnoreReason = "unsupported event " + eventType
	}
	return out
}

func parseGitLab(out ParsedEvent, raw map[string]any) ParsedEvent {
	kind := stringField(raw, "object_kind")
	if kind == "" {
		kind = stringField(raw, "event_name")
	}
	out.EventType = kind
	project, _ := raw["project"].(map[string]any)
	out.RepoName = stringField(project, "name")
	path := stringField(project, "path_with_namespace")
	if parts := strings.Split(path, "/"); len(parts) >= 2 {
		out.RepoOwner = parts[0]
		out.RepoName = parts[len(parts)-1]
	}
	switch kind {
	case "push":
		out.GitRef = stringField(raw, "ref")
		out.GitSHA = stringField(raw, "checkout_sha")
		if out.GitSHA == "" {
			out.GitSHA = stringField(raw, "after")
		}
		out.ShouldBuild = out.GitSHA != "" && out.GitSHA != strings.Repeat("0", 40)
		out.Title = "push " + out.GitRef
	case "merge_request":
		attrs, _ := raw["object_attributes"].(map[string]any)
		out.PRNumber = intField(attrs, "iid")
		out.Action = stringField(attrs, "action")
		out.GitRef = stringField(attrs, "source_branch")
		out.GitSHA = stringField(attrs, "last_commit", "id")
		if last, ok := attrs["last_commit"].(map[string]any); ok {
			out.GitSHA = stringField(last, "id")
		}
		out.Title = stringField(attrs, "title")
		out.ShouldBuild = out.Action == "open" || out.Action == "update" || out.Action == "reopen"
		if !out.ShouldBuild {
			out.IgnoreReason = "merge_request action " + out.Action
		}
	default:
		out.IgnoreReason = "unsupported gitlab event " + kind
	}
	return out
}

func stringField(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	if len(keys) == 1 {
		v, _ := m[keys[0]].(string)
		return v
	}
	cur := any(m)
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[k]
	}
	s, _ := cur.(string)
	return s
}

func intField(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

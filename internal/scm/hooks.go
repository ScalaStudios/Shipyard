package scm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func RegisterWebhook(ctx context.Context, cred ForgeCredential, owner, repo, hookURL, secret string) error {
	family := Family(cred.Provider)
	switch family {
	case "gitea":
		return registerGiteaHook(ctx, cred, owner, repo, hookURL, secret)
	case "github":
		return registerGitHubHook(ctx, cred, owner, repo, hookURL, secret)
	default:
		return fmt.Errorf("webhook registration not supported for %s", cred.Provider)
	}
}

func registerGiteaHook(ctx context.Context, cred ForgeCredential, owner, repo, hookURL, secret string) error {
	payload := map[string]any{
		"type": "gitea",
		"config": map[string]any{
			"url":          hookURL,
			"content_type": "json",
			"secret":       secret,
		},
		"events": []string{"push", "pull_request", "pull_request_sync"},
		"active": true,
	}
	body, _ := json.Marshal(payload)
	endpoint := forgeAPIBase(cred) + "/repos/" + owner + "/" + repo + "/hooks"
	return postJSON(ctx, endpoint, cred.AccessToken, body)
}

func registerGitHubHook(ctx context.Context, cred ForgeCredential, owner, repo, hookURL, secret string) error {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": []string{"push", "pull_request"},
		"config": map[string]any{
			"url":          hookURL,
			"content_type": "json",
			"secret":       secret,
			"insecure_ssl": "0",
		},
	}
	body, _ := json.Marshal(payload)
	endpoint := forgeAPIBase(cred) + "/repos/" + owner + "/" + repo + "/hooks"
	return postJSON(ctx, endpoint, cred.AccessToken, body)
}

func postJSON(ctx context.Context, endpoint, token string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

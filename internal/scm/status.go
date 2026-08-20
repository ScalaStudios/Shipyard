package scm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type StatusInput struct {
	Connection  Connection
	GitSHA      string
	State       string
	TargetURL   string
	Description string
}

const statusContext = "shipyard"

func githubAPIBase(baseURL string) string {
	api := baseURL
	if strings.Contains(api, "github.com") && !strings.Contains(api, "api.github.com") {
		api = "https://api.github.com"
	}
	return strings.TrimRight(api, "/")
}

func StatusStateForRun(runStatus string) string {
	switch runStatus {
	case "started", "queued", "running":
		return "pending"
	case "succeeded":
		return "success"
	case "failed":
		return "failure"
	case "canceled":
		return "error"
	default:
		return ""
	}
}

func PostCommitStatus(ctx context.Context, in StatusInput) error {
	if in.Connection.AccessToken == "" || in.GitSHA == "" || in.State == "" {
		return nil
	}
	client := &http.Client{Timeout: 12 * time.Second}
	switch Family(in.Connection.Provider) {
	case "github":
		return postGitHubStatus(ctx, client, in)
	case "gitea":
		return postGiteaStatus(ctx, client, in)
	case "gitlab":
		return postGitLabStatus(ctx, client, in)
	default:
		return nil
	}
}

func postGitHubStatus(ctx context.Context, client *http.Client, in StatusInput) error {
	path := fmt.Sprintf("%s/repos/%s/%s/statuses/%s",
		githubAPIBase(in.Connection.BaseURL), url.PathEscape(in.Connection.RepoOwner), url.PathEscape(in.Connection.RepoName), url.PathEscape(in.GitSHA))
	payload, _ := json.Marshal(map[string]string{
		"state":       in.State,
		"target_url":  in.TargetURL,
		"description": in.Description,
		"context":     statusContext,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+in.Connection.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	return sendStatus(client, req, "github status")
}

func postGiteaStatus(ctx context.Context, client *http.Client, in StatusInput) error {
	base := strings.TrimRight(in.Connection.BaseURL, "/")
	path := fmt.Sprintf("%s/api/v1/repos/%s/%s/statuses/%s",
		base, url.PathEscape(in.Connection.RepoOwner), url.PathEscape(in.Connection.RepoName), url.PathEscape(in.GitSHA))
	payload, _ := json.Marshal(map[string]string{
		"state":       in.State,
		"target_url":  in.TargetURL,
		"description": in.Description,
		"context":     statusContext,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+in.Connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	return sendStatus(client, req, "gitea status")
}

func postGitLabStatus(ctx context.Context, client *http.Client, in StatusInput) error {
	base := strings.TrimRight(in.Connection.BaseURL, "/")
	project := url.PathEscape(in.Connection.RepoOwner + "/" + in.Connection.RepoName)
	path := fmt.Sprintf("%s/api/v4/projects/%s/statuses/%s", base, project, url.PathEscape(in.GitSHA))
	state := in.State
	switch state {
	case "failure":
		state = "failed"
	case "error":
		state = "canceled"
	}
	payload, _ := json.Marshal(map[string]string{
		"state":       state,
		"target_url":  in.TargetURL,
		"description": in.Description,
		"name":        statusContext,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", in.Connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	return sendStatus(client, req, "gitlab status")
}

func sendStatus(client *http.Client, req *http.Request, label string) error {
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", label, strings.TrimSpace(string(body)))
	}
	return nil
}

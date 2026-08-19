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

type CommentInput struct {
	Connection Connection
	PRNumber   int
	GitSHA     string
	Body       string
}

type CommentResult struct {
	ID  string
	URL string
}

// PostStatusComment posts a Vercel-style status comment on a PR/MR when possible.
func PostStatusComment(ctx context.Context, in CommentInput) (CommentResult, error) {
	if in.Connection.AccessToken == "" || in.PRNumber <= 0 || strings.TrimSpace(in.Body) == "" {
		return CommentResult{}, nil
	}
	family := Family(in.Connection.Provider)
	client := &http.Client{Timeout: 12 * time.Second}
	switch family {
	case "github":
		return postGitHubPRComment(ctx, client, in)
	case "gitea":
		return postGiteaPRComment(ctx, client, in)
	case "gitlab":
		return postGitLabMRNote(ctx, client, in)
	default:
		return CommentResult{}, nil
	}
}

func postGitHubPRComment(ctx context.Context, client *http.Client, in CommentInput) (CommentResult, error) {
	path := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments",
		githubAPIBase(in.Connection.BaseURL), url.PathEscape(in.Connection.RepoOwner), url.PathEscape(in.Connection.RepoName), in.PRNumber)
	payload, _ := json.Marshal(map[string]string{"body": in.Body})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return CommentResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+in.Connection.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return CommentResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return CommentResult{}, fmt.Errorf("github comment: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		ID  int64  `json:"id"`
		URL string `json:"html_url"`
	}
	_ = json.Unmarshal(body, &parsed)
	return CommentResult{ID: fmt.Sprintf("%d", parsed.ID), URL: parsed.URL}, nil
}

func postGiteaPRComment(ctx context.Context, client *http.Client, in CommentInput) (CommentResult, error) {
	base := strings.TrimRight(in.Connection.BaseURL, "/")
	path := fmt.Sprintf("%s/api/v1/repos/%s/%s/issues/%d/comments",
		base, url.PathEscape(in.Connection.RepoOwner), url.PathEscape(in.Connection.RepoName), in.PRNumber)
	payload, _ := json.Marshal(map[string]string{"body": in.Body})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return CommentResult{}, err
	}
	req.Header.Set("Authorization", "token "+in.Connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return CommentResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return CommentResult{}, fmt.Errorf("gitea comment: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		ID  int64  `json:"id"`
		URL string `json:"html_url"`
	}
	_ = json.Unmarshal(body, &parsed)
	return CommentResult{ID: fmt.Sprintf("%d", parsed.ID), URL: parsed.URL}, nil
}

func postGitLabMRNote(ctx context.Context, client *http.Client, in CommentInput) (CommentResult, error) {
	base := strings.TrimRight(in.Connection.BaseURL, "/")
	project := url.PathEscape(in.Connection.RepoOwner + "/" + in.Connection.RepoName)
	path := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/notes", base, project, in.PRNumber)
	payload, _ := json.Marshal(map[string]string{"body": in.Body})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		return CommentResult{}, err
	}
	req.Header.Set("PRIVATE-TOKEN", in.Connection.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return CommentResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return CommentResult{}, fmt.Errorf("gitlab note: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(body, &parsed)
	return CommentResult{ID: fmt.Sprintf("%d", parsed.ID)}, nil
}

func FormatStartedComment(botName, runURL string, runNumber int64, ref, sha string) string {
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	return fmt.Sprintf("**%s** — build started for `%s` (`%s`).\n\n[View run #%d](%s)\n\n<sub>Shipyard delivery bot</sub>",
		botName, ref, short, runNumber, runURL)
}

func FormatFinishedComment(botName, runURL string, runNumber int64, status, ref string) string {
	icon := "⏳"
	label := status
	switch status {
	case "succeeded":
		icon = "✅"
		label = "built successfully"
	case "failed":
		icon = "❌"
		label = "build failed"
	case "canceled":
		icon = "⚪"
		label = "canceled"
	}
	return fmt.Sprintf("%s **%s** — %s on `%s`.\n\n[View run #%d](%s)\n\n<sub>Shipyard delivery bot</sub>",
		icon, botName, label, ref, runNumber, runURL)
}

package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var forgeClient = &http.Client{Timeout: 30 * time.Second}

type RemoteOrg struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type RemoteRepo struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description,omitempty"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
	HTMLURL       string `json:"html_url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

func (s *Service) ListRemoteOrgs(ctx context.Context, cred ForgeCredential) ([]RemoteOrg, error) {
	family := Family(cred.Provider)
	switch family {
	case "github":
		if cred.Kind == KindGitHubApp {
			return listGitHubAppOrgs(ctx, cred)
		}
		return listGitHubOrgs(ctx, cred)
	case "gitea":
		return listGiteaOrgs(ctx, cred)
	default:
		return nil, fmt.Errorf("remote org browse not supported for provider %s", cred.Provider)
	}
}

func (s *Service) ListRemoteRepos(ctx context.Context, cred ForgeCredential, remoteOrg, query string, page, limit int, includeArchived bool) ([]RemoteRepo, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	family := Family(cred.Provider)
	switch family {
	case "github":
		if cred.Kind == KindGitHubApp {
			return listGitHubAppRepos(ctx, cred, remoteOrg, query, includeArchived)
		}
		return listGitHubRepos(ctx, cred, remoteOrg, query, page, limit, includeArchived)
	case "gitea":
		return listGiteaRepos(ctx, cred, remoteOrg, query, page, limit, includeArchived)
	default:
		return nil, fmt.Errorf("remote repo browse not supported for provider %s", cred.Provider)
	}
}

func forgeAPIBase(cred ForgeCredential) string {
	base := strings.TrimRight(cred.BaseURL, "/")
	family := Family(cred.Provider)
	if family == "github" {
		if base == "" || base == "https://github.com" {
			return "https://api.github.com"
		}
		if strings.HasSuffix(base, "/api/v3") {
			return base
		}
		return base + "/api/v3"
	}
	if strings.HasSuffix(base, "/api/v1") {
		return base
	}
	return base + "/api/v1"
}

func doJSON(ctx context.Context, method, endpoint, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if strings.Contains(endpoint, "api.github.com") || strings.Contains(endpoint, "/api/v3/") {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	resp, err := forgeClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("forge api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func listGiteaOrgs(ctx context.Context, cred ForgeCredential) ([]RemoteOrg, error) {
	var raw []struct {
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := doJSON(ctx, http.MethodGet, forgeAPIBase(cred)+"/user/orgs?limit=50", cred.AccessToken, &raw); err != nil {
		return nil, err
	}
	out := make([]RemoteOrg, 0, len(raw))
	for _, o := range raw {
		name := o.FullName
		if name == "" {
			name = o.Username
		}
		out = append(out, RemoteOrg{Login: o.Username, Name: name, Description: o.Description, AvatarURL: o.AvatarURL})
	}
	return out, nil
}

func listGiteaRepos(ctx context.Context, cred ForgeCredential, org, query string, page, limit int, includeArchived bool) ([]RemoteRepo, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, fmt.Errorf("remote_org required")
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("q", query)
	var raw []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		Archived      bool   `json:"archived"`
		HTMLURL       string `json:"html_url"`
		DefaultBranch string `json:"default_branch"`
		Owner         struct {
			Username string `json:"username"`
			Login    string `json:"login"`
		} `json:"owner"`
	}
	endpoint := forgeAPIBase(cred) + "/orgs/" + url.PathEscape(org) + "/repos?" + q.Encode()
	if err := doJSON(ctx, http.MethodGet, endpoint, cred.AccessToken, &raw); err != nil {
		return nil, err
	}
	out := make([]RemoteRepo, 0, len(raw))
	qlower := strings.ToLower(strings.TrimSpace(query))
	for _, r := range raw {
		if r.Archived && !includeArchived {
			continue
		}
		owner := r.Owner.Username
		if owner == "" {
			owner = r.Owner.Login
		}
		if owner == "" {
			owner = org
		}
		if qlower != "" {
			hay := strings.ToLower(r.Name + " " + r.FullName + " " + r.Description)
			if !strings.Contains(hay, qlower) {
				continue
			}
		}
		out = append(out, RemoteRepo{
			Owner: owner, Name: r.Name, FullName: r.FullName, Description: r.Description,
			Private: r.Private, Archived: r.Archived, HTMLURL: r.HTMLURL, DefaultBranch: r.DefaultBranch,
		})
	}
	return out, nil
}

func listGitHubOrgs(ctx context.Context, cred ForgeCredential) ([]RemoteOrg, error) {
	var raw []struct {
		Login       string `json:"login"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := doJSON(ctx, http.MethodGet, forgeAPIBase(cred)+"/user/orgs?per_page=100", cred.AccessToken, &raw); err != nil {
		return nil, err
	}
	out := make([]RemoteOrg, 0, len(raw))
	for _, o := range raw {
		out = append(out, RemoteOrg{Login: o.Login, Name: o.Login, Description: o.Description, AvatarURL: o.AvatarURL})
	}
	return out, nil
}

func listGitHubRepos(ctx context.Context, cred ForgeCredential, org, query string, page, limit int, includeArchived bool) ([]RemoteRepo, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, fmt.Errorf("remote_org required")
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(limit))
	q.Set("type", "all")
	var raw []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		Archived      bool   `json:"archived"`
		HTMLURL       string `json:"html_url"`
		DefaultBranch string `json:"default_branch"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	endpoint := forgeAPIBase(cred) + "/orgs/" + url.PathEscape(org) + "/repos?" + q.Encode()
	if err := doJSON(ctx, http.MethodGet, endpoint, cred.AccessToken, &raw); err != nil {
		return nil, err
	}
	out := make([]RemoteRepo, 0, len(raw))
	qlower := strings.ToLower(strings.TrimSpace(query))
	for _, r := range raw {
		if r.Archived && !includeArchived {
			continue
		}
		if qlower != "" {
			hay := strings.ToLower(r.Name + " " + r.FullName + " " + r.Description)
			if !strings.Contains(hay, qlower) {
				continue
			}
		}
		owner := r.Owner.Login
		if owner == "" {
			owner = org
		}
		out = append(out, RemoteRepo{
			Owner: owner, Name: r.Name, FullName: r.FullName, Description: r.Description,
			Private: r.Private, Archived: r.Archived, HTMLURL: r.HTMLURL, DefaultBranch: r.DefaultBranch,
		})
	}
	return out, nil
}

type installationRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Archived      bool   `json:"archived"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Owner         struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

func listInstallationRepos(ctx context.Context, cred ForgeCredential) ([]installationRepo, error) {
	var out []installationRepo
	for page := 1; page <= 10; page++ {
		var raw struct {
			TotalCount   int                `json:"total_count"`
			Repositories []installationRepo `json:"repositories"`
		}
		endpoint := fmt.Sprintf("%s/installation/repositories?per_page=100&page=%d", forgeAPIBase(cred), page)
		if err := doJSON(ctx, http.MethodGet, endpoint, cred.AccessToken, &raw); err != nil {
			return nil, err
		}
		out = append(out, raw.Repositories...)
		if len(raw.Repositories) < 100 || len(out) >= raw.TotalCount {
			break
		}
	}
	return out, nil
}

func listGitHubAppOrgs(ctx context.Context, cred ForgeCredential) ([]RemoteOrg, error) {
	repos, err := listInstallationRepos(ctx, cred)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []RemoteOrg{}
	for _, r := range repos {
		login := r.Owner.Login
		if login == "" || seen[login] {
			continue
		}
		seen[login] = true
		out = append(out, RemoteOrg{Login: login, Name: login, AvatarURL: r.Owner.AvatarURL})
	}
	return out, nil
}

func listGitHubAppRepos(ctx context.Context, cred ForgeCredential, org, query string, includeArchived bool) ([]RemoteRepo, error) {
	repos, err := listInstallationRepos(ctx, cred)
	if err != nil {
		return nil, err
	}
	org = strings.ToLower(strings.TrimSpace(org))
	qlower := strings.ToLower(strings.TrimSpace(query))
	out := []RemoteRepo{}
	for _, r := range repos {
		if r.Archived && !includeArchived {
			continue
		}
		if org != "" && strings.ToLower(r.Owner.Login) != org {
			continue
		}
		if qlower != "" {
			hay := strings.ToLower(r.Name + " " + r.FullName + " " + r.Description)
			if !strings.Contains(hay, qlower) {
				continue
			}
		}
		out = append(out, RemoteRepo{
			Owner:         r.Owner.Login,
			Name:          r.Name,
			FullName:      r.FullName,
			Description:   r.Description,
			Private:       r.Private,
			Archived:      r.Archived,
			HTMLURL:       r.HTMLURL,
			DefaultBranch: r.DefaultBranch,
		})
	}
	return out, nil
}

package scm

import (
	"strings"
)

// Provider identifiers supported for forge connections and inbound webhooks.
const (
	ProviderGitHub    = "github"
	ProviderGitea     = "gitea"
	ProviderForgejo   = "forgejo"
	ProviderGitLab    = "gitlab"
	ProviderGogs      = "gogs"
	ProviderOneDev    = "onedev"
	ProviderGitBucket = "gitbucket"
	ProviderCodebase  = "codebase"
	ProviderPagure    = "pagure"
	ProviderCodeberg  = "codeberg"
	ProviderGeneric   = "generic"
)

type ProviderInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	DefaultURL  string `json:"default_url"`
	Family      string `json:"family"` // github | gitea | gitlab | generic
	Description string `json:"description"`
}

var Catalog = []ProviderInfo{
	{ID: ProviderGitHub, Label: "GitHub", DefaultURL: "https://api.github.com", Family: "github", Description: "GitHub.com or GitHub Enterprise"},
	{ID: ProviderGitea, Label: "Gitea", DefaultURL: "", Family: "gitea", Description: "Self-hosted Gitea"},
	{ID: ProviderForgejo, Label: "Forgejo", DefaultURL: "https://git.lunarlabs.dev", Family: "gitea", Description: "Forgejo / Codeberg-compatible"},
	{ID: ProviderGitLab, Label: "GitLab", DefaultURL: "https://gitlab.com", Family: "gitlab", Description: "GitLab.com or self-hosted"},
	{ID: ProviderGogs, Label: "Gogs", DefaultURL: "", Family: "gitea", Description: "Gogs (Gitea-like API)"},
	{ID: ProviderOneDev, Label: "OneDev", DefaultURL: "", Family: "generic", Description: "OneDev (webhook + generic status)"},
	{ID: ProviderGitBucket, Label: "GitBucket", DefaultURL: "", Family: "generic", Description: "GitBucket"},
	{ID: ProviderCodebase, Label: "Codebase", DefaultURL: "https://api3.codebasehq.com", Family: "generic", Description: "Codebase HQ"},
	{ID: ProviderPagure, Label: "Pagure", DefaultURL: "", Family: "generic", Description: "Pagure"},
	{ID: ProviderCodeberg, Label: "Codeberg", DefaultURL: "https://codeberg.org", Family: "gitea", Description: "Codeberg (Forgejo)"},
	{ID: ProviderGeneric, Label: "Generic Git", DefaultURL: "", Family: "generic", Description: "Any git forge via signed webhook"},
}

func NormalizeProvider(raw string) string {
	p := strings.ToLower(strings.TrimSpace(raw))
	switch p {
	case "gh":
		return ProviderGitHub
	case "gl":
		return ProviderGitLab
	default:
		return p
	}
}

func ValidProvider(p string) bool {
	p = NormalizeProvider(p)
	for _, info := range Catalog {
		if info.ID == p {
			return true
		}
	}
	return false
}

func Family(provider string) string {
	provider = NormalizeProvider(provider)
	for _, info := range Catalog {
		if info.ID == provider {
			return info.Family
		}
	}
	return "generic"
}

func DefaultBaseURL(provider string) string {
	provider = NormalizeProvider(provider)
	for _, info := range Catalog {
		if info.ID == provider {
			return info.DefaultURL
		}
	}
	return ""
}

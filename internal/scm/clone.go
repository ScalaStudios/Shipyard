package scm

import "strings"

func CloneURL(c Connection) string {
	if c.RepoOwner == "" || c.RepoName == "" {
		return ""
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if Family(c.Provider) == "github" {
		if base == "" || strings.Contains(base, "api.github.com") {
			base = "https://github.com"
		} else {
			base = strings.TrimSuffix(base, "/api/v3")
		}
	} else {
		base = strings.TrimSuffix(base, "/api/v1")
		base = strings.TrimSuffix(base, "/api/v4")
	}
	return base + "/" + c.RepoOwner + "/" + c.RepoName + ".git"
}

func GitUsername(provider string) string {
	if Family(provider) == "github" {
		return "x-access-token"
	}
	return "oauth2"
}

package scm

import "testing"

func TestCloneURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		conn     Connection
		expected string
	}{
		{
			name:     "github api base",
			conn:     Connection{Provider: ProviderGitHub, BaseURL: "https://api.github.com", RepoOwner: "o", RepoName: "r"},
			expected: "https://github.com/o/r.git",
		},
		{
			name:     "github enterprise",
			conn:     Connection{Provider: ProviderGitHub, BaseURL: "https://ghe.example/api/v3", RepoOwner: "o", RepoName: "r"},
			expected: "https://ghe.example/o/r.git",
		},
		{
			name:     "gitea",
			conn:     Connection{Provider: ProviderGitea, BaseURL: "https://gitea.example", RepoOwner: "o", RepoName: "r"},
			expected: "https://gitea.example/o/r.git",
		},
		{
			name:     "gitlab api base",
			conn:     Connection{Provider: ProviderGitLab, BaseURL: "https://gitlab.example/api/v4", RepoOwner: "o", RepoName: "r"},
			expected: "https://gitlab.example/o/r.git",
		},
		{
			name:     "missing repo",
			conn:     Connection{Provider: ProviderGitHub, BaseURL: "https://api.github.com", RepoOwner: "o"},
			expected: "",
		},
	}
	for _, tc := range cases {
		if got := CloneURL(tc.conn); got != tc.expected {
			t.Fatalf("%s: CloneURL = %q, want %q", tc.name, got, tc.expected)
		}
	}
}

func TestGitUsername(t *testing.T) {
	t.Parallel()
	if got := GitUsername(ProviderGitHub); got != "x-access-token" {
		t.Fatalf("github username = %q", got)
	}
	if got := GitUsername(ProviderGitea); got != "oauth2" {
		t.Fatalf("gitea username = %q", got)
	}
}

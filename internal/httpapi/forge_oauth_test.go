package httpapi

import (
	"regexp"
	"testing"
)

func TestForgeCredentialName(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

	cases := []struct {
		provider string
		username string
		email    string
		want     string
	}{
		{"forgejo", "Luna", "", "forgejo-luna"},
		{"github", "ohemilyy", "", "github-ohemilyy"},
		{"forgejo", "some.user_name", "", "forgejo-some-user-name"},
		{"github", "", "dev@example.com", "github-dev"},
		{"forgejo", "", "", "forgejo-user"},
		{"github", "--weird--", "", "github-weird"},
	}
	for _, tc := range cases {
		got := forgeCredentialName(tc.provider, tc.username, tc.email)
		if got != tc.want {
			t.Errorf("forgeCredentialName(%q,%q,%q) = %q, want %q", tc.provider, tc.username, tc.email, got, tc.want)
		}
		if !valid.MatchString(got) {
			t.Errorf("name %q violates forge_credentials_name_format", got)
		}
	}

	long := forgeCredentialName("forgejo", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "")
	if !valid.MatchString(long) {
		t.Errorf("long name %q violates forge_credentials_name_format", long)
	}
}

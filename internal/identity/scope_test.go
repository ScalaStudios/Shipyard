package identity

import "testing"

func TestScopeAllows(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		req    string
		want   bool
	}{
		{"empty allows read", nil, ScopeRegistryRead, true},
		{"empty allows write", []string{}, ScopeRegistryWrite, true},
		{"read allows read", []string{ScopeRegistryRead}, ScopeRegistryRead, true},
		{"read denies write", []string{ScopeRegistryRead}, ScopeRegistryWrite, false},
		{"write allows read", []string{ScopeRegistryWrite}, ScopeRegistryRead, true},
		{"write allows write", []string{ScopeRegistryWrite}, ScopeRegistryWrite, true},
		{"unrelated denies read", []string{"other"}, ScopeRegistryRead, false},
		{"unrelated denies write", []string{"other"}, ScopeRegistryWrite, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ScopeAllows(c.scopes, c.req); got != c.want {
				t.Fatalf("ScopeAllows(%v, %q) = %v, want %v", c.scopes, c.req, got, c.want)
			}
		})
	}
}

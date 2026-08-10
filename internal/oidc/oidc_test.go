package oidc

import "testing"

func TestNormalizeGitHub(t *testing.T) {
	t.Parallel()
	p := Normalize(ProviderConfig{Name: "github", ClientID: "abc"})
	if p.Kind != KindGitHub {
		t.Fatalf("kind %s", p.Kind)
	}
	if p.Issuer != "https://github.com" {
		t.Fatalf("issuer %s", p.Issuer)
	}
}

func TestDetectForgejo(t *testing.T) {
	t.Parallel()
	p := Normalize(ProviderConfig{
		Name:     "company",
		Issuer:   "https://git.example.com",
		ClientID: "x",
		Kind:     KindForgejo,
	})
	if authEndpoint(p) != "https://git.example.com/login/oauth/authorize" {
		t.Fatalf("auth %s", authEndpoint(p))
	}
}

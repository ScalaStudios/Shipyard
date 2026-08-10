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

func TestNormalizeEntraAndDiscord(t *testing.T) {
	t.Parallel()
	entra := Normalize(ProviderConfig{Name: "entra", ClientID: "abc", Kind: KindEntra})
	if entra.Kind != KindEntra {
		t.Fatalf("entra kind %s", entra.Kind)
	}
	wantAuth := "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	if authEndpoint(entra) != wantAuth {
		t.Fatalf("entra auth %s", authEndpoint(entra))
	}
	discord := Normalize(ProviderConfig{Name: "discord", ClientID: "xyz"})
	if discord.Kind != KindDiscord {
		t.Fatalf("discord kind %s", discord.Kind)
	}
	if authEndpoint(discord) != "https://discord.com/api/oauth2/authorize" {
		t.Fatalf("discord auth %s", authEndpoint(discord))
	}
}

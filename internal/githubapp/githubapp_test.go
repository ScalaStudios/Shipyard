package githubapp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func testKeyPEM(t *testing.T, pkcs8 bool) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var block *pem.Block
	if pkcs8 {
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatalf("marshal pkcs8: %v", err)
		}
		block = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	} else {
		block = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	}
	return string(pem.EncodeToMemory(block)), key
}

func TestAppJWTIsVerifiableAndWellFormed(t *testing.T) {
	for _, pkcs8 := range []bool{false, true} {
		keyPEM, key := testKeyPEM(t, pkcs8)
		now := time.Unix(1_700_000_000, 0).UTC()
		token, err := AppJWT(Config{AppID: "12345", PrivateKeyPEM: keyPEM}, now)
		if err != nil {
			t.Fatalf("pkcs8=%v AppJWT: %v", pkcs8, err)
		}

		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("expected 3 JWT segments, got %d", len(parts))
		}

		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			t.Fatalf("decode signature: %v", err)
		}
		if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
			t.Fatalf("pkcs8=%v signature does not verify: %v", pkcs8, err)
		}

		var header map[string]string
		raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
		_ = json.Unmarshal(raw, &header)
		if header["alg"] != "RS256" {
			t.Fatalf("alg = %q, want RS256", header["alg"])
		}

		var claims map[string]any
		raw, _ = base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(raw, &claims)
		if claims["iss"] != "12345" {
			t.Fatalf("iss = %v, want 12345", claims["iss"])
		}
		iat := int64(claims["iat"].(float64))
		exp := int64(claims["exp"].(float64))
		if iat > now.Unix() {
			t.Fatalf("iat %d is in the future relative to %d; github rejects that", iat, now.Unix())
		}
		if exp-now.Unix() > 600 {
			t.Fatalf("exp is %d seconds out; github rejects anything over 10 minutes", exp-now.Unix())
		}
	}
}

func TestAppJWTRejectsBadConfig(t *testing.T) {
	if _, err := AppJWT(Config{AppID: "1"}, time.Now()); err == nil {
		t.Fatal("expected error when private key is missing")
	}
	if _, err := AppJWT(Config{AppID: "1", PrivateKeyPEM: "not a pem"}, time.Now()); err == nil {
		t.Fatal("expected error for malformed PEM")
	}
}

func TestInstallURL(t *testing.T) {
	if got := InstallURL(Config{Slug: "shipyard-ci"}, "abc"); got != "https://github.com/apps/shipyard-ci/installations/new?state=abc" {
		t.Fatalf("InstallURL = %q", got)
	}
	if got := InstallURL(Config{}, "abc"); got != "" {
		t.Fatalf("InstallURL without slug = %q, want empty", got)
	}
	got := InstallURL(Config{Slug: "sy", APIBaseURL: "https://ghe.example.com/api/v3"}, "")
	if got != "https://ghe.example.com/apps/sy/installations/new" {
		t.Fatalf("enterprise InstallURL = %q", got)
	}
}

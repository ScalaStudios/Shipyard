package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	return &Box{gcm: gcm}
}

func TestSealOpenRoundTrip(t *testing.T) {
	b := testBox(t)

	sealed, err := b.SealString("ghp_supersecret")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !strings.HasPrefix(sealed, sealPrefix) {
		t.Fatalf("sealed value missing prefix: %q", sealed)
	}
	if strings.Contains(sealed, "ghp_supersecret") {
		t.Fatal("plaintext leaked into sealed value")
	}

	got, err := b.OpenString(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != "ghp_supersecret" {
		t.Fatalf("round trip = %q", got)
	}
}

func TestSealIsRandomized(t *testing.T) {
	b := testBox(t)
	a, _ := b.SealString("same")
	c, _ := b.SealString("same")
	if a == c {
		t.Fatal("two seals of the same value are identical; nonce is not random")
	}
}

func TestOpenPassesThroughLegacyPlaintext(t *testing.T) {
	b := testBox(t)
	got, err := b.OpenString("legacy-plaintext-token")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got != "legacy-plaintext-token" {
		t.Fatalf("legacy passthrough = %q", got)
	}
}

func TestOpenRejectsTamperedCiphertext(t *testing.T) {
	b := testBox(t)
	sealed, _ := b.SealString("value")
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, sealPrefix))
	raw[len(raw)-1] ^= 0xff
	if _, err := b.OpenString(sealPrefix + base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("expected tampered ciphertext to fail authentication")
	}
}

func TestNilBoxRefusesToSeal(t *testing.T) {
	var b *Box
	if _, err := b.SealString("value"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil box seal error = %v, want ErrNotConfigured", err)
	}
	if got, err := b.OpenString("plain"); err != nil || got != "plain" {
		t.Fatalf("nil box should pass through plaintext, got %q %v", got, err)
	}
	if _, err := b.OpenString(sealPrefix + "abc"); !errors.Is(err, ErrNotConfigured) {
		t.Fatal("nil box should refuse to open sealed values")
	}
	if got, err := b.SealString(""); err != nil || got != "" {
		t.Fatalf("empty value should stay empty, got %q %v", got, err)
	}
}

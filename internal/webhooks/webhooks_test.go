package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	t.Parallel()
	body := []byte(`{"zen":"shipyard"}`)
	secret := "topsecret"
	if VerifySignature(secret, "sha256=deadbeef", body) {
		t.Fatal("expected mismatch")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature(secret, sig, body) {
		t.Fatal("expected valid signature")
	}
}

package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword(hash, "correct-horse-battery")
	if err != nil || !ok {
		t.Fatalf("verify good password: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword(hash, "wrong-password")
	if err != nil {
		t.Fatalf("verify bad password err: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch")
	}
}

func TestHashPasswordRejectsShort(t *testing.T) {
	t.Parallel()
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error")
	}
}

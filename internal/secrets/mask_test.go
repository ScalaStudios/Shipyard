package secrets

import "testing"

func TestMaskLine(t *testing.T) {
	t.Parallel()
	got := MaskLine("token=super-secret-value and again super-secret-value", []string{"super-secret-value"})
	want := "token=*** and again ***"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEnvName(t *testing.T) {
	t.Parallel()
	if got := EnvName("db-password"); got != "DB_PASSWORD" {
		t.Fatalf("got %q", got)
	}
	if got := EnvName("1secret"); got != "S_1SECRET" {
		t.Fatalf("got %q", got)
	}
}

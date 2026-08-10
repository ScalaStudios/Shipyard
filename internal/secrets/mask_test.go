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

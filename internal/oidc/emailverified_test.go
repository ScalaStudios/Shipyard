package oidc

import "testing"

func TestClaimBool(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"false", false},
		{"", false},
		{nil, false},
		{float64(1), false},
	}
	for _, c := range cases {
		if got := claimBool(c.in); got != c.want {
			t.Fatalf("claimBool(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

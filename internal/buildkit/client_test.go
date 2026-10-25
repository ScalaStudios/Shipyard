package buildkit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPassesAddrAndTLSDir(t *testing.T) {
	t.Parallel()
	fake := filepath.Join(t.TempDir(), "buildctl")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tlsDir string
		want   string
	}{
		{"", "--addr tcp://buildkitd:1234 build"},
		{"/certs/buildkit", "--addr tcp://buildkitd:1234 --tlsdir /certs/buildkit build"},
	}
	for _, c := range cases {
		client := &Client{Buildctl: fake, Addr: "tcp://buildkitd:1234", TLSDir: c.tlsDir}
		var out []string
		if err := client.Build(context.Background(), BuildRequest{}, func(_, line string) { out = append(out, line) }); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(out, "\n"); !strings.HasPrefix(got, c.want) {
			t.Fatalf("tlsDir %q: got %q want prefix %q", c.tlsDir, got, c.want)
		}
	}
}

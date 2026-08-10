package main

import (
	"runtime"
	"slices"
	"testing"
)

func TestRunnerLabels(t *testing.T) {
	osLabel := "os:" + runtime.GOOS

	got := runnerLabels("linux, GPU ,")
	want := []string{"linux", "gpu", osLabel}
	if !slices.Equal(got, want) {
		t.Fatalf("runnerLabels = %v, want %v", got, want)
	}

	got = runnerLabels("linux," + osLabel)
	want = []string{"linux", osLabel}
	if !slices.Equal(got, want) {
		t.Fatalf("runnerLabels deduped = %v, want %v", got, want)
	}
}

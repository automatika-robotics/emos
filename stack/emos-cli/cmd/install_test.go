package cmd

import (
	"slices"
	"testing"
)

func TestPixiCloneArgsFollowTheRefTheBinaryWasBuiltFrom(t *testing.T) {
	url, dir := "https://github.com/automatika-robotics/emos.git", "/home/robot/.local/share/emos"

	// A release binary carries no ref and clones the default branch.
	want := []string{"clone", "--depth", "1", url, dir}
	if got := pixiCloneArgs("", url, dir); !slices.Equal(got, want) {
		t.Errorf("no ref: %v, want %v", got, want)
	}
	// A development build installs the branch it was built from.
	want = []string{"clone", "--depth", "1", "--branch", "release/0.8.0", url, dir}
	if got := pixiCloneArgs("release/0.8.0", url, dir); !slices.Equal(got, want) {
		t.Errorf("with a ref: %v, want %v", got, want)
	}
}

// Package testhome runs a package's tests under a throwaway $HOME so global
// files such as ~/.imprint/embed.yaml on the developer machine cannot leak in.
package testhome

import (
	"fmt"
	"os"
	"testing"
)

// Run points HOME (and USERPROFILE) at a fresh temp dir, runs m, and cleans up.
// Use from TestMain: func TestMain(m *testing.M) { os.Exit(testhome.Run(m)) }
func Run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "imprint-testhome-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testhome: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	return m.Run()
}

package cli

import (
	"strings"
	"testing"
)

func TestDownHelp(t *testing.T) {
	app := &App{Stdout: discardWriter{}}
	if code := app.Run([]string{"help", "down"}); code != 0 {
		t.Fatalf("code %d", code)
	}
}

func TestDownUsageExtraArgs(t *testing.T) {
	var buf strings.Builder
	app := &App{Stdout: &buf, Stderr: &buf}
	if code := app.Run([]string{"down", "extra"}); code != 2 {
		t.Fatalf("code %d", code)
	}
}

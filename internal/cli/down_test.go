package cli

import (
	"strings"
	"testing"
)

func TestUpDownHelp(t *testing.T) {
	app := &App{Stdout: discardWriter{}}
	if code := app.Run([]string{"help", "up"}); code != 0 {
		t.Fatalf("help up code %d", code)
	}
	if code := app.Run([]string{"help", "down"}); code != 0 {
		t.Fatalf("help down code %d", code)
	}
}

func TestDownStopsWithoutConfig(t *testing.T) {
	var buf strings.Builder
	app := &App{
		Stdout:  &buf,
		Stderr:  &buf,
		Getwd:   func() (string, error) { return t.TempDir(), nil },
		Environ: func(string) string { return "" },
		Home:    func() (string, error) { return t.TempDir(), nil },
	}
	if code := app.Run([]string{"down"}); code != 0 {
		t.Fatalf("down code %d stderr=%s", code, buf.String())
	}
}

func TestHostStartHelp(t *testing.T) {
	app := &App{Stdout: discardWriter{}}
	if code := app.Run([]string{"help", "host"}); code != 0 {
		t.Fatalf("code %d", code)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

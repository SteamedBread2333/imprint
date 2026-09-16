package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/shelves"
)

func TestPrintShelvesBlockEmptyShowsRebuildHint(t *testing.T) {
	var buf bytes.Buffer
	c := newConsole(&buf)
	printShelvesBlock(c, shelves.Status{Enabled: true}, []string{"docs"}, true)
	out := buf.String()
	if !strings.Contains(out, "empty") {
		t.Fatalf("expected empty badge, got:\n%s", out)
	}
	if !strings.Contains(out, "imprint down && imprint up") {
		t.Fatalf("expected rebuild command, got:\n%s", out)
	}
}

func TestPrintHostBlockStoppedShowsStartHint(t *testing.T) {
	var buf bytes.Buffer
	c := newConsole(&buf)
	printHostBlock(c, false, ":9470", 0)
	out := buf.String()
	if !strings.Contains(out, "stopped") {
		t.Fatalf("expected stopped badge, got:\n%s", out)
	}
	if !strings.Contains(out, "imprint up") {
		t.Fatalf("expected start command, got:\n%s", out)
	}
}

func TestStatusHintsSkipsWhenShelvesEmpty(t *testing.T) {
	report := statusReport{
		Host: runtimeProbe{Listening: true},
		Shelves: shelves.Status{
			Enabled: true,
			Indexed: false,
		},
	}
	if why, _ := statusHints(report, false); why != "" {
		t.Fatalf("expected no duplicate hint, got %q", why)
	}
}

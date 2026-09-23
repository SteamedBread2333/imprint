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

// SimilarAdvisory stays quiet when the list is empty — a clean write must
// not print a stub header.
func TestSimilarAdvisoryEmptyProducesNoOutput(t *testing.T) {
	var buf bytes.Buffer
	c := newConsole(&buf)
	c.SimilarAdvisory(nil)
	if buf.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buf.String())
	}
}

// SimilarAdvisory renders each advisory rule with id, score, and claim,
// plus a follow-up hint pointing at reinforce / supersede.
func TestSimilarAdvisoryRendersIdScoreAndHint(t *testing.T) {
	var buf bytes.Buffer
	c := newConsole(&buf)
	c.SimilarAdvisory([]similarItem{
		{ID: "R-001", Score: 0.94, Claim: "Use tabs for indentation"},
		{ID: "R-002", Score: 0.81, Claim: "Prefer tabs over spaces"},
	})
	out := buf.String()
	for _, want := range []string{"R-001", "0.94", "Use tabs for indentation", "R-002", "0.81", "reinforce", "supersede"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

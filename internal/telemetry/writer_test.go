package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestWriterDailyFilesAndRetention(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	old := filepath.Join(dir, "2026-08-01.jsonl")
	if err := os.WriteFile(old, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := New(dir, 30, func() time.Time { return now }, nil)
	if err := w.Record(imprint.TelemetryEvent{
		Op: "find", ScopeCount: 2, QueryTermCount: 3, RuleHitCount: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old telemetry was not removed")
	}
	data, err := os.ReadFile(filepath.Join(dir, "2026-09-21.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"op":"find"`) || strings.Contains(text, "claim") || strings.Contains(text, "evidence") {
		t.Fatalf("unsafe telemetry: %s", text)
	}
}

func TestSummarize(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	w := New(dir, 30, func() time.Time { return now }, nil)
	_ = w.Record(imprint.TelemetryEvent{Op: "find", LatencyMS: 10})
	_ = w.Record(imprint.TelemetryEvent{Op: "write_rejected", Code: "duplicate", LatencyMS: 20})
	got, err := Summarize(dir, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got.Events != 2 || got.ByCode["duplicate"] != 1 || got.AvgLatency != 15 {
		t.Fatalf("summary = %+v", got)
	}
}

package migrate_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/migrate"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestImportShardsDryRun(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "imprint-0001.md")
	if err := os.WriteFile(shard, []byte(`---
id: r-2026-09-11-001
claim: legacy rule
scope: [demo]
confidence: 0.6
status: active
reinforcement_count: 0
created_at: 2026-09-11T12:00:00Z
updated_at: 2026-09-11T12:00:00Z
last_touched_at: 2026-09-11T12:00:00Z
supersedes: []
related: []
conflicts_with: []
evidence_log:
  - at: 2026-09-11T12:00:00Z
    kind: original
    text: legacy
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := imprint.OpenWithNow(dir, func() time.Time {
		return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := migrate.ImportShards(v, true)
	if err != nil || res.FromMarkdown != 1 || res.Total != 1 || res.Imported {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	list, err := v.List("active", 0)
	if err != nil || len(list) != 0 {
		t.Fatalf("dry-run should not write: %+v err=%v", list, err)
	}
}

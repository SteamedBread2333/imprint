package legacy_test

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/vault/legacy"
	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

func TestParseShard(t *testing.T) {
	raw := `---
id: r-2026-09-11-001
claim: use snake_case
scope: [python, naming]
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
    text: snake_case please
---
`
	recs, err := legacy.ParseShard([]byte(raw))
	if err != nil || len(recs) != 1 {
		t.Fatalf("parse: %+v err=%v", recs, err)
	}
	if recs[0].ID != "r-2026-09-11-001" || recs[0].Status != model.StatusActive {
		t.Fatalf("rec = %+v", recs[0])
	}
}

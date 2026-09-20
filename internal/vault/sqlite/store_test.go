package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/sqlite"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func testStore(t *testing.T) (*sqlite.Store, func()) {
	t.Helper()
	dir := t.TempDir()
	fixed := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	st, err := sqlite.Open(dir, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	return st, func() { _ = st.Close() }
}

func TestPutGetEvidence(t *testing.T) {
	st, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	rec := &imprint.Record{
		ID:         "r-2026-09-18-001",
		Claim:      "use snake_case",
		Scope:      []string{"python", "naming"},
		Confidence: 0.6,
		Status:     imprint.StatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastTouchedAt: now,
		EvidenceLog: []imprint.Evidence{{
			At: now, Kind: imprint.EvidenceOriginal, Text: "snake_case please",
		}},
	}
	if err := st.PutRecord(rec); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendEvidence(rec.ID, imprint.Evidence{
		At: now.Add(time.Minute), Kind: imprint.EvidenceReinforce, Text: "still yes",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRecord(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.EvidenceLog) != 2 {
		t.Fatalf("evidence = %d", len(got.EvidenceLog))
	}
}

func TestScopeANDCandidates(t *testing.T) {
	st, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	for _, r := range []*imprint.Record{
		{ID: "r-2026-09-18-001", Claim: "a", Scope: []string{"go", "naming"}, Status: imprint.StatusActive, Confidence: 0.8, CreatedAt: now, UpdatedAt: now, LastTouchedAt: now},
		{ID: "r-2026-09-18-002", Claim: "b", Scope: []string{"go"}, Status: imprint.StatusActive, Confidence: 0.8, CreatedAt: now, UpdatedAt: now, LastTouchedAt: now},
	} {
		if err := st.PutRecord(r); err != nil {
			t.Fatal(err)
		}
	}
	cands, err := st.ActiveCandidates([]string{"go", "naming"}, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].ID != "r-2026-09-18-001" {
		t.Fatalf("candidates = %+v", cands)
	}
}

func TestRulesReferencingDoc(t *testing.T) {
	st, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	rec := &imprint.Record{
		ID: "r-2026-09-18-001", Claim: "follow style", Scope: []string{"docs"},
		Status: imprint.StatusActive, Confidence: 0.7,
		CreatedAt: now, UpdatedAt: now, LastTouchedAt: now,
		Sources: []imprint.DocRef{{Path: "docs/style.md", Heading: "Naming"}},
	}
	if err := st.PutRecord(rec); err != nil {
		t.Fatal(err)
	}
	refs, err := st.RulesReferencingDoc("docs/style.md", "Naming", "")
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs = %+v err=%v", refs, err)
	}
	byChunk, err := st.RulesReferencingDoc("docs/style.md", "Naming", "c-naming")
	if err != nil || len(byChunk) != 1 {
		t.Fatalf("byChunk = %+v err=%v", byChunk, err)
	}
	numbered, err := st.RulesReferencingDoc("docs/style.md", "1. Naming", "")
	if err != nil || len(numbered) != 1 {
		t.Fatalf("numbered heading = %+v err=%v", numbered, err)
	}
}

func TestQueryLocalRoundTrip(t *testing.T) {
	st, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	rec := &imprint.Record{
		ID: "r-2026-09-18-001", Claim: "Avoid disclaimer stacking", Scope: []string{"docs"},
		QueryLocal: "否定式堆砌 文档写作", Status: imprint.StatusActive, Confidence: 0.85,
		CreatedAt: now, UpdatedAt: now, LastTouchedAt: now,
	}
	if err := st.PutRecord(rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRecord(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.QueryLocal != rec.QueryLocal {
		t.Fatalf("query_local = %q want %q", got.QueryLocal, rec.QueryLocal)
	}
}

func TestImportAndDBPath(t *testing.T) {
	st, cleanup := testStore(t)
	defer cleanup()
	dir := st.Dir
	if filepath.Base(sqlite.DBPath(dir)) != "vault.db" {
		t.Fatal(sqlite.DBPath(dir))
	}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	if err := st.ImportRecords([]*imprint.Record{
		{ID: "r-2026-09-18-001", Claim: "x", Scope: []string{"a"}, Status: imprint.StatusActive, Confidence: 0.6, CreatedAt: now, UpdatedAt: now, LastTouchedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	all, err := st.AllRecords(true)
	if err != nil || len(all) != 1 {
		t.Fatalf("all = %d err=%v", len(all), err)
	}
}

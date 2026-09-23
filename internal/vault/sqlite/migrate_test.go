package sqlite_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
	"github.com/SteamedBread2333/imprint/internal/vault/sqlite"
	_ "modernc.org/sqlite"
)

// legacyV1Schema mirrors a v1.2.1 vault: rules has last_touched_at and no
// query_local; rule_stats / rule_vectors do not exist yet.
const legacyV1Schema = `
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE rules (
  id                  TEXT PRIMARY KEY,
  claim               TEXT NOT NULL,
  body                TEXT NOT NULL DEFAULT '',
  status              TEXT NOT NULL,
  confidence          REAL NOT NULL,
  reinforcement_count INTEGER NOT NULL DEFAULT 0,
  created_at          TEXT NOT NULL,
  updated_at          TEXT NOT NULL,
  last_touched_at     TEXT NOT NULL DEFAULT ''
);
CREATE TABLE rule_scopes (rule_id TEXT NOT NULL, tag TEXT NOT NULL, PRIMARY KEY (rule_id, tag));
CREATE TABLE evidence_events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  rule_id TEXT NOT NULL, at TEXT NOT NULL, kind TEXT NOT NULL, text TEXT NOT NULL DEFAULT ''
);
CREATE TABLE rule_edges (
  from_id TEXT NOT NULL, to_id TEXT NOT NULL, kind TEXT NOT NULL, PRIMARY KEY (from_id, to_id, kind)
);
CREATE TABLE rule_sources (
  rule_id TEXT NOT NULL, path TEXT NOT NULL DEFAULT '', heading TEXT NOT NULL DEFAULT '',
  chunk TEXT NOT NULL DEFAULT '', PRIMARY KEY (rule_id, path, heading, chunk)
);
CREATE TABLE id_sequences (day TEXT PRIMARY KEY, next_value INTEGER NOT NULL);
`

func newLegacyV1(t *testing.T, dir string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(legacyV1Schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO meta(key, value) VALUES('schema_version', '1'), ('created_at', '2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO rules(id, claim, body, status, confidence, reinforcement_count, created_at, updated_at, last_touched_at)
VALUES('r-2026-01-01-001', 'Go exported identifiers must use PascalCase', '', 'active', 0.7, 2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-05T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO rule_scopes(rule_id, tag) VALUES('r-2026-01-01-001', 'go')`); err != nil {
		t.Fatal(err)
	}
}

// Opening a v1 vault must migrate in place: no data loss, no "delete the db"
// instruction, and the new columns/tables usable afterwards.
func TestOpenMigratesLegacyV1Vault(t *testing.T) {
	dir := t.TempDir()
	newLegacyV1(t, dir)
	fixed := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	st, err := sqlite.Open(dir, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("open legacy vault: %v", err)
	}
	defer st.Close()

	rec, err := st.GetRecord("r-2026-01-01-001")
	if err != nil {
		t.Fatalf("read migrated record: %v", err)
	}
	if rec.Claim != "Go exported identifiers must use PascalCase" || rec.Confidence != 0.7 {
		t.Fatalf("record mutated by migration: %+v", rec)
	}
	if rec.Status != model.StatusActive {
		t.Fatalf("status = %s", rec.Status)
	}

	var version string
	db, err := sql.Open("sqlite", filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "2" {
		t.Fatalf("schema_version = %q, want 2", version)
	}
	var base float64
	var confirmed string
	if err := db.QueryRow(
		`SELECT base_confidence, last_confirmed_at FROM rule_stats WHERE rule_id = 'r-2026-01-01-001'`,
	).Scan(&base, &confirmed); err != nil {
		t.Fatalf("rule_stats not seeded: %v", err)
	}
	if base != 0.7 {
		t.Fatalf("base_confidence backfill = %v, want 0.7", base)
	}
	// Legacy last_touched_at is carried into the new decay clock.
	if confirmed != "2026-01-05T00:00:00Z" {
		t.Fatalf("last_confirmed_at = %q, want legacy last_touched_at", confirmed)
	}
	if _, err := db.Exec(`UPDATE rules SET query_local = 'x' WHERE id = 'r-2026-01-01-001'`); err != nil {
		t.Fatalf("query_local column missing after migration: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO rule_vectors(rule_id, model, dim, vec, updated_at) VALUES('r-2026-01-01-001','m',1,X'00','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("rule_vectors table missing after migration: %v", err)
	}
}

// A vault written by a newer binary must fail loudly, never silently.
func TestOpenRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(legacyV1Schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO meta(key, value) VALUES('schema_version', '99'), ('created_at', '2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatal(err)
	}
	_, err = sqlite.Open(dir, time.Now)
	if err == nil {
		t.Fatal("expected error for newer schema")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Fatalf("error should explain the vault is newer: %v", err)
	}
}

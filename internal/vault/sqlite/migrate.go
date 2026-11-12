package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

const schemaVersion = "2"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS rules (
  id                  TEXT PRIMARY KEY,
  claim               TEXT NOT NULL,
  body                TEXT NOT NULL DEFAULT '',
  query_local         TEXT NOT NULL DEFAULT '',
  status              TEXT NOT NULL,
  confidence          REAL NOT NULL,
  reinforcement_count INTEGER NOT NULL DEFAULT 0,
  created_at          TEXT NOT NULL,
  updated_at          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rules_status ON rules(status);
CREATE TABLE IF NOT EXISTS rule_stats (
  rule_id           TEXT PRIMARY KEY,
  recall_count      INTEGER NOT NULL DEFAULT 0,
  last_recalled_at  TEXT,
  last_confirmed_at TEXT NOT NULL,
  base_confidence   REAL NOT NULL DEFAULT -1
);
CREATE INDEX IF NOT EXISTS idx_rule_stats_confirmed ON rule_stats(last_confirmed_at);
CREATE TABLE IF NOT EXISTS rule_events (
  seq           INTEGER PRIMARY KEY AUTOINCREMENT,
  rule_id       TEXT NOT NULL,
  at            TEXT NOT NULL,
  kind          TEXT NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_rule_events_at ON rule_events(at);
CREATE INDEX IF NOT EXISTS idx_rule_events_rule ON rule_events(rule_id, at);
CREATE TABLE IF NOT EXISTS rule_scopes (
  rule_id TEXT NOT NULL,
  tag     TEXT NOT NULL,
  PRIMARY KEY (rule_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_rule_scopes_tag ON rule_scopes(tag);
CREATE TABLE IF NOT EXISTS evidence_events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  rule_id TEXT NOT NULL,
  at      TEXT NOT NULL,
  kind    TEXT NOT NULL,
  text    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_evidence_rule ON evidence_events(rule_id);
CREATE TABLE IF NOT EXISTS rule_edges (
  from_id TEXT NOT NULL,
  to_id   TEXT NOT NULL,
  kind    TEXT NOT NULL,
  PRIMARY KEY (from_id, to_id, kind)
);
CREATE INDEX IF NOT EXISTS idx_rule_edges_to ON rule_edges(to_id, kind);
CREATE TABLE IF NOT EXISTS rule_sources (
  rule_id TEXT NOT NULL,
  path    TEXT NOT NULL DEFAULT '',
  heading TEXT NOT NULL DEFAULT '',
  chunk   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (rule_id, path, heading, chunk)
);
CREATE INDEX IF NOT EXISTS idx_sources_path ON rule_sources(path);
CREATE INDEX IF NOT EXISTS idx_sources_chunk ON rule_sources(chunk);
CREATE TABLE IF NOT EXISTS id_sequences (
  day        TEXT PRIMARY KEY,
  next_value INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS rule_vectors (
  rule_id    TEXT PRIMARY KEY,
  model      TEXT NOT NULL,
  dim        INTEGER NOT NULL,
  vec        BLOB NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rule_vectors_model ON rule_vectors(model);
`

func (s *Store) ensureMeta() error {
	now := s.instant().Format(time.RFC3339)
	var current string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&current)
	if err == sql.ErrNoRows {
		// Fresh vault: schemaSQL above already created v2 structures.
		// Concurrent Open calls may race here; ON CONFLICT preserves the first writer.
		_, err = s.db.Exec(
			`INSERT INTO meta(key, value) VALUES('schema_version', ?), ('created_at', ?)
			 ON CONFLICT(key) DO NOTHING`,
			schemaVersion, now,
		)
		if err != nil {
			return err
		}
		err = s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&current)
	}
	if err == sql.ErrNoRows {
		return fmt.Errorf("initialize vault meta: schema_version missing after seed")
	}
	if err != nil {
		return err
	}
	if current == schemaVersion {
		return nil
	}
	// A newer vault (written by a future binary) must fail loudly instead of
	// being silently downgraded.
	var currentNum, wantNum int
	if _, err := fmt.Sscanf(current, "%d", &currentNum); err != nil {
		return fmt.Errorf("unsupported vault schema version %q", current)
	}
	if _, err := fmt.Sscanf(schemaVersion, "%d", &wantNum); err != nil {
		return fmt.Errorf("unsupported vault schema version %q", schemaVersion)
	}
	if currentNum > wantNum {
		return fmt.Errorf(
			"vault schema version %q is newer than this binary supports (%q); upgrade imprint",
			current, schemaVersion,
		)
	}
	if err := s.migrate(current); err != nil {
		return fmt.Errorf("migrate vault schema %q -> %q: %w", current, schemaVersion, err)
	}
	_, err = s.db.Exec(
		`UPDATE meta SET value = ? WHERE key = 'schema_version'`,
		schemaVersion,
	)
	return err
}

// migrate upgrades an older vault in-place. Steps probe with PRAGMA
// table_info so they are idempotent and tolerate both v1.2.1 vaults
// (last_touched_at column, no query_local) and v1.3.2 vaults (query_local,
// no base_confidence). Old columns are kept, never dropped.
func (s *Store) migrate(from string) error {
	return s.withTx(func(tx *sql.Tx) error {
		// rules.query_local (missing only in pre-v1.3.2 vaults).
		if !columnExists(tx, "rules", "query_local") {
			if _, err := tx.Exec(`ALTER TABLE rules ADD COLUMN query_local TEXT NOT NULL DEFAULT ''`); err != nil {
				return fmt.Errorf("add rules.query_local: %w", err)
			}
		}
		// rule_stats.base_confidence (added in v2; DEFAULT -1 marks rows
		// awaiting backfill so a real confidence of 0 is never confused).
		if !columnExists(tx, "rule_stats", "base_confidence") {
			if _, err := tx.Exec(`ALTER TABLE rule_stats ADD COLUMN base_confidence REAL NOT NULL DEFAULT -1`); err != nil {
				return fmt.Errorf("add rule_stats.base_confidence: %w", err)
			}
		}
		if _, err := tx.Exec(`
UPDATE rule_stats SET base_confidence = (
  SELECT confidence FROM rules WHERE rules.id = rule_stats.rule_id
) WHERE base_confidence < 0`); err != nil {
			return fmt.Errorf("backfill base_confidence: %w", err)
		}
		// Rows with no stats at all (e.g. a pre-v1.3.2 vault where rule_stats
		// was just created empty): seed from rules, preferring the legacy
		// last_touched_at column when it still exists.
		hasLastTouched := columnExists(tx, "rules", "last_touched_at")
		seed := `
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at, base_confidence)
SELECT id, 0, NULL, updated_at, confidence FROM rules
WHERE id NOT IN (SELECT rule_id FROM rule_stats)`
		if hasLastTouched {
			seed = `
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at, base_confidence)
SELECT id, 0, NULL,
       COALESCE(NULLIF(last_touched_at, ''), updated_at),
       confidence
FROM rules
WHERE id NOT IN (SELECT rule_id FROM rule_stats)`
		}
		if _, err := tx.Exec(seed); err != nil {
			return fmt.Errorf("seed rule_stats: %w", err)
		}
		return nil
	})
}

func columnExists(tx *sql.Tx, table, column string) bool {
	rows, err := tx.Query(`SELECT 1 FROM pragma_table_info(?) WHERE name = ?`, table, column)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

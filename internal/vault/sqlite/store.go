package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/heading"
	"github.com/SteamedBread2333/imprint/internal/sqliteutil"
	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

const schemaVersion = "1"

const rulesSelectCols = `id, claim, body, query_local, status, confidence, reinforcement_count, created_at, updated_at`

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
  last_confirmed_at TEXT NOT NULL
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
`

// Store persists vault rules in SQLite.
type Store struct {
	Dir string
	db  *sql.DB
	now func() time.Time
}

// DBPath returns the vault database file path for a vault directory.
func DBPath(dir string) string {
	return filepath.Join(dir, "vault.db")
}

// Open creates or opens vault.db under dir.
func Open(dir string, now func() time.Time) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("vault path is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	db, err := sqliteutil.Open(DBPath(abs))
	if err != nil {
		return nil, err
	}
	if err := sqliteutil.Retry(func() error {
		_, err := db.Exec(schemaSQL)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{Dir: abs, db: db, now: now}
	if err := sqliteutil.Retry(s.ensureMeta); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) instant() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *Store) ensureMeta() error {
	now := s.instant().Format(time.RFC3339)
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO meta(key, value) VALUES('schema_version', ?), ('created_at', ?)`,
		schemaVersion, now,
	); err != nil {
		return err
	}
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&v)
	if err != nil {
		return err
	}
	if v == schemaVersion {
		return nil
	}
	return fmt.Errorf("unsupported vault schema version %q; delete vault.db and rebuild", v)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// NextID returns the next r-YYYY-MM-DD-NNN id for date t.
func (s *Store) NextID(t time.Time) (string, error) {
	var id string
	err := s.withTx(func(tx *sql.Tx) error {
		var err error
		id, err = nextIDTx(tx, t)
		return err
	})
	return id, err
}

func nextIDTx(tx *sql.Tx, t time.Time) (string, error) {
	day := t.UTC().Format("2006-01-02")
	var n int
	err := tx.QueryRow(`
UPDATE id_sequences
SET next_value = next_value + 1
WHERE day = ?
RETURNING next_value - 1`, day).Scan(&n)
	if err == nil {
		return fmt.Sprintf("r-%s-%03d", day, n), nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	prefix := "r-" + t.UTC().Format("2006-01-02") + "-"
	rows, err := tx.Query(`SELECT id FROM rules WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return "", err
		}
		rest := strings.TrimPrefix(id, prefix)
		var n int
		if _, err := fmt.Sscanf(rest, "%d", &n); err == nil && n > maxN {
			maxN = n
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	n = maxN + 1
	if _, err := tx.Exec(
		`INSERT INTO id_sequences(day, next_value) VALUES(?, ?)`,
		day, n+1,
	); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, n), nil
}

func (s *Store) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func insertScopes(tx *sql.Tx, ruleID string, scope []string) error {
	for _, tag := range scope {
		if _, err := tx.Exec(`INSERT INTO rule_scopes(rule_id, tag) VALUES(?, ?)`, ruleID, tag); err != nil {
			return err
		}
	}
	return nil
}

func insertEdges(tx *sql.Tx, fromID string, toIDs []string, kind string) error {
	for _, to := range toIDs {
		if to == "" || to == fromID {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO rule_edges(from_id, to_id, kind) VALUES(?, ?, ?)`,
			fromID, to, kind,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertSources(tx *sql.Tx, ruleID string, sources []model.DocRef) error {
	for _, src := range sources {
		if _, err := tx.Exec(
			`INSERT INTO rule_sources(rule_id, path, heading, chunk) VALUES(?, ?, ?, ?)`,
			ruleID, src.Path, src.Heading, src.Chunk,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertEvidence(tx *sql.Tx, ruleID string, log []model.Evidence) error {
	for _, e := range log {
		if _, err := tx.Exec(
			`INSERT INTO evidence_events(rule_id, at, kind, text) VALUES(?, ?, ?, ?)`,
			ruleID, formatTime(e.At), string(e.Kind), e.Text,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertInitialStats(tx *sql.Tx, ruleID string, confirmedAt time.Time) error {
	_, err := tx.Exec(`
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at)
VALUES(?, 0, NULL, ?)
ON CONFLICT(rule_id) DO NOTHING`, ruleID, formatTime(confirmedAt))
	return err
}

func insertRuleEvent(tx *sql.Tx, ruleID string, at time.Time, kind, metadata string) error {
	_, err := tx.Exec(
		`INSERT INTO rule_events(rule_id, at, kind, metadata_json) VALUES(?, ?, ?, ?)`,
		ruleID, formatTime(at), kind, metadata,
	)
	return err
}

func upsertRuleRow(tx *sql.Tx, r *model.Record) error {
	_, err := tx.Exec(`
INSERT INTO rules(id, claim, body, query_local, status, confidence, reinforcement_count, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  claim = excluded.claim,
  body = excluded.body,
  query_local = excluded.query_local,
  status = excluded.status,
  confidence = excluded.confidence,
  reinforcement_count = excluded.reinforcement_count,
  updated_at = excluded.updated_at`,
		r.ID, r.Claim, r.Body, r.QueryLocal, string(r.Status), r.Confidence, r.ReinforcementCount,
		formatTime(r.CreatedAt), formatTime(r.UpdatedAt),
	)
	return err
}

func replaceChildren(tx *sql.Tx, r *model.Record) error {
	if _, err := tx.Exec(`DELETE FROM rule_scopes WHERE rule_id = ?`, r.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rule_edges WHERE from_id = ?`, r.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rule_sources WHERE rule_id = ?`, r.ID); err != nil {
		return err
	}
	if err := insertScopes(tx, r.ID, r.Scope); err != nil {
		return err
	}
	if err := insertEdges(tx, r.ID, r.Supersedes, "supersedes"); err != nil {
		return err
	}
	if err := insertEdges(tx, r.ID, r.Related, "related"); err != nil {
		return err
	}
	if err := insertEdges(tx, r.ID, r.ConflictsWith, "conflicts_with"); err != nil {
		return err
	}
	return insertSources(tx, r.ID, r.Sources)
}

func putRecordTx(tx *sql.Tx, r *model.Record) error {
	if err := upsertRuleRow(tx, r); err != nil {
		return err
	}
	if err := replaceChildren(tx, r); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM evidence_events WHERE rule_id = ?`, r.ID); err != nil {
		return err
	}
	return insertEvidence(tx, r.ID, r.EvidenceLog)
}

// PutRecord writes a full rule (metadata + children). Evidence is replaced entirely.
func (s *Store) PutRecord(r *model.Record) error {
	return s.withTx(func(tx *sql.Tx) error {
		return putRecordTx(tx, r)
	})
}

// SupersedePair inserts newRec and updates oldRec in one transaction. The
// optimistic timestamp check prevents two writers from superseding the same
// version of a rule.
func (s *Store) SupersedePair(oldRec, newRec *model.Record, expectedUpdatedAt time.Time) error {
	return s.withTx(func(tx *sql.Tx) error {
		var status, updated string
		if err := tx.QueryRow(
			`SELECT status, updated_at FROM rules WHERE id = ?`,
			oldRec.ID,
		).Scan(&status, &updated); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("record %s not found", oldRec.ID)
			}
			return err
		}
		if status == string(model.StatusSuperseded) {
			return fmt.Errorf("record %s is already superseded", oldRec.ID)
		}
		if parseTime(updated) != expectedUpdatedAt {
			return fmt.Errorf("record %s changed concurrently; retry supersede", oldRec.ID)
		}
		if err := putRecordTx(tx, newRec); err != nil {
			return err
		}
		if err := upsertInitialStats(tx, newRec.ID, newRec.CreatedAt); err != nil {
			return err
		}
		if err := putRecordTx(tx, oldRec); err != nil {
			return err
		}
		if err := insertRuleEvent(tx, oldRec.ID, newRec.CreatedAt, "supersede", `{"successor":"`+newRec.ID+`"}`); err != nil {
			return err
		}
		return insertRuleEvent(tx, newRec.ID, newRec.CreatedAt, "add", `{"supersedes":"`+oldRec.ID+`"}`)
	})
}

// AppendEvidence adds one evidence row without touching other fields.
func (s *Store) AppendEvidence(ruleID string, e model.Evidence) error {
	_, err := s.db.Exec(
		`INSERT INTO evidence_events(rule_id, at, kind, text) VALUES(?, ?, ?, ?)`,
		ruleID, formatTime(e.At), string(e.Kind), e.Text,
	)
	return err
}

// ReinforceRecord atomically applies diminishing confidence gain, wakes dormant rules,
// updates query_local when supplied, and appends evidence.
func (s *Store) ReinforceRecord(id string, now time.Time, e model.Evidence, queryLocal string, maxConfidence float64) (float64, int, error) {
	var confidence float64
	var count int
	err := s.withTx(func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRow(`SELECT status FROM rules WHERE id = ?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("record %s not found", id)
			}
			return err
		}
		if status == string(model.StatusSuperseded) {
			return fmt.Errorf("record %s is superseded and cannot be reinforced", id)
		}
		err := tx.QueryRow(`
UPDATE rules
SET confidence = MIN(?, confidence + (? - confidence) * 0.25),
    reinforcement_count = reinforcement_count + 1,
    status = CASE WHEN status = ? THEN ? ELSE status END,
    query_local = CASE WHEN ? = '' THEN query_local ELSE ? END,
    updated_at = ?
WHERE id = ?
RETURNING confidence, reinforcement_count`,
			maxConfidence, maxConfidence,
			string(model.StatusDormant), string(model.StatusActive),
			queryLocal, queryLocal,
			formatTime(now), id,
		).Scan(&confidence, &count)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(
			`INSERT INTO evidence_events(rule_id, at, kind, text) VALUES(?, ?, ?, ?)`,
			id, formatTime(e.At), string(e.Kind), e.Text,
		); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE rule_stats SET last_confirmed_at = ? WHERE rule_id = ?`, formatTime(now), id); err != nil {
			return err
		}
		return insertRuleEvent(tx, id, now, "reinforce", "")
	})
	return confidence, count, err
}

// SweepActive atomically decays stale active rules and marks active rules below
// threshold dormant. It never rewrites child tables or evidence.
func (s *Store) SweepActive(cutoff, now time.Time, decayAmount, dormantThreshold float64) (decayed, archived int, err error) {
	err = s.withTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`
UPDATE rules
SET confidence = MAX(0, confidence - ?),
    status = CASE
      WHEN MAX(0, confidence - ?) < ? THEN ?
      ELSE status
    END,
    updated_at = ?
WHERE status = ? AND id IN (
  SELECT rule_id FROM rule_stats WHERE last_confirmed_at <= ?
)
RETURNING id, status`,
			decayAmount, decayAmount, dormantThreshold, string(model.StatusDormant),
			formatTime(now), string(model.StatusActive), formatTime(cutoff),
		)
		if err != nil {
			return err
		}
		var dormantIDs []string
		for rows.Next() {
			var id, status string
			if err := rows.Scan(&id, &status); err != nil {
				_ = rows.Close()
				return err
			}
			decayed++
			if status == string(model.StatusDormant) {
				archived++
				dormantIDs = append(dormantIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}

		rows, err = tx.Query(`
UPDATE rules
SET status = ?, updated_at = ?
WHERE status = ? AND confidence < ?
RETURNING id`,
			string(model.StatusDormant), formatTime(now),
			string(model.StatusActive), dormantThreshold,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			archived++
			dormantIDs = append(dormantIDs, id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range dormantIDs {
			if err := insertRuleEvent(tx, id, now, "sweep_dormant", ""); err != nil {
				return err
			}
		}
		return nil
	})
	return decayed, archived, err
}

// InsertRecord creates a new rule with all fields.
func (s *Store) InsertRecord(r *model.Record) error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := putRecordTx(tx, r); err != nil {
			return err
		}
		if err := upsertInitialStats(tx, r.ID, r.CreatedAt); err != nil {
			return err
		}
		return insertRuleEvent(tx, r.ID, r.CreatedAt, "add", "")
	})
}

// RecordHits updates operational recall statistics without touching rule facts.
func (s *Store) RecordHits(ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.withTx(func(tx *sql.Tx) error {
		for _, id := range ids {
			if _, err := tx.Exec(`
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at)
SELECT ?, 1, ?, created_at
FROM rules WHERE id = ?
ON CONFLICT(rule_id) DO UPDATE SET
  recall_count = recall_count + 1,
  last_recalled_at = excluded.last_recalled_at`,
				id, formatTime(at), id); err != nil {
				return err
			}
		}
		return nil
	})
}

// StatsForRules returns operational stats keyed by rule id.
func (s *Store) StatsForRules(ids []string) (map[string]model.RuleStats, error) {
	out := map[string]model.RuleStats{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`
SELECT rule_id, recall_count, last_recalled_at, last_confirmed_at
FROM rule_stats WHERE rule_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var st model.RuleStats
		var recalled sql.NullString
		var confirmed string
		if err := rows.Scan(&st.RuleID, &st.RecallCount, &recalled, &confirmed); err != nil {
			return nil, err
		}
		st.LastConfirmedAt = parseTime(confirmed)
		if recalled.Valid {
			t := parseTime(recalled.String)
			st.LastRecalledAt = &t
		}
		out[st.RuleID] = st
	}
	return out, rows.Err()
}

// EventsSince returns lifecycle events at or after since.
func (s *Store) EventsSince(since time.Time) ([]model.RuleEvent, error) {
	rows, err := s.db.Query(`
SELECT seq, rule_id, at, kind, metadata_json
FROM rule_events WHERE at >= ? ORDER BY seq`, formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RuleEvent
	for rows.Next() {
		var ev model.RuleEvent
		var at string
		if err := rows.Scan(&ev.Seq, &ev.RuleID, &at, &ev.Kind, &ev.Metadata); err != nil {
			return nil, err
		}
		ev.At = parseTime(at)
		out = append(out, ev)
	}
	return out, rows.Err()
}

// DeleteRecord removes a rule and all related rows.
func (s *Store) DeleteRecord(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := insertRuleEvent(tx, id, s.instant(), "forget", ""); err != nil {
			return err
		}
		for _, q := range []string{
			`DELETE FROM evidence_events WHERE rule_id = ?`,
			`DELETE FROM rule_scopes WHERE rule_id = ?`,
			`DELETE FROM rule_edges WHERE from_id = ? OR to_id = ?`,
			`DELETE FROM rule_sources WHERE rule_id = ?`,
			`DELETE FROM rule_stats WHERE rule_id = ?`,
			`DELETE FROM rules WHERE id = ?`,
		} {
			args := []any{id}
			if strings.Count(q, "?") == 2 {
				args = append(args, id)
			}
			if _, err := tx.Exec(q, args...); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) loadScopes(ruleIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ruleIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT rule_id, tag FROM rule_scopes WHERE rule_id IN (`+placeholders+`) ORDER BY tag`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ruleID, tag string
		if err := rows.Scan(&ruleID, &tag); err != nil {
			return nil, err
		}
		out[ruleID] = append(out[ruleID], tag)
	}
	return out, rows.Err()
}

func (s *Store) loadEdges(ruleIDs []string) (map[string]map[string][]string, error) {
	out := make(map[string]map[string][]string, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ruleIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT from_id, to_id, kind FROM rule_edges WHERE from_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var from, to, kind string
		if err := rows.Scan(&from, &to, &kind); err != nil {
			return nil, err
		}
		if out[from] == nil {
			out[from] = map[string][]string{}
		}
		out[from][kind] = append(out[from][kind], to)
	}
	return out, rows.Err()
}

func (s *Store) loadSources(ruleIDs []string) (map[string][]model.DocRef, error) {
	out := make(map[string][]model.DocRef, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ruleIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT rule_id, path, heading, chunk FROM rule_sources WHERE rule_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ruleID string
		var ref model.DocRef
		if err := rows.Scan(&ruleID, &ref.Path, &ref.Heading, &ref.Chunk); err != nil {
			return nil, err
		}
		out[ruleID] = append(out[ruleID], ref)
	}
	return out, rows.Err()
}

func (s *Store) loadEvidence(ruleIDs []string) (map[string][]model.Evidence, error) {
	out := make(map[string][]model.Evidence, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ruleIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT rule_id, at, kind, text FROM evidence_events WHERE rule_id IN (`+placeholders+`) ORDER BY id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ruleID, at, kind, text string
		if err := rows.Scan(&ruleID, &at, &kind, &text); err != nil {
			return nil, err
		}
		out[ruleID] = append(out[ruleID], model.Evidence{
			At:   parseTime(at),
			Kind: model.EvidenceKind(kind),
			Text: text,
		})
	}
	return out, rows.Err()
}

func (s *Store) assembleRecords(rows *sql.Rows) ([]*model.Record, error) {
	var ids []string
	var recs []*model.Record
	for rows.Next() {
		var r model.Record
		var status, created, updated string
		if err := rows.Scan(
			&r.ID, &r.Claim, &r.Body, &r.QueryLocal, &status, &r.Confidence, &r.ReinforcementCount,
			&created, &updated,
		); err != nil {
			return nil, err
		}
		r.Status = model.Status(status)
		r.CreatedAt = parseTime(created)
		r.UpdatedAt = parseTime(updated)
		r.Path = DBPath(s.Dir)
		ids = append(ids, r.ID)
		recs = append(recs, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	scopes, err := s.loadScopes(ids)
	if err != nil {
		return nil, err
	}
	edges, err := s.loadEdges(ids)
	if err != nil {
		return nil, err
	}
	sources, err := s.loadSources(ids)
	if err != nil {
		return nil, err
	}
	evidence, err := s.loadEvidence(ids)
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		r.Scope = scopes[r.ID]
		if r.Scope == nil {
			r.Scope = []string{}
		}
		if em := edges[r.ID]; em != nil {
			r.Supersedes = em["supersedes"]
			r.Related = em["related"]
			r.ConflictsWith = em["conflicts_with"]
		}
		if r.Supersedes == nil {
			r.Supersedes = []string{}
		}
		if r.Related == nil {
			r.Related = []string{}
		}
		if r.ConflictsWith == nil {
			r.ConflictsWith = []string{}
		}
		r.Sources = sources[r.ID]
		if r.Sources == nil {
			r.Sources = []model.DocRef{}
		}
		r.EvidenceLog = evidence[r.ID]
		if r.EvidenceLog == nil {
			r.EvidenceLog = []model.Evidence{}
		}
	}
	return recs, nil
}

// GetRecord returns one rule by id.
func (s *Store) GetRecord(id string) (*model.Record, error) {
	rows, err := s.db.Query(`
SELECT `+rulesSelectCols+`
FROM rules WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recs, err := s.assembleRecords(rows)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("record %s not found", id)
	}
	return recs[0], nil
}

// AllRecords returns every rule, optionally excluding non-active archived statuses.
func (s *Store) AllRecords(includeArchived bool) ([]*model.Record, error) {
	q := `SELECT ` + rulesSelectCols + ` FROM rules`
	if !includeArchived {
		q += ` WHERE status != 'superseded'`
	}
	q += ` ORDER BY id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.assembleRecords(rows)
}

// ActiveCandidates returns active rules matching scope AND and min confidence.
func (s *Store) ActiveCandidates(scope []string, minConfidence float64) ([]*model.Record, error) {
	if len(scope) == 0 {
		rows, err := s.db.Query(`
SELECT `+rulesSelectCols+`
FROM rules WHERE status = 'active' AND confidence >= ? ORDER BY id`, minConfidence)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return s.assembleRecords(rows)
	}
	// scope AND: rule must have all tags
	rows, err := s.db.Query(`
SELECT r.id, r.claim, r.body, r.query_local, r.status, r.confidence, r.reinforcement_count, r.created_at, r.updated_at
FROM rules r
WHERE r.status = 'active' AND r.confidence >= ?
AND (SELECT COUNT(DISTINCT LOWER(rs.tag)) FROM rule_scopes rs
     WHERE rs.rule_id = r.id AND LOWER(rs.tag) IN (`+scopePlaceholders(scope)+`)) = ?
ORDER BY r.id`, scopeArgs(minConfidence, scope)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.assembleRecords(rows)
}

// DormantCandidates returns dormant rules matching scope AND. They are used
// only as a low-weight fallback when active recall under-fills a query.
func (s *Store) DormantCandidates(scope []string) ([]*model.Record, error) {
	if len(scope) == 0 {
		rows, err := s.db.Query(`
SELECT ` + rulesSelectCols + `
FROM rules WHERE status = 'dormant' ORDER BY id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return s.assembleRecords(rows)
	}
	rows, err := s.db.Query(`
SELECT r.id, r.claim, r.body, r.query_local, r.status, r.confidence, r.reinforcement_count, r.created_at, r.updated_at
FROM rules r
WHERE r.status = 'dormant'
AND (SELECT COUNT(DISTINCT LOWER(rs.tag)) FROM rule_scopes rs
     WHERE rs.rule_id = r.id AND LOWER(rs.tag) IN (`+scopePlaceholders(scope)+`)) = ?
ORDER BY r.id`, scopeArgsWithoutConfidence(scope)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.assembleRecords(rows)
}

func scopePlaceholders(scope []string) string {
	parts := make([]string, len(scope))
	for i := range scope {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func scopeArgsWithoutConfidence(scope []string) []any {
	args := make([]any, 0, len(scope)+1)
	for _, tag := range scope {
		args = append(args, strings.ToLower(strings.TrimSpace(tag)))
	}
	return append(args, len(scope))
}

func scopeArgs(minConf float64, scope []string) []any {
	args := []any{minConf}
	for _, tag := range scope {
		args = append(args, strings.ToLower(strings.TrimSpace(tag)))
	}
	args = append(args, len(scope))
	return args
}

// ConflictPairs returns directed conflicts_with edges whose endpoints are both
// present in ids.
func (s *Store) ConflictPairs(ids []string) ([][2]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)*2)
	for _, id := range ids {
		args = append(args, id)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.Query(`
SELECT from_id, to_id
FROM rule_edges
WHERE kind = 'conflicts_with'
  AND from_id IN (`+placeholders+`)
  AND to_id IN (`+placeholders+`)
ORDER BY from_id, to_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][2]string
	for rows.Next() {
		var pair [2]string
		if err := rows.Scan(&pair[0], &pair[1]); err != nil {
			return nil, err
		}
		out = append(out, pair)
	}
	return out, rows.Err()
}

// Backlinks returns inbound edge references to id.
func (s *Store) Backlinks(id string) ([]model.Backlink, error) {
	rows, err := s.db.Query(`SELECT from_id, kind FROM rule_edges WHERE to_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Backlink
	for rows.Next() {
		var from, kind string
		if err := rows.Scan(&from, &kind); err != nil {
			return nil, err
		}
		out = append(out, model.Backlink{ID: from, Kind: kind})
	}
	return out, rows.Err()
}

// SourcesForIDs returns sources map for given rule ids.
func (s *Store) SourcesForIDs(ids []string) (map[string][]model.DocRef, error) {
	return s.loadSources(ids)
}

// RulesReferencingDoc finds active rules whose sources match path/heading/chunk.
func (s *Store) RulesReferencingDoc(path, heading, chunkID string) ([]model.ReferencedRule, error) {
	path = filepathSlash(strings.TrimSpace(strings.TrimPrefix(path, "./")))
	heading = strings.TrimSpace(heading)
	chunkID = strings.TrimSpace(chunkID)
	if path == "" && chunkID == "" {
		return nil, nil
	}
	var rows *sql.Rows
	var err error
	switch {
	case chunkID != "":
		rows, err = s.db.Query(`
SELECT DISTINCT r.id, r.claim, s.heading, s.chunk FROM rules r
JOIN rule_sources s ON s.rule_id = r.id
WHERE r.status = 'active' AND (s.chunk = ? OR (s.path = ? AND s.chunk = ''))`, chunkID, path)
	case path != "":
		rows, err = s.db.Query(`
SELECT DISTINCT r.id, r.claim, s.heading FROM rules r
JOIN rule_sources s ON s.rule_id = r.id
WHERE r.status = 'active' AND s.path = ?`, path)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ReferencedRule
	seen := map[string]struct{}{}
	for rows.Next() {
		var id, claim string
		var srcHeading, srcChunk sql.NullString
		if chunkID != "" {
			if err := rows.Scan(&id, &claim, &srcHeading, &srcChunk); err != nil {
				return nil, err
			}
			if srcChunk.String == "" && !sourceHeadingMatch(srcHeading.String, heading) {
				continue
			}
		} else {
			if err := rows.Scan(&id, &claim, &srcHeading); err != nil {
				return nil, err
			}
			if !sourceHeadingMatch(srcHeading.String, heading) {
				continue
			}
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, model.ReferencedRule{ID: id, Kind: "sources", Claim: claim})
	}
	return out, rows.Err()
}

func sourceHeadingMatch(srcHeading, queryHeading string) bool {
	srcHeading = strings.TrimSpace(srcHeading)
	queryHeading = strings.TrimSpace(queryHeading)
	if srcHeading == "" {
		return true
	}
	if queryHeading == "" {
		return false
	}
	return heading.Match(srcHeading, queryHeading)
}

func filepathSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// Clear removes all vault data.
func (s *Store) Clear() error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rule_stats", "rule_events", "rules"} {
			if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
				return err
			}
		}
		return nil
	})
}

// ImportRecords replaces the entire vault contents.
func (s *Store) ImportRecords(recs []*model.Record) error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rule_stats", "rule_events", "rules"} {
			if _, err := tx.Exec(`DELETE FROM ` + t); err != nil {
				return err
			}
		}
		for _, r := range recs {
			if err := upsertRuleRow(tx, r); err != nil {
				return err
			}
			if err := replaceChildren(tx, r); err != nil {
				return err
			}
			if err := insertEvidence(tx, r.ID, r.EvidenceLog); err != nil {
				return err
			}
			if err := upsertInitialStats(tx, r.ID, r.CreatedAt); err != nil {
				return err
			}
			if err := insertRuleEvent(tx, r.ID, r.CreatedAt, "add", `{"imported":true}`); err != nil {
				return err
			}
		}
		return nil
	})
}

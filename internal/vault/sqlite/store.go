package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"

	_ "modernc.org/sqlite"
)

const schemaVersion = "1"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS rules (
  id                  TEXT PRIMARY KEY,
  claim               TEXT NOT NULL,
  body                TEXT NOT NULL DEFAULT '',
  status              TEXT NOT NULL,
  confidence          REAL NOT NULL,
  reinforcement_count INTEGER NOT NULL DEFAULT 0,
  created_at          TEXT NOT NULL,
  updated_at          TEXT NOT NULL,
  last_touched_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rules_status ON rules(status);
CREATE INDEX IF NOT EXISTS idx_rules_last_touched ON rules(last_touched_at);
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
	db, err := sql.Open("sqlite", DBPath(abs))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{Dir: abs, db: db, now: now}
	if err := s.ensureMeta(); err != nil {
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
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&v)
	if err == sql.ErrNoRows {
		now := s.instant().Format(time.RFC3339)
		if _, err := s.db.Exec(
			`INSERT INTO meta(key, value) VALUES('schema_version', ?), ('created_at', ?)`,
			schemaVersion, now,
		); err != nil {
			return err
		}
		return nil
	}
	return err
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
	prefix := "r-" + t.UTC().Format("2006-01-02") + "-"
	rows, err := s.db.Query(`SELECT id FROM rules WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		rest := strings.TrimPrefix(id, prefix)
		var n int
		if _, err := fmt.Sscanf(rest, "%d", &n); err == nil && n > maxN {
			maxN = n
		}
	}
	return fmt.Sprintf("%s%03d", prefix, maxN+1), nil
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

func upsertRuleRow(tx *sql.Tx, r *model.Record) error {
	_, err := tx.Exec(`
INSERT INTO rules(id, claim, body, status, confidence, reinforcement_count, created_at, updated_at, last_touched_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  claim = excluded.claim,
  body = excluded.body,
  status = excluded.status,
  confidence = excluded.confidence,
  reinforcement_count = excluded.reinforcement_count,
  updated_at = excluded.updated_at,
  last_touched_at = excluded.last_touched_at`,
		r.ID, r.Claim, r.Body, string(r.Status), r.Confidence, r.ReinforcementCount,
		formatTime(r.CreatedAt), formatTime(r.UpdatedAt), formatTime(r.LastTouchedAt),
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

// SupersedePair inserts newRec and updates oldRec in one transaction.
func (s *Store) SupersedePair(oldRec, newRec *model.Record) error {
	return s.withTx(func(tx *sql.Tx) error {
		if err := putRecordTx(tx, newRec); err != nil {
			return err
		}
		return putRecordTx(tx, oldRec)
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

// InsertRecord creates a new rule with all fields.
func (s *Store) InsertRecord(r *model.Record) error {
	return s.PutRecord(r)
}

// DeleteRecord removes a rule and all related rows.
func (s *Store) DeleteRecord(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, q := range []string{
			`DELETE FROM evidence_events WHERE rule_id = ?`,
			`DELETE FROM rule_scopes WHERE rule_id = ?`,
			`DELETE FROM rule_edges WHERE from_id = ? OR to_id = ?`,
			`DELETE FROM rule_sources WHERE rule_id = ?`,
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

// StripInboundEdges removes to_id references pointing at id.
func (s *Store) StripInboundEdges(id string) error {
	_, err := s.db.Exec(`DELETE FROM rule_edges WHERE to_id = ?`, id)
	return err
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
		var status, created, updated, touched string
		if err := rows.Scan(
			&r.ID, &r.Claim, &r.Body, &status, &r.Confidence, &r.ReinforcementCount,
			&created, &updated, &touched,
		); err != nil {
			return nil, err
		}
		r.Status = model.Status(status)
		r.CreatedAt = parseTime(created)
		r.UpdatedAt = parseTime(updated)
		r.LastTouchedAt = parseTime(touched)
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
SELECT id, claim, body, status, confidence, reinforcement_count, created_at, updated_at, last_touched_at
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
	q := `SELECT id, claim, body, status, confidence, reinforcement_count, created_at, updated_at, last_touched_at FROM rules`
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
SELECT id, claim, body, status, confidence, reinforcement_count, created_at, updated_at, last_touched_at
FROM rules WHERE status = 'active' AND confidence >= ? ORDER BY id`, minConfidence)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return s.assembleRecords(rows)
	}
	// scope AND: rule must have all tags
	rows, err := s.db.Query(`
SELECT r.id, r.claim, r.body, r.status, r.confidence, r.reinforcement_count, r.created_at, r.updated_at, r.last_touched_at
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

func scopePlaceholders(scope []string) string {
	parts := make([]string, len(scope))
	for i := range scope {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func scopeArgs(minConf float64, scope []string) []any {
	args := []any{minConf}
	for _, tag := range scope {
		args = append(args, strings.ToLower(strings.TrimSpace(tag)))
	}
	args = append(args, len(scope))
	return args
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
	return strings.EqualFold(srcHeading, queryHeading) ||
		strings.Contains(strings.ToLower(queryHeading), strings.ToLower(srcHeading))
}

func filepathSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// Clear removes all vault data.
func (s *Store) Clear() error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rules"} {
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
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rules"} {
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
		}
		return nil
	})
}

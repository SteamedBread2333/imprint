package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

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
	if err := insertEvidence(tx, r.ID, r.EvidenceLog); err != nil {
		return err
	}
	// A full rewrite resets the decay baseline: confidence changes made here
	// are authoritative, not the result of time-based decay. Existing
	// last_confirmed_at / recall stats are preserved.
	_, err := tx.Exec(`
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at, base_confidence)
VALUES(?, 0, NULL, ?, ?)
ON CONFLICT(rule_id) DO UPDATE SET base_confidence = excluded.base_confidence`,
		r.ID, formatTime(r.UpdatedAt), r.Confidence)
	return err
}

// ReplaceRecord overwrites a rule and all its child rows with r. The stored
// evidence log becomes exactly r.EvidenceLog, so rows added via AppendEvidence
// are dropped unless r carries them. It is not a partial update.
func (s *Store) ReplaceRecord(r *model.Record) error {
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
		if _, err = tx.Exec(`UPDATE rule_stats SET last_confirmed_at = ?, base_confidence = ? WHERE rule_id = ?`, formatTime(now), confidence, id); err != nil {
			return err
		}
		return insertRuleEvent(tx, id, now, "reinforce", "")
	})
	return confidence, count, err
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
INSERT INTO rule_stats(rule_id, recall_count, last_recalled_at, last_confirmed_at, base_confidence)
SELECT ?, 1, ?, created_at, confidence
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
			`DELETE FROM rule_vectors WHERE rule_id = ?`,
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

// Clear removes all vault data.
func (s *Store) Clear() error {
	return s.withTx(func(tx *sql.Tx) error {
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rule_stats", "rule_events", "rule_vectors", "rules"} {
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
		for _, t := range []string{"evidence_events", "rule_scopes", "rule_edges", "rule_sources", "rule_stats", "rule_events", "rule_vectors", "rules"} {
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

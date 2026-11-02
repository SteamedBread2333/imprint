package sqlite

import (
	"encoding/binary"
	"math"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/sqliteutil"
)

func encodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func decodeVector(blob []byte) []float32 {
	n := len(blob) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return out
}

// UpsertRuleVector stores the embedding for one rule. Rows for rules that no
// longer exist are rejected so a racing forget after add cannot leave orphan
// vectors behind.
func (s *Store) UpsertRuleVector(ruleID, model string, vec []float32, at time.Time) error {
	return sqliteutil.Retry(func() error {
		_, err := s.db.Exec(`
INSERT INTO rule_vectors(rule_id, model, dim, vec, updated_at)
SELECT ?, ?, ?, ?, ?
WHERE EXISTS(SELECT 1 FROM rules WHERE id = ?)
ON CONFLICT(rule_id) DO UPDATE SET
  model = excluded.model,
  dim = excluded.dim,
  vec = excluded.vec,
  updated_at = excluded.updated_at`,
			ruleID, model, len(vec), encodeVector(vec), formatTime(at), ruleID)
		return err
	})
}

// DeleteRuleVectors removes embeddings for the given rule ids.
func (s *Store) DeleteRuleVectors(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return sqliteutil.Retry(func() error {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		_, err := s.db.Exec(`DELETE FROM rule_vectors WHERE rule_id IN (`+placeholders+`)`, args...)
		return err
	})
}

// RuleVectors returns stored embeddings keyed by rule id for one model.
// Vectors from other models are ignored (they live in a different space).
func (s *Store) RuleVectors(model string) (map[string][]float32, error) {
	rows, err := s.db.Query(`
SELECT rule_id, vec FROM rule_vectors WHERE model = ?`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]float32{}
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		out[id] = decodeVector(blob)
	}
	return out, rows.Err()
}

// VectorCount returns the number of stored embeddings for one model.
func (s *Store) VectorCount(model string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM rule_vectors WHERE model = ?`, model).Scan(&n)
	return n, err
}

// RulesWithoutVectors returns active+dormant rule ids whose claims are not
// yet encoded for the given model. Used by the backfill command to catch up
// after enabling the embed sidecar or importing a vault. limit ≤ 0 means
// return every missing rule.
func (s *Store) RulesWithoutVectors(model string, limit int) ([]string, error) {
	q := `
SELECT r.id FROM rules r
LEFT JOIN rule_vectors v ON v.rule_id = r.id AND v.model = ?
WHERE r.status IN ('active','dormant') AND v.rule_id IS NULL
ORDER BY r.id`
	args := []any{model}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

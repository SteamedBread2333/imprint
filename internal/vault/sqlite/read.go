package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

const rulesSelectCols = `id, claim, body, query_local, status, confidence, reinforcement_count, created_at, updated_at`

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

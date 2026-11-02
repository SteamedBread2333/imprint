package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/heading"
	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

// ConflictPairs returns directed conflicts_with edges whose endpoints are both
// present in ids.
// AddEdges inserts relationship edges from fromID to each existing target id.
// kind must be "related" or "conflicts_with" ("supersedes" is written only by
// the supersede flow). Inserts are idempotent (INSERT OR IGNORE on the edge
// primary key); unknown target ids are rejected so a typo cannot mint a
// dangling edge. Returns the number of edges actually created.
func (s *Store) AddEdges(fromID string, toIDs []string, kind string) (int, error) {
	if kind != "related" && kind != "conflicts_with" {
		return 0, fmt.Errorf("unsupported edge kind %q", kind)
	}
	added := 0
	err := s.withTx(func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(1) FROM rules WHERE id = ?`, fromID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("record %s not found", fromID)
		}
		now := s.instant()
		for _, to := range toIDs {
			if to == "" || to == fromID {
				continue
			}
			if err := tx.QueryRow(`SELECT COUNT(1) FROM rules WHERE id = ?`, to).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return fmt.Errorf("link target %s not found", to)
			}
			res, err := tx.Exec(
				`INSERT OR IGNORE INTO rule_edges(from_id, to_id, kind) VALUES(?, ?, ?)`,
				fromID, to, kind,
			)
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err == nil && n > 0 {
				added++
				if err := insertRuleEvent(tx, fromID, now, "link", kind+":"+to); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return added, err
}

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

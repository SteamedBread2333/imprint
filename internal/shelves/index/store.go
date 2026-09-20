package index

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chunks (
  id         TEXT PRIMARY KEY,
  path       TEXT NOT NULL,
  heading    TEXT NOT NULL DEFAULT '',
  text       TEXT NOT NULL,
  line_start INTEGER NOT NULL,
  line_end   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chunks_path ON chunks(path);
CREATE TABLE IF NOT EXISTS rule_refs (
  chunk_id  TEXT NOT NULL,
  rule_id   TEXT NOT NULL,
  line_no   INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (chunk_id, rule_id, line_no)
);
CREATE INDEX IF NOT EXISTS idx_rule_refs_rule ON rule_refs(rule_id);
`

// Store is the persisted index on disk.
type Store struct {
	Fingerprint string    `json:"fingerprint"`
	BuiltAt     time.Time `json:"built_at"`
	FileCount   int       `json:"file_count"`
	Chunks      []Chunk   `json:"chunks"`
	RuleRefs    []RuleRef `json:"rule_refs,omitempty"`
}

// DBPath returns the SQLite index file path.
func DBPath(stateDir string) string {
	return filepath.Join(stateDir, "index.db")
}

// StorePath is kept for compatibility checks (legacy JSON).
func StorePath(stateDir string) string {
	return filepath.Join(stateDir, "index.json")
}

func openDB(stateDir string) (*sql.DB, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", DBPath(stateDir))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func metaGet(db *sql.DB, key string) (string, bool) {
	var v string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	return v, err == nil
}

func metaSet(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// LoadStore reads index.db from stateDir.
func LoadStore(stateDir string) (*Store, error) {
	if _, err := os.Stat(DBPath(stateDir)); os.IsNotExist(err) {
		return nil, err
	}
	db, err := openDB(stateDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	s := &Store{Chunks: []Chunk{}}
	if fp, ok := metaGet(db, "fingerprint"); ok {
		s.Fingerprint = fp
	}
	if built, ok := metaGet(db, "built_at"); ok {
		if t, err := time.Parse(time.RFC3339Nano, built); err == nil {
			s.BuiltAt = t
		} else if t, err := time.Parse(time.RFC3339, built); err == nil {
			s.BuiltAt = t
		}
	}
	if fc, ok := metaGet(db, "file_count"); ok {
		fmt.Sscanf(fc, "%d", &s.FileCount)
	}
	rows, err := db.Query(`SELECT id, path, heading, text, line_start, line_end FROM chunks ORDER BY path, line_start`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.Path, &c.Heading, &c.Text, &c.LineStart, &c.LineEnd); err != nil {
			return nil, err
		}
		s.Chunks = append(s.Chunks, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	refRows, err := db.Query(`SELECT chunk_id, rule_id, line_no FROM rule_refs ORDER BY chunk_id, line_no`)
	if err != nil {
		return nil, err
	}
	defer refRows.Close()
	for refRows.Next() {
		var ref RuleRef
		if err := refRows.Scan(&ref.ChunkID, &ref.RuleID, &ref.LineNo); err != nil {
			return nil, err
		}
		s.RuleRefs = append(s.RuleRefs, ref)
	}
	return s, refRows.Err()
}

// SaveStore writes index.db atomically.
func SaveStore(stateDir string, s *Store) error {
	db, err := openDB(stateDir)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM chunks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rule_refs`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chunks(id, path, heading, text, line_start, line_end) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range s.Chunks {
		if _, err := stmt.Exec(c.ID, c.Path, c.Heading, c.Text, c.LineStart, c.LineEnd); err != nil {
			return err
		}
	}
	refStmt, err := tx.Prepare(`INSERT INTO rule_refs(chunk_id, rule_id, line_no) VALUES(?, ?, ?)`)
	if err != nil {
		return err
	}
	defer refStmt.Close()
	for _, ref := range s.RuleRefs {
		if _, err := refStmt.Exec(ref.ChunkID, ref.RuleID, ref.LineNo); err != nil {
			return err
		}
	}
	built := s.BuiltAt.UTC()
	if built.IsZero() {
		built = time.Now().UTC()
		s.BuiltAt = built
	}
	if err := metaSet(tx, "fingerprint", s.Fingerprint); err != nil {
		return err
	}
	if err := metaSet(tx, "built_at", built.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if err := metaSet(tx, "file_count", fmt.Sprintf("%d", s.FileCount)); err != nil {
		return err
	}
	return tx.Commit()
}

func loadLegacyJSON(stateDir string) (*Store, error) {
	path := StorePath(stateDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func migrateJSONIfNeeded(stateDir string) error {
	if _, err := os.Stat(DBPath(stateDir)); err == nil {
		return nil
	}
	legacy, err := loadLegacyJSON(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := SaveStore(stateDir, legacy); err != nil {
		return err
	}
	_ = os.Rename(StorePath(stateDir), StorePath(stateDir)+".bak")
	return nil
}

// ChunkByID finds one chunk.
func (s *Store) ChunkByID(id string) (Chunk, bool) {
	for _, c := range s.Chunks {
		if c.ID == id {
			return c, true
		}
	}
	return Chunk{}, false
}

// SearchHit is one API search result.
type SearchHit struct {
	ID      string  `json:"id"`
	Path    string  `json:"path"`
	Heading string  `json:"heading"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// SearchStore runs BM25 and maps to API hits.
func (s *Store) SearchStore(query string, topK int) []SearchHit {
	raw := Search(s.Chunks, query, topK)
	out := make([]SearchHit, 0, len(raw))
	for _, r := range raw {
		out = append(out, SearchHit{
			ID:      r.Chunk.ID,
			Path:    r.Chunk.Path,
			Heading: r.Chunk.Heading,
			Score:   r.Score,
			Snippet: Snippet(r.Chunk.Text, 240),
		})
	}
	return out
}

// SearchStoreMerged runs BM25 for query and queryLocal, deduping chunk ids by max score.
func SearchStoreMerged(st *Store, query, queryLocal string, topK int) []SearchHit {
	if st == nil {
		return nil
	}
	if topK <= 0 {
		topK = 5
	}
	q := strings.TrimSpace(query)
	localQ := strings.TrimSpace(queryLocal)
	if q == "" && localQ == "" {
		return nil
	}
	var merged []SearchHit
	if q != "" {
		merged = mergeSearchHits(merged, st.SearchStore(q, topK*3))
	}
	if localQ != "" && localQ != q {
		merged = mergeSearchHits(merged, st.SearchStore(localQ, topK*3))
	}
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged
}

func mergeSearchHits(a, b []SearchHit) []SearchHit {
	byID := make(map[string]SearchHit, len(a)+len(b))
	for _, h := range a {
		byID[h.ID] = h
	}
	for _, h := range b {
		if prev, ok := byID[h.ID]; !ok || h.Score > prev.Score {
			byID[h.ID] = h
		}
	}
	out := make([]SearchHit, 0, len(byID))
	for _, h := range byID {
		out = append(out, h)
	}
	sortSearchHits(out)
	return out
}

func sortSearchHits(h []SearchHit) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].Score > h[j-1].Score; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}

// EmptyStore returns a zero store for missing index.
func EmptyStore() *Store {
	return &Store{BuiltAt: time.Now().UTC(), Chunks: []Chunk{}}
}

// LoadOrEmpty reads SQLite index or migrates legacy JSON.
func LoadOrEmpty(stateDir string) (*Store, error) {
	if err := migrateJSONIfNeeded(stateDir); err != nil {
		return nil, fmt.Errorf("migrate index: %w", err)
	}
	s, err := LoadStore(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return EmptyStore(), nil
		}
		return nil, fmt.Errorf("load index: %w", err)
	}
	return s, nil
}

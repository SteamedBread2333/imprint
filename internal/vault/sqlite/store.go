package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/sqliteutil"
)

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

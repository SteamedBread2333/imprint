package sqliteutil

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sqlite3 "modernc.org/sqlite"
)

const BusyTimeoutMS = 5000

// Open opens a local SQLite database with the concurrency settings shared by
// vault and shelves. One connection per process keeps writes serialized while
// WAL and busy_timeout coordinate readers and writers across processes.
func Open(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var db *sql.DB
	err = Retry(func() error {
		candidate, err := openOnce(abs)
		if err != nil {
			return err
		}
		db = candidate
		return nil
	})
	return db, err
}

func openOnce(abs string) (*sql.DB, error) {
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := u.Query()
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", BusyTimeoutMS))
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := verifyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Retry reruns short SQLite operations that failed because another process
// holds a transient database lock. Constraint and validation errors are not
// retried.
func Retry(fn func() error) error {
	var err error
	delay := 20 * time.Millisecond
	for attempt := 0; attempt < 8; attempt++ {
		err = fn()
		if err == nil || !IsBusy(err) {
			return err
		}
		if attempt == 7 {
			break
		}
		time.Sleep(delay)
		if delay < 250*time.Millisecond {
			delay *= 2
		}
	}
	return err
}

// IsBusy reports SQLite BUSY/LOCKED errors, including extended result codes.
func IsBusy(err error) bool {
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code() & 0xff
		return code == 5 || code == 6
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") || strings.Contains(s, "sqlite_busy") || strings.Contains(s, "sqlite_locked")
}

func verifyPragmas(db *sql.DB) error {
	var journal string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
		return fmt.Errorf("read SQLite journal_mode: %w", err)
	}
	if !strings.EqualFold(journal, "wal") {
		return fmt.Errorf("SQLite journal_mode=%q, want WAL", journal)
	}
	var timeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		return fmt.Errorf("read SQLite busy_timeout: %w", err)
	}
	if timeout < BusyTimeoutMS {
		return fmt.Errorf("SQLite busy_timeout=%s, want at least %d", strconv.Itoa(timeout), BusyTimeoutMS)
	}
	return nil
}

package sqliteutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAppliesConcurrencyPragmas(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var journal string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journal, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journal)
	}
	var timeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatal(err)
	}
	if timeout < BusyTimeoutMS {
		t.Fatalf("busy_timeout = %d, want >= %d", timeout, BusyTimeoutMS)
	}
	var synchronous int
	if err := db.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	// SQLite NORMAL is 1.
	if synchronous != 1 {
		t.Fatalf("synchronous = %d, want NORMAL (1)", synchronous)
	}
}

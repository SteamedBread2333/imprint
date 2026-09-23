package imprint

import (
	"testing"
	"time"
)

func linkVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(OpenOptions{
		Dir: t.TempDir(),
		Now: func() time.Time { return day(0) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestLinkAddsConflictAndRelatedEdges(t *testing.T) {
	v := linkVault(t)
	a, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.Add("Use spaces for indentation", []string{"go", "style"}, "spaces", 0.8)
	if err != nil {
		t.Fatal(err)
	}

	res, err := v.Link(a.ID, nil, []string{b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if res.ConflictsAdded != 1 {
		t.Fatalf("conflicts added = %d, want 1", res.ConflictsAdded)
	}
	// Idempotent: a second link creates nothing new.
	res, err = v.Link(a.ID, nil, []string{b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if res.ConflictsAdded != 0 {
		t.Fatalf("second link added = %d, want 0", res.ConflictsAdded)
	}

	// The edge is visible to conflict lookup and find penalization.
	pairs, err := v.ConflictsAmong([]string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("conflict pairs = %d, want 1", len(pairs))
	}
	hits, err := v.Find([]string{"go", "style"}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0].Score <= hits[1].Score {
		t.Fatalf("conflict loser was not penalized: %v >= %v", hits[1].Score, hits[0].Score)
	}
}

func TestLinkRejectsUnknownIDs(t *testing.T) {
	v := linkVault(t)
	a, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Link(a.ID, []string{"r-1900-01-01-000"}, nil); err == nil {
		t.Fatal("link to a missing rule was accepted")
	}
	if _, err := v.Link("r-1900-01-01-000", nil, []string{a.ID}); err == nil {
		t.Fatal("link from a missing rule was accepted")
	}
	if _, err := v.Link(a.ID, nil, nil); err == nil {
		t.Fatal("empty link was accepted")
	}
}

func TestAddWithConflictsParam(t *testing.T) {
	v := linkVault(t)
	a, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.8)
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.AddRecord("Use spaces for indentation", []string{"go", "style"}, "spaces", 0.8, nil, nil, []string{a.ID}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := v.ConflictsAmong([]string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("conflict pairs = %d, want 1", len(pairs))
	}
}

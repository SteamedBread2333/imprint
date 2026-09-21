package imprint

import (
	"errors"
	"testing"
)

func TestAddRejectsDuplicateActiveClaim(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	first, err := v.Add("Always use pnpm workspaces", []string{"js", "packages"}, "team policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Add("Use pnpm workspaces always", []string{"js"}, "repeated policy", 0.6)
	var guarded *WriteGuardError
	if !errors.As(err, &guarded) {
		t.Fatalf("error = %v, want WriteGuardError", err)
	}
	if guarded.Code != "duplicate" || len(guarded.Candidates) != 1 || guarded.Candidates[0].ID != first.ID {
		t.Fatalf("guard = %+v", guarded)
	}
}

func TestAddAllowsSameClaimInUnrelatedScope(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Keep names concise", []string{"go"}, "go policy", 0.6); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Keep names concise", []string{"design"}, "design policy", 0.6); err != nil {
		t.Fatalf("unrelated scope rejected: %v", err)
	}
}

func TestWritePrivacyGuards(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Store credential", []string{"security"}, "password=hunter2", 0.6); err == nil {
		t.Fatal("secret add was accepted")
	}
	added, err := v.Add("Keep credentials external", []string{"security"}, "team policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Reinforce(added.ID, "contact dev@example.com"); err == nil {
		t.Fatal("email evidence was accepted")
	}
	if _, err := v.Supersede(added.ID, "Use private host", []string{"security"}, "host 10.0.0.8", ""); err == nil {
		t.Fatal("private IP supersede reason was accepted")
	}
	if _, err := v.AddWithSources("Cite safe docs", []string{"docs"}, "policy", 0.6, []DocRef{{Path: "../.env"}}); err == nil {
		t.Fatal("unsafe source path was accepted")
	}
}

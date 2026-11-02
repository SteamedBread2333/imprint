package linking_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestFindShapes(t *testing.T) {
	v, err := imprint.Open(imprint.OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddWithSources("Use table-driven tests in Go", []string{"go"}, "text", 0.6, []imprint.DocRef{
		{Path: "docs/testing.md", Heading: "Style"},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := linking.Find(v, nil, linking.FindRequest{Scope: []string{"go"}, Query: "table-driven"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Docs != nil || out.Rules != nil || len(out.Hits) != 1 {
		t.Fatalf("no shelves: %+v", out)
	}

	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "docs", "testing.md"), []byte("# Style\n\nPrefer table-driven tests.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := shelves.New(shelves.Config{Enabled: true, Roots: []string{"docs"}, Workspace: ws, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	out, err = linking.Find(v, svc, linking.FindRequest{Scope: []string{"go"}, Query: "table-driven"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Docs == nil || len(out.Docs.Rules) != 1 || len(out.Docs.Documents) == 0 {
		t.Fatalf("shelves query: %+v", out)
	}
	if hits := out.RuleHits(); len(hits) != 1 || hits[0].Title == "" {
		t.Fatalf("RuleHits = %+v", hits)
	}

	out, err = linking.Find(v, svc, linking.FindRequest{Scope: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Docs != nil || len(out.Rules) != 1 || len(out.Rules[0].ResolvedSources) != 1 {
		t.Fatalf("scope-only with index: %+v", out)
	}
}

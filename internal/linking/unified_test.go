package linking_test

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestBuildUnifiedGraphSourcesAndCitedBy(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(imprint.OpenOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	added, err := v.AddWithSources("Use snake_case", []string{"python"}, "text", 0.6, []imprint.DocRef{
		{Path: "docs/style.md", Heading: "Naming"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cited, err := v.Add("Cited in doc", []string{"demo"}, "cited", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "abc123def4567890", Path: "docs/style.md", Heading: "Naming", Text: "naming rules", LineStart: 1, LineEnd: 5},
		},
		RuleRefs: []index.RuleRef{
			{ChunkID: "abc123def4567890", RuleID: cited.ID, LineNo: 3},
		},
	}
	g, err := linking.BuildUnifiedGraph(v, st, false)
	if err != nil {
		t.Fatal(err)
	}
	hasSources := false
	hasCitedBy := false
	for _, e := range g.Edges {
		if e.Kind == "sources" && e.Source == added.ID && e.Target == "abc123def4567890" {
			hasSources = true
		}
		if e.Kind == "cited_by" && e.Source == "abc123def4567890" && e.Target == cited.ID {
			hasCitedBy = true
		}
	}
	if !hasSources {
		t.Fatalf("missing sources edge: %+v", g.Edges)
	}
	if !hasCitedBy {
		t.Fatalf("missing cited_by edge: %+v", g.Edges)
	}
	kinds := map[string]int{}
	for _, n := range g.Nodes {
		kinds[n.Kind]++
	}
	if kinds["rule"] < 1 || kinds["chunk"] < 1 || kinds["file"] < 1 {
		t.Fatalf("nodes = %+v", kinds)
	}
}

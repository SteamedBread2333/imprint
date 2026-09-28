package index

import "testing"

func TestSearchStoreMergedSizedPathsFilter(t *testing.T) {
	st := &Store{Chunks: []Chunk{
		{ID: "a1", Path: "docs/a.md", Heading: "A", Text: "alpha token"},
		{ID: "b1", Path: "docs/b.md", Heading: "B", Text: "alpha token"},
		{ID: "c1", Path: "docs/c.md", Heading: "C", Text: "alpha token"},
	}}
	all := SearchStoreMergedSized(st, "alpha", "", 3, 80, nil)
	if len(all) != 3 {
		t.Fatalf("unfiltered = %+v", all)
	}
	narrow := SearchStoreMergedSized(st, "alpha", "", 3, 80, []string{"docs/a.md", "docs/b.md"})
	if len(narrow) != 2 {
		t.Fatalf("narrow = %+v", narrow)
	}
	for _, h := range narrow {
		if h.Path != "docs/a.md" && h.Path != "docs/b.md" {
			t.Fatalf("out of set: %q", h.Path)
		}
	}
	empty := SearchStoreMergedSized(st, "alpha", "", 3, 80, []string{"internal/foo.go"})
	if len(empty) != 0 {
		t.Fatalf("non-index path should yield no hits: %+v", empty)
	}
}

func TestNormalizeDocPathAlignsRelative(t *testing.T) {
	if NormalizeDocPath("./docs/x.md") != "docs/x.md" {
		t.Fatal(NormalizeDocPath("./docs/x.md"))
	}
	st := &Store{Chunks: []Chunk{
		{ID: "x1", Path: "docs/x.md", Text: "needle here"},
	}}
	hits := SearchStoreMergedSized(st, "needle", "", 1, 80, []string{"x.md"})
	if len(hits) != 1 || hits[0].Path != "docs/x.md" {
		t.Fatalf("basename align: %+v", hits)
	}
}

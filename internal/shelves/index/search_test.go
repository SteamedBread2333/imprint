package index

import "testing"

func TestSearchStoreMergedCapsAndDedupsPath(t *testing.T) {
	st := &Store{Chunks: []Chunk{
		{ID: "a1", Path: "docs/a.md", Heading: "One", Text: "alpha token here"},
		{ID: "a2", Path: "docs/a.md", Heading: "Two", Text: "alpha token again"},
		{ID: "b1", Path: "docs/b.md", Heading: "B", Text: "alpha in b"},
		{ID: "c1", Path: "docs/c.md", Heading: "C", Text: "alpha in c"},
	}}
	hits := SearchStoreMergedSized(st, "alpha", "", 2, 80)
	if len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Path == hits[1].Path {
		t.Fatalf("same path twice: %+v", hits)
	}
	for _, h := range hits {
		if n := len([]rune(h.Snippet)); n > 80 && !hasEllipsis(h.Snippet) {
			t.Fatalf("snippet too long: %q", h.Snippet)
		}
	}
}

func hasEllipsis(s string) bool {
	return len(s) > 0 && (s[0] == 0xe2 || s[len(s)-1] == 0x26 || containsEllipsis(s))
}

func containsEllipsis(s string) bool {
	for _, r := range s {
		if r == '…' {
			return true
		}
	}
	return false
}

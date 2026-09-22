package index

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSearchHeadingOutranksBody(t *testing.T) {
	chunks := []Chunk{
		{ID: "body", Path: "docs/a.md", Heading: "Intro", Text: "vitest appears in the body once"},
		{ID: "head", Path: "docs/b.md", Heading: "vitest", Text: "unrelated paragraph about storage"},
	}
	hits := Search(chunks, "vitest", 5)
	if len(hits) < 2 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Chunk.ID != "head" {
		t.Fatalf("want heading hit first, got %+v", hits)
	}
}

func TestQuerySnippetCentersOnTerm(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("padding ")
	}
	b.WriteString("queryterm sits here ")
	for i := 0; i < 40; i++ {
		b.WriteString("tail ")
	}
	got := QuerySnippet(b.String(), "queryterm", DefaultSnippetRunes)
	if !strings.Contains(got, "queryterm") {
		t.Fatalf("snippet missing query term: %q", got)
	}
	n := utf8.RuneCountInString(strings.Trim(got, "…"))
	if n > DefaultSnippetRunes {
		t.Fatalf("snippet runes %d > %d (%q)", n, DefaultSnippetRunes, got)
	}
}

func TestQuerySnippetPrefixWhenNoTerm(t *testing.T) {
	got := QuerySnippet("abcdefghij", "zzzz", 4)
	if got != "abcd…" {
		t.Fatalf("got %q", got)
	}
}

package textseg

import (
	"slices"
	"testing"
)

func TestTokenizeChinese(t *testing.T) {
	toks := Tokenize("切忌否定式堆砌")
	if len(toks) == 0 {
		t.Fatal("expected tokens")
	}
	if !slices.Contains(toks, "否定") && !slices.Contains(toks, "堆砌") {
		t.Fatalf("expected word segments, got %v", toks)
	}
}

func TestTokenizeEnglish(t *testing.T) {
	toks := Tokenize("storage retrieval vault")
	if !slices.Contains(toks, "storage") || !slices.Contains(toks, "retrieval") {
		t.Fatalf("expected english tokens, got %v", toks)
	}
}

func TestTokenizeDedup(t *testing.T) {
	toks := Tokenize("test test test")
	if len(toks) != 1 || toks[0] != "test" {
		t.Fatalf("expected deduped token, got %v", toks)
	}
}

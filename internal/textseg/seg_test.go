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

func TestTokenizeIdentifierVariants(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"camelCase", []string{"camelcase", "camel", "case"}},
		{"CamelCase", []string{"camelcase", "camel", "case"}},
		{"HTTPServer", []string{"httpserver", "http", "server"}},
		{"snake_case", []string{"snakecase", "snake", "case"}},
		{"kebab-case", []string{"kebabcase", "kebab", "case"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Tokenize(tt.input)
			for _, want := range tt.want {
				if !slices.Contains(got, want) {
					t.Errorf("Tokenize(%q) = %v, missing %q", tt.input, got, want)
				}
			}
		})
	}
}

func TestTokenizeIdentifierFormsShareTerms(t *testing.T) {
	pairs := [][2]string{
		{"camelCase", "Camel Case"},
		{"HTTPServer", "HTTP Server"},
		{"snake_case", "snake case"},
		{"kebab-case", "kebab case"},
	}
	for _, pair := range pairs {
		left, right := Tokenize(pair[0]), Tokenize(pair[1])
		shared := false
		for _, token := range left {
			if slices.Contains(right, token) {
				shared = true
				break
			}
		}
		if !shared {
			t.Errorf("%q tokens %v do not overlap %q tokens %v", pair[0], left, pair[1], right)
		}
	}
}

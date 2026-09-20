package imprint

import (
	"strings"
	"unicode"
)

// HasCJK reports whether s contains Han or kana runes.
func HasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// EffectiveQueryLocal returns the local-language query for shelves/vault local BM25 pass.
func EffectiveQueryLocal(query, queryLocal string) string {
	if ql := strings.TrimSpace(queryLocal); ql != "" {
		return ql
	}
	q := strings.TrimSpace(query)
	if q != "" && HasCJK(q) {
		return q
	}
	return ""
}

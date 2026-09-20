package imprint

import (
	"strings"
	"unicode"

	"github.com/SteamedBread2333/imprint/internal/textseg"
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

// MergeQueryLocalForStore combines agent-provided query_local with gse tokens from CJK evidence
// so numeric and domain terms from user speech are not dropped on write.
func MergeQueryLocalForStore(agentQL, evidence string) string {
	agentQL = strings.TrimSpace(agentQL)
	evidence = strings.TrimSpace(evidence)
	if evidence == "" || !HasCJK(evidence) {
		return agentQL
	}
	evidenceToks := textseg.Tokenize(evidence)
	if len(evidenceToks) == 0 {
		return agentQL
	}
	if agentQL == "" {
		return strings.Join(evidenceToks, " ")
	}
	seen := map[string]struct{}{}
	var parts []string
	for _, t := range textseg.Tokenize(agentQL) {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		parts = append(parts, t)
	}
	for _, t := range evidenceToks {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		parts = append(parts, t)
	}
	return strings.Join(parts, " ")
}

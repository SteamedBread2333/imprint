// Package textseg tokenizes text for BM25 using go-ego/gse (search mode).
package textseg

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/go-ego/gse"
)

var (
	once sync.Once
	seg  *gse.Segmenter
)

func segmenter() *gse.Segmenter {
	once.Do(func() {
		s, err := gse.NewEmbed("zh", "en")
		if err != nil {
			panic("textseg: load gse dictionary: " + err.Error())
		}
		seg = &s
	})
	return seg
}

// Tokenize splits s into BM25 terms. Uses gse CutSearch (search engine mode).
func Tokenize(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	raw := segmenter().CutSearch(s, true)
	raw = append(raw, identifierTerms(s)...)
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, tok := range raw {
		tok = strings.TrimSpace(strings.ToLower(tok))
		if tok == "" || !keepToken(tok) {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return out
}

func identifierTerms(s string) []string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})
	var out []string
	for _, word := range words {
		parts, split := splitIdentifier(word)
		if !split {
			continue
		}
		full := strings.Map(func(r rune) rune {
			if r == '_' || r == '-' {
				return -1
			}
			return unicode.ToLower(r)
		}, word)
		if full != "" {
			out = append(out, full)
		}
		for _, part := range parts {
			if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func splitIdentifier(s string) ([]string, bool) {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil, false
	}
	var parts []string
	start := 0
	split := false
	flush := func(end int) {
		if end > start {
			parts = append(parts, string(runes[start:end]))
		}
	}
	for i, r := range runes {
		if r == '_' || r == '-' {
			flush(i)
			start = i + 1
			split = true
			continue
		}
		if i <= start {
			continue
		}
		prev := runes[i-1]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		lowerToUpper := (unicode.IsLower(prev) || unicode.IsDigit(prev)) && unicode.IsUpper(r)
		acronymBoundary := unicode.IsUpper(prev) && unicode.IsUpper(r) && next != 0 && unicode.IsLower(next)
		letterDigitBoundary := unicode.IsLetter(prev) != unicode.IsLetter(r) &&
			(unicode.IsDigit(prev) || unicode.IsDigit(r))
		if lowerToUpper || acronymBoundary || letterDigitBoundary {
			flush(i)
			start = i
			split = true
		}
	}
	flush(len(runes))
	return parts, split
}

// LoadGlossary loads optional domain terms (word or word<TAB>freq per line) into gse.
func LoadGlossary(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	var dict []map[string]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		entry := map[string]string{"text": parts[0], "freq": "100"}
		if len(parts) > 1 {
			entry["freq"] = parts[1]
		}
		dict = append(dict, entry)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(dict) == 0 {
		return nil
	}
	if err := segmenter().LoadDictMap(dict); err != nil {
		return fmt.Errorf("textseg glossary %s: %w", path, err)
	}
	return nil
}

func keepToken(tok string) bool {
	if len(tok) >= 2 {
		return true
	}
	for _, r := range tok {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

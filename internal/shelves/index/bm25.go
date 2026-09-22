package index

import (
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/SteamedBread2333/imprint/internal/textseg"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

type scored struct {
	Chunk Chunk
	Score float64
}

type fieldToks struct {
	heading []string
	path    []string
	body    []string
}

func chunkFields(c Chunk) fieldToks {
	return fieldToks{
		heading: textseg.Tokenize(c.Heading),
		path:    textseg.Tokenize(filepath.Base(c.Path)),
		body:    textseg.Tokenize(c.Text),
	}
}

func (f fieldToks) weightedTF(term string) float64 {
	return headingFieldWeight*countTerm(f.heading, term) +
		pathFieldWeight*countTerm(f.path, term) +
		bodyFieldWeight*countTerm(f.body, term)
}

func (f fieldToks) weightedLen() float64 {
	return headingFieldWeight*float64(len(f.heading)) +
		pathFieldWeight*float64(len(f.path)) +
		bodyFieldWeight*float64(len(f.body))
}

func countTerm(tokens []string, term string) float64 {
	n := 0.0
	for _, t := range tokens {
		if t == term {
			n++
		}
	}
	return n
}

// Search runs field-weighted BM25 over indexed chunks (heading > path > body).
func Search(chunks []Chunk, query string, topK int) []scored {
	if topK <= 0 {
		topK = DefaultFindTopK
	}
	terms := textseg.Tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	n := len(chunks)
	df := map[string]int{}
	fields := make([]fieldToks, n)
	var avgdl float64
	for i, c := range chunks {
		f := chunkFields(c)
		fields[i] = f
		avgdl += f.weightedLen()
		seen := map[string]struct{}{}
		for _, t := range append(append(append([]string{}, f.heading...), f.path...), f.body...) {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			df[t]++
		}
	}
	if n > 0 {
		avgdl /= float64(n)
	}
	var hits []scored
	for i, c := range chunks {
		f := fields[i]
		dl := f.weightedLen()
		score := 0.0
		for _, term := range terms {
			tf := f.weightedTF(term)
			if tf == 0 {
				continue
			}
			dfi := df[term]
			if dfi == 0 {
				continue
			}
			idf := math.Log(1 + (float64(n)-float64(dfi)+0.5)/(float64(dfi)+0.5))
			denom := tf + bm25K1*(1-bm25B+bm25B*dl/avgdl)
			score += idf * (tf * (bm25K1 + 1)) / denom
		}
		if score > 0 {
			hits = append(hits, scored{Chunk: c, Score: score})
		}
	}
	sortScored(hits)
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

func sortScored(h []scored) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].Score > h[j-1].Score; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}

// Snippet truncates text from the start for search previews.
func Snippet(text string, max int) string {
	return QuerySnippet(text, "", max)
}

// QuerySnippet keeps up to max runes centered on the earliest query term hit.
func QuerySnippet(text, query string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 {
		max = DefaultSnippetRunes
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= max {
		return text
	}
	start := 0
	if hit := firstQueryRune(runes, query); hit >= 0 {
		start = hit - max/2
		if start < 0 {
			start = 0
		}
		if start+max > len(runes) {
			start = len(runes) - max
		}
	}
	out := string(runes[start : start+max])
	if start > 0 {
		out = "…" + out
	}
	if start+max < len(runes) {
		out += "…"
	}
	return out
}

func firstQueryRune(runes []rune, query string) int {
	terms := snippetTerms(query)
	if len(terms) == 0 {
		return -1
	}
	lower := strings.ToLower(string(runes))
	best := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 {
			ri := utf8.RuneCountInString(lower[:i])
			if best < 0 || ri < best {
				best = ri
			}
		}
	}
	return best
}

func snippetTerms(query string) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, t := range textseg.Tokenize(query) {
		add(t)
	}
	for _, w := range strings.Fields(query) {
		add(w)
	}
	return out
}

package index

import (
	"math"
	"strings"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

type scored struct {
	Chunk Chunk
	Score float64
}

// Search runs BM25 over indexed chunks.
func Search(chunks []Chunk, query string, topK int) []scored {
	if topK <= 0 {
		topK = 5
	}
	terms := Tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	n := len(chunks)
	df := map[string]int{}
	var avgdl float64
	docTokens := make([][]string, n)
	for i, c := range chunks {
		toks := Tokenize(c.Heading + " " + c.Text)
		docTokens[i] = toks
		avgdl += float64(len(toks))
		seen := map[string]struct{}{}
		for _, t := range toks {
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
		toks := docTokens[i]
		dl := float64(len(toks))
		score := 0.0
		for _, term := range terms {
			tf := 0.0
			for _, t := range toks {
				if t == term {
					tf++
				}
			}
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

// Snippet truncates text for search previews.
func Snippet(text string, max int) string {
	text = strings.TrimSpace(text)
	if len(text) <= max {
		return text
	}
	return text[:max] + "…"
}

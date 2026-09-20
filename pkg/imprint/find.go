package imprint

import (
	"math"
	"sort"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/textseg"
)

const (
	bm25K1         = 1.2
	bm25B          = 0.75
	weightClaim    = 3.0
	weightScope    = 2.0
	weightEvidence = 1.5
	weightBody     = 1.0
)

func tokenize(s string) []string {
	return textseg.Tokenize(s)
}

func evidenceText(r *Record) string {
	var b strings.Builder
	for _, e := range r.EvidenceLog {
		b.WriteString(e.Text)
		b.WriteByte(' ')
	}
	return b.String()
}

func fieldTokens(rec *Record) (claim, scope, evidence, queryLocal, body []string) {
	return tokenize(rec.Claim),
		tokenize(strings.Join(rec.Scope, " ")),
		tokenize(evidenceText(rec)),
		tokenize(rec.QueryLocal),
		tokenize(rec.Body)
}

func weightedLen(claim, scope, evidence, queryLocal, body []string) float64 {
	return weightClaim*float64(len(claim)) +
		weightScope*float64(len(scope)) +
		weightEvidence*float64(len(evidence)) +
		weightEvidence*float64(len(queryLocal)) +
		weightBody*float64(len(body))
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

type termIndex struct {
	n     int
	df    map[string]int
	avgdl float64
}

func buildIndex(recs []*Record) *termIndex {
	idx := &termIndex{df: map[string]int{}}
	if len(recs) == 0 {
		return idx
	}
	var totalDL float64
	for _, rec := range recs {
		claim, scope, evidence, queryLocal, body := fieldTokens(rec)
		totalDL += weightedLen(claim, scope, evidence, queryLocal, body)
		seen := map[string]struct{}{}
		for _, group := range [][]string{claim, scope, evidence, queryLocal, body} {
			for _, t := range group {
				seen[t] = struct{}{}
			}
		}
		for t := range seen {
			idx.df[t]++
		}
		idx.n++
	}
	idx.avgdl = totalDL / float64(idx.n)
	if idx.avgdl <= 0 {
		idx.avgdl = 1
	}
	return idx
}

func (idx *termIndex) idf(term string) float64 {
	n := float64(idx.n)
	df := float64(idx.df[term])
	return math.Log(1 + (n-df+0.5)/(df+0.5))
}

func (idx *termIndex) score(rec *Record, query string) float64 {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0
	}
	terms := tokenize(q)
	claim, scope, evidence, queryLocal, body := fieldTokens(rec)
	hay := strings.ToLower(rec.Claim + " " + evidenceText(rec) + " " + rec.QueryLocal + " " + rec.Body + " " + strings.Join(rec.Scope, " "))
	if len(terms) == 0 {
		if strings.Contains(hay, strings.ToLower(q)) {
			return 0.5
		}
		return 0
	}
	dl := weightedLen(claim, scope, evidence, queryLocal, body)
	if dl <= 0 {
		dl = 1
	}
	avgdl := idx.avgdl
	if avgdl <= 0 {
		avgdl = 1
	}
	raw := 0.0
	for _, term := range terms {
		tf := weightClaim*countTerm(claim, term) +
			weightScope*countTerm(scope, term) +
			weightEvidence*countTerm(evidence, term) +
			weightEvidence*countTerm(queryLocal, term) +
			weightBody*countTerm(body, term)
		if tf == 0 {
			continue
		}
		denom := tf + bm25K1*(1-bm25B+bm25B*dl/avgdl)
		raw += idx.idf(term) * tf * (bm25K1 + 1) / denom
	}
	if raw == 0 {
		if strings.Contains(hay, strings.ToLower(q)) {
			return 0.4
		}
		return 0
	}
	s := raw / (raw + 1)
	if s > 1 {
		return 1
	}
	return s
}

func queryScore(rec *Record, query string) float64 {
	return buildIndex([]*Record{rec}).score(rec, query)
}

func scopeScore(recScope, queryScope []string) (score float64, matched int) {
	if len(queryScope) == 0 {
		return 0, 0
	}
	set := make(map[string]struct{}, len(recScope))
	for _, s := range recScope {
		set[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range queryScope {
		if _, ok := set[strings.ToLower(s)]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(queryScope)), matched
}

func rankScore(rec *Record, scope []string, query string, idx *termIndex) (float64, bool) {
	hasScope := len(scope) > 0
	hasQuery := strings.TrimSpace(query) != ""
	ss, matched := scopeScore(rec.Scope, scope)
	qs := 0.0
	if hasQuery {
		if idx == nil {
			qs = queryScore(rec, query)
		} else {
			qs = idx.score(rec, query)
		}
	}

	switch {
	case hasScope && matched != len(scope):
		return 0, false
	case hasQuery && qs == 0:
		return 0, false
	}

	var s float64
	switch {
	case hasScope && hasQuery:
		s = 0.7*ss + 0.3*qs
	case hasScope:
		s = ss
	case hasQuery:
		s = qs
	default:
		s = rec.Confidence
	}
	s = 0.9*s + 0.1*rec.Confidence
	if s < 0 {
		s = 0
	}
	if s > 1 {
		s = 1
	}
	return math.Round(s*10000) / 10000, true
}

// Find ranks active rules with confidence >= 0.3.
// Scope is an AND filter: every requested tag must be present.
// Query is optional BM25 ranking over claim, scope, evidence, query_local, and body.
func (v *Vault) Find(scope []string, query string, topK int) ([]FindHit, error) {
	return v.findRanked(scope, query, topK)
}

// FindMerged runs BM25 for query and effective query_local, merging by rule id (max score).
func (v *Vault) FindMerged(scope []string, query, queryLocal string, topK int) ([]FindHit, error) {
	if topK <= 0 {
		topK = DefaultTopK
	}
	q := strings.TrimSpace(query)
	localQ := EffectiveQueryLocal(query, queryLocal)
	if q == "" && localQ == "" {
		return v.Find(scope, "", topK)
	}
	if localQ == "" || localQ == q {
		return v.Find(scope, q, topK)
	}
	h1, err := v.findRanked(scope, q, topK*3)
	if err != nil {
		return nil, err
	}
	h2, err := v.findRanked(scope, localQ, topK*3)
	if err != nil {
		return nil, err
	}
	merged := mergeFindHits(h1, h2)
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}

func mergeFindHits(a, b []FindHit) []FindHit {
	byID := make(map[string]FindHit, len(a)+len(b))
	for _, h := range a {
		byID[h.ID] = h
	}
	for _, h := range b {
		if prev, ok := byID[h.ID]; !ok || h.Score > prev.Score {
			byID[h.ID] = h
		}
	}
	out := make([]FindHit, 0, len(byID))
	for _, h := range byID {
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func (v *Vault) findRanked(scope []string, query string, topK int) ([]FindHit, error) {
	if topK <= 0 {
		topK = DefaultTopK
	}
	scope = cleanScope(scope)
	corpus, err := v.store.ActiveCandidates(scope, MinRecallConfidence)
	if err != nil {
		return nil, err
	}
	idx := buildIndex(corpus)
	type scored struct {
		rec   *Record
		score float64
	}
	var hits []scored
	for _, r := range corpus {
		s, ok := rankScore(r, scope, query, idx)
		if !ok {
			continue
		}
		hits = append(hits, scored{rec: r, score: s})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].rec.Confidence != hits[j].rec.Confidence {
			return hits[i].rec.Confidence > hits[j].rec.Confidence
		}
		return hits[i].rec.LastTouchedAt.After(hits[j].rec.LastTouchedAt)
	})
	if len(hits) > topK {
		hits = hits[:topK]
	}
	out := make([]FindHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, FindHit{
			ID:         h.rec.ID,
			Title:      h.rec.Title(),
			Scope:      h.rec.Scope,
			Confidence: h.rec.Confidence,
			Score:      h.score,
			QueryLocal: h.rec.QueryLocal,
		})
	}
	return out, nil
}

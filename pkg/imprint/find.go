package imprint

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var out []string
	for _, f := range fields {
		if len(f) <= 1 {
			continue
		}
		out = append(out, f)
	}
	return out
}

func evidenceText(r *Record) string {
	var b strings.Builder
	for _, e := range r.EvidenceLog {
		b.WriteString(e.Text)
		b.WriteByte(' ')
	}
	return b.String()
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

func queryScore(rec *Record, query string) float64 {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0
	}
	tokens := tokenize(q)
	hay := strings.ToLower(rec.Claim + " " + evidenceText(rec) + " " + rec.Body)
	if len(tokens) == 0 {
		if strings.Contains(hay, strings.ToLower(q)) {
			return 0.5
		}
		return 0
	}
	set := make(map[string]struct{})
	for _, t := range tokenize(hay) {
		set[t] = struct{}{}
	}
	hit := 0
	for _, t := range tokens {
		if _, ok := set[t]; ok {
			hit++
		}
	}
	score := float64(hit) / float64(len(tokens))
	if score == 0 && strings.Contains(hay, strings.ToLower(q)) {
		return 0.4
	}
	return score
}

func rankScore(rec *Record, scope []string, query string) (float64, bool) {
	hasScope := len(scope) > 0
	hasQuery := strings.TrimSpace(query) != ""
	ss, matched := scopeScore(rec.Scope, scope)
	qs := queryScore(rec, query)

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
// Query is optional lexical ranking over claim + evidence.
func (v *Vault) Find(scope []string, query string, topK int) ([]FindHit, error) {
	if topK <= 0 {
		topK = DefaultTopK
	}
	scope = cleanScope(scope)
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	type scored struct {
		rec   *Record
		score float64
	}
	var hits []scored
	for _, r := range recs {
		if r.Status != StatusActive || r.Confidence < MinRecallConfidence {
			continue
		}
		s, ok := rankScore(r, scope, query)
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
		})
	}
	return out, nil
}

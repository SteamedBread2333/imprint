package imprint

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/privacy"
	"github.com/SteamedBread2333/imprint/internal/textseg"
)

const duplicateClaimThreshold = 0.82

type guardedText struct {
	field string
	text  string
}

func checkSensitiveText(values ...guardedText) error {
	for _, value := range values {
		if finding := privacy.CheckText(value.text); finding != nil {
			return &WriteGuardError{
				Code:    "privacy_rejected",
				Message: "write rejected because it may contain sensitive data",
				Field:   value.field,
				Kind:    finding.Kind,
			}
		}
	}
	return nil
}

func (v *Vault) checkSourcePaths(sources []DocRef) error {
	root, _ := FindProjectRootFromVault(v.Dir)
	for _, source := range sources {
		if finding := privacy.CheckSourcePath(root, source.Path); finding != nil {
			return &WriteGuardError{
				Code:    "privacy_rejected",
				Message: "source path is not allowed",
				Field:   "sources.path",
				Kind:    finding.Kind,
			}
		}
	}
	return nil
}

func (v *Vault) duplicateCandidates(claim string, scope []string) ([]DuplicateCandidate, error) {
	want := tokenSet(textseg.Tokenize(claim))
	if len(want) == 0 {
		return nil, nil
	}
	records, err := v.store.ActiveCandidates(nil, 0)
	if err != nil {
		return nil, err
	}
	var out []DuplicateCandidate
	for _, rec := range records {
		if !scopesOverlap(scope, rec.Scope) {
			continue
		}
		got := tokenSet(textseg.Tokenize(rec.Claim))
		score := 0.0
		if strings.EqualFold(strings.TrimSpace(claim), strings.TrimSpace(rec.Claim)) {
			score = 1
		} else {
			// Very short token sets are dominated by dropped one-letter terms
			// and generic verbs, which makes Jaccard too eager.
			if len(want) < 3 || len(got) < 3 {
				continue
			}
			score = jaccard(want, got)
		}
		if score >= duplicateClaimThreshold {
			out = append(out, DuplicateCandidate{ID: rec.ID, Score: score, Claim: rec.Claim})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}

// MergeEmbedDuplicates adds semantic near-neighbours to the lexical duplicate
// candidates. It returns either:
//   - a WriteGuardError when a semantic duplicate is confident enough to reject
//     the write, or
//   - nil with advisory candidates when cosine is high but polarity differs,
//     because cosine cannot see negation ("never", "instead of") and antonym
//     pairs score as high as true restatements.
//
// When the embedder is absent, unreachable, or errors, the lexical candidates
// pass through unchanged — this path must never block a write.
func (v *Vault) MergeEmbedDuplicates(claim string, lexical []DuplicateCandidate) ([]DuplicateCandidate, []DuplicateCandidate, error) {
	trace := newTraceID()
	q, ok := v.embedClaim(claim, trace)
	return v.mergeEmbedDuplicatesVec(claim, q, ok, lexical, trace)
}

// MergeEmbedDuplicatesVec is MergeEmbedDuplicates with the query vector
// already computed, so a write path that also stores the vector embeds the
// claim exactly once. ok=false (no embedder / sidecar down / timeout) passes
// the lexical candidates through unchanged.
func (v *Vault) MergeEmbedDuplicatesVec(claim string, query []float32, ok bool, lexical []DuplicateCandidate) ([]DuplicateCandidate, []DuplicateCandidate, error) {
	return v.mergeEmbedDuplicatesVec(claim, query, ok, lexical, newTraceID())
}

// mergeEmbedDuplicatesVec is the traced core of the semantic duplicate gate.
//
// The comparison is O(N) over every active+dormant rule vector — fine for a
// personal or project vault (thousands of rules), but document the ceiling:
// beyond that, move this to an ANN index (e.g. sqlite-vec) instead of a scan.
func (v *Vault) mergeEmbedDuplicatesVec(claim string, query []float32, ok bool, lexical []DuplicateCandidate, trace string) ([]DuplicateCandidate, []DuplicateCandidate, error) {
	if v.embed == nil || !ok {
		return lexical, nil, nil
	}
	start := time.Now()
	vectors, err := v.store.RuleVectors(v.embed.Model())
	if err != nil {
		v.emit(TelemetryEvent{Op: "embed_unavailable", Code: "vector_read_failed", LatencyMS: time.Since(start).Milliseconds()}, trace)
		return lexical, nil, nil
	}
	if len(vectors) == 0 {
		return lexical, nil, nil
	}
	// Claims are needed to render candidates; the pool is active + dormant
	// rules (a dormant rule is still the same policy, just low-confidence).
	records, err := v.store.ActiveCandidates(nil, 0)
	if err != nil {
		v.emit(TelemetryEvent{Op: "embed_unavailable", Code: "vector_read_failed", LatencyMS: time.Since(start).Milliseconds()}, trace)
		return lexical, nil, nil
	}
	if dormant, err := v.store.DormantCandidates(nil); err == nil {
		records = append(records, dormant...)
	}

	q := query
	var blocking, advisory []DuplicateCandidate
	seen := map[string]struct{}{}
	for _, c := range lexical {
		seen[c.ID] = struct{}{}
	}
	for _, rec := range records {
		vec, ok := vectors[rec.ID]
		if !ok {
			continue
		}
		if _, ok := seen[rec.ID]; ok {
			continue
		}
		score := cosine(q, vec)
		if score < v.embedThresh {
			continue
		}
		cand := DuplicateCandidate{ID: rec.ID, Score: score, Claim: rec.Claim}
		if PolarityConflict(claim, rec.Claim) {
			// Same topic, likely opposite policy: surface it, do not reject.
			advisory = append(advisory, cand)
			continue
		}
		blocking = append(blocking, cand)
	}
	sort.Slice(blocking, func(i, j int) bool { return blocking[i].Score > blocking[j].Score })
	sort.Slice(advisory, func(i, j int) bool { return advisory[i].Score > advisory[j].Score })
	if len(blocking) > 3 {
		blocking = blocking[:3]
	}
	if len(advisory) > 3 {
		advisory = advisory[:3]
	}

	if len(advisory) > 0 {
		v.emit(TelemetryEvent{Op: "embed_advisory", RuleHitCount: len(advisory), LatencyMS: time.Since(start).Milliseconds()}, trace)
	}
	if len(blocking) > 0 {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "embed_duplicate", RuleHitCount: len(blocking), LatencyMS: time.Since(start).Milliseconds()}, trace)
		return blocking, advisory, &WriteGuardError{
			Code:       "duplicate",
			Message:    "semantically similar active rule already exists",
			Candidates: blocking,
			Hint: "this rule matches an existing one by meaning; reinforce it if the user " +
				"repeated the same policy, supersede it if the policy changed, " +
				"or reword the claim to distinguish them",
		}
	}
	v.emit(TelemetryEvent{Op: "embed", LatencyMS: time.Since(start).Milliseconds()}, trace)
	return lexical, advisory, nil
}

// cosine returns the cosine similarity of two equal-length vectors, or 0 when
// they are empty, of different lengths, or degenerate.
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na <= 0 || nb <= 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func tokenSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

func scopesOverlap(a, b []string) bool {
	seen := make(map[string]struct{}, len(a))
	for _, scope := range a {
		seen[strings.ToLower(scope)] = struct{}{}
	}
	for _, scope := range b {
		if _, ok := seen[strings.ToLower(scope)]; ok {
			return true
		}
	}
	return false
}

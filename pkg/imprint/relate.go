package imprint

import (
	"context"
	"sort"
)

func (v *Vault) relater() Relater {
	if v == nil || v.embed == nil {
		return nil
	}
	r, ok := v.embed.(Relater)
	if !ok {
		return nil
	}
	return r
}

// mergeBidirectionalRelate combines forward and reverse NLI scores for one pair.
func mergeBidirectionalRelate(ab, ba RelateScores) RelateScores {
	return RelateScores{
		Contradiction: maxF(ab.Contradiction, ba.Contradiction),
		Entailment:    maxF(ab.Entailment, ba.Entailment),
		Neutral:       minF(ab.Neutral, ba.Neutral),
	}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// classifyDuplicateCandidate maps cosine + relate scores to block vs advisory.
// Order: contradiction advisory → entail block → optional high-cos block (yaml).
func (v *Vault) classifyDuplicateCandidate(cosine float64, sc RelateScores) (blocking, advisory bool) {
	if sc.Contradiction >= v.relateConMin {
		return false, true
	}
	if sc.Entailment >= v.relateEntMin {
		return true, false
	}
	if v.duplicateHighCosBlock > 0 && cosine >= v.duplicateHighCosBlock {
		return true, false
	}
	return false, false
}

func (v *Vault) splitCandidatesByRelate(ctx context.Context, claim string, cands []DuplicateCandidate, trace string) (blocking, advisory []DuplicateCandidate) {
	if len(cands) == 0 {
		return nil, nil
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	if len(cands) > maxRelatePairsPerCall {
		cands = cands[:maxRelatePairsPerCall]
	}
	rel := v.relater()
	if rel == nil {
		return append([]DuplicateCandidate(nil), cands...), nil
	}
	pairs := make([][2]string, len(cands))
	for i, c := range cands {
		pairs[i] = [2]string{claim, c.Claim}
	}
	scores, err := rel.Relate(ctx, pairs)
	if err != nil || len(scores) != len(cands) {
		v.emit(TelemetryEvent{Op: "relate_unavailable", Code: "relate_failed"}, trace)
		return append([]DuplicateCandidate(nil), cands...), nil
	}
	if v.relateBidirectional {
		rev := make([][2]string, len(cands))
		for i, c := range cands {
			rev[i] = [2]string{c.Claim, claim}
		}
		revScores, err := rel.Relate(ctx, rev)
		if err != nil || len(revScores) != len(cands) {
			v.emit(TelemetryEvent{Op: "relate_unavailable", Code: "relate_reverse_failed"}, trace)
		} else {
			for i := range scores {
				scores[i] = mergeBidirectionalRelate(scores[i], revScores[i])
			}
		}
	}
	for i, c := range cands {
		block, adv := v.classifyDuplicateCandidate(c.Score, scores[i])
		if adv {
			advisory = append(advisory, c)
		} else if block {
			blocking = append(blocking, c)
		}
	}
	return blocking, advisory
}

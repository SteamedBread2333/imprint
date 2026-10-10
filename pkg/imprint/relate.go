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

// classifyRelate maps sidecar scores to blocking vs advisory duplicate signals.
func (v *Vault) classifyRelate(sc RelateScores) (blocking, advisory bool) {
	if sc.Contradiction >= v.relateConMin {
		return false, true
	}
	if sc.Entailment >= v.relateEntMin {
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
	for i, c := range cands {
		block, adv := v.classifyRelate(scores[i])
		if adv {
			advisory = append(advisory, c)
		} else if block {
			blocking = append(blocking, c)
		}
	}
	return blocking, advisory
}

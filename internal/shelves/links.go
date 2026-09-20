package shelves

import (
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// BuildFindLinks joins top-K rule and document hits with persisted sources and doc citations.
func BuildFindLinks(v *imprint.Vault, st *index.Store, rules []imprint.EnrichedFindHit, docs []index.SearchHit) []imprint.FindLink {
	if st == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var links []imprint.FindLink
	add := func(ruleID, chunkID, kind string, score float64) {
		if ruleID == "" || chunkID == "" {
			return
		}
		key := ruleID + "\x00" + chunkID + "\x00" + kind
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		links = append(links, imprint.FindLink{
			RuleID:  ruleID,
			ChunkID: chunkID,
			Kind:    kind,
			Score:   score,
		})
	}

	docScore := map[string]float64{}
	docIDs := map[string]struct{}{}
	for _, d := range docs {
		docScore[d.ID] = d.Score
		docIDs[d.ID] = struct{}{}
	}

	for _, r := range rules {
		for _, rs := range r.ResolvedSources {
			if rs.ChunkID != "" {
				add(r.ID, rs.ChunkID, "sources", 1.0)
			}
		}
	}

	for _, d := range docs {
		if v != nil {
			refs, err := v.RulesReferencingDoc(d.Path, d.Heading, d.ID)
			if err == nil {
				for _, ref := range refs {
					add(ref.ID, d.ID, "sources", 1.0)
				}
			}
		}
		for _, cited := range CitedRules(st, d.ID) {
			add(cited.ID, d.ID, "cited_by", 1.0)
		}
	}

	topDoc := 0.0
	for _, s := range docScore {
		if s > topDoc {
			topDoc = s
		}
	}
	coMin := topDoc * FindDocMinRelativeScore

	ruleIDs := map[string]float64{}
	for _, r := range rules {
		ruleIDs[r.ID] = r.Score
	}
	for ruleID, rScore := range ruleIDs {
		for chunkID, dScore := range docScore {
			if _, ok := seen[ruleID+"\x00"+chunkID+"\x00sources"]; ok {
				continue
			}
			if _, ok := seen[ruleID+"\x00"+chunkID+"\x00cited_by"]; ok {
				continue
			}
			if topDoc > 0 && dScore < coMin {
				continue
			}
			co := (rScore + dScore) / 2
			if co <= 0 {
				continue
			}
			add(ruleID, chunkID, "co_search", co)
		}
	}
	return links
}

// EnrichFindHits attaches sources and resolved excerpts to rule hits.
func EnrichFindHits(st *index.Store, hits []imprint.FindHit, sourcesByID map[string][]imprint.DocRef) []imprint.EnrichedFindHit {
	out := make([]imprint.EnrichedFindHit, 0, len(hits))
	for _, h := range hits {
		sources := sourcesByID[h.ID]
		out = append(out, imprint.EnrichedFindHit{
			FindHit:         h,
			Sources:         sources,
			ResolvedSources: ResolveSources(st, sources),
		})
	}
	return out
}

// SourcesByIDs loads sources for the given rule ids from the vault.
func SourcesByIDs(v *imprint.Vault, ids []string) (map[string][]imprint.DocRef, error) {
	return v.SourcesForIDs(ids)
}

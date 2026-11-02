package linking

import (
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// FindRequest is one find call shared by the CLI, MCP tools, and host API.
type FindRequest struct {
	Scope      []string
	Query      string
	QueryLocal string
	TopK       int
	Paths      []string
}

// FindOutcome holds exactly one populated result shape:
// Docs when the shelves document leg ran, Rules when a shelves index exists
// (rules carry resolved sources), otherwise Hits from the vault alone.
type FindOutcome struct {
	Docs  *FindResult
	Rules []imprint.EnrichedFindHit
	Hits  []imprint.FindHit
}

// RuleHits returns the rule hits of whichever shape is populated.
func (o FindOutcome) RuleHits() []imprint.FindHit {
	var src []imprint.EnrichedFindHit
	switch {
	case o.Docs != nil:
		src = o.Docs.Rules
	case o.Rules != nil:
		src = o.Rules
	default:
		return o.Hits
	}
	out := make([]imprint.FindHit, len(src))
	for i, h := range src {
		out[i] = h.FindHit
	}
	return out
}

// Find runs vault recall plus optional shelves enrichment. svc may be nil.
func Find(v *imprint.Vault, svc *shelves.Service, req FindRequest) (FindOutcome, error) {
	if req.TopK <= 0 {
		req.TopK = imprint.DefaultTopK
	}
	var st *index.Store
	if svc != nil {
		st = svc.IndexStore()
	}
	if st != nil && shelves.MatchFindQuery(svc.Config(), req.Query, req.QueryLocal) {
		cfg := svc.Config()
		docs, _, err := EnrichFindDocs(v, st, req.Scope, req.Query, req.QueryLocal, req.TopK, FindDocs{
			TopK: cfg.FindTopK, SnippetRunes: cfg.SnippetRunes, Paths: req.Paths,
		})
		if err != nil {
			return FindOutcome{}, err
		}
		return FindOutcome{Docs: docs}, nil
	}
	hits, err := v.FindMerged(req.Scope, req.Query, req.QueryLocal, req.TopK)
	if err != nil {
		return FindOutcome{}, err
	}
	if st == nil || len(hits) == 0 {
		return FindOutcome{Hits: hits}, nil
	}
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	sourcesByID, err := shelves.SourcesByIDs(v, ids)
	if err != nil {
		return FindOutcome{}, err
	}
	return FindOutcome{Rules: shelves.EnrichFindHits(st, hits, sourcesByID)}, nil
}

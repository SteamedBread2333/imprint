package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func registerTools(server *sdkmcp.Server, v *imprint.Vault, shelvesSvc *shelves.Service) {
	s := &vaultTools{v: v, shelves: shelvesSvc}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "find",
		Description: "Recall rules and optional shelves documents. Call at most once per user message; prepare all query terms together.",
	}, s.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Store a new durable policy after ADD classification. Duplicate and sensitive writes are rejected.",
	}, s.add)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reinforce",
		Description: "Reinforce an existing rule only when the user explicitly repeats the same policy.",
	}, s.reinforce)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "supersede",
		Description: "Archive an old rule and write its replacement when policy changes.",
	}, s.supersede)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "forget",
		Description: "Delete a rule the user asked to forget; resolve its id without asking the user.",
	}, s.forget)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get",
		Description: "Load a compact rule or a shelves chunk by id; evidence and full records are opt-in.",
	}, s.get)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list",
		Description: "Filter rule summaries for maintenance; not a substitute for find before writes.",
	}, s.list)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "show",
		Description: "Audit-only census of recorded rules.",
	}, s.show)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "sweep",
		Description: "Maintainer-only decay of stale rules; not a per-turn action.",
	}, s.sweep)
}

type vaultTools struct {
	v       *imprint.Vault
	shelves *shelves.Service
}

type findArgs struct {
	Scope      string `json:"scope" jsonschema:"comma-separated scope tags (AND); prepare with query and query_local before the single find"`
	Query      string `json:"query,omitempty" jsonschema:"BM25 for vault and shelves (policy/English terms); pass together with query_local when local terms differ"`
	QueryLocal string `json:"query_local,omitempty" jsonschema:"LLM-distilled local-language search terms, not user verbatim; extra BM25 pass, merge by id max score"`
	TopK       int    `json:"top_k,omitempty" jsonschema:"max hits (default 5)"`
	Full       bool   `json:"full,omitempty" jsonschema:"include source excerpts and query_local for audit; default false"`
}

func (s *vaultTools) find(_ context.Context, _ *sdkmcp.CallToolRequest, args findArgs) (*sdkmcp.CallToolResult, any, error) {
	topK := args.TopK
	if topK == 0 {
		topK = imprint.DefaultTopK
	}
	scope := splitCSV(args.Scope)
	query := args.Query
	queryLocal := args.QueryLocal

	if s.shelves != nil && shelves.MatchFindQuery(s.shelves.Config(), query, queryLocal) {
		enriched, _, err := linking.EnrichFind(s.v, s.shelves.IndexStore(), scope, query, queryLocal, topK)
		if err != nil {
			return toolErr(err), nil, nil
		}
		if args.Full {
			return jsonOK(enriched)
		}
		compact, err := linking.CompactFind(s.v, enriched)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return jsonOK(compact)
	}

	hits, err := s.v.FindMerged(scope, query, queryLocal, topK)
	if err != nil {
		return toolErr(err), nil, nil
	}
	if s.shelves != nil && s.shelves.IndexStore() != nil && len(hits) > 0 {
		ids := make([]string, len(hits))
		for i, h := range hits {
			ids[i] = h.ID
		}
		sourcesByID, err := shelves.SourcesByIDs(s.v, ids)
		if err != nil {
			return toolErr(err), nil, nil
		}
		enriched := shelves.EnrichFindHits(s.shelves.IndexStore(), hits, sourcesByID)
		if args.Full {
			return jsonOK(enriched)
		}
		return s.compactHits(enriched)
	}
	if args.Full {
		return jsonOK(hits)
	}
	enriched := make([]imprint.EnrichedFindHit, 0, len(hits))
	for _, hit := range hits {
		enriched = append(enriched, imprint.EnrichedFindHit{FindHit: hit})
	}
	return s.compactHits(enriched)
}

func (s *vaultTools) compactHits(hits []imprint.EnrichedFindHit) (*sdkmcp.CallToolResult, any, error) {
	compact, err := linking.CompactFind(s.v, &linking.FindResult{Rules: hits, Documents: []index.SearchHit{}})
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(compact)
}

type docRefArg struct {
	Path    string `json:"path,omitempty" jsonschema:"workspace-relative document path"`
	Heading string `json:"heading,omitempty" jsonschema:"optional section title; matched after markdown inline to plain text"`
	Chunk   string `json:"chunk,omitempty" jsonschema:"optional shelves chunk id"`
}

func parseDocRefs(in []docRefArg) []imprint.DocRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]imprint.DocRef, 0, len(in))
	for _, d := range in {
		out = append(out, imprint.DocRef{Path: d.Path, Heading: d.Heading, Chunk: d.Chunk})
	}
	return out
}

type addArgs struct {
	Claim      string      `json:"claim" jsonschema:"English imperative claim for agents; user wording in text, local search terms in query_local"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Text       string      `json:"text" jsonschema:"user's original words (evidence); do not copy into query_local"`
	Confidence float64     `json:"confidence,omitempty" jsonschema:"omit for 0.6; 0.85 only when user corrects; never 0.9 on add"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"LLM local-language search terms, not verbatim; vault merges CJK evidence tokens"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"[{path, heading?, chunk?}] only when excerpt substantively supports the claim, not topical overlap; vault only"`
}

func (s *vaultTools) add(_ context.Context, _ *sdkmcp.CallToolRequest, args addArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.AddRecord(args.Claim, splitCSV(args.Scope), args.Text, args.Confidence, nil, nil, nil, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type reinforceArgs struct {
	ID         string `json:"id" jsonschema:"rule id from find (do not ask the user)"`
	Evidence   string `json:"evidence" jsonschema:"required user reaffirmation evidence; empty evidence is rejected"`
	QueryLocal string `json:"query_local,omitempty" jsonschema:"optional LLM local-language terms, not verbatim; updates stored query_local"`
}

func (s *vaultTools) reinforce(_ context.Context, _ *sdkmcp.CallToolRequest, args reinforceArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.ReinforceQueryLocal(args.ID, args.Evidence, args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type supersedeArgs struct {
	OldID      string      `json:"old_id" jsonschema:"rule id to archive (from find; do not ask the user)"`
	Claim      string      `json:"claim" jsonschema:"English imperative new claim"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Reason     string      `json:"reason,omitempty" jsonschema:"optional supersede reason (evidence if text omitted)"`
	Text       string      `json:"text,omitempty" jsonschema:"user's original words for the new rule"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"LLM local-language terms, not verbatim; omit to inherit from old rule"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"[{path, heading?, chunk?}]; omit to inherit from old rule"`
}

func (s *vaultTools) supersede(_ context.Context, _ *sdkmcp.CallToolRequest, args supersedeArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.SupersedeWithSources(args.OldID, args.Claim, splitCSV(args.Scope), args.Reason, args.Text, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type forgetArgs struct {
	ID string `json:"id" jsonschema:"rule id from find (do not ask the user)"`
}

func (s *vaultTools) forget(_ context.Context, _ *sdkmcp.CallToolRequest, args forgetArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Forget(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type getArgs struct {
	ID              string `json:"id" jsonschema:"r-… rule id, or shelves chunk id"`
	IncludeEvidence bool   `json:"include_evidence,omitempty" jsonschema:"include latest evidence entries; default false"`
	EvidenceLimit   int    `json:"evidence_limit,omitempty" jsonschema:"max evidence entries when included (default 3)"`
	Full            bool   `json:"full,omitempty" jsonschema:"return the legacy full record with source excerpts; audit only"`
}

func (s *vaultTools) get(_ context.Context, _ *sdkmcp.CallToolRequest, args getArgs) (*sdkmcp.CallToolResult, any, error) {
	if s.shelves != nil && s.shelves.HasChunk(args.ID) {
		if c, ok := s.shelves.ChunkByID(args.ID); ok {
			out, err := linking.EnrichChunkGet(s.v, s.shelves.IndexStore(), c)
			if err != nil {
				return toolErr(err), nil, nil
			}
			return jsonOK(out)
		}
	}
	rec, err := s.v.Get(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	if args.Full {
		if s.shelves != nil {
			return jsonOK(linking.EnrichRuleGet(s.shelves.IndexStore(), rec))
		}
		return jsonOK(rec)
	}
	limit := 0
	if args.IncludeEvidence {
		limit = args.EvidenceLimit
		if limit <= 0 {
			limit = 3
		}
	}
	if s.shelves != nil {
		resolved := shelves.ResolveSources(s.shelves.IndexStore(), rec.Sources)
		return jsonOK(linking.CompactRule(rec, resolved, limit))
	}
	return jsonOK(linking.CompactRule(rec, nil, limit))
}

type listArgs struct {
	Status        string  `json:"status,omitempty" jsonschema:"active, dormant, or superseded"`
	Scope         string  `json:"scope,omitempty" jsonschema:"comma-separated scope tags (AND filter)"`
	Query         string  `json:"query,omitempty" jsonschema:"optional BM25 filter on rules"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence threshold"`
	Since         string  `json:"since,omitempty" jsonschema:"YYYY-MM-DD or RFC3339; rules touched since"`
	Limit         int     `json:"limit,omitempty" jsonschema:"max rows (default unlimited)"`
}

func (s *vaultTools) list(_ context.Context, _ *sdkmcp.CallToolRequest, args listArgs) (*sdkmcp.CallToolResult, any, error) {
	when, err := parseSince(args.Since)
	if err != nil {
		return toolErr(err), nil, nil
	}
	items, err := s.v.ListFilter(imprint.ListFilter{
		Status:        args.Status,
		Scope:         splitCSV(args.Scope),
		MinConfidence: args.MinConfidence,
		Query:         args.Query,
		Since:         when,
		Limit:         args.Limit,
	})
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(items)
}

type showArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"max active rules to return (default all)"`
}

func (s *vaultTools) show(_ context.Context, _ *sdkmcp.CallToolRequest, args showArgs) (*sdkmcp.CallToolResult, any, error) {
	recs, err := s.v.Show(args.Limit)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(recs)
}

type sweepArgs struct {
	DecayDays        int     `json:"decay_days,omitempty" jsonschema:"days without touch before decay (default 90)"`
	DecayAmount      float64 `json:"decay_amount,omitempty" jsonschema:"confidence subtracted per decay (default 0.05)"`
	DormantThreshold float64 `json:"dormant_threshold,omitempty" jsonschema:"mark dormant below this confidence (default 0.3)"`
}

func (s *vaultTools) sweep(_ context.Context, _ *sdkmcp.CallToolRequest, args sweepArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Sweep(args.DecayDays, args.DecayAmount, args.DormantThreshold)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

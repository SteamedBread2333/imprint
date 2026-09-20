package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func registerTools(server *sdkmcp.Server, v *imprint.Vault, shelvesSvc *shelves.Service) {
	s := &vaultTools{v: v, shelves: shelvesSvc}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "find",
		Description: "Search active rules (scope tags AND-filtered; optional BM25 query + optional query_local). query and query_local each run vault+shelves BM25 when shelves on; hits merge by id (max score). query_local = LLM local-language search terms (persisted on rules via add/supersede/reinforce). Shelves+MCP: { rules (resolved_sources), documents, links }. links kinds: sources, cited_by, co_search. CLI find omits documents/links/resolution.",
	}, s.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Add after ADD classification. Required: claim, scope, text. Optional sources [{path, heading?, chunk?}] persist rule→doc links in vault (default: no project markdown edits). Use when user points at docs or find returned a matching document.",
	}, s.add)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reinforce",
		Description: "Reinforce an existing rule (+0.1 confidence, cap 0.95).",
	}, s.reinforce)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "supersede",
		Description: "Mark old_id superseded; write new active rule (claim, scope). Inherits old sources unless sources is set. Optional sources [{path, heading?, chunk?}] replace inherited rule→doc links in vault.",
	}, s.supersede)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "forget",
		Description: "Delete a rule and strip inbound links.",
	}, s.forget)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get",
		Description: "Load by id. r-… → vault + referenced_by + resolved_sources (MCP+shelves). chunk id → document + referenced_rules (vault sources reverse; no markdown edits) + cited_rules ([[r-…]] / [imprint:r-…] only). CLI get is vault-only for rules; no chunk ids.",
	}, s.get)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list",
		Description: "List rules with optional status, scope, query, min_confidence, since, limit filters.",
	}, s.list)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "show",
		Description: "Census of active rules (optionally limited).",
	}, s.show)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "sweep",
		Description: "Decay confidence and mark low-confidence rules dormant.",
	}, s.sweep)
}

type vaultTools struct {
	v       *imprint.Vault
	shelves *shelves.Service
}

type findArgs struct {
	Scope       string `json:"scope" jsonschema:"comma-separated scope tags (AND filter)"`
	Query       string `json:"query,omitempty" jsonschema:"optional BM25 query (vault + shelves)"`
	QueryLocal  string `json:"query_local,omitempty" jsonschema:"optional local-language BM25 pass (vault + shelves); use when LLM terms differ from query"`
	TopK        int    `json:"top_k,omitempty" jsonschema:"max hits (default 5)"`
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
		return jsonOK(enriched)
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
		return jsonOK(shelves.EnrichFindHits(s.shelves.IndexStore(), hits, sourcesByID))
	}
	return jsonOK(hits)
}

type docRefArg struct {
	Path    string `json:"path,omitempty"`
	Heading string `json:"heading,omitempty"`
	Chunk   string `json:"chunk,omitempty"`
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
	Claim      string      `json:"claim" jsonschema:"imperative claim text"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Text       string      `json:"text" jsonschema:"user's original words"`
	Confidence float64     `json:"confidence,omitempty" jsonschema:"starting confidence; 0 uses default 0.6"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"optional LLM local-language search terms for future find"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"optional workspace document sources"`
}

func (s *vaultTools) add(_ context.Context, _ *sdkmcp.CallToolRequest, args addArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.AddRecord(args.Claim, splitCSV(args.Scope), args.Text, args.Confidence, nil, nil, nil, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type reinforceArgs struct {
	ID         string `json:"id" jsonschema:"rule id"`
	Evidence   string `json:"evidence,omitempty" jsonschema:"optional evidence note"`
	QueryLocal string `json:"query_local,omitempty" jsonschema:"optional update stored query_local"`
}

func (s *vaultTools) reinforce(_ context.Context, _ *sdkmcp.CallToolRequest, args reinforceArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.ReinforceQueryLocal(args.ID, args.Evidence, args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type supersedeArgs struct {
	OldID      string      `json:"old_id" jsonschema:"rule id to archive"`
	Claim      string      `json:"claim" jsonschema:"new claim"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Reason     string      `json:"reason,omitempty"`
	Text       string      `json:"text,omitempty" jsonschema:"user's original words for the new rule"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"optional query_local; omit to inherit from old rule"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"optional sources; omit to inherit from old rule"`
}

func (s *vaultTools) supersede(_ context.Context, _ *sdkmcp.CallToolRequest, args supersedeArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.SupersedeWithSources(args.OldID, args.Claim, splitCSV(args.Scope), args.Reason, args.Text, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type forgetArgs struct {
	ID string `json:"id" jsonschema:"rule id"`
}

func (s *vaultTools) forget(_ context.Context, _ *sdkmcp.CallToolRequest, args forgetArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Forget(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type getArgs struct {
	ID string `json:"id" jsonschema:"rule id or shelves chunk id"`
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
	if s.shelves != nil {
		return jsonOK(linking.EnrichRuleGet(s.shelves.IndexStore(), rec))
	}
	return jsonOK(rec)
}

type listArgs struct {
	Status            string  `json:"status,omitempty"`
	Scope             string  `json:"scope,omitempty" jsonschema:"comma-separated scope tags (AND)"`
	Query             string  `json:"query,omitempty"`
	MinConfidence     float64 `json:"min_confidence,omitempty"`
	Since             string  `json:"since,omitempty" jsonschema:"YYYY-MM-DD or RFC3339"`
	Limit             int     `json:"limit,omitempty"`
}

func (s *vaultTools) list(_ context.Context, _ *sdkmcp.CallToolRequest, args listArgs) (*sdkmcp.CallToolResult, any, error) {
	when, err := parseSince(args.Since)
	if err != nil {
		return toolErr(err), nil, nil
	}
	items, err := s.v.ListFilter(imprint.ListFilter{
		Status:            args.Status,
		Scope:             splitCSV(args.Scope),
		MinConfidence:     args.MinConfidence,
		Query:             args.Query,
		Since:             when,
		Limit:             args.Limit,
	})
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(items)
}

type showArgs struct {
	Limit int `json:"limit,omitempty"`
}

func (s *vaultTools) show(_ context.Context, _ *sdkmcp.CallToolRequest, args showArgs) (*sdkmcp.CallToolResult, any, error) {
	recs, err := s.v.Show(args.Limit)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(recs)
}

type sweepArgs struct {
	DecayDays        int     `json:"decay_days,omitempty"`
	DecayAmount      float64 `json:"decay_amount,omitempty"`
	DormantThreshold float64 `json:"dormant_threshold,omitempty"`
}

func (s *vaultTools) sweep(_ context.Context, _ *sdkmcp.CallToolRequest, args sweepArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Sweep(args.DecayDays, args.DecayAmount, args.DormantThreshold)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}


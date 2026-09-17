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
		Name:        "find",
		Description: "Search active rules: scope tags are AND-filtered, then optional BM25 query ranks matches. When shelves is enabled and query is set, returns enriched rules (resolved_sources), documents, and links.",
	}, s.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Add a rule after classifying ADD. claim, scope, and text are required. Optional sources link the rule to workspace docs.",
	}, s.add)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reinforce",
		Description: "Reinforce an existing rule (+0.1 confidence, cap 0.95).",
	}, s.reinforce)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "supersede",
		Description: "Archive old_id and write a new rule with claim and scope. Inherits old sources unless sources is set.",
	}, s.supersede)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "forget",
		Description: "Delete a rule and strip inbound links.",
	}, s.forget)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get",
		Description: "Load one rule with evidence_log, referenced_by, and resolved_sources; or a document chunk with referenced_rules (vault sources) and optional cited_rules ([[r-…]] in text).",
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
		Description: "Decay confidence and archive dormant rules.",
	}, s.sweep)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "viz",
		Description: "Generate mermaid on stdout or notes/ cards; returns path and stats. Interactive graph: desk plugin.",
	}, s.viz)
}

type vaultTools struct {
	v       *imprint.Vault
	shelves *shelves.Service
}

type findArgs struct {
	Scope string `json:"scope" jsonschema:"comma-separated scope tags (AND filter)"`
	Query string `json:"query,omitempty" jsonschema:"optional BM25 query"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"max hits (default 5)"`
}

func (s *vaultTools) find(_ context.Context, _ *sdkmcp.CallToolRequest, args findArgs) (*sdkmcp.CallToolResult, any, error) {
	topK := args.TopK
	if topK == 0 {
		topK = imprint.DefaultTopK
	}
	scope := splitCSV(args.Scope)
	query := args.Query

	if s.shelves != nil && shelves.MatchQuery(s.shelves.Config(), query) {
		enriched, _, err := linking.EnrichFind(s.v, s.shelves.IndexStore(), scope, query, topK)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return jsonOK(enriched)
	}

	hits, err := s.v.Find(scope, query, topK)
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
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"optional workspace document sources"`
}

func (s *vaultTools) add(_ context.Context, _ *sdkmcp.CallToolRequest, args addArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.AddWithSources(args.Claim, splitCSV(args.Scope), args.Text, args.Confidence, parseDocRefs(args.Sources))
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type reinforceArgs struct {
	ID       string `json:"id" jsonschema:"rule id"`
	Evidence string `json:"evidence,omitempty" jsonschema:"optional evidence note"`
}

func (s *vaultTools) reinforce(_ context.Context, _ *sdkmcp.CallToolRequest, args reinforceArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Reinforce(args.ID, args.Evidence)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type supersedeArgs struct {
	OldID   string      `json:"old_id" jsonschema:"rule id to archive"`
	Claim   string      `json:"claim" jsonschema:"new claim"`
	Scope   string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Reason  string      `json:"reason,omitempty"`
	Text    string      `json:"text,omitempty" jsonschema:"user's original words for the new rule"`
	Sources []docRefArg `json:"sources,omitempty" jsonschema:"optional sources; omit to inherit from old rule"`
}

func (s *vaultTools) supersede(_ context.Context, _ *sdkmcp.CallToolRequest, args supersedeArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.SupersedeWithSources(args.OldID, args.Claim, splitCSV(args.Scope), args.Reason, args.Text, parseDocRefs(args.Sources))
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
	Status        string  `json:"status,omitempty"`
	Scope         string  `json:"scope,omitempty" jsonschema:"comma-separated scope tags (AND)"`
	Query         string  `json:"query,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	Since         string  `json:"since,omitempty" jsonschema:"YYYY-MM-DD or RFC3339"`
	Limit         int     `json:"limit,omitempty"`
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

type vizArgs struct {
	Out             string `json:"out,omitempty" jsonschema:"output path"`
	Format          string `json:"format,omitempty" jsonschema:"mermaid or notes"`
	IncludeArchived bool   `json:"include_archived,omitempty"`
}

func (s *vaultTools) viz(_ context.Context, _ *sdkmcp.CallToolRequest, args vizArgs) (*sdkmcp.CallToolResult, any, error) {
	format := args.Format
	if format == "" {
		format = "mermaid"
	}
	res, err := s.v.Viz(args.Out, format, args.IncludeArchived)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

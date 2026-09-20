package shelves

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func TestResolveSources(t *testing.T) {
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "abc123", Path: "docs/style.md", Heading: "Naming", Text: "Use snake_case in Python.", LineStart: 1, LineEnd: 3},
		},
	}
	res := ResolveSources(st, []imprint.DocRef{
		{Path: "docs/style.md", Heading: "Naming"},
		{Chunk: "missing", Path: "docs/style.md"},
	})
	if len(res) != 2 {
		t.Fatalf("resolved = %+v", res)
	}
	if res[0].ChunkID != "abc123" || res[0].Snippet == "" {
		t.Fatalf("first: %+v", res[0])
	}
	if !res[1].StaleChunk || res[1].ChunkID != "abc123" {
		t.Fatalf("second: %+v", res[1])
	}
}

func TestResolveSourcesNormalizesNumberedHeading(t *testing.T) {
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "h1", Path: "docs/storage-retrieval.zh.md", Heading: "智能体记忆系统：数据存储与检索技术说明", Text: "intro", LineStart: 1, LineEnd: 4},
			{ID: "s52", Path: "docs/storage-retrieval.zh.md", Heading: "5.2 协议层合并检索与 `query_local`", Text: "dual-path BM25", LineStart: 257, LineEnd: 301},
		},
	}
	res := ResolveSources(st, []imprint.DocRef{
		{Path: "docs/storage-retrieval.zh.md", Heading: "协议层合并检索与 query_local"},
	})
	if len(res) != 1 || res[0].ChunkID != "s52" || res[0].HeadingUnresolved {
		t.Fatalf("want §5.2 chunk, got %+v", res)
	}
}

func TestResolveSourcesHeadingMissDoesNotFallBackToFirstChunk(t *testing.T) {
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "h1", Path: "docs/correction.zh.md", Heading: "记忆写入", Text: "intro", LineStart: 1, LineEnd: 6},
			{ID: "h2", Path: "docs/correction.zh.md", Heading: "四种写入分类", Text: "add reinforce", LineStart: 61, LineEnd: 68},
		},
	}
	res := ResolveSources(st, []imprint.DocRef{
		{Path: "docs/correction.zh.md", Heading: "Correction loop"},
	})
	if len(res) != 1 {
		t.Fatalf("resolved = %+v", res)
	}
	got := res[0]
	if !got.HeadingUnresolved || got.ChunkID != "" || got.Snippet != "" {
		t.Fatalf("want unresolved without snippet, got %+v", got)
	}
	if got.Heading != "Correction loop" || got.OutOfIndex {
		t.Fatalf("preserve requested heading, not out_of_index: %+v", got)
	}
}

func TestResolveSourcesPathOnlyUsesFirstChunk(t *testing.T) {
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "h1", Path: "docs/mcp.zh.md", Heading: "imprint MCP 服务", Text: "intro", LineStart: 1, LineEnd: 7},
			{ID: "h2", Path: "docs/mcp.zh.md", Heading: "MCP vs CLI", Text: "table", LineStart: 8, LineEnd: 16},
		},
	}
	res := ResolveSources(st, []imprint.DocRef{{Path: "docs/mcp.zh.md"}})
	if len(res) != 1 || res[0].ChunkID != "h1" {
		t.Fatalf("path-only should use first chunk, got %+v", res)
	}
}

func TestBuildFindLinksVaultReverse(t *testing.T) {
	dir := t.TempDir()
	v, err := imprint.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.AddWithSources("Use Vitest", []string{"testing"}, "vitest", 0.85, []imprint.DocRef{
		{Path: "docs/a.md", Heading: "A"},
	})
	if err != nil {
		t.Fatal(err)
	}
	st := &index.Store{
		Chunks: []index.Chunk{{ID: "doc1", Path: "docs/a.md", Heading: "A", Text: "vitest", LineStart: 1, LineEnd: 1}},
	}
	docs := []index.SearchHit{{ID: "doc1", Path: "docs/a.md", Heading: "A", Score: 0.8}}
	links := BuildFindLinks(v, st, nil, docs)
	if len(links) != 1 || links[0].Kind != "sources" {
		t.Fatalf("links = %+v", links)
	}
}

func TestBuildFindLinks(t *testing.T) {
	st := &index.Store{
		Chunks: []index.Chunk{
			{ID: "doc1", Path: "docs/a.md", Heading: "A", Text: "see [[r-2026-09-11-001]]", LineStart: 1, LineEnd: 1},
		},
		RuleRefs: []index.RuleRef{{ChunkID: "doc1", RuleID: "r-2026-09-11-001", LineNo: 1}},
	}
	rules := []imprint.EnrichedFindHit{{
		FindHit:         imprint.FindHit{ID: "r-2026-09-16-001", Score: 0.9},
		ResolvedSources: []imprint.ResolvedSource{{ChunkID: "doc1", Path: "docs/a.md"}},
	}}
	docs := []index.SearchHit{{ID: "doc1", Score: 0.8}}
	links := BuildFindLinks(nil, st, rules, docs)
	if len(links) < 2 {
		t.Fatalf("links = %+v", links)
	}
}

// Seed demo vault rules with document sources for desk unified graph testing.
// Idempotent: skips active rules that already match claim (and have sources when required).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func main() {
	vault := flag.String("vault", ".imprint", "vault directory")
	flag.Parse()

	v, err := imprint.Open(imprint.OpenOptions{Dir: *vault})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, d := range demoRules {
		if err := ensure(v, d); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", d.claim, err)
			os.Exit(1)
		}
	}
}

type demoRule struct {
	claim      string
	scope      string
	text       string
	confidence float64
	sources    []imprint.DocRef
}

var demoRules = []demoRule{
	{
		claim: "Shelves is built into imprint core (top-level shelves: in imprint.yaml), not an external plugin",
		scope: "imprint,architecture,shelves",
		text:  "shelves 是内置的，写在 imprint.yaml 顶层，不是外部插件。",
		sources: []imprint.DocRef{
			{Path: "docs/shelves-builtin.zh.md", Heading: "配置"},
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "问题"},
		},
	},
	{
		claim: "CLI docs and user-facing hints should promote up/down, not host/plugin subcommands",
		scope: "documentation,cli",
		text:  "CLI 常用命令写 up/down，别主推 host/plugin 子命令。",
		sources: []imprint.DocRef{
			{Path: "README.zh.md", Heading: "常用 — 本地服务"},
			{Path: "README.zh.md", Heading: "CLI"},
		},
	},
	{
		claim: "Do not write migration documentation for imprint",
		scope: "documentation,imprint",
		text:  "以后别写 migration 文档，shelves 是内置的。",
		sources: []imprint.DocRef{
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "迁移与兼容"},
			{Path: "docs/shelves-builtin.zh.md", Heading: "Shelves"},
		},
	},
	{
		claim: "No .plugins runtime files under .imprint; clean rich CLI output",
		scope: "imprint,cli,runtime",
		text:  "Do not create .imprint/.plugins or log files under .imprint. Console output should be rich and polished but always clean.",
		sources: []imprint.DocRef{
			{Path: "README.zh.md", Heading: "Vault 布局"},
			{Path: "docs/mcp.zh.md", Heading: "架构"},
		},
	},
	{
		claim:      "Persistent imprint-to-doc links use vault sources only; do not require [[r-…]] in project markdown",
		scope:      "imprint,architecture,linking",
		text:       "文档和 imprint 的持久关联只写 vault 的 sources，默认不要在项目 markdown 里加 [[r-…]]。",
		confidence: 0.85,
		sources: []imprint.DocRef{
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "推荐：不污染项目 markdown"},
			{Path: "docs/shelves-builtin.zh.md", Heading: "与 vault 关联（不污染文档的推荐做法）"},
		},
	},
	{
		claim:      "Agent recall before coding uses MCP find with scope and query; CLI find/get stays vault-only",
		scope:      "imprint,mcp,agent",
		text:       "写代码前 Agent 用 MCP find(scope, query)；CLI find/get 见 mcp 文档对比表。",
		confidence: 0.85,
		sources: []imprint.DocRef{
			{Path: "docs/mcp.zh.md", Heading: "MCP vs CLI"},
			{Path: "docs/correction.zh.md", Heading: "写入循环"},
		},
	},
	{
		claim:      "Human audit of imprint-doc links uses desk unified graph, not agent find",
		scope:      "imprint,desk,audit",
		text:       "人要审计 imprint 和文档怎么连上的，用 desk 的统一视图；Agent 日常召回还是 MCP find/get。",
		confidence: 0.85,
		sources: []imprint.DocRef{
			{Path: "docs/shelves-builtin.zh.md", Heading: "Desk UI"},
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "新端点：GET /graph/unified"},
		},
	},
	{
		claim:      "On ADD when find returns a matching document, attach sources in the same turn",
		scope:      "imprint,agent,writes",
		text:       "ADD 时如果 find 命中了 document，同轮 add 就要带上 sources，只写 vault。",
		confidence: 0.9,
		sources: []imprint.DocRef{
			{Path: "docs/correction.zh.md", Heading: "写入循环"},
			{Path: "docs/mcp.zh.md", Heading: "MCP vs CLI"},
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "8. 智能体工作流（写入 + 文档）"},
		},
	},
	{
		claim:      "After imprint feature work, update README flowchart and all related docs before considering done",
		scope:      "imprint,documentation,workflow",
		text:       "imprint 功能开发完成后要更新 README 流程图和相关文档，并自测。",
		confidence: 0.85,
		sources: []imprint.DocRef{
			{Path: "README.zh.md", Heading: "快速开始"},
			{Path: "docs/imprint-shelves-linking.zh.md", Heading: "实现分期"},
			{Path: ".cursor/rules/imprint-memory.mdc"},
		},
	},
	{
		claim: "Python function names must always be snake_case",
		scope: "python,naming",
		text:  "use snake_case",
		sources: []imprint.DocRef{
			{Path: "docs/correction.zh.md", Heading: "场景示例"},
			{Path: "docs/correction.md", Heading: "Worked examples"},
		},
	},
}

func ensure(v *imprint.Vault, d demoRule) error {
	id, rec, err := findActive(v, d.claim)
	if err != nil {
		return err
	}
	conf := d.confidence
	if conf == 0 {
		conf = 0.6
	}
	if id != "" {
		if len(d.sources) == 0 || len(rec.Sources) > 0 {
			fmt.Printf("skip %s (%s)\n", id, d.claim)
			return nil
		}
		res, err := v.SupersedeWithSources(id, d.claim, splitCSV(d.scope), "attach demo sources", d.text, d.sources, "")
		if err != nil {
			return err
		}
		fmt.Printf("superseded %s -> %s (sources)\n", id, res.ID)
		return nil
	}
	res, err := v.AddWithSources(d.claim, splitCSV(d.scope), d.text, conf, d.sources)
	if err != nil {
		return err
	}
	fmt.Printf("added %s\n", res.ID)
	return nil
}

func findActive(v *imprint.Vault, claim string) (string, *imprint.Record, error) {
	items, err := v.List("active", 0)
	if err != nil {
		return "", nil, err
	}
	for _, it := range items {
		if it.Title != claim {
			continue
		}
		rec, err := v.Get(it.ID)
		if err != nil {
			return it.ID, nil, err
		}
		return it.ID, rec, nil
	}
	return "", nil, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

package main

import (
	"fmt"
	"os"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func main() {
	v, err := imprint.Open("./.imprint/memory")
	if err != nil {
		fatal(err)
	}

	type item struct {
		claim, text string
		scope       []string
		conf        float64
		related     []int
	}

	items := []item{
		{"Apply thinking-process first, then at most one additional skill; load only the references that skill names", "AGENTS.md / thinking-process: dispatcher first", []string{"dsnu", "thinking-process"}, 0.9, nil},
		{"Do not name skill identifiers in operator-facing copy", "Skill identifiers are not part of the operator vocabulary", []string{"dsnu", "ux"}, 0.9, []int{0}},
		{"Operator-facing confirmations match the conversation language; do not force a language", "Do not force a language", []string{"dsnu", "ux"}, 0.85, []int{1}},
		{"Every operator decision uses AskQuestion; do not end a turn with a typed-reply question", "LangGraph HITL; do not list options in chat", []string{"dsnu", "hitl"}, 0.9, []int{0}},
		{"This copilot serves the Content Nexus administration console only", "Do not use for unrelated products", []string{"dsnu", "isolation"}, 0.9, []int{0}},
		{"Raster captures or Figma ingress dispatch to page-implement (Loop A)", "Compose or extend a page from raster or Figma", []string{"dsnu", "thinking-process", "page-implement"}, 0.85, []int{0}},
		{"Incremental modification or review-only is Loop B: mutate when requested, then repo-review", "Do not use page-implement for review-only", []string{"dsnu", "thinking-process", "repo-review"}, 0.85, []int{0}},
		{"Run e2e-playwright only when the operator requests verification; do not offer it after Loop A or Loop B", "Inactive until the operator requests verification", []string{"dsnu", "e2e-playwright"}, 0.9, []int{0, 5, 6}},
		{"When starting consumer start or dev, use shell required_permissions all on the first call", "Do not sandbox-fail then retry", []string{"dsnu", "shell"}, 0.85, []int{0}},
		{"Load the matching repo-map fragment, then only consumer files the task needs; do not ingest the whole tree", "Context budget", []string{"dsnu", "thinking-process"}, 0.85, []int{0}},
		{"Do not write application code until a design ingress is selected", "page-implement: halt until ingress", []string{"dsnu", "page-implement"}, 0.9, []int{5}},
		{"Page composition ingress is multi-state raster captures or Figma MCP only", "Exactly two design ingresses", []string{"dsnu", "page-implement"}, 0.9, []int{5, 10}},
		{"Emit @derbysoft/neat-design; never import antd", "Do not emit antd imports", []string{"dsnu", "page-implement", "ui"}, 0.9, []int{10}},
		{"Resolve components, illustrations, and icons from the live neat-design catalog; do not vendor inventories", "Query catalog at call time", []string{"dsnu", "page-implement", "ui"}, 0.85, []int{12}},
		{"Layout mutations are confined to src/layouts/", "Layout namespace exclusively", []string{"dsnu", "page-implement"}, 0.85, []int{10}},
		{"Hook files are useXxx.ts unless the consumer already uses another suffix", "Match consumer hook filenames", []string{"dsnu", "page-implement", "naming"}, 0.7, []int{10}},
		{"Page module is src/pages/<Module>/: index.tsx is shell only; components are presentational; hooks own fetch and overlay state", "composition.md encapsulation", []string{"dsnu", "page-implement"}, 0.85, []int{10, 15}},
		{"components/* must not import service clients or @@initialState; props in, callbacks out", "composition.md", []string{"dsnu", "page-implement"}, 0.85, []int{16}},
		{"Do not fabricate service-contract fields", "Hard constraint", []string{"dsnu", "page-implement", "api"}, 0.9, []int{10}},
		{"Do not treat any consumer page as a gold sample to clone", "Skill-owned templates fill gaps", []string{"dsnu", "page-implement"}, 0.85, []int{10, 16}},
		{"A single-portal change must declare the other portal covered or explicitly out of scope", "Dual-portal declaration", []string{"dsnu", "page-implement", "rbac"}, 0.85, []int{10}},
		{"User-visible copy goes through useSupaIntl; pair every key in en-US and zh-CN", "locale pairing", []string{"dsnu", "i18n"}, 0.9, []int{10}},
		{"DsAccess name must match src/access.ts and route access; do not mix menu ids with button ids", "RBAC identifiers", []string{"dsnu", "rbac"}, 0.85, []int{20}},
		{"HTTP uses request from @umijs/max; REST unwraps data/error; JSON-RPC unwraps result/error when the URL path ends with .rpc", "wiring.md HTTP", []string{"dsnu", "api"}, 0.8, []int{18}},
		{"Do not mutate src/app.tsx unmarshalling unless that file is the assigned change", "Do not touch unmarshalling by default", []string{"dsnu", "api"}, 0.85, []int{18, 23}},
		{"Do not overwrite smoking-intl translation workbooks", "Do not overwrite translation workbooks", []string{"dsnu", "i18n"}, 0.9, []int{21}},
		{"repo-review inspects the current git diff or operator-named paths only; full-tree review is disallowed", "Differential review", []string{"dsnu", "repo-review"}, 0.9, []int{6}},
		{"From the consumer root run check-i18n-pair.mjs and check-access-orphan.mjs against identifiers the diff touched", "Prefer scripts over corpus scans", []string{"dsnu", "repo-review"}, 0.8, []int{21, 22, 26}},
		{"In review, remove dead code and unused imports in the diff; do not refactor unrelated modules", "Remediate in place", []string{"dsnu", "repo-review"}, 0.8, []int{26}},
		{"The only browser driver is Playwright MCP attached to host Chrome or Edge via cdp-endpoint or the Playwright extension", "Do not use the editor-embedded browser; do not pass --headless or --isolated", []string{"dsnu", "e2e-playwright"}, 0.9, []int{7}},
		{"Do not install Playwright, do not write .cursor/mcp.json, and do not substitute another browser", "dsnu-agent setup does not install Playwright", []string{"dsnu", "e2e-playwright"}, 0.9, []int{29}},
		{"Author complete Playwright scripts for every case before any verification browser step; do not codegen while driving", "Hard split: finish authoring, then execute", []string{"dsnu", "e2e-playwright"}, 0.9, []int{7, 29}},
		{"Default a new browser tab; reuse an existing tab only when the operator names it; do not close tabs they already had", "Tab: default browser_tabs new", []string{"dsnu", "e2e-playwright"}, 0.8, []int{29, 31}},
		{"Locators target role, label, or testid of the control, never payload text", "Locator contract", []string{"dsnu", "e2e-playwright"}, 0.85, []int{31}},
		{"If Playwright MCP is missing, give a short install reminder and link playwright-first-run.md; do not paste that tutorial", "Do not paste references/playwright-first-run.md", []string{"dsnu", "e2e-playwright"}, 0.85, []int{29, 30}},
		{"Commit messages are {emoji} {single-line rationale} for the staged diff only; do not pass --no-verify", "002_git_commit.mdc / Git Commit message Emoji", []string{"dsnu", "git"}, 0.85, []int{1}},
		{"Consumers bind the workspace with npx dsnu-agent setup; do not attach setup to postinstall or prepare", "setup is idempotent; not a postinstall hook", []string{"dsnu", "package"}, 0.85, []int{4}},
		{"Do not persist host-local invariants: absolute home paths, usernames, hostnames, or workstation credentials", "Artifact hygiene", []string{"dsnu", "package"}, 0.9, []int{4, 36}},
		{"Overlay default when the spec is silent: drawer for create/edit/detail; modal for short confirm", "composition.md overlay choice", []string{"dsnu", "page-implement", "ui"}, 0.7, []int{16}},
		{"Keep page-local units under components/ until a second page needs them; then promote to shared chrome", "Do not extract a wrapper that only re-exports neat-design", []string{"dsnu", "page-implement"}, 0.7, []int{16, 17}},
	}

	ids := make([]string, len(items))
	for i, it := range items {
		var rel []string
		for _, j := range it.related {
			if j >= 0 && j < i {
				rel = append(rel, ids[j])
			}
		}
		res, err := v.AddRecord(it.claim, it.scope, it.text, it.conf, nil, rel, nil)
		if err != nil {
			fatal(err)
		}
		ids[i] = res.ID
		fmt.Printf("%s  %s\n", res.ID, it.claim)
	}

	out, err := v.Viz("", "html", true)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("viz %s rules=%d bytes=%d\n", out.Path, out.RulesCount, out.SizeBytes)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

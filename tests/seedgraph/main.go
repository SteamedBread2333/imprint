package main

import (
	"fmt"
	"os"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func main() {
	v, err := imprint.Open("./memory")
	if err != nil {
		fatal(err)
	}
	add := func(claim string, scope []string, text string, conf float64, sup, rel, conflicts []string) string {
		res, err := v.AddRecord(claim, scope, text, conf, sup, rel, conflicts)
		if err != nil {
			fatal(err)
		}
		fmt.Println("added", res.ID, claim)
		return res.ID
	}

	const (
		r001 = "r-2026-09-11-001"
		r002 = "r-2026-09-11-002"
		r003 = "r-2026-09-11-003"
		r004 = "r-2026-09-11-004"
		r005 = "r-2026-09-11-005"
		r007 = "r-2026-09-11-007"
	)

	r008 := add("Ship only as MIT; do not switch the license", []string{"git", "branding"}, "License is MIT", 0.85, nil, []string{r001, r005}, nil)
	r009 := add("Cross-compile with make dist into dist/; do not commit those archives", []string{"go", "build"}, "make dist packs platform tarballs", 0.8, nil, []string{r002}, nil)
	r010 := add("Default project vault is ./memory; --global uses ~/.imprint", []string{"imprint", "storage"}, "vault discovery order in README", 0.85, nil, []string{r003}, nil)
	r011 := add("Rule IDs are r-YYYY-MM-DD-NNN and must stay human-readable", []string{"imprint", "storage"}, "ID format from §21", 0.8, nil, []string{r003, r010}, nil)
	r012 := add("imprint_find uses AND on every requested scope tag", []string{"imprint", "find"}, "precise recall by scope", 0.8, nil, []string{r003}, nil)
	r013 := add("Do not recall rules with confidence below 0.3", []string{"imprint", "find"}, "dormant should not come back from find", 0.85, nil, []string{r012}, nil)
	r014 := add("MCP tool names and JSON shapes must match PROMPT.md §21 exactly", []string{"go", "mcp"}, "stdio tools are imprint_*", 0.85, nil, []string{r004, r001}, nil)
	r015 := add("Speak MCP as hand-rolled stdio JSON-RPC; do not add an MCP SDK dependency", []string{"go", "mcp", "cli"}, "keep the binary small", 0.8, nil, []string{r014, r004, r002}, nil)
	r016 := add("viz dashboard is one HTML file; Cytoscape comes from a CDN", []string{"imprint", "viz"}, "single-file zero local JS deps", 0.8, nil, []string{r003, r002}, nil)
	r017 := add("viz graphs relationships (supersede / related / conflict), not a spreadsheet of claims", []string{"imprint", "viz"}, "filter by scope then inspect chains", 0.75, nil, []string{r016}, nil)
	r018 := add("Tests are table-driven Go tests next to the package they cover", []string{"go", "testing"}, "pkg/imprint vault_test.go style", 0.7, nil, []string{r001, r007}, nil)
	r019 := add("CLI --json stdout is the machine contract; human tables go to the same commands without --json", []string{"go", "cli"}, "--json for agents", 0.8, nil, []string{r004, r014}, nil)
	r020 := add("sweep decays untouched rules 0.05 per 90 days and archives below 0.3; it never deletes", []string{"imprint", "sweep"}, "PROMPT §7 / §14", 0.85, nil, []string{r013, r010}, nil)
	_ = add("forget removes a rule from its shard; that is the only delete path", []string{"imprint", "sweep"}, "user sovereignty", 0.85, nil, []string{r020}, nil)
	r022 := add("Put project MCP config in .cursor/mcp.json pointing at ./memory", []string{"cursor", "mcp"}, "do not rely on a TTY imprint mcp process", 0.75, nil, []string{r014, r010}, nil)
	_ = add("Cursor rule imprint-memory.mdc is alwaysApply and tells the agent to find by scope first", []string{"cursor", "branding"}, "alwaysApply memory rule", 0.7, nil, []string{r022, r005}, nil)
	r024 := add("Keep loadgen / synthetic vaults out of ./memory so demo rules stay readable", []string{"imprint", "testing"}, "tests/loadvault is gitignored", 0.7, nil, []string{r017, r018}, nil)
	r025 := add("go:embed the dashboard template into the binary", []string{"go", "viz"}, "pkg/imprint/dashboard.html", 0.75, nil, []string{r016, r002}, nil)

	blob := add("Persist the whole vault as one JSON blob", []string{"imprint", "storage"}, "first idea: single json file", 0.55, nil, []string{r003}, nil)
	yamlOnly, err := v.Supersede(blob, "Persist each rule as a YAML file in memory/", []string{"imprint", "storage"}, "split per rule", "one file per rule, yaml only")
	if err != nil {
		fatal(err)
	}
	fmt.Println("supersede", blob, "→", yamlOnly.ID)
	md, err := v.Supersede(yamlOnly.ID, "Each rule is markdown with YAML frontmatter; body is optional notes", []string{"imprint", "storage"}, "markdown is portable and grep-able", "portable markdown")
	if err != nil {
		fatal(err)
	}
	fmt.Println("supersede", yamlOnly.ID, "→", md.ID)
	packed, err := v.Supersede(md.ID, "Pack multiple rules into imprint-NNNN.md shards; start a new shard at 32768 lines or 1 MiB, whichever comes first", []string{"imprint", "storage"}, "one file per rule wasted inodes; pack until 32768 lines or 1MiB", "能不能在一个文件里多写几条")
	if err != nil {
		fatal(err)
	}
	fmt.Println("supersede", md.ID, "→", packed.ID)

	sqlite := add("Use SQLite for the vault so find can be SQL", []string{"imprint", "storage", "build"}, "maybe a db is easier", 0.5, nil, []string{r003, r002}, []string{r003, r010})
	_ = sqlite

	cobra := add("Use spf13/cobra for the CLI", []string{"go", "cli"}, "cobra is common", 0.45, nil, []string{r004}, []string{r004, r015})
	_ = cobra

	embedSDK := add("Depend on the official MCP Go SDK for stdio", []string{"go", "mcp"}, "SDK would save protocol work", 0.45, nil, []string{r014}, []string{r015, r002})
	_ = embedSDK

	_ = add("Table-driven tests must not hit the network", []string{"go", "testing"}, "keep CI hermetic", 0.7, nil, []string{r018, r001}, nil)
	_ = add("Branch and PR titles should mention imprint, not memory", []string{"git", "branding"}, "brand consistency", 0.65, nil, []string{r005, r008}, nil)
	_ = add("When graphing, prefer include-archived so supersede arrows still render", []string{"imprint", "viz"}, "archived nodes complete the chain", 0.7, nil, []string{r017, r016, r007}, nil)
	_ = add("Default find top_k is 5", []string{"imprint", "find"}, "§21 default", 0.6, nil, []string{r012, r013}, nil)
	_ = r009
	_ = r011
	_ = r019
	_ = r024
	_ = r025

	res, err := v.Viz("", "html", true)
	if err != nil {
		fatal(err)
	}
	fmt.Println("viz", res.Path, "rules", res.RulesCount, "bytes", res.SizeBytes)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

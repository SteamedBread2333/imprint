package index

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	wikiRuleRefRE  = regexp.MustCompile(`\[\[(r-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{3})\]\]`)
	imprintRuleRefRE = regexp.MustCompile(`\[imprint:(r-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{3})\]`)
)

// RuleRef is a document chunk citing a vault rule id.
type RuleRef struct {
	ChunkID string
	RuleID  string
	LineNo  int
}

// ExtractRuleRefs scans chunk text for imprint rule citations.
func ExtractRuleRefs(chunkID, text string) []RuleRef {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []RuleRef
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lineNo := i + 1
		add := func(id string) {
			key := id + "\x00" + strconv.Itoa(lineNo)
			if _, ok := seen[key]; ok {
				return
			}
			seen[key] = struct{}{}
			out = append(out, RuleRef{ChunkID: chunkID, RuleID: id, LineNo: lineNo})
		}
		for _, m := range wikiRuleRefRE.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
		for _, m := range imprintRuleRefRE.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
	}
	return out
}

// ExtractAllRuleRefs scans every chunk in the store.
func ExtractAllRuleRefs(chunks []Chunk) []RuleRef {
	var out []RuleRef
	for _, c := range chunks {
		out = append(out, ExtractRuleRefs(c.ID, c.Text)...)
	}
	return out
}

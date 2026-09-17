package index

import "testing"

func TestExtractRuleRefs(t *testing.T) {
	text := "Follow [[r-2026-09-11-001]] for naming.\nSee [imprint:r-2026-09-14-002] too.\n"
	refs := ExtractRuleRefs("chunk1", text)
	if len(refs) != 2 {
		t.Fatalf("refs = %+v", refs)
	}
	if refs[0].RuleID != "r-2026-09-11-001" || refs[0].LineNo != 1 {
		t.Fatalf("first ref: %+v", refs[0])
	}
	if refs[1].RuleID != "r-2026-09-14-002" || refs[1].LineNo != 2 {
		t.Fatalf("second ref: %+v", refs[1])
	}
}

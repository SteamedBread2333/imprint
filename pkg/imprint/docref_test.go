package imprint

import "testing"

func TestRulesReferencingDoc(t *testing.T) {
	dir := t.TempDir()
	v := frozen(t, dir, day(0))
	added, err := v.AddWithSources("Use Vitest", []string{"testing"}, "vitest", 0.85, []DocRef{
		{Path: "docs/testing.md", Heading: "Unit tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := v.RulesReferencingDoc("docs/testing.md", "Unit tests", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].ID != added.ID || refs[0].Kind != "sources" {
		t.Fatalf("refs = %+v", refs)
	}
	other, err := v.RulesReferencingDoc("docs/other.md", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("other = %+v", other)
	}
}

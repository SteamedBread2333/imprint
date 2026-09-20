package heading

import "testing"

func TestMatchNumberedBacktickHeading(t *testing.T) {
	want := "协议层合并检索与 query_local"
	got := "5.2 协议层合并检索与 `query_local`"
	if !Match(want, got) {
		t.Fatalf("Normalize(%q)=%q Normalize(%q)=%q", want, Normalize(want), got, Normalize(got))
	}
}

func TestMatchLinkLabelNotTarget(t *testing.T) {
	if !Match("MCP vs CLI", "[MCP vs CLI](mcp.zh.md)") {
		t.Fatal("link label should match; target path must not be required")
	}
	if Match("mcp.zh.md", "[MCP vs CLI](mcp.zh.md)") {
		t.Fatal("must not match on the link target")
	}
}

func TestMatchEmphasisAndQueryLocalUnderscore(t *testing.T) {
	if !Match("MCP vs CLI", "**MCP vs CLI**") {
		t.Fatal("emphasis markers should unwrap")
	}
	if Normalize("query_local") != "query_local" {
		t.Fatalf("intra-word underscore must stay: %q", Normalize("query_local"))
	}
}

func TestPlainImageAlt(t *testing.T) {
	if Plain("![diagram](arch.png)") != "diagram" {
		t.Fatalf("Plain image = %q", Plain("![diagram](arch.png)"))
	}
}

func TestMatchRejectsUnrelated(t *testing.T) {
	if Match("协议层合并检索与 query_local", "智能体记忆系统：数据存储与检索技术说明") {
		t.Fatal("unrelated headings should not match")
	}
	if Match("Correction loop", "记忆写入") {
		t.Fatal("translated titles should not match")
	}
}

func TestMatchEmpty(t *testing.T) {
	if Match("", "Naming") || Match("Naming", "") {
		t.Fatal("empty heading must not match")
	}
}

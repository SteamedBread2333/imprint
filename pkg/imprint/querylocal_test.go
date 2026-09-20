package imprint

import (
	"strings"
	"testing"
)

func TestEffectiveQueryLocal(t *testing.T) {
	if got := EffectiveQueryLocal("storage", "否定式堆砌"); got != "否定式堆砌" {
		t.Fatalf("explicit local = %q", got)
	}
	if got := EffectiveQueryLocal("切忌否定式堆砌", ""); got != "切忌否定式堆砌" {
		t.Fatalf("cjk query fallback = %q", got)
	}
	if got := EffectiveQueryLocal("storage retrieval", ""); got != "" {
		t.Fatalf("english only = %q", got)
	}
}

func TestMergeQueryLocalForStore(t *testing.T) {
	evidence := "以后host 的 HTTP handler 默认 timeout 30 秒，别无限 hang"
	got := MergeQueryLocalForStore("HTTP handler 超时 hang", evidence)
	if got == "" {
		t.Fatal("empty merge")
	}
	if !strings.Contains(got, "30") && !strings.Contains(got, "秒") {
		t.Fatalf("expected 30s tokens from evidence, got %q", got)
	}
	if !strings.Contains(got, "hang") {
		t.Fatalf("expected hang from agent ql, got %q", got)
	}
}

func TestInitialAddConfidence(t *testing.T) {
	if got := initialAddConfidence(0); got != DefaultConfidence {
		t.Fatalf("zero = %v", got)
	}
	if got := initialAddConfidence(0.9); got != DefaultConfidence {
		t.Fatalf("0.9 on add should cap to default, got %v", got)
	}
	if got := initialAddConfidence(0.85); got != 0.85 {
		t.Fatalf("0.85 correction tier = %v", got)
	}
}

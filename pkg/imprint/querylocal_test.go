package imprint

import "testing"

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

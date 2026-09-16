package imprint

import "testing"

func TestResolveVersion(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":         "devel",
		"devel":    "devel",
		"1.2.3":    "1.2.3",
		"v1.2.3":   "1.2.3",
		"  v2.0.0": "2.0.0",
	}
	for in, want := range cases {
		if got := resolveVersion(in); got != want {
			t.Fatalf("resolveVersion(%q)=%q want %q", in, got, want)
		}
	}
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

package imprint

import "testing"

func TestFindMatchesIdentifierFormattingVariants(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	tests := []struct {
		claim string
		query string
	}{
		{"Use camelCase for local variables", "Camel Case"},
		{"Keep HTTPServer handlers small", "HTTP Server"},
		{"Use snake_case for Python functions", "snake case"},
		{"Use kebab-case for route names", "kebab case"},
	}
	for _, tt := range tests {
		added, err := v.Add(tt.claim, []string{"naming"}, tt.claim, 0.6)
		if err != nil {
			t.Fatal(err)
		}
		hits, err := v.Find([]string{"naming"}, tt.query, 20)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, hit := range hits {
			if hit.ID == added.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("query %q did not find claim %q; hits=%+v", tt.query, tt.claim, hits)
		}
	}
}

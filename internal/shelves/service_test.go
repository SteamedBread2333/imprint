package shelves

import "testing"

func TestNewRejectsSensitiveRoot(t *testing.T) {
	_, err := New(Config{
		Enabled:   true,
		Workspace: t.TempDir(),
		StateDir:  t.TempDir(),
		Roots:     []string{"credentials"},
	})
	if err == nil {
		t.Fatal("credentials root was accepted")
	}
}

package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".imprint", ".plugins", "pids.json")
	if err := setPluginPID(path, "desk", 4242); err != nil {
		t.Fatal(err)
	}
	st, err := loadPluginState(path)
	if err != nil {
		t.Fatal(err)
	}
	if st["desk"] != 4242 {
		t.Fatalf("got %v", st)
	}
	clearPluginPID(path, "desk")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, stat err=%v", err)
	}
}

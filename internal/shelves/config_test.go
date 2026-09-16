package shelves

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func TestConfigFromNil(t *testing.T) {
	cfg := ConfigFrom(nil)
	if cfg.Enabled {
		t.Fatal("nil config should not enable shelves")
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "docs" {
		t.Fatalf("roots = %v", cfg.Roots)
	}
}

func TestConfigFromTopLevel(t *testing.T) {
	pcfg := &plugin.Config{
		Shelves: plugin.ShelvesConfig{
			Enabled: true,
			Config: map[string]any{
				"roots": []any{"docs", "README.md"},
			},
		},
	}
	cfg := ConfigFrom(pcfg)
	if !cfg.Enabled {
		t.Fatal("expected enabled")
	}
	if len(cfg.Roots) != 2 {
		t.Fatalf("roots = %v", cfg.Roots)
	}
}

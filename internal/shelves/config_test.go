package shelves

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
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

func TestConfigFromDefaults(t *testing.T) {
	cfg := ConfigFrom(nil)
	if cfg.FindTopK != index.DefaultFindTopK || cfg.SnippetRunes != index.DefaultSnippetRunes || cfg.MaxChunkLines != index.DefaultMaxChunkLines {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestConfigFromFindLimits(t *testing.T) {
	pcfg := &plugin.Config{
		Shelves: plugin.ShelvesConfig{
			Enabled: true,
			Config: map[string]any{
				"find_top_k":      3,
				"snippet_runes":   40,
				"max_chunk_lines": 8,
			},
		},
	}
	cfg := ConfigFrom(pcfg)
	if cfg.FindTopK != 3 || cfg.SnippetRunes != 40 || cfg.MaxChunkLines != 8 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestConfigFromInvalidLimitsFallBack(t *testing.T) {
	pcfg := &plugin.Config{
		Shelves: plugin.ShelvesConfig{
			Config: map[string]any{
				"find_top_k":      0,
				"snippet_runes":   -1,
				"max_chunk_lines": "nope",
			},
		},
	}
	cfg := ConfigFrom(pcfg)
	if cfg.FindTopK != index.DefaultFindTopK || cfg.SnippetRunes != index.DefaultSnippetRunes || cfg.MaxChunkLines != index.DefaultMaxChunkLines {
		t.Fatalf("fallback = %+v", cfg)
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

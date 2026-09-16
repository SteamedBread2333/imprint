package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func TestVaultPathsEqual(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "vault")
	b := filepath.Join(dir, "vault")
	if !vaultPathsEqual(a, b) {
		t.Fatal("expected equal clean paths")
	}
}

func TestFetchHostHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(host.HealthResponse{
			OK:    true,
			Vault: "/tmp/vault",
		})
	}))
	defer srv.Close()

	v, ok := fetchHostHealth(srv.URL)
	if !ok || v != "/tmp/vault" {
		t.Fatalf("got vault=%q ok=%v", v, ok)
	}
}

func TestParseListenPort(t *testing.T) {
	if got := plugin.ParseListenPort("127.0.0.1:9470"); got != 9470 {
		t.Fatalf("got %d", got)
	}
	if got := plugin.ParseListenPort("bad"); got != 0 {
		t.Fatalf("got %d", got)
	}
}

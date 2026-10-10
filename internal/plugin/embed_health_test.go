package plugin

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedSidecarReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"ready":true}`))
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	if !EmbedSidecarReady(port) {
		t.Fatal("want ready sidecar")
	}
}

func TestEmbedSidecarReadyNotWarm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"ready":false}`))
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	if EmbedSidecarReady(port) {
		t.Fatal("want not ready")
	}
}

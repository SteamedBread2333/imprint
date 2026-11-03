package embed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientEmbedCacheAvoidsRepeatHTTP(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if _, ok := payload["model"]; ok {
			t.Errorf("embed request included model: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"vectors":[[0.1,0.2]],"dim":2,"model":"test-model"}`))
	}))
	defer srv.Close()

	c := NewClient(portOf(srv.URL), time.Second)
	if c.Model() != "" {
		t.Fatalf("Model() before embed = %q", c.Model())
	}
	ctx := context.Background()

	if _, err := c.Embed(ctx, []string{"same claim"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Embed(ctx, []string{"same claim"}); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("HTTP embed calls = %d, want 1 (second hit from cache)", got)
	}
	if c.Model() != "test-model" {
		t.Fatalf("Model() = %q", c.Model())
	}
}

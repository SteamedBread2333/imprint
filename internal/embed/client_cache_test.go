package embed

import (
	"context"
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"vectors":[[0.1,0.2]],"dim":2,"model":"test-model"}`))
	}))
	defer srv.Close()

	c := NewClient(portOf(srv.URL), "test-model", time.Second)
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
}

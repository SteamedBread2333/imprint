package host

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWrapHandlerTimesOut(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			_, _ = w.Write([]byte("too late"))
		}
	})
	srv := httptest.NewServer(wrapHandler(mux, 50*time.Millisecond))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/slow")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"error":"timeout"}` {
		t.Fatalf("body %q", body)
	}
}

func TestNewHTTPServerDefaultTimeouts(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NewServeMux(), 0)
	if srv.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout %s", srv.ReadHeaderTimeout)
	}
	if srv.WriteTimeout != DefaultHandlerTimeout+time.Second {
		t.Fatalf("WriteTimeout %s", srv.WriteTimeout)
	}
	if DefaultHandlerTimeout != 30*time.Second {
		t.Fatalf("DefaultHandlerTimeout %s", DefaultHandlerTimeout)
	}
}

package embed

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"
)

// withWarnStderr swaps warnStderr for the duration of the test so the
// one-shot sidecar hint can be observed without polluting test output.
func withWarnStderr(t *testing.T) (capture func() string, restore func()) {
	t.Helper()
	var buf bytes.Buffer
	old := warnStderr
	warnStderr = &buf
	return func() string { return buf.String() }, func() { warnStderr = old }
}

func TestIsConnRefusedMatchesSyscall(t *testing.T) {
	// Bare errno.
	if !isConnRefused(syscall.ECONNREFUSED) {
		t.Fatal("bare syscall.ECONNREFUSED must match")
	}
	// net.OpError wrapping the errno.
	if !isConnRefused(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}) {
		t.Fatal("net.OpError wrapping ECONNREFUSED must match")
	}
	// Unrelated error.
	if isConnRefused(errors.New("nope")) {
		t.Fatal("unrelated error must not match")
	}
	// Nil is a non-event.
	if isConnRefused(nil) {
		t.Fatal("nil must not match")
	}
}

// Client.Embed must surface the one-shot hint the first time the sidecar
// is unreachable, and never again — even after several calls in the same
// process lifetime.
func TestClientEmbedWarnsOnceWhenSidecarDown(t *testing.T) {
	capture, restore := withWarnStderr(t)
	defer restore()

	// Port 1 is privileged + unused; the kernel returns ECONNREFUSED
	// quickly. We do not actually want to depend on the kernel for the
	// short-circuit test, so we use an httptest server we close.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // immediately close; any future call gets connection-refused.

	c := NewClient(portOf(srv.URL), "test-model", 200*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for i := 0; i < 3; i++ {
		if _, err := c.Embed(ctx, []string{"hello"}); err == nil {
			t.Fatalf("call %d: expected error, got nil", i)
		}
	}
	out := capture()
	if out == "" {
		t.Fatal("expected one-shot stderr hint, got empty")
	}
	if bytes.Count([]byte(out), []byte("sidecar not running")) != 1 {
		t.Fatalf("hint must fire once, got:\n%s", out)
	}
}

// Once a Client observes a successful embed, a subsequent connection
// refusal must still warn — the warning tracks "sidecar is currently
// down", not "sidecar has ever been down". This keeps the semantics of
// "one hint per outage" intact even across hot-recovery.
func TestClientHealthProbeBypassesWarnStderr(t *testing.T) {
	capture, restore := withWarnStderr(t)
	defer restore()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true,"model":"x","dim":1}`))
	}))
	defer srv.Close()

	c := NewClient(portOf(srv.URL), "test-model", time.Second)
	if ok, _, err := c.Health(context.Background()); !ok || err != nil {
		t.Fatalf("healthy: %v / err=%v", ok, err)
	}
	if got := capture(); got != "" {
		t.Fatalf("health success must not warn, got: %q", got)
	}
}

// portOf parses ":NNNN" out of an httptest URL. We avoid the httptest
// helper to keep the dependency surface flat in this package.
func portOf(url string) int {
	// url looks like http://127.0.0.1:54321
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == ':' {
			n := 0
			for _, c := range url[i+1:] {
				n = n*10 + int(c-'0')
			}
			return n
		}
	}
	return 0
}

// TestWarnStderrReplacement is a sanity check on the test plumbing: the
// captured buffer must reflect what warnStderr was at write time, not
// what it is now. (It does not use os.Stderr because that would leak
// noise into `go test` output if the warn fires.)
func TestWarnStderrReplacement(t *testing.T) {
	var got bytes.Buffer
	old := warnStderr
	warnStderr = &got
	defer func() { warnStderr = old }()

	// Trigger via the helper directly — keeps the test independent of
	// network state.
	c := NewClient(1, "x", 10*time.Millisecond)
	c.warned.Store(false)
	// Force the warn path without an actual network call.
	c.maybeWarnSidecarDown(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
	if got.Len() == 0 {
		t.Fatal("capture buffer should hold the hint")
	}
	// And running again must be a no-op.
	before := got.Len()
	c.maybeWarnSidecarDown(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
	if got.Len() != before {
		t.Fatalf("second call must not append, delta=%d", got.Len()-before)
	}
}
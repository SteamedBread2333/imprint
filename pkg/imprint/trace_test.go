package imprint

import (
	"sync"
	"testing"
	"time"
)

// sinkStub captures telemetry events for trace assertions.
type sinkStub struct {
	mu     sync.Mutex
	events []TelemetryEvent
}

func (s *sinkStub) Record(e TelemetryEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *sinkStub) byOp(op string) []TelemetryEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []TelemetryEvent
	for _, e := range s.events {
		if e.Op == op {
			out = append(out, e)
		}
	}
	return out
}

// Every event of one add (embed + vector store + add) must share one trace id.
func TestAddEventsShareTraceID(t *testing.T) {
	sink := &sinkStub{}
	stub := newStub(64)
	v, err := Open(OpenOptions{
		Dir:                     t.TempDir(),
		Now:                     func() time.Time { return day(0) },
		Telemetry:               sink,
		Embed:                   stub,
		EmbedDuplicateThreshold: 0.70,
		EmbedTimeout:            time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	if _, err := v.Add("Keep functions under 50 lines", []string{"go"}, "short", 0.8); err != nil {
		t.Fatal(err)
	}
	adds := sink.byOp("add")
	if len(adds) != 1 {
		t.Fatalf("add events = %d, want 1", len(adds))
	}
	trace := adds[0].TraceID
	if trace == "" {
		t.Fatal("add event has no trace id")
	}
	embeds := sink.byOp("embed")
	if len(embeds) == 0 {
		t.Fatal("no embed events recorded")
	}
	for _, e := range embeds {
		if e.TraceID != trace {
			t.Fatalf("embed event trace = %q, want %q", e.TraceID, trace)
		}
	}

	// A second add gets its own trace, not a replay of the first.
	if _, err := v.Add("Name interfaces by behavior", []string{"go"}, "naming", 0.8); err != nil {
		t.Fatal(err)
	}
	adds = sink.byOp("add")
	if len(adds) != 2 {
		t.Fatalf("add events = %d, want 2", len(adds))
	}
	if adds[1].TraceID == "" || adds[1].TraceID == trace {
		t.Fatalf("second add trace = %q, want a fresh id", adds[1].TraceID)
	}
}

// A rejected write must carry the same trace through the rejection chain.
func TestRejectedAddTraceLinksChain(t *testing.T) {
	sink := &sinkStub{}
	stub := newStub(64)
	stub.makeSimilar(
		"Go exported identifiers must use PascalCase",
		"Exported things use Pascal Case naming",
		0.95,
	)
	v, err := Open(OpenOptions{
		Dir:                     t.TempDir(),
		Now:                     func() time.Time { return day(0) },
		Telemetry:               sink,
		Embed:                   stub,
		EmbedDuplicateThreshold: 0.70,
		EmbedTimeout:            time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	if _, err := v.Add("Go exported identifiers must use PascalCase", []string{"go"}, "pascal", 0.8); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Add("Exported things use Pascal Case naming", []string{"go"}, "naming", 0.8); err == nil {
		t.Fatal("semantic duplicate was accepted")
	}
	rejects := sink.byOp("write_rejected")
	if len(rejects) == 0 {
		t.Fatal("no write_rejected event")
	}
	trace := rejects[0].TraceID
	if trace == "" {
		t.Fatal("rejection has no trace id")
	}
	for _, e := range rejects {
		if e.TraceID != trace {
			t.Fatalf("reject trace = %q, want %q", e.TraceID, trace)
		}
	}
}

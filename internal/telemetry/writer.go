// Package telemetry writes privacy-safe operational metrics under .imprint/state.
package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

type Writer struct {
	dir       string
	retention int
	now       func() time.Time
	warn      io.Writer
	mu        sync.Mutex
	cleaned   string
}

type event struct {
	TS             time.Time `json:"ts"`
	Op             string    `json:"op"`
	ScopeCount     int       `json:"scope_count,omitempty"`
	QueryTermCount int       `json:"query_term_count,omitempty"`
	RuleHitCount   int       `json:"rule_hit_count,omitempty"`
	DocHitCount    int       `json:"doc_hit_count,omitempty"`
	WakeCandidate  bool      `json:"wake_candidate,omitempty"`
	LatencyMS      int64     `json:"latency_ms,omitempty"`
	Code           string    `json:"code,omitempty"`
}

func New(dir string, retentionDays int, now func() time.Time, warn io.Writer) *Writer {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if now == nil {
		now = time.Now
	}
	return &Writer{dir: dir, retention: retentionDays, now: now, warn: warn}
}

func (w *Writer) Record(in imprint.TelemetryEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now().UTC()
	if in.At.IsZero() {
		in.At = now
	}
	day := now.Format("2006-01-02")
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return w.fail(err)
	}
	if w.cleaned != day {
		if err := w.cleanup(now); err != nil {
			return w.fail(err)
		}
		w.cleaned = day
	}
	f, err := os.OpenFile(filepath.Join(w.dir, day+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return w.fail(err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(event{
		TS: in.At.UTC(), Op: in.Op, ScopeCount: in.ScopeCount,
		QueryTermCount: in.QueryTermCount, RuleHitCount: in.RuleHitCount,
		DocHitCount: in.DocHitCount, WakeCandidate: in.WakeCandidate,
		LatencyMS: in.LatencyMS, Code: in.Code,
	})
	if err != nil {
		return w.fail(err)
	}
	return nil
}

func (w *Writer) cleanup(now time.Time) error {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	cutoff := now.AddDate(0, 0, -w.retention)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		day, err := time.Parse("2006-01-02", strings.TrimSuffix(name, ".jsonl"))
		if err == nil && day.Before(cutoff) {
			if err := os.Remove(filepath.Join(w.dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Writer) fail(err error) error {
	if w.warn != nil {
		fmt.Fprintf(w.warn, "imprint telemetry: %v\n", err)
	}
	return err
}

type Summary struct {
	Events     int            `json:"events"`
	ByOp       map[string]int `json:"by_op"`
	ByCode     map[string]int `json:"by_code"`
	LatencySum int64          `json:"-"`
	LatencyN   int            `json:"-"`
	AvgLatency float64        `json:"avg_latency_ms"`
}

func Summarize(dir string, since time.Time) (Summary, error) {
	out := Summary{ByOp: map[string]int{}, ByCode: map[string]int{}}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			return out, err
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var ev event
			if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.TS.Before(since) {
				continue
			}
			out.Events++
			out.ByOp[ev.Op]++
			if ev.Code != "" {
				out.ByCode[ev.Code]++
			}
			if ev.LatencyMS > 0 {
				out.LatencySum += ev.LatencyMS
				out.LatencyN++
			}
		}
		err = sc.Err()
		_ = f.Close()
		if err != nil {
			return out, err
		}
	}
	if out.LatencyN > 0 {
		out.AvgLatency = float64(out.LatencySum) / float64(out.LatencyN)
	}
	return out, nil
}

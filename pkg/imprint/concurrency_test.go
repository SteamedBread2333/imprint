package imprint

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestConcurrentAddsAcrossVaultInstances(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	const writers = 16

	start := make(chan struct{})
	ids := make(chan string, writers)
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := OpenWithNow(dir, func() time.Time { return now })
			if err != nil {
				errs <- fmt.Errorf("open: %w", err)
				return
			}
			defer v.Close()
			<-start
			res, err := v.Add(
				fmt.Sprintf("Concurrent rule %d", i),
				[]string{"concurrency"},
				fmt.Sprintf("writer %d", i),
				0.6,
			)
			if err != nil {
				errs <- fmt.Errorf("add: %w", err)
				return
			}
			ids <- res.ID
		}()
	}
	close(start)
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Errorf("concurrent add: %v", err)
	}
	if t.Failed() {
		return
	}
	seen := map[string]struct{}{}
	for id := range ids {
		if _, duplicate := seen[id]; duplicate {
			t.Errorf("duplicate id %s", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != writers {
		t.Fatalf("unique ids = %d, want %d", len(seen), writers)
	}

	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	items, err := v.List("active", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != writers {
		t.Fatalf("stored rules = %d, want %d", len(items), writers)
	}
}

func TestConcurrentReinforceIsAtomic(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	v, err := OpenWithNow(dir, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	added, err := v.Add("Keep atomic counters", []string{"concurrency"}, "original", 0.4)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}

	const writers = 4
	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := OpenWithNow(dir, func() time.Time { return now.Add(time.Duration(i+1) * time.Second) })
			if err != nil {
				errs <- err
				return
			}
			defer v.Close()
			<-start
			_, err = v.Reinforce(added.ID, fmt.Sprintf("confirm %d", i))
			if err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent reinforce: %v", err)
	}
	if t.Failed() {
		return
	}

	v, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	got, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReinforcementCount != writers {
		t.Fatalf("reinforcement_count = %d, want %d", got.ReinforcementCount, writers)
	}
	if math.Abs(got.Confidence-0.8) > 1e-9 {
		t.Fatalf("confidence = %v, want 0.8", got.Confidence)
	}
	if len(got.EvidenceLog) != writers+1 {
		t.Fatalf("evidence count = %d, want %d", len(got.EvidenceLog), writers+1)
	}
}

func TestConcurrentSupersedeAllowsOneSuccessor(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	v, err := OpenWithNow(dir, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	old, err := v.Add("Use the old policy", []string{"concurrency"}, "original", 0.7)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := OpenWithNow(dir, func() time.Time { return now.Add(time.Duration(i+1) * time.Second) })
			if err != nil {
				results <- err
				return
			}
			defer v.Close()
			<-start
			_, err = v.Supersede(
				old.ID,
				fmt.Sprintf("Use successor policy %d", i),
				[]string{"concurrency"},
				"changed",
				fmt.Sprintf("successor %d", i),
			)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful supersedes = %d, want 1", successes)
	}

	v, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	all, err := v.Show(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("rules after concurrent supersede = %d, want 2", len(all))
	}
}

func TestConcurrentAddsAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "start")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const writers = 8
	cmds := make([]*exec.Cmd, 0, writers)
	outputs := make([]*bytes.Buffer, writers)
	for i := range writers {
		outputs[i] = &bytes.Buffer{}
		cmd := exec.Command(exe, "-test.run=^TestConcurrencyWorker$")
		cmd.Env = append(os.Environ(),
			"IMPRINT_TEST_WORKER=1",
			"IMPRINT_TEST_VAULT="+dir,
			"IMPRINT_TEST_GATE="+gate,
			"IMPRINT_TEST_INDEX="+strconv.Itoa(i),
		)
		cmd.Stdout = outputs[i]
		cmd.Stderr = outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	if err := os.WriteFile(gate, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Errorf("worker %d failed: %v\n%s", i, err, outputs[i].String())
		}
	}
	if t.Failed() {
		return
	}
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	items, err := v.List("active", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != writers {
		t.Fatalf("stored rules = %d, want %d", len(items), writers)
	}
}

func TestConcurrencyWorker(t *testing.T) {
	if os.Getenv("IMPRINT_TEST_WORKER") != "1" {
		t.Skip("subprocess helper")
	}
	gate := os.Getenv("IMPRINT_TEST_GATE")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(gate); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for start gate")
		}
		time.Sleep(5 * time.Millisecond)
	}
	i, err := strconv.Atoi(os.Getenv("IMPRINT_TEST_INDEX"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	v, err := OpenWithNow(os.Getenv("IMPRINT_TEST_VAULT"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	words := []string{"amber", "cobalt", "denim", "emerald", "fuchsia", "golden", "hazel", "indigo"}
	if _, err := v.Add(
		fmt.Sprintf("Prefer %s architecture", words[i]),
		[]string{"subprocess", strconv.Itoa(i)},
		fmt.Sprintf("worker %d", i),
		0.6,
	); err != nil {
		t.Fatal(err)
	}
}

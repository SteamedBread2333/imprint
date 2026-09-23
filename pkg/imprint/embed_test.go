package imprint

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubEmbedder is a deterministic in-process stand-in for the Python sidecar:
// it hashes each text into a small vector so identical-meaning claims land
// close together and unrelated ones do not.
type stubEmbedder struct {
	model  string
	vecs   map[string][]float32
	fail   bool
	dim    int
	called int
}

func newStub(dim int) *stubEmbedder {
	return &stubEmbedder{model: "stub-model", vecs: map[string][]float32{}, dim: dim}
}

func (s *stubEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	s.called++
	if s.fail {
		return nil, errors.New("sidecar down")
	}
	out := make([][]float32, 0, len(texts))
	for _, t := range texts {
		if v, ok := s.vecs[t]; ok {
			out = append(out, v)
			continue
		}
		v := hashVector(t, s.dim)
		s.vecs[t] = v
		out = append(out, v)
	}
	return out, nil
}

func (s *stubEmbedder) Model() string { return s.model }

// hashVector builds a stable unit-ish vector from a text. Semantically related
// texts are made to share most components so cosine is high.
func hashVector(text string, dim int) []float32 {
	vec := make([]float32, dim)
	var h uint32 = 2166136261
	for i := 0; i < dim; i++ {
		h = h*16777619 + uint32(i)*31
		for _, r := range text {
			h = h*16777619 + uint32(r)
		}
		vec[i] = float32(int(h)%1000)/1000 - 0.5
	}
	return vec
}

// closeTo nudges two vectors together so their cosine clears the threshold —
// how a paraphrase looks to a real embedding model.
func (s *stubEmbedder) makeSimilar(a, b string, similarity float64) {
	va := hashVector(a, s.dim)
	vb := make([]float32, s.dim)
	for i := range va {
		vb[i] = float32(float64(va[i])*similarity + (1-similarity)*(float64(i)/float64(s.dim)-0.5))
	}
	s.vecs[a] = va
	s.vecs[b] = vb
}

func embedVault(t *testing.T, stub *stubEmbedder, thresh float64) *Vault {
	t.Helper()
	v, err := Open(OpenOptions{
		Dir:                     t.TempDir(),
		Now:                     func() time.Time { return day(0) },
		Embed:                   stub,
		EmbedDuplicateThreshold: thresh,
		EmbedTimeout:            time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

// The acceptance criterion from the handover: Jaccard (~0.38) cannot see this
// pair, the semantic gate must.
func TestEmbedBlocksSemanticDuplicate(t *testing.T) {
	stub := newStub(64)
	stub.makeSimilar(
		"Go exported identifiers must use PascalCase",
		"Exported things use Pascal Case naming",
		0.95,
	)
	v := embedVault(t, stub, 0.70)

	if _, err := v.Add("Go exported identifiers must use PascalCase", []string{"go", "naming"}, "use PascalCase", 0.8); err != nil {
		t.Fatal(err)
	}
	// storeEmbedding is synchronous: the vector has landed before Add returns.
	if n, _ := v.store.VectorCount(stub.Model()); n != 1 {
		t.Fatalf("vector not stored after add: count=%d", n)
	}

	_, err := v.Add("Exported things use Pascal Case naming", []string{"go", "naming"}, "naming", 0.8)
	if err == nil {
		t.Fatal("semantic duplicate was accepted")
	}
	var guard *WriteGuardError
	if !errors.As(err, &guard) {
		t.Fatalf("expected WriteGuardError, got %T: %v", err, guard)
	}
	if guard.Code != "duplicate" {
		t.Fatalf("code = %q", guard.Code)
	}
	if len(guard.Candidates) == 0 {
		t.Fatal("no candidates returned")
	}
	// The hint must expose both reinforce and supersede: cosine cannot tell a
	// restatement from an opposite policy.
	if !containsAll(guard.Hint, "reinforce", "supersede") {
		t.Fatalf("hint must offer reinforce and supersede: %q", guard.Hint)
	}
}

// A dead or absent sidecar must never block a write.
func TestEmbedUnavailableStillWrites(t *testing.T) {
	stub := newStub(64)
	stub.fail = true
	v := embedVault(t, stub, 0.70)
	res, err := v.Add("Go exported identifiers must use PascalCase", []string{"go", "naming"}, "use PascalCase", 0.8)
	if err != nil {
		t.Fatalf("write blocked when embed plugin is down: %v", err)
	}
	if res.ID == "" {
		t.Fatal("empty id")
	}
	// And the lexical path still rejects an exact duplicate.
	if _, err := v.Add("go exported identifiers must use pascalcase", []string{"go", "naming"}, "dup", 0.8); err == nil {
		t.Fatal("lexical duplicate accepted while embed was down")
	}
}

// Polarity must flip a blocking candidate into an advisory one.
func TestEmbedAdvisoryOnPolarityConflict(t *testing.T) {
	stub := newStub(64)
	stub.makeSimilar("Use tabs for indentation", "Never use tabs for indentation", 0.98)
	v := embedVault(t, stub, 0.70)

	if _, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.8); err != nil {
		t.Fatal(err)
	}

	// Opposite policy: same topic, polarity differs. It must be written, but
	// flagged so the caller can decide.
	res, err := v.Add("Never use tabs for indentation", []string{"go", "style"}, "no tabs", 0.8)
	if err != nil {
		t.Fatalf("opposite policy was rejected: %v", err)
	}
	if len(res.Similar) == 0 {
		t.Fatal("advisory candidates missing for a polarity conflict")
	}
}

// Vectors must not survive the rules they belong to.
func TestEmbedVectorsFollowRuleLifecycle(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	added, err := v.Add("Keep functions under 50 lines", []string{"go"}, "short", 0.8)
	if err != nil {
		t.Fatal(err)
	}
	waitVectors(t, v, stub.Model(), 1)

	// supersede: old vector dropped, successor gets one
	if _, err := v.Supersede(added.ID, "Keep functions under 80 lines", []string{"go"}, "user correction", "80 lines"); err != nil {
		t.Fatal(err)
	}
	waitVectors(t, v, stub.Model(), 1)

	if _, err := v.Forget(v.mustID(t, "Keep functions under 80 lines")); err != nil {
		t.Fatal(err)
	}
	waitVectors(t, v, stub.Model(), 0)
}

func (v *Vault) mustID(t *testing.T, claim string) string {
	t.Helper()
	recs, err := v.loadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Claim == claim {
			return r.ID
		}
	}
	t.Fatalf("rule %q not found", claim)
	return ""
}

func waitVectors(t *testing.T, v *Vault, model string, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n, _ := v.store.VectorCount(model); n == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := v.store.VectorCount(model)
	t.Fatalf("vector count = %d, want %d", got, want)
}

// A write must embed the claim exactly once: the same vector serves the
// duplicate check and the vector store (two ONNX round-trips would double
// write latency).
func TestAddEmbedsClaimOnce(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	if _, err := v.Add("Keep functions under 50 lines", []string{"go"}, "short", 0.8); err != nil {
		t.Fatal(err)
	}
	if stub.called != 1 {
		t.Fatalf("embed calls = %d, want 1", stub.called)
	}
	if n, _ := v.store.VectorCount(stub.Model()); n != 1 {
		t.Fatalf("vector count = %d, want 1", n)
	}
}

// Find must lift a zero-lexical-hit rule when its vector is close to the
// query vector, and leave the floor unchanged when it is not.
func TestFindSemanticRecallLiftsZeroHitRule(t *testing.T) {
	stub := newStub(64)
	const claim = "Wrap errors with %w"
	const query = "fault propagation strategy" // zero token overlap with claim
	stub.makeSimilar(query, claim, 0.95)
	v := embedVault(t, stub, 0.70)
	if _, err := v.Add(claim, []string{"go"}, "wrap", 0.8); err != nil {
		t.Fatal(err)
	}

	hits, err := v.Find(nil, query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	// Zero-hit floor would be 0.9*(0.3*0.8)+0.1*0.8 = 0.296; the semantic
	// score 0.9*(cos~0.95*0.8)+0.1*0.8 must clear it decisively.
	floor := 0.9*(zeroHitFloor*0.8) + 0.1*0.8
	if hits[0].Score <= floor+0.05 {
		t.Fatalf("semantic recall did not lift the hit: score=%v floor=%v", hits[0].Score, floor)
	}
}

// Without a similar vector the zero-hit rule stays on the floor, and with no
// embedder at all the score is exactly the floor — recall must never invent
// similarity.
func TestFindSemanticRecallFloorPreserved(t *testing.T) {
	const claim = "Wrap errors with %w"
	const query = "fault propagation strategy"

	stub := newStub(64) // no makeSimilar: query and claim hash far apart
	v := embedVault(t, stub, 0.70)
	if _, err := v.Add(claim, []string{"go"}, "wrap", 0.8); err != nil {
		t.Fatal(err)
	}
	hits, err := v.Find(nil, query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	floor := 0.9*(zeroHitFloor*0.8) + 0.1*0.8
	if hits[0].Score > floor+0.05 {
		t.Fatalf("unrelated rule was lifted: score=%v floor=%v", hits[0].Score, floor)
	}

	plain, err := Open(OpenOptions{Dir: t.TempDir(), Now: func() time.Time { return day(0) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = plain.Close() })
	if _, err := plain.Add(claim, []string{"go"}, "wrap", 0.8); err != nil {
		t.Fatal(err)
	}
	phits, err := plain.Find(nil, query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(phits) != 1 {
		t.Fatalf("hits = %d, want 1", len(phits))
	}
	want := float64(int(floor*10000)) / 10000
	if phits[0].Score != want {
		t.Fatalf("no-embed floor = %v, want %v", phits[0].Score, want)
	}
}

// A dead sidecar must not break find: results fall back to lexical ranking.
func TestFindDegradesWhenEmbedDown(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	if _, err := v.Add("Wrap errors with %w", []string{"go"}, "wrap", 0.8); err != nil {
		t.Fatal(err)
	}
	stub.fail = true
	if _, err := v.Find(nil, "wrap errors", 5); err != nil {
		t.Fatalf("find failed with dead sidecar: %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestPolarityConflictDetection(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Use tabs for indentation", "Use spaces for indentation", true},
		{"Always wrap errors with %w", "Return bare errors without wrapping", true},
		{"Commit directly to main", "Never commit directly to main", true},
		{"Go exported identifiers must use PascalCase", "Exported things use Pascal Case naming", false},
		{"Wrap errors with fmt.Errorf", "Always wrap errors using fmt.Errorf", false},
		{"Use table-driven tests", "Deploy to staging before release", false},
		// Chinese: the bare "别" must not fire inside 分别/别人/特别/性别.
		{"不要提交到主分支", "直接提交到主分支", true},
		{"请分别处理这两种情况", "统一处理所有情况", false},
		{"特别要求代码审查", "要求代码审查", false},
		{"性别字段必须脱敏", "性别字段必须脱敏保存", false},
		{"别用全局变量", "使用全局变量", true},
	}
	for _, c := range cases {
		if got := PolarityConflict(c.a, c.b); got != c.want {
			t.Errorf("PolarityConflict(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// Backfill: rules that pre-date embed enablement (no vector row) must get
// encoded on demand. Dry-run reports without writing. Force re-encodes
// rules that already have a vector.
func TestBackfillEmbedsMissingRules(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)

	for _, claim := range []string{"Use tabs for indentation", "Always wrap errors", "Avoid global state"} {
		if _, err := v.Add(claim, []string{"go"}, "doc", 0.7); err != nil {
			t.Fatal(err)
		}
	}
	waitVectors(t, v, stub.Model(), 3)

	// Wipe vectors to simulate a pre-embed vault.
	recs, err := v.loadAll()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	if err := v.store.DeleteRuleVectors(ids); err != nil {
		t.Fatal(err)
	}

	// Dry-run reports the count without writing.
	res := v.BackfillEmbeddings(0, false, true, 0)
	if !res.DryRun {
		t.Fatalf("dry-run flag missing: %+v", res)
	}
	if res.Scanned != 3 {
		t.Fatalf("dry-run should report 3 missing, got %d", res.Scanned)
	}
	if n, _ := v.store.VectorCount(stub.Model()); n != 0 {
		t.Fatalf("dry-run wrote vectors: count=%d", n)
	}

	// Real run actually writes.
	res = v.BackfillEmbeddings(0, false, false, 0)
	if res.Encoded != 3 {
		t.Fatalf("encoded = %d, want 3", res.Encoded)
	}
	waitVectors(t, v, stub.Model(), 3)

	// Idempotent: a second pass with force=false encodes nothing new.
	res = v.BackfillEmbeddings(0, false, false, 0)
	if res.Scanned != 0 {
		t.Fatalf("idempotent re-run scanned: %d, want 0", res.Scanned)
	}
}

// Force mode re-encodes everything — the path taken after a model upgrade.
func TestBackfillForceReencodes(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	for _, claim := range []string{"Use tabs for indentation", "Always wrap errors"} {
		if _, err := v.Add(claim, []string{"go"}, "doc", 0.7); err != nil {
			t.Fatal(err)
		}
	}
	waitVectors(t, v, stub.Model(), 2)
	called := stub.called

	res := v.BackfillEmbeddings(0, true, false, 0)
	if res.Scanned != 2 || res.Encoded != 2 {
		t.Fatalf("force backfill = %+v, want Scanned=2 Encoded=2", res)
	}
	if stub.called <= called {
		t.Fatalf("force did not re-embed: calls=%d", stub.called)
	}
}

// Backfill with no embedder installed is a silent no-op (Scanned=0). The
// user gets no rows encoded and no error.
func TestBackfillWithoutEmbedder(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir(), Now: func() time.Time { return day(0) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	if _, err := v.Add("Use tabs for indentation", []string{"go"}, "doc", 0.7); err != nil {
		t.Fatal(err)
	}
	res := v.BackfillEmbeddings(0, false, false, 0)
	if res.Scanned != 0 || res.Encoded != 0 || res.Failed != 0 || len(res.FailedIDs) != 0 {
		t.Fatalf("backfill without embedder should be zero result, got %+v", res)
	}
}

// Limit caps how many rules a backfill pass encodes.
func TestBackfillLimit(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	for _, claim := range []string{"Use tabs for indentation", "Always wrap errors", "Avoid global state"} {
		if _, err := v.Add(claim, []string{"go"}, "doc", 0.7); err != nil {
			t.Fatal(err)
		}
	}
	waitVectors(t, v, stub.Model(), 3)
	// Wipe all vectors.
	recs, _ := v.loadAll()
	ids := make([]string, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	if err := v.store.DeleteRuleVectors(ids); err != nil {
		t.Fatal(err)
	}

	res := v.BackfillEmbeddings(2, false, false, 0)
	if res.Encoded != 2 {
		t.Fatalf("encoded = %d, want 2", res.Encoded)
	}
	if res.Scanned != 2 {
		t.Fatalf("scanned = %d, want 2 (limit caps the work)", res.Scanned)
	}
}

// Import wipes rule_vectors and must re-encode the imported rules. Without
// this, every imported vault has a silent blank semantic layer until the
// user notices — see Issue 6.
func TestImportRecordsTriggersBackfill(t *testing.T) {
	stub := newStub(64)
	v := embedVault(t, stub, 0.70)
	recs := []*Record{
		{
			ID:          "r-2026-09-23-001",
			Scope:       []string{"go"},
			Claim:       "Use tabs for indentation",
			Confidence:  0.7,
			Status:      StatusActive,
			CreatedAt:   day(0),
			UpdatedAt:   day(0),
			EvidenceLog: []Evidence{{At: day(0), Kind: EvidenceOriginal, Text: "tabs"}},
		},
		{
			ID:          "r-2026-09-23-002",
			Scope:       []string{"go"},
			Claim:       "Always wrap errors",
			Confidence:  0.7,
			Status:      StatusActive,
			CreatedAt:   day(0),
			UpdatedAt:   day(0),
			EvidenceLog: []Evidence{{At: day(0), Kind: EvidenceOriginal, Text: "wrap"}},
		},
	}
	if err := v.ImportRecords(recs); err != nil {
		t.Fatal(err)
	}
	waitVectors(t, v, stub.Model(), 2)
}

// cross_scope_policy=strict: a high-cosine hit in a different scope must
// not surface at all (filtered out of the candidate pool). The default
// advisory_only keeps the cross-project recall signal.
func TestEmbedCrossScopeStrictFilters(t *testing.T) {
	stub := newStub(64)
	stub.makeSimilar("Use tabs for indentation", "Use spaces for indentation", 0.95)
	v := embedVault(t, stub, 0.70)
	v.embedCrossScope = EmbedCrossScopeStrict

	if _, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.7); err != nil {
		t.Fatal(err)
	}
	// Different scope — strict mode must not surface as advisory or blocker.
	res, err := v.Add("Use spaces for indentation", []string{"python", "style"}, "spaces", 0.7)
	if err != nil {
		t.Fatalf("strict mode rejected: %v", err)
	}
	if len(res.Similar) > 0 {
		t.Fatalf("strict mode surfaced cross-scope advisory: %+v", res.Similar)
	}
}

// Default (advisory_only) policy keeps the cross-project recall signal:
// foreign-scope paraphrase surfaces as advisory, never blocks.
func TestEmbedCrossScopeAdvisoryOnlyKeepsRecall(t *testing.T) {
	stub := newStub(64)
	stub.makeSimilar("Use tabs for indentation", "Use spaces for indentation", 0.95)
	v := embedVault(t, stub, 0.70)
	// v.embedCrossScope stays at default (advisory_only).

	if _, err := v.Add("Use tabs for indentation", []string{"go", "style"}, "tabs", 0.7); err != nil {
		t.Fatal(err)
	}
	res, err := v.Add("Use spaces for indentation", []string{"python", "style"}, "spaces", 0.7)
	if err != nil {
		t.Fatalf("advisory_only rejected: %v", err)
	}
	if len(res.Similar) == 0 {
		t.Fatal("advisory_only should surface cross-scope paraphrase as advisory")
	}
}

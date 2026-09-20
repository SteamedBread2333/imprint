package imprint

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/sqlite"
)

// Vault is a SQLite-backed imprint store at Dir/vault.db.
type Vault struct {
	Dir   string
	store *sqlite.Store
	now   func() time.Time
}

// Open creates (if needed) and returns a vault at dir.
func Open(dir string) (*Vault, error) {
	return OpenWithNow(dir, time.Now)
}

// OpenWithNow is Open with an injectable clock (tests, CLI -- now).
func OpenWithNow(dir string, now func() time.Time) (*Vault, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("vault path is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	st, err := sqlite.Open(abs, now)
	if err != nil {
		return nil, err
	}
	return &Vault{Dir: abs, store: st, now: now}, nil
}

func (v *Vault) instant() time.Time {
	if v.now == nil {
		return time.Now().UTC()
	}
	return v.now().UTC().Truncate(time.Second)
}

func (v *Vault) load(id string) (*Record, error) {
	return v.store.GetRecord(id)
}

func (v *Vault) loadAll() ([]*Record, error) {
	return v.store.AllRecords(true)
}

func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > MaxConfidence {
		return MaxConfidence
	}
	return c
}

func defaultConfidence(c float64) float64 {
	if c <= 0 {
		return DefaultConfidence
	}
	return clampConfidence(c)
}

func cleanScope(scope []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range scope {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (v *Vault) save(r *Record) error {
	return v.store.PutRecord(r)
}

// Add writes a new active rule. confidence <= 0 means 0.6.
func (v *Vault) Add(claim string, scope []string, text string, confidence float64) (*AddResult, error) {
	return v.AddRecord(claim, scope, text, confidence, nil, nil, nil, nil, "")
}

// AddWithSources is Add plus document sources for agent traceability.
func (v *Vault) AddWithSources(claim string, scope []string, text string, confidence float64, sources []DocRef) (*AddResult, error) {
	return v.AddRecord(claim, scope, text, confidence, nil, nil, nil, sources, "")
}

// AddRecord is Add plus optional relationship fields and query_local.
func (v *Vault) AddRecord(claim string, scope []string, text string, confidence float64, supersedes, related, conflicts []string, sources []DocRef, queryLocal string) (*AddResult, error) {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return nil, fmt.Errorf("claim is required")
	}
	scope = cleanScope(scope)
	if len(scope) == 0 {
		return nil, fmt.Errorf("scope is required")
	}
	now := v.instant()
	id, err := v.store.NextID(now)
	if err != nil {
		return nil, err
	}
	rec := &Record{
		ID:                 id,
		Claim:              claim,
		Scope:              scope,
		Confidence:         defaultConfidence(confidence),
		Status:             StatusActive,
		ReinforcementCount: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastTouchedAt:      now,
		Supersedes:         cleanScope(supersedes),
		Related:            cleanScope(related),
		ConflictsWith:      cleanScope(conflicts),
		Sources:            cleanDocRefs(sources),
		QueryLocal:         strings.TrimSpace(queryLocal),
		EvidenceLog: []Evidence{{
			At:   now,
			Kind: EvidenceOriginal,
			Text: text,
		}},
		Path: sqlite.DBPath(v.Dir),
	}
	if err := v.store.InsertRecord(rec); err != nil {
		return nil, err
	}
	return &AddResult{ID: rec.ID, Confidence: rec.Confidence, Path: rec.Path}, nil
}

// Get returns the full record by id, including reverse links.
func (v *Vault) Get(id string) (*Record, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	rec, err := v.store.GetRecord(id)
	if err != nil {
		return nil, err
	}
	backlinks, err := v.store.Backlinks(id)
	if err != nil {
		return nil, err
	}
	rec.ReferencedBy = backlinks
	return rec, nil
}

func dropID(ids []string, id string) ([]string, bool) {
	if len(ids) == 0 {
		return ids, false
	}
	kept := make([]string, 0, len(ids))
	changed := false
	for _, x := range ids {
		if x == id {
			changed = true
			continue
		}
		kept = append(kept, x)
	}
	if !changed {
		return ids, false
	}
	return kept, true
}

// SourcesForIDs returns sources for the given rule ids.
func (v *Vault) SourcesForIDs(ids []string) (map[string][]DocRef, error) {
	want := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}
	if len(want) == 0 {
		return map[string][]DocRef{}, nil
	}
	filtered := make([]string, 0, len(want))
	for id := range want {
		filtered = append(filtered, id)
	}
	return v.store.SourcesForIDs(filtered)
}

// List returns summaries, optionally filtered by status.
func (v *Vault) List(status string, limit int) ([]ListItem, error) {
	return v.ListFilter(ListFilter{Status: status, Limit: limit})
}

// ListFilter returns summaries sliced by status, scope AND, confidence, query, and recency.
func (v *Vault) ListFilter(f ListFilter) ([]ListItem, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(strings.ToLower(f.Status))
	scope := cleanScope(f.Scope)
	query := strings.TrimSpace(f.Query)
	var corpus []*Record
	if query != "" {
		corpus = recs
	}
	idx := buildIndex(corpus)
	var items []ListItem
	for _, r := range recs {
		if status != "" && string(r.Status) != status {
			continue
		}
		if f.MinConfidence > 0 && r.Confidence < f.MinConfidence {
			continue
		}
		if !f.Since.IsZero() && r.LastTouchedAt.Before(f.Since) {
			continue
		}
		if len(scope) > 0 {
			_, matched := scopeScore(r.Scope, scope)
			if matched != len(scope) {
				continue
			}
		}
		if query != "" && idx.score(r, query) == 0 {
			continue
		}
		items = append(items, ListItem{
			ID:         r.ID,
			Title:      r.Title(),
			Confidence: r.Confidence,
			Status:     string(r.Status),
			Scope:      r.Scope,
		})
		if f.Limit > 0 && len(items) >= f.Limit {
			break
		}
	}
	if items == nil {
		items = []ListItem{}
	}
	return items, nil
}

// Show returns full records for a user-facing listing.
func (v *Vault) Show(limit int) ([]*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool {
		order := func(s Status) int {
			switch s {
			case StatusActive:
				return 0
			case StatusDormant:
				return 1
			default:
				return 2
			}
		}
		if oi, oj := order(recs[i].Status), order(recs[j].Status); oi != oj {
			return oi < oj
		}
		return recs[i].ID > recs[j].ID
	})
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}
	return recs, nil
}

// Forget permanently deletes a record and strips inbound relationship ids.
func (v *Vault) Forget(id string) (*ForgetResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if _, err := v.load(id); err != nil {
		return nil, err
	}
	if err := v.store.StripInboundEdges(id); err != nil {
		return nil, err
	}
	if err := v.store.DeleteRecord(id); err != nil {
		return nil, err
	}
	return &ForgetResult{Success: true}, nil
}

func (v *Vault) stripInbound(id string) error {
	return v.store.StripInboundEdges(id)
}

// ExportJSON writes every record as a JSON array.
func (v *Vault) ExportJSON(w io.Writer) error {
	recs, err := v.loadAll()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(true)
	return enc.Encode(recs)
}

// Clear permanently deletes every rule in the vault.
func (v *Vault) Clear() error {
	return v.store.Clear()
}

// ImportRecords replaces vault contents with recs.
func (v *Vault) ImportRecords(recs []*Record) error {
	for _, r := range recs {
		r.Path = sqlite.DBPath(v.Dir)
	}
	return v.store.ImportRecords(recs)
}

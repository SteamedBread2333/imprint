package imprint

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Vault is a directory of packed imprint markdown shards.
type Vault struct {
	Dir           string
	now           func() time.Time
	MaxShardLines int
	MaxShardBytes int
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
	if err := os.MkdirAll(filepath.Join(abs, "archive"), 0o755); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	v := &Vault{Dir: abs, now: now}
	if err := v.compactLegacy(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *Vault) instant() time.Time {
	if v.now == nil {
		return time.Now().UTC()
	}
	return v.now().UTC().Truncate(time.Second)
}

func (v *Vault) load(id string) (*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("record %s not found", id)
}

func (v *Vault) loadAll() ([]*Record, error) {
	var out []*Record
	seen := map[string]struct{}{}
	for _, dir := range []string{v.Dir, filepath.Join(v.Dir, "archive")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !isRecordFile(e.Name()) {
				continue
			}
			recs, err := readRecords(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			for _, rec := range recs {
				if rec.ID == "" {
					continue
				}
				if _, ok := seen[rec.ID]; ok {
					continue
				}
				seen[rec.ID] = struct{}{}
				out = append(out, rec)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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

func (v *Vault) save(r *Record, archived bool) error {
	destDir := v.Dir
	if archived {
		destDir = filepath.Join(v.Dir, "archive")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
	}
	oldPath := r.Path
	if oldPath == "" {
		if existing, err := v.load(r.ID); err == nil {
			oldPath = existing.Path
		}
	}
	if oldPath != "" && filepath.Dir(oldPath) == destDir {
		recs, err := readRecords(oldPath)
		if err != nil {
			return err
		}
		replaced := false
		for i, old := range recs {
			if old.ID == r.ID {
				recs[i] = r
				replaced = true
				break
			}
		}
		if !replaced {
			recs = append(recs, r)
		}
		return writeRecords(oldPath, recs)
	}
	if oldPath != "" {
		recs, err := readRecords(oldPath)
		if err != nil {
			return err
		}
		kept := recs[:0]
		for _, old := range recs {
			if old.ID != r.ID {
				kept = append(kept, old)
			}
		}
		if err := writeRecords(oldPath, kept); err != nil {
			return err
		}
	}
	path, err := v.shardForAppend(destDir, r)
	if err != nil {
		return err
	}
	recs, err := readRecords(path)
	if err != nil {
		return err
	}
	filtered := recs[:0]
	for _, old := range recs {
		if old.ID != r.ID {
			filtered = append(filtered, old)
		}
	}
	filtered = append(filtered, r)
	return writeRecords(path, filtered)
}

// Add writes a new active rule. confidence <= 0 means 0.6.
func (v *Vault) Add(claim string, scope []string, text string, confidence float64) (*AddResult, error) {
	return v.AddRecord(claim, scope, text, confidence, nil, nil, nil)
}

// AddRecord is Add plus optional relationship fields (abstraction lift).
func (v *Vault) AddRecord(claim string, scope []string, text string, confidence float64, supersedes, related, conflicts []string) (*AddResult, error) {
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return nil, fmt.Errorf("claim is required")
	}
	scope = cleanScope(scope)
	if len(scope) == 0 {
		return nil, fmt.Errorf("scope is required")
	}
	now := v.instant()
	id, err := v.nextID(now)
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
		EvidenceLog: []Evidence{{
			At:   now,
			Kind: EvidenceOriginal,
			Text: text,
		}},
	}
	if err := v.save(rec, false); err != nil {
		return nil, err
	}
	return &AddResult{ID: rec.ID, Confidence: rec.Confidence, Path: rec.Path}, nil
}

// Get returns the full record by id (active or archived).
func (v *Vault) Get(id string) (*Record, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return v.load(id)
}

// List returns summaries, optionally filtered by status.
func (v *Vault) List(status string, limit int) ([]ListItem, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(strings.ToLower(status))
	var items []ListItem
	for _, r := range recs {
		if status != "" && string(r.Status) != status {
			continue
		}
		items = append(items, ListItem{
			ID:         r.ID,
			Title:      r.Title(),
			Confidence: r.Confidence,
			Status:     string(r.Status),
			Scope:      r.Scope,
		})
		if limit > 0 && len(items) >= limit {
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

// Forget permanently deletes a record.
func (v *Vault) Forget(id string) (*ForgetResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	rec, err := v.load(id)
	if err != nil {
		return nil, err
	}
	recs, err := readRecords(rec.Path)
	if err != nil {
		return nil, err
	}
	kept := recs[:0]
	for _, old := range recs {
		if old.ID != id {
			kept = append(kept, old)
		}
	}
	if err := writeRecords(rec.Path, kept); err != nil {
		return nil, err
	}
	return &ForgetResult{Success: true}, nil
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

// Clear permanently deletes every rule in the vault. Dashboard files are left alone.
func (v *Vault) Clear() error {
	recs, err := v.loadAll()
	if err != nil {
		return err
	}
	paths := map[string]struct{}{}
	for _, r := range recs {
		if r.Path != "" {
			paths[r.Path] = struct{}{}
		}
	}
	for path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

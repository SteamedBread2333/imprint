package imprint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Status is the lifecycle state of a rule.
type Status string

const (
	StatusActive     Status = "active"
	StatusDormant    Status = "dormant"
	StatusSuperseded Status = "superseded"
)

// EvidenceKind is why an evidence_log entry exists.
type EvidenceKind string

const (
	EvidenceOriginal  EvidenceKind = "original"
	EvidenceReinforce EvidenceKind = "reinforce"
	EvidenceSupersede EvidenceKind = "supersede"
)

const (
	DefaultConfidence    = 0.6
	MaxConfidence        = 0.95
	MinRecallConfidence  = 0.3
	DefaultDecayDays     = 90
	DefaultDecayAmount   = 0.05
	DefaultDormantThresh = 0.3
	DefaultTopK          = 5
	IDPrefix             = "r-"
	ShardFilePrefix      = "imprint-"
	DefaultMaxShardLines = 32768
	DefaultMaxShardBytes = 1 << 20
)

// Evidence is one timestamped citation backing a rule.
type Evidence struct {
	At   time.Time    `yaml:"at" json:"at"`
	Kind EvidenceKind `yaml:"kind" json:"kind"`
	Text string       `yaml:"text,omitempty" json:"text,omitempty"`
}

// Record is one imprint rule stored as markdown + YAML frontmatter.
type Record struct {
	ID                 string     `yaml:"id" json:"id"`
	Claim              string     `yaml:"claim" json:"claim"`
	Scope              []string   `yaml:"scope" json:"scope"`
	Confidence         float64    `yaml:"confidence" json:"confidence"`
	Status             Status     `yaml:"status" json:"status"`
	ReinforcementCount int        `yaml:"reinforcement_count" json:"reinforcement_count"`
	CreatedAt          time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt          time.Time  `yaml:"updated_at" json:"updated_at"`
	LastTouchedAt      time.Time  `yaml:"last_touched_at" json:"last_touched_at"`
	Supersedes         []string   `yaml:"supersedes" json:"supersedes"`
	Related            []string   `yaml:"related" json:"related"`
	ConflictsWith      []string   `yaml:"conflicts_with" json:"conflicts_with"`
	Sources            []DocRef   `yaml:"sources,omitempty" json:"sources,omitempty"`
	EvidenceLog        []Evidence `yaml:"evidence_log" json:"evidence_log"`
	Body               string     `yaml:"-" json:"body,omitempty"`
	Path               string     `yaml:"-" json:"path,omitempty"`
	ReferencedBy       []Backlink `yaml:"-" json:"referenced_by,omitempty"`
}

const seeAlsoMark = "<!-- imprint:see-also -->"

func linkIDs(r *Record) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range append(append(append([]string{}, r.Supersedes...), r.Related...), r.ConflictsWith...) {
		id = strings.TrimSpace(id)
		if id == "" || id == r.ID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func attachSeeAlso(r *Record) string {
	ids := linkIDs(r)
	body := strings.TrimSpace(r.Body)
	if len(ids) == 0 {
		return body
	}
	var b strings.Builder
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	b.WriteString(seeAlsoMark)
	b.WriteString("\nSee also:")
	for _, id := range ids {
		b.WriteString(" [[")
		b.WriteString(id)
		b.WriteString("]]")
	}
	b.WriteByte('\n')
	return b.String()
}

func stripSeeAlso(body string) string {
	if i := strings.Index(body, seeAlsoMark); i >= 0 {
		body = body[:i]
	}
	return strings.TrimSpace(body)
}

func (r *Record) normalize() {
	if r.Scope == nil {
		r.Scope = []string{}
	}
	if r.Supersedes == nil {
		r.Supersedes = []string{}
	}
	if r.Related == nil {
		r.Related = []string{}
	}
	if r.ConflictsWith == nil {
		r.ConflictsWith = []string{}
	}
	if r.Sources == nil {
		r.Sources = []DocRef{}
	}
	if r.EvidenceLog == nil {
		r.EvidenceLog = []Evidence{}
	}
	if r.Status == "" {
		r.Status = StatusActive
	}
}

func (r *Record) Title() string {
	return r.Claim
}

// MarshalRecord encodes a record as markdown with YAML frontmatter.
func MarshalRecord(r *Record) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("nil record")
	}
	dup := *r
	dup.normalize()
	meta, err := yaml.Marshal(&dup)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(meta)
	if !bytes.HasSuffix(meta, []byte("\n")) {
		b.WriteByte('\n')
	}
	b.WriteString("---\n")
	body := attachSeeAlso(&dup)
	if body != "" {
		b.WriteByte('\n')
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.Bytes(), nil
}

// MarshalRecords concatenates rules into one markdown shard.
func MarshalRecords(recs []*Record) ([]byte, error) {
	var b bytes.Buffer
	for i, r := range recs {
		chunk, err := MarshalRecord(r)
		if err != nil {
			return nil, err
		}
		if i > 0 && b.Len() > 0 && b.Bytes()[b.Len()-1] != '\n' {
			b.WriteByte('\n')
		}
		b.Write(chunk)
	}
	return b.Bytes(), nil
}

// UnmarshalRecord parses markdown with YAML frontmatter.
func UnmarshalRecord(data []byte) (*Record, error) {
	recs, err := UnmarshalRecords(data)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}
	return recs[0], nil
}

// UnmarshalRecords parses one or more frontmatter documents from a shard.
func UnmarshalRecords(data []byte) ([]*Record, error) {
	s := strings.TrimPrefix(string(data), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = stripLeadingComments(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return []*Record{}, nil
	}
	var recs []*Record
	rest := s
	for rest != "" {
		rec, leftover, err := consumeRecord(rest)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
		rest = strings.TrimSpace(leftover)
	}
	return recs, nil
}

func stripLeadingComments(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "<!--") {
		end := strings.Index(s, "-->")
		if end < 0 {
			break
		}
		s = strings.TrimSpace(s[end+3:])
	}
	return s
}

func consumeRecord(s string) (*Record, string, error) {
	if !strings.HasPrefix(s, "---") {
		return nil, "", fmt.Errorf("missing YAML frontmatter")
	}
	rest := s[3:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	idx := strings.Index(rest, "\n---")
	var yamlPart, after string
	if idx < 0 {
		yamlPart = rest
	} else {
		yamlPart = rest[:idx]
		after = rest[idx+len("\n---"):]
		if strings.HasPrefix(after, "\n") {
			after = after[1:]
		}
	}
	body, leftover := splitBody(after)
	var r Record
	if err := yaml.Unmarshal([]byte(yamlPart), &r); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if r.ID == "" {
		return nil, "", fmt.Errorf("frontmatter missing id")
	}
	r.Body = stripSeeAlso(body)
	r.normalize()
	return &r, leftover, nil
}

func splitBody(after string) (body, rest string) {
	if after == "" {
		return "", ""
	}
	if isRecordStart(after) {
		return "", after
	}
	searchFrom := 0
	for {
		i := strings.Index(after[searchFrom:], "\n---")
		if i < 0 {
			return after, ""
		}
		pos := searchFrom + i + 1
		if isRecordStart(after[pos:]) {
			return after[:pos], after[pos:]
		}
		searchFrom = pos + 3
	}
}

func isRecordStart(s string) bool {
	if !strings.HasPrefix(s, "---") {
		return false
	}
	rest := s[3:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	head := rest
	if i := strings.Index(rest, "\n---"); i >= 0 {
		head = rest[:i]
	}
	trim := strings.TrimSpace(head)
	return strings.HasPrefix(trim, "id:") || strings.Contains(head, "\nid:")
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

func writeRecords(path string, recs []*Record) error {
	if len(recs) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := MarshalRecords(recs)
	if err != nil {
		return err
	}
	if looksLikeShard(strings.TrimSuffix(filepath.Base(path), ".md")) {
		header := fmt.Sprintf("<!-- imprint pack (%d rules) -->\n", len(recs))
		data = append([]byte(header), data...)
	}
	for _, r := range recs {
		r.Path = path
	}
	return writeFileAtomic(path, data)
}

func readRecords(path string) ([]*Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	recs, err := UnmarshalRecords(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for _, r := range recs {
		r.Path = path
	}
	return recs, nil
}

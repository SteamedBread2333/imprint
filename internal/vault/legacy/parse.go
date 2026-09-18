// Package legacy parses pre-SQLite imprint-NNNN.md markdown shards.
package legacy

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

const seeAlsoMark = "<!-- imprint:see-also -->"

// ParseShardFile reads one imprint-NNNN.md shard.
func ParseShardFile(path string) ([]*model.Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	recs, err := ParseShard(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return recs, nil
}

// ParseShard parses one or more YAML-frontmatter rules from shard bytes.
func ParseShard(data []byte) ([]*model.Record, error) {
	s := strings.TrimPrefix(string(data), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = stripLeadingComments(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var recs []*model.Record
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

func consumeRecord(s string) (*model.Record, string, error) {
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
	var r model.Record
	if err := yaml.Unmarshal([]byte(yamlPart), &r); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if r.ID == "" {
		return nil, "", fmt.Errorf("frontmatter missing id")
	}
	r.Body = stripSeeAlso(body)
	normalizeRecord(&r)
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

func stripSeeAlso(body string) string {
	if i := strings.Index(body, seeAlsoMark); i >= 0 {
		body = body[:i]
	}
	return strings.TrimSpace(body)
}

func normalizeRecord(r *model.Record) {
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
		r.Sources = []model.DocRef{}
	}
	if r.EvidenceLog == nil {
		r.EvidenceLog = []model.Evidence{}
	}
	if r.Status == "" {
		r.Status = model.StatusActive
	}
}

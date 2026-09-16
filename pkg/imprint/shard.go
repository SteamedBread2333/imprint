package imprint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func looksLikeShard(base string) bool {
	name := base
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	_, ok := parseShardNum(name)
	return ok
}

func parseShardNum(name string) (int, bool) {
	if !strings.HasSuffix(name, ".md") {
		return 0, false
	}
	base := strings.TrimSuffix(name, ".md")
	if !strings.HasPrefix(base, ShardFilePrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(base, ShardFilePrefix))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func isRecordFile(name string) bool {
	if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	return looksLikeID(base) || looksLikeShard(base)
}

func isLegacyRecordFile(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	return looksLikeID(strings.TrimSuffix(name, ".md"))
}

func shardName(n int) string {
	return fmt.Sprintf("%s%04d.md", ShardFilePrefix, n)
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func (v *Vault) maxShardLines() int {
	if v.MaxShardLines > 0 {
		return v.MaxShardLines
	}
	return DefaultMaxShardLines
}

func (v *Vault) maxShardBytes() int {
	if v.MaxShardBytes > 0 {
		return v.MaxShardBytes
	}
	return DefaultMaxShardBytes
}

func (v *Vault) inArchive(path string) bool {
	if path == "" {
		return false
	}
	return filepath.Base(filepath.Dir(path)) == "archive"
}

func (v *Vault) highestShard(dir string) (int, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	maxN := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n, ok := parseShardNum(e.Name())
		if ok && n > maxN {
			maxN = n
		}
	}
	if maxN == 0 {
		return 0, ""
	}
	return maxN, filepath.Join(dir, shardName(maxN))
}

func (v *Vault) shardForAppend(dir string, extra *Record) (string, error) {
	chunk, err := MarshalRecord(extra)
	if err != nil {
		return "", err
	}
	maxN, last := v.highestShard(dir)
	if last == "" {
		return filepath.Join(dir, shardName(1)), nil
	}
	data, err := os.ReadFile(last)
	if err != nil {
		if os.IsNotExist(err) {
			return last, nil
		}
		return "", err
	}
	if len(data) == 0 {
		return last, nil
	}
	if lineCount(data)+lineCount(chunk) <= v.maxShardLines() && len(data)+len(chunk) <= v.maxShardBytes() {
		return last, nil
	}
	return filepath.Join(dir, shardName(maxN+1)), nil
}

func (v *Vault) hasLegacy() bool {
	for _, dir := range []string{v.Dir, filepath.Join(v.Dir, "archive")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && isLegacyRecordFile(e.Name()) {
				return true
			}
		}
	}
	return false
}

func (v *Vault) compactLegacy() error {
	if !v.hasLegacy() {
		return nil
	}
	recs, err := v.loadAll()
	if err != nil {
		return err
	}
	var active, archived []*Record
	for _, r := range recs {
		if v.inArchive(r.Path) || r.Status != StatusActive {
			archived = append(archived, r)
		} else {
			active = append(active, r)
		}
	}
	if err := v.rewriteDir(v.Dir, active); err != nil {
		return err
	}
	return v.rewriteDir(filepath.Join(v.Dir, "archive"), archived)
}

func (v *Vault) rewriteDir(dir string, recs []*Record) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !isRecordFile(e.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	shards := splitShards(recs, v.maxShardLines(), v.maxShardBytes())
	for i, shard := range shards {
		path := filepath.Join(dir, shardName(i+1))
		if err := writeRecords(path, shard); err != nil {
			return err
		}
	}
	return nil
}

func splitShards(recs []*Record, maxLines, maxBytes int) [][]*Record {
	if maxLines <= 0 {
		maxLines = DefaultMaxShardLines
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxShardBytes
	}
	var shards [][]*Record
	var cur []*Record
	lines, size := 0, 0
	for _, r := range recs {
		chunk, err := MarshalRecord(r)
		if err != nil {
			continue
		}
		cl, cb := lineCount(chunk), len(chunk)
		if len(cur) > 0 && (lines+cl > maxLines || size+cb > maxBytes) {
			shards = append(shards, cur)
			cur = nil
			lines, size = 0, 0
		}
		cur = append(cur, r)
		lines += cl
		size += cb
	}
	if len(cur) > 0 {
		shards = append(shards, cur)
	}
	return shards
}

// ImportRecords replaces vault contents with recs, packed into shards.
func (v *Vault) ImportRecords(recs []*Record) error {
	var active, archived []*Record
	for _, r := range recs {
		if r == nil {
			continue
		}
		r.normalize()
		if r.Status == StatusActive {
			active = append(active, r)
		} else {
			archived = append(archived, r)
		}
	}
	if err := v.rewriteDir(v.Dir, active); err != nil {
		return err
	}
	return v.rewriteDir(filepath.Join(v.Dir, "archive"), archived)
}

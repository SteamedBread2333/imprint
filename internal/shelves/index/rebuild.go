package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Rebuild scans workspace roots and writes a fresh index if fingerprint changed.
func Rebuild(workspace string, roots []string, stateDir string, maxChunkLines int) (*Store, bool, error) {
	if maxChunkLines <= 0 {
		maxChunkLines = DefaultMaxChunkLines
	}
	fp, err := Fingerprint(workspace, roots, maxChunkLines)
	if err != nil {
		return nil, false, err
	}
	if old, err := LoadStore(stateDir); err == nil && old.Fingerprint == fp && len(old.Chunks) > 0 {
		return old, false, nil
	}
	chunks, fileCount, err := ChunkFiles(workspace, roots, maxChunkLines)
	if err != nil {
		return nil, false, err
	}
	s := &Store{
		Fingerprint: fp,
		BuiltAt:     time.Now().UTC(),
		FileCount:   fileCount,
		Chunks:      chunks,
		RuleRefs:    ExtractAllRuleRefs(chunks),
	}
	if err := SaveStore(stateDir, s); err != nil {
		return nil, false, err
	}
	return s, true, nil
}

// Fingerprint hashes file paths with size and modtime under roots.
func Fingerprint(workspace string, roots []string, maxChunkLines int) (string, error) {
	if maxChunkLines <= 0 {
		maxChunkLines = DefaultMaxChunkLines
	}
	h := sha256.New()
	fmt.Fprintf(h, "max_chunk_lines:%d\n", maxChunkLines)
	for _, root := range roots {
		base := filepath.Join(workspace, root)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".mdc" && ext != ".txt" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(workspace, path)
			fmt.Fprintf(h, "%s:%d:%d\n", rel, info.Size(), info.ModTime().UnixNano())
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

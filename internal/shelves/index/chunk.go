package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Chunk is one searchable document segment.
type Chunk struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Heading   string `json:"heading"`
	Text      string `json:"text"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
}

// ChunkFiles splits markdown files under roots into chunks.
func ChunkFiles(workspace string, roots []string) ([]Chunk, int, error) {
	var chunks []Chunk
	fileCount := 0
	for _, root := range roots {
		base := filepath.Join(workspace, root)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") && path != base {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".mdc" && ext != ".txt" {
				return nil
			}
			fileCount++
			rel, _ := filepath.Rel(workspace, path)
			rel = filepath.ToSlash(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			chunks = append(chunks, chunkMarkdown(rel, string(data))...)
			return nil
		})
		if err != nil {
			return chunks, fileCount, err
		}
	}
	return chunks, fileCount, nil
}

func chunkMarkdown(relPath, content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	heading := filepath.Base(relPath)
	var buf []string
	start := 1

	flush := func(end int) {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		if text == "" {
			return
		}
		id := chunkID(relPath, start, end, heading)
		chunks = append(chunks, Chunk{
			ID:        id,
			Path:      relPath,
			Heading:   heading,
			Text:      text,
			LineStart: start,
			LineEnd:   end,
		})
		buf = buf[:0]
	}

	for i, line := range lines {
		ln := i + 1
		if strings.HasPrefix(line, "#") {
			if len(buf) > 0 {
				flush(ln - 1)
			}
			heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
			start = ln
			continue
		}
		buf = append(buf, line)
		if len(buf) >= 40 {
			flush(ln)
			start = ln + 1
		}
	}
	if len(buf) > 0 {
		flush(len(lines))
	}
	return chunks
}

func chunkID(path string, start, end int, heading string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%s", path, start, end, heading)))
	return hex.EncodeToString(h[:8])
}

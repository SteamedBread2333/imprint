package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkMarkdownRespectsMaxLines(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line body")
	}
	chunks := chunkMarkdown("docs/a.md", strings.Join(lines, "\n"), 12)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2: %+v", len(chunks), chunks)
	}
	if chunks[0].LineEnd-chunks[0].LineStart+1 > 12 {
		t.Fatalf("first chunk too long: %+v", chunks[0])
	}
}

func TestChunkFilesUsesMaxLines(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line body")
	}
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	got, n, err := ChunkFiles(root, []string{"docs"}, 12)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(got) != 2 {
		t.Fatalf("files=%d chunks=%d", n, len(got))
	}
}

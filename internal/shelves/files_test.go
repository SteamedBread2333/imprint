package shelves

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDocFileUnderRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "docs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "a.md")
	if err := os.WriteFile(path, []byte("# Hello\n\nworld"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readDocFile(dir, []string{"docs"}, "docs/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "docs/a.md" || got.Content != "# Hello\n\nworld" {
		t.Fatalf("got %+v", got)
	}
}

func TestReadDocFileRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := readDocFile(dir, []string{"docs"}, "../secret.md")
	if err != ErrDocNotFound {
		t.Fatalf("want ErrDocNotFound, got %v", err)
	}
}

func TestReadDocFileRejectsOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readDocFile(dir, []string{"docs"}, "other.md")
	if err != ErrDocNotFound {
		t.Fatalf("want ErrDocNotFound, got %v", err)
	}
}

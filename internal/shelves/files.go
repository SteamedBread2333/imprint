package shelves

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrDocNotFound is returned when a document path is missing or not allowed.
var ErrDocNotFound = errors.New("document not found")

// DocFile is a workspace markdown file for preview.
type DocFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Bytes   int    `json:"bytes"`
}

// ReadDocFile returns raw file content when relPath is under configured roots.
func (s *Service) ReadDocFile(relPath string) (DocFile, error) {
	return readDocFile(s.cfg.Workspace, s.cfg.Roots, relPath)
}

func readDocFile(workspace string, roots []string, relPath string) (DocFile, error) {
	relPath = strings.TrimSpace(filepath.ToSlash(relPath))
	if relPath == "" || strings.Contains(relPath, "..") {
		return DocFile{}, ErrDocNotFound
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext != ".md" && ext != ".mdc" && ext != ".txt" {
		return DocFile{}, ErrDocNotFound
	}
	abs, ok := docPathUnderRoots(workspace, roots, relPath)
	if !ok {
		return DocFile{}, ErrDocNotFound
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return DocFile{}, ErrDocNotFound
		}
		return DocFile{}, err
	}
	return DocFile{
		Path:    relPath,
		Content: string(data),
		Bytes:   len(data),
	}, nil
}

func docPathUnderRoots(workspace string, roots []string, relPath string) (string, bool) {
	workspace = filepath.Clean(workspace)
	abs := filepath.Clean(filepath.Join(workspace, filepath.FromSlash(relPath)))
	if !pathWithin(abs, workspace) {
		return "", false
	}
	for _, root := range roots {
		base := filepath.Clean(filepath.Join(workspace, filepath.FromSlash(root)))
		if pathWithin(abs, base) {
			return abs, true
		}
	}
	return "", false
}

func pathWithin(path, base string) bool {
	if path == base {
		return true
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// ReadDocFileError maps read errors to HTTP-friendly messages.
func ReadDocFileError(err error) error {
	if errors.Is(err, ErrDocNotFound) {
		return ErrDocNotFound
	}
	return fmt.Errorf("read document: %w", err)
}

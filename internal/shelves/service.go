package shelves

import (
	"errors"
	"sync"

	"github.com/SteamedBread2333/imprint/internal/shelves/index"
)

// ErrDisabled is returned when shelves indexing/search is turned off.
var ErrDisabled = errors.New("shelves is disabled")

// Service is the built-in workspace document index.
type Service struct {
	cfg Config
	mu  sync.RWMutex
	st  *index.Store
}

// New loads or rebuilds the shelves index according to cfg.
func New(cfg Config) (*Service, error) {
	cfg.StateDir = cfg.ResolveStateDir()
	s := &Service{cfg: cfg}
	if cfg.Enabled {
		st, _, err := index.Rebuild(cfg.Workspace, cfg.Roots, cfg.StateDir)
		if err != nil {
			return nil, err
		}
		s.st = st
		return s, nil
	}
	st, err := index.LoadOrEmpty(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	s.st = st
	return s, nil
}

// Config returns a copy of the service configuration.
func (s *Service) Config() Config {
	return s.cfg
}

// Status returns shelves health for GET /health.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return statusFromStore(s.cfg, s.st)
}

func (s *Service) snap() *index.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st
}

// Search runs BM25 when shelves is enabled.
func (s *Service) Search(query string, topK int) ([]index.SearchHit, error) {
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	st := s.snap()
	if st == nil {
		return nil, nil
	}
	if topK <= 0 {
		topK = 5
	}
	return st.SearchStore(query, topK), nil
}

// ChunkByID returns one chunk from cache (works when disabled).
func (s *Service) ChunkByID(id string) (index.Chunk, bool) {
	st := s.snap()
	if st == nil {
		return index.Chunk{}, false
	}
	return st.ChunkByID(id)
}

// Graph builds the document relationship graph from cache.
func (s *Service) Graph() DocGraph {
	st := s.snap()
	if st == nil {
		return BuildGraph(nil)
	}
	return BuildGraph(st.Chunks)
}

// Stats returns index metadata from cache.
func (s *Service) Stats() Stats {
	st := s.snap()
	if st == nil {
		return Stats{Roots: s.cfg.Roots}
	}
	return Stats{
		ChunkCount:  len(st.Chunks),
		FileCount:   st.FileCount,
		BuiltAt:     st.BuiltAt,
		Fingerprint: st.Fingerprint,
		Roots:       s.cfg.Roots,
	}
}

// Rebuild forces a rescan when shelves is enabled.
func (s *Service) Rebuild() (*index.Store, error) {
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	st, _, err := index.Rebuild(s.cfg.Workspace, s.cfg.Roots, s.cfg.StateDir)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.st = st
	s.mu.Unlock()
	return st, nil
}

// HasChunk reports whether id is a document chunk in the loaded index.
func (s *Service) HasChunk(id string) bool {
	return HasChunk(s.snap(), id)
}

// IndexStore returns the loaded shelves index (may be nil).
func (s *Service) IndexStore() *index.Store {
	return s.snap()
}

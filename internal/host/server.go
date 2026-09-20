package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Config holds vault read-only HTTP server settings.
type Config struct {
	Listen  string
	Vault   *imprint.Vault
	Shelves *shelves.Service
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	OK      bool           `json:"ok"`
	Vault   string         `json:"vault"`
	Version string         `json:"version"`
	Shelves shelves.Status `json:"shelves"`
}

// Serve starts the read-only vault HTTP API until ctx is cancelled.
func Serve(ctx context.Context, cfg Config) error {
	if cfg.Vault == nil {
		return fmt.Errorf("vault is required")
	}
	listen := strings.TrimSpace(cfg.Listen)
	if listen == "" {
		listen = imprint.DefaultHostListen
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		resp := HealthResponse{
			OK:      true,
			Vault:   cfg.Vault.Dir,
			Version: imprint.Version,
		}
		if cfg.Shelves != nil {
			resp.Shelves = cfg.Shelves.Status()
		}
		writeJSON(w, resp)
	})
	mux.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		inc := r.URL.Query().Get("include_archived") == "true"
		data, err := cfg.Vault.Graph(inc)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, data)
	})
	mux.HandleFunc("/graph/unified", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		inc := r.URL.Query().Get("include_archived") == "true"
		var st *index.Store
		if cfg.Shelves != nil {
			st = cfg.Shelves.IndexStore()
		}
		data, err := linking.BuildUnifiedGraph(cfg.Vault, st, inc)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, data)
	})
	mux.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		f := imprint.ListFilter{
			Status: r.URL.Query().Get("status"),
			Query:  r.URL.Query().Get("query"),
		}
		if s := r.URL.Query().Get("scope"); s != "" {
			f.Scope = splitCSV(s)
		}
		if s := r.URL.Query().Get("min_confidence"); s != "" {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid min_confidence"))
				return
			}
			f.MinConfidence = v
		}
		if s := r.URL.Query().Get("limit"); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid limit"))
				return
			}
			f.Limit = v
		}
		if s := r.URL.Query().Get("since"); s != "" {
			t, err := parseSince(s)
			if err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			f.Since = t
		}
		items, err := cfg.Vault.ListFilter(f)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, items)
	})
	mux.HandleFunc("/rules/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/rules/")
		id = strings.TrimSpace(id)
		if id == "" || strings.Contains(id, "/") {
			writeErr(w, http.StatusNotFound, fmt.Errorf("rule not found"))
			return
		}
		rec, err := cfg.Vault.Get(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		if cfg.Shelves != nil {
			writeJSON(w, linking.EnrichRuleGet(cfg.Shelves.IndexStore(), rec))
			return
		}
		writeJSON(w, rec)
	})
	mux.HandleFunc("/find", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		topK := imprint.DefaultTopK
		if s := r.URL.Query().Get("top_k"); s != "" {
			v, err := strconv.Atoi(s)
			if err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid top_k"))
				return
			}
			topK = v
		}
		query := r.URL.Query().Get("query")
		queryLocal := r.URL.Query().Get("query_local")
		scope := splitCSV(r.URL.Query().Get("scope"))
		if cfg.Shelves != nil && shelves.MatchFindQuery(cfg.Shelves.Config(), query, queryLocal) {
			enriched, _, err := linking.EnrichFind(cfg.Vault, cfg.Shelves.IndexStore(), scope, query, queryLocal, topK)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, enriched)
			return
		}
		hits, err := cfg.Vault.FindMerged(scope, query, queryLocal, topK)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if cfg.Shelves != nil && cfg.Shelves.IndexStore() != nil && len(hits) > 0 {
			ids := make([]string, len(hits))
			for i, h := range hits {
				ids[i] = h.ID
			}
			sourcesByID, err := shelves.SourcesByIDs(cfg.Vault, ids)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, shelves.EnrichFindHits(cfg.Shelves.IndexStore(), hits, sourcesByID))
			return
		}
		writeJSON(w, hits)
	})
	mux.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := cfg.Vault.ExportJSON(w); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
		}
	})
	registerDocsRoutes(mux, cfg.Vault, cfg.Shelves)

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	return srv.Serve(ln)
}

func registerDocsRoutes(mux *http.ServeMux, vault *imprint.Vault, svc *shelves.Service) {
	mux.HandleFunc("/docs/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if svc == nil {
			writeErr(w, http.StatusServiceUnavailable, shelves.ErrDisabled)
			return
		}
		var req struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(req.Query) == "" {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("query is required"))
			return
		}
		hits, err := svc.Search(req.Query, req.TopK)
		if err != nil {
			writeErr(w, http.StatusServiceUnavailable, err)
			return
		}
		writeJSON(w, map[string]any{"hits": hits})
	})
	mux.HandleFunc("/docs/graph", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if svc == nil {
			writeJSON(w, shelves.BuildGraph(nil))
			return
		}
		writeJSON(w, svc.Graph())
	})
	mux.HandleFunc("/docs/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if svc == nil {
			writeJSON(w, shelves.Stats{})
			return
		}
		writeJSON(w, svc.Stats())
	})
	mux.HandleFunc("/docs/rebuild", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		if svc == nil || !svc.Config().Enabled {
			writeErr(w, http.StatusForbidden, shelves.ErrDisabled)
			return
		}
		st, err := svc.Rebuild()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]any{
			"rebuilt":     true,
			"chunk_count": len(st.Chunks),
			"fingerprint": st.Fingerprint,
		})
	})
	mux.HandleFunc("/docs/chunks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/docs/chunks/")
		id = strings.TrimSpace(id)
		if id == "" {
			writeErr(w, http.StatusNotFound, fmt.Errorf("chunk not found"))
			return
		}
		if svc == nil {
			writeErr(w, http.StatusNotFound, fmt.Errorf("chunk not found"))
			return
		}
		c, ok := svc.ChunkByID(id)
		if !ok {
			writeErr(w, http.StatusNotFound, fmt.Errorf("chunk not found"))
			return
		}
		out, err := linking.EnrichChunkGet(vault, svc.IndexStore(), c)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("/docs/file", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if svc == nil {
			writeErr(w, http.StatusNotFound, shelves.ErrDocNotFound)
			return
		}
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if path == "" {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("path query is required"))
			return
		}
		doc, err := svc.ReadDocFile(path)
		if err != nil {
			if errors.Is(err, shelves.ErrDocNotFound) {
				writeErr(w, http.StatusNotFound, err)
				return
			}
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, doc)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(imprint.ErrorBody{Error: err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid since %q (use YYYY-MM-DD or RFC3339)", s)
}

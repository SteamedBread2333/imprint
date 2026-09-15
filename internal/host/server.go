package host

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Config holds vault read-only HTTP server settings.
type Config struct {
	Listen string
	Vault  *imprint.Vault
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Vault   string `json:"vault"`
	Version string `json:"version"`
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
		writeJSON(w, HealthResponse{
			OK:      true,
			Vault:   cfg.Vault.Dir,
			Version: imprint.Version,
		})
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
		hits, err := cfg.Vault.Find(splitCSV(r.URL.Query().Get("scope")), r.URL.Query().Get("query"), topK)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
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

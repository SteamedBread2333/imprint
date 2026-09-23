// Package embed is a thin HTTP client for the imprint-embed Python sidecar.
//
// The client is deliberately dumb: it models one plugin endpoint and never
// retries internally beyond a single bounded attempt, because every caller
// (semantic duplicate detection) must degrade to the lexical Jaccard path
// rather than wait on a cold or crashed plugin.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// DefaultPort is the imprint-embed sidecar port.
const DefaultPort = imprint.DefaultEmbedPort

// warnStderr is the sink for one-shot sidecar-not-running hints. Tests swap
// it; the default is os.Stderr so the message reaches the operator's
// terminal. The hint is emitted at most once per Client lifetime so a
// long-running command with many embed calls does not flood stderr.
var warnStderr io.Writer = os.Stderr

// Client calls a running embed plugin over loopback HTTP.
// It is safe for concurrent use.
type Client struct {
	baseURL string
	model   string
	http    *http.Client

	warned atomic.Bool
}

// NewClient returns a client for a sidecar listening on port.
func NewClient(port int, model string, timeout time.Duration) *Client {
	if port <= 0 {
		port = DefaultPort
	}
	if timeout <= 0 {
		timeout = imprint.DefaultEmbedTimeoutSeconds * time.Second
	}
	return &Client{
		baseURL: imprint.LocalURL(port),
		model:   model,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        8,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     60 * time.Second,
			},
		},
	}
}

// Model returns the embedding model id for this client's vector space.
func (c *Client) Model() string {
	if c.model == "" {
		return imprint.EmbedModel
	}
	return c.model
}

// BaseURL returns the sidecar root URL.
func (c *Client) BaseURL() string { return c.baseURL }

type embedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model,omitempty"`
}

type embedResponse struct {
	Vectors [][]float32 `json:"vectors"`
	Dim     int         `json:"dim"`
	Model   string      `json:"model"`
	Error   string      `json:"error,omitempty"`
}

type healthResponse struct {
	OK    bool   `json:"ok"`
	Model string `json:"model"`
	Dim   int    `json:"dim"`
}

// Embed returns one vector per text. It honors ctx and the client timeout; a
// non-2xx response or a sidecar-reported error is returned as an error so the
// caller can fall back to the lexical path.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embedRequest{Texts: texts, Model: c.Model()})
	if err != nil {
		return nil, fmt.Errorf("encode embed request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.maybeWarnSidecarDown(err)
		return nil, fmt.Errorf("call embed plugin: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embed plugin returned HTTP %d", resp.StatusCode)
	}
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if out.Error != "" {
		return nil, errors.New("embed plugin: " + out.Error)
	}
	if len(out.Vectors) != len(texts) {
		return nil, fmt.Errorf("embed plugin returned %d vectors for %d texts", len(out.Vectors), len(texts))
	}
	return out.Vectors, nil
}

// maybeWarnSidecarDown emits a single-shot stderr hint when the connection
// error suggests the sidecar is not running at all (rather than e.g. a
// transient timeout). The check is local: it scans the error chain for the
// common "connection refused" signals the stdlib produces across platforms.
//
// We only fire this once per Client; embedding servers (MCP) live for the
// full process lifetime, and even short CLI commands can produce several
// embed calls in one run.
func (c *Client) maybeWarnSidecarDown(err error) {
	if c.warned.Load() {
		return
	}
	if !isConnRefused(err) {
		return
	}
	c.warned.Store(true)
	fmt.Fprintf(warnStderr,
		"imprint embed: sidecar not running on %s — semantic gate will fall back to Jaccard.\n"+
			"imprint embed: start it manually with: imprint plugin start embed\n",
		c.baseURL,
	)
}

// isConnRefused recognises the "no one is listening on the port" failure
// modes produced by net/http and net across platforms.
func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		// ECONNREFUSED on Linux/macOS; on some Windows builds the inner
		// error wraps a syscall.Errno. Match both.
		if errors.Is(netErr.Err, syscall.ECONNREFUSED) {
			return true
		}
	}
	return false
}

// Health reports whether the sidecar is up and which model it serves.
func (c *Client) Health(ctx context.Context) (bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false, "", fmt.Errorf("build health request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("call embed plugin: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("embed plugin health HTTP %d", resp.StatusCode)
	}
	var out healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "", fmt.Errorf("decode health response: %w", err)
	}
	return out.OK, out.Model, nil
}

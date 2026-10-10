// Package loopback provides HTTP clients for 127.0.0.1 services.
// They bypass HTTP(S)_PROXY so local sidecars stay reachable when a
// corporate proxy is configured for the shell or IDE.
package loopback

import (
	"net"
	"net/http"
	"time"
)

// Client returns an *http.Client that never uses a proxy.
func Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           nil,
			DialContext:     dialer.DialContext,
			MaxIdleConns:    4,
			IdleConnTimeout: 60 * time.Second,
		},
	}
}

package plugin

import (
	"net"
	"strconv"
)

// ParseListenPort extracts the TCP port from a host:port listen address.
func ParseListenPort(listen string) int {
	_, portStr, err := net.SplitHostPort(listen)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

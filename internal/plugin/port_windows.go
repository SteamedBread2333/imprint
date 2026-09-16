//go:build windows

package plugin

import (
	"fmt"
	"time"
)

func pidsOnPort(port int) ([]int, error) {
	return nil, nil
}

// FreeListenPort kills processes bound to host:port.
func FreeListenPort(listen string, wait time.Duration) error {
	_ = listen
	_ = wait
	return fmt.Errorf("FreeListenPort not implemented on windows")
}

func freePort(port int, wait time.Duration) error {
	_ = wait
	return fmt.Errorf("port %d: freePort not implemented on windows", port)
}

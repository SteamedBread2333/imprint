//go:build windows

package plugin

import (
	"fmt"
	"time"
)

func pidsOnPort(port int) ([]int, error) {
	return nil, nil
}

func freePort(port int, wait time.Duration) error {
	_ = wait
	return fmt.Errorf("port %d: freePort not implemented on windows", port)
}

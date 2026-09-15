//go:build unix

package plugin

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func pidsOnPort(port int) ([]int, error) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port)).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pids []int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func freePort(port int, wait time.Duration) error {
	pids, err := pidsOnPort(port)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		killPID(pid)
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		pids, err = pidsOnPort(port)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("port %d still in use", port)
}

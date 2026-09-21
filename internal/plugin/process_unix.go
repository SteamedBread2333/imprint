//go:build unix

package plugin

import "syscall"

func killPID(pid int) {
	if pid <= 0 {
		return
	}
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

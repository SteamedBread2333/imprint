//go:build unix

package plugin

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// killPID stops pid.
// A detached server leads its own process group, so the group is signaled and
// its children exit with the listener. A group that contains this process or
// an ancestor — the shell or Cursor — is never signaled. Only that listener
// pid is stopped, and an IDE process is left alone.
func killPID(pid int) {
	if pid <= 1 || isAncestor(pid) || ideProcess(pid) {
		return
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil || pgid <= 1 || protectedProcessGroup(pgid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

func protectedProcessGroup(pgid int) bool {
	if pgid <= 1 {
		return true
	}
	pid := os.Getpid()
	for i := 0; i < 64 && pid > 1; i++ {
		if got, err := syscall.Getpgid(pid); err == nil && got == pgid {
			return true
		}
		next, err := parentPID(pid)
		if err != nil || next <= 1 || next == pid {
			return false
		}
		pid = next
	}
	return false
}

func isAncestor(pid int) bool {
	cur := os.Getpid()
	for i := 0; i < 64 && cur > 1; i++ {
		if cur == pid {
			return true
		}
		next, err := parentPID(cur)
		if err != nil || next <= 1 || next == cur {
			return false
		}
		cur = next
	}
	return false
}

func parentPID(pid int) (int, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid=").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func ideProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	cmd := string(out)
	return strings.Contains(cmd, "Cursor.app") || strings.Contains(cmd, "Cursor Helper")
}

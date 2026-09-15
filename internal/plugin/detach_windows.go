//go:build windows

package plugin

import (
	"os/exec"
	"syscall"
)

// DetachProcess starts the child in a new process group on Windows.
func DetachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

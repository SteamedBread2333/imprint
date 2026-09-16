//go:build unix

package plugin

import (
	"os/exec"
	"syscall"
)

// DetachProcess starts the child in a new session so it outlives the CLI terminal.
func DetachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

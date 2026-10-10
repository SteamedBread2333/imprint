//go:build unix

package plugin

import (
	"fmt"
	"os"
	"syscall"
)

// withEmbedFileLock serializes embed prepare/start so concurrent MCP + CLI
// runs do not fight over Hugging Face cache locks.
func withEmbedFileLock(pluginState string, fn func() error) error {
	if err := os.MkdirAll(pluginState, 0o755); err != nil {
		return err
	}
	lockPath := pluginState + "/download.lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("embed lock: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("embed lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}

//go:build unix

package plugin

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestStopAllFreesConfiguredPluginPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("python3", "-c", fmt.Sprintf(
		"import socket,time;s=socket.socket();s.bind(('127.0.0.1',%d));s.listen(1);time.sleep(30)", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		pids, err := pidsOnPort(port)
		if err == nil && len(pids) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener never bound port %d: %v", port, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	mgr := NewManager(&Config{
		Plugins: map[string]PluginEntry{
			"desk": {Enabled: false, Config: map[string]any{"port": port}},
		},
	})
	mgr.StopAll()

	pids, err := pidsOnPort(port)
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) != 0 {
		t.Fatalf("configured plugin port %d still held by %v", port, pids)
	}
}

//go:build unix

package plugin

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
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

// TestStopAllExceptLeavesOmittedPluginRunning confirms the contract that
// `imprint down` relies on: when embed is in the omit list, the manager
// must not reap a process listening on embed's port, even though the port
// is in cfg.Plugins. A listener started by `imprint-mcp` or `imprint plugin
// start embed` is the embed sidecar's concern, not the manager's.
func TestStopAllExceptLeavesOmittedPluginRunning(t *testing.T) {
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
			"embed": {Enabled: true, Config: map[string]any{"port": port}},
		},
	})
	mgr.StopAllExcept([]string{"embed"})

	pids, err := pidsOnPort(port)
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) == 0 {
		t.Fatalf("omitted plugin port %d was freed by StopAllExcept — embed sidecar was reaped", port)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("listener pid was killed despite omit list: %v", err)
	}
}

// StartEnabledExcept must not free an omitted plugin's port. `imprint up`
// uses this path; killing embed here would drop the MCP sidecar.
func TestStartEnabledExceptDoesNotReapOmittedPort(t *testing.T) {
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
			"embed": {Enabled: true, Config: map[string]any{"port": port}},
		},
	})
	if _, err := mgr.StartEnabledExcept(context.Background(), []string{"embed"}); err != nil {
		t.Fatalf("StartEnabledExcept: %v", err)
	}

	pids, err := pidsOnPort(port)
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) == 0 {
		t.Fatalf("omitted plugin port %d was freed by StartEnabledExcept", port)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("listener pid was killed despite omit list: %v", err)
	}
}

func TestStopPluginFreesConfiguredPort(t *testing.T) {
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
			"embed": {Enabled: true, Config: map[string]any{"port": port}},
		},
	})
	if err := mgr.StopPlugin("embed"); err != nil {
		t.Fatalf("StopPlugin: %v", err)
	}

	pids, err := pidsOnPort(port)
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) != 0 {
		t.Fatalf("plugin port %d still held by %v", port, pids)
	}
}

func TestFreePortDoesNotKillConnectedClient(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	listener := exec.Command("python3", "-c", fmt.Sprintf(
		"import socket,time;s=socket.socket();s.bind(('127.0.0.1',%d));s.listen(1);time.sleep(30)", port))
	listener.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := listener.Start(); err != nil {
		t.Skip(err)
	}
	defer func() { _ = listener.Process.Kill() }()

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

	client := exec.Command("python3", "-c", fmt.Sprintf(
		"import socket,time\n"+
			"s=None\n"+
			"for _ in range(50):\n"+
			"    try:\n"+
			"        s=socket.create_connection(('127.0.0.1',%d),0.2)\n"+
			"        break\n"+
			"    except OSError:\n"+
			"        time.sleep(0.05)\n"+
			"if s is None:\n"+
			"    raise SystemExit(1)\n"+
			"time.sleep(30)", port))
	if err := client.Start(); err != nil {
		t.Skip(err)
	}
	defer func() { _ = client.Process.Kill() }()

	deadline = time.Now().Add(2 * time.Second)
	for {
		out, _ := exec.Command("lsof", "-nP", "-iTCP:"+fmt.Sprint(port)).Output()
		if strings.Contains(string(out), "ESTABLISHED") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client never connected")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := freePort(port, 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := client.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("connected client was killed: %v", err)
	}
}

func TestKillPIDDoesNotSignalCallerProcessGroup(t *testing.T) {
	sibling := exec.Command("sleep", "30")
	if err := sibling.Start(); err != nil {
		t.Skip(err)
	}
	defer func() { _ = sibling.Process.Kill() }()

	target := exec.Command("sleep", "30")
	if err := target.Start(); err != nil {
		t.Skip(err)
	}
	defer func() { _ = target.Process.Kill() }()

	killPID(target.Process.Pid)

	if err := sibling.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("caller process group was killed: %v", err)
	}
}

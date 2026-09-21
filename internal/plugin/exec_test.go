package plugin

import "testing"

func TestStartNeedsShell(t *testing.T) {
	cases := map[string]bool{
		"go run ./cmd/imprint-desk serve":         false,
		"node dist/cli.js serve":                  false,
		"npm run build && node dist/cli.js serve": true,
		"echo a | wc -l":                          true,
	}
	for line, want := range cases {
		if got := startNeedsShell(line); got != want {
			t.Fatalf("startNeedsShell(%q) = %v want %v", line, got, want)
		}
	}
}

func TestExecStartCommandSimple(t *testing.T) {
	cmd, err := execStartCommand(t.Context(), "echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path == "" {
		t.Fatal("empty path")
	}
	if startNeedsShell("echo hello") {
		t.Fatal("echo should not need shell")
	}
}

func TestExecStartCommandChained(t *testing.T) {
	cmd, err := execStartCommand(t.Context(), "npm run build && node dist/cli.js serve")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "sh" && cmd.Args[0] != "cmd" {
		t.Fatalf("chained start should use shell, got %v", cmd.Args)
	}
}

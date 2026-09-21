package privacy

import "testing"

func TestCheckText(t *testing.T) {
	tests := []struct {
		text string
		kind string
	}{
		{"password=hunter2", "secret"},
		{"token sk-1234567890abcdefghijkl", "secret"},
		{"contact dev@example.com", "email"},
		{"host 192.168.1.20", "private_ip"},
		{"read /Users/alice/project/.env", "absolute_path"},
		{"-----BEGIN PRIVATE KEY-----", "pem"},
		{"Use tokens rather than sessions", ""},
		{"Bind the public example 203.0.113.8", ""},
	}
	for _, tt := range tests {
		got := CheckText(tt.text)
		if tt.kind == "" {
			if got != nil {
				t.Errorf("CheckText(%q) = %s, want nil", tt.text, got.Kind)
			}
			continue
		}
		if got == nil || got.Kind != tt.kind {
			t.Errorf("CheckText(%q) = %+v, want %s", tt.text, got, tt.kind)
		}
	}
}

func TestCheckSourcePath(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"docs/guide.md", "README.md"} {
		if got := CheckSourcePath(root, path); got != nil {
			t.Errorf("CheckSourcePath(%q) = %+v", path, got)
		}
	}
	for _, path := range []string{"../secret.md", "/tmp/secret.md", ".env.local", "certs/client.key", "credentials/app.json"} {
		if got := CheckSourcePath(root, path); got == nil {
			t.Errorf("CheckSourcePath(%q) accepted denied path", path)
		}
	}
}

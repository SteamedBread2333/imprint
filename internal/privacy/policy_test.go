package privacy

import "testing"

func TestShouldBlock(t *testing.T) {
	tests := []struct {
		field string
		kind  string
		want  bool
	}{
		{"claim", "email", false},
		{"claim", "secret", true},
		{"claim", "pem", true},
		{"claim", "private_ip", false},
		{"reason", "absolute_path", false},
		{"text", "email", true},
		{"text", "private_ip", true},
		{"evidence", "private_ip", false},
		{"evidence", "email", true},
		{"query_local", "absolute_path", false},
		{"query_local", "phone", true},
		{"unknown", "email", true},
		{"", "secret", true},
	}
	for _, tt := range tests {
		if got := ShouldBlock(tt.field, tt.kind); got != tt.want {
			t.Errorf("ShouldBlock(%q, %q) = %v, want %v", tt.field, tt.kind, got, tt.want)
		}
	}
}

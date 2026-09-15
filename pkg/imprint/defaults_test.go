package imprint

import "testing"

func TestLocalURL(t *testing.T) {
	if got := LocalURL(DefaultHostPort); got != "http://127.0.0.1:9470" {
		t.Fatalf("LocalURL(host) = %q", got)
	}
}

func TestDefaultPortForPlugin(t *testing.T) {
	cases := map[string]int{
		"desk":    DefaultDeskPort,
		"shelves": DefaultShelvesPort,
		"other":   DefaultPluginPort,
	}
	for id, want := range cases {
		if got := DefaultPortForPlugin(id); got != want {
			t.Fatalf("DefaultPortForPlugin(%q) = %d want %d", id, got, want)
		}
	}
}

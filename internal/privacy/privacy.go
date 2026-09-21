// Package privacy implements conservative write-time checks for vault data.
package privacy

import (
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

type Finding struct {
	Kind string
}

var (
	pemPattern       = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*(PRIVATE KEY|CERTIFICATE)-----`)
	assignmentSecret = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret|password|passwd|connection[_-]?string|database[_-]?url)\b\s*[:=]\s*["']?[^\s"']{4,}`)
	tokenPattern     = regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{16,}|gh[pousr]_[a-z0-9]{20,}|xox[baprs]-[a-z0-9-]{16,})\b`)
	emailPattern     = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	ipv4Pattern      = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	homePathPattern  = regexp.MustCompile(`(?:^|[\s"'(])(?:/Users/|/home/|/root/|[A-Za-z]:\\Users\\)[^\s"']+`)
	phonePattern     = regexp.MustCompile(`(?:^|\D)1[3-9]\d{9}(?:\D|$)`)
	chinaIDPattern   = regexp.MustCompile(`(?:^|\D)\d{17}[\dXx](?:\D|$)`)
)

// CheckText reports the first sensitive-data class found without returning the
// matched value.
func CheckText(s string) *Finding {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	switch {
	case pemPattern.MatchString(s):
		return &Finding{Kind: "pem"}
	case assignmentSecret.MatchString(s), tokenPattern.MatchString(s):
		return &Finding{Kind: "secret"}
	case emailPattern.MatchString(s):
		return &Finding{Kind: "email"}
	case homePathPattern.MatchString(s):
		return &Finding{Kind: "absolute_path"}
	case phonePattern.MatchString(s):
		return &Finding{Kind: "phone"}
	case chinaIDPattern.MatchString(s):
		return &Finding{Kind: "government_id"}
	}
	for _, raw := range ipv4Pattern.FindAllString(s, -1) {
		if ip := net.ParseIP(raw); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
			return &Finding{Kind: "private_ip"}
		}
	}
	return nil
}

// CheckSourcePath rejects absolute, escaping, and credential-bearing source
// paths. Sources are workspace-relative pointers.
func CheckSourcePath(root, path string) *Finding {
	path = strings.TrimSpace(path)
	if path == "" {
		return &Finding{Kind: "empty_path"}
	}
	if filepath.IsAbs(path) {
		return &Finding{Kind: "absolute_path"}
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return &Finding{Kind: "outside_workspace"}
	}
	if root != "" {
		target := filepath.Join(root, clean)
		rel, err := filepath.Rel(root, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return &Finding{Kind: "outside_workspace"}
		}
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		lower := strings.ToLower(part)
		if lower == "credentials" || lower == ".env" || strings.HasPrefix(lower, ".env.") {
			return &Finding{Kind: "denied_path"}
		}
	}
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".pem", ".key":
		return &Finding{Kind: "denied_path"}
	}
	return nil
}

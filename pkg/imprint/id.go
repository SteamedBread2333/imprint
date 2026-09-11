package imprint

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func formatID(t time.Time, n int) string {
	return fmt.Sprintf("r-%s-%03d", t.UTC().Format("2006-01-02"), n)
}

func idDatePrefix(t time.Time) string {
	return "r-" + t.UTC().Format("2006-01-02") + "-"
}

func parseIDSeq(id, prefix string) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func (v *Vault) nextID(t time.Time) (string, error) {
	prefix := idDatePrefix(t)
	recs, err := v.loadAll()
	if err != nil {
		return "", err
	}
	maxN := 0
	for _, r := range recs {
		if n, ok := parseIDSeq(r.ID, prefix); ok && n > maxN {
			maxN = n
		}
	}
	return formatID(t, maxN+1), nil
}

func looksLikeID(id string) bool {
	return strings.HasPrefix(id, IDPrefix) && len(id) >= len("r-2006-01-02-001")
}

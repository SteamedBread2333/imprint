package privacy

// ShouldBlock reports whether a write to field should be rejected for kind.
//
// Detectors always run; this only gates privacy_rejected. Unknown fields
// block every kind (safe default).
//
// Field policy:
//   - claim, reason: block pem, secret, government_id only
//   - text: block all kinds CheckText returns
//   - evidence, query_local: block pem, secret, government_id, email, phone;
//     ignore private_ip and absolute_path
var fieldBlockKinds = map[string]map[string]struct{}{
	"claim": {
		"pem": {}, "secret": {}, "government_id": {},
	},
	"reason": {
		"pem": {}, "secret": {}, "government_id": {},
	},
	"text": {
		"pem": {}, "secret": {}, "email": {}, "phone": {},
		"government_id": {}, "private_ip": {}, "absolute_path": {},
	},
	"evidence": {
		"pem": {}, "secret": {}, "email": {}, "phone": {}, "government_id": {},
	},
	"query_local": {
		"pem": {}, "secret": {}, "email": {}, "phone": {}, "government_id": {},
	},
}

// ShouldBlock returns true when field must reject kind.
func ShouldBlock(field, kind string) bool {
	if kind == "" {
		return false
	}
	set, ok := fieldBlockKinds[field]
	if !ok {
		return true
	}
	_, ok = set[kind]
	return ok
}

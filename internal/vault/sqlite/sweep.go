package sqlite

import (
	"database/sql"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

// minIdleDays is the smallest idle span that counts as decayable. Below it a
// rule is "just confirmed" and must keep its confidence exactly — otherwise
// sub-second idle times produce float noise that decays a rule on every sweep,
// breaking idempotence.
const minIdleDays = 1.0 / 24.0

// SweepActive decays stale active rules and archives those below threshold.
//
// Decay is a pure function of elapsed time, not of how often sweep runs:
//
//	confidence' = max(0, base_confidence - λ * idle_days)
//
// where λ = decayAmount / decayDays and idle_days is measured from the most
// recent activity (explicit confirmation or recall hit). Running sweep daily,
// monthly, or once yields the same confidence — the old fixed −0.05 per call
// coupled decay speed to cron frequency and never decayed if never run.
//
// It never rewrites child tables or evidence.
func (s *Store) SweepActive(now time.Time, decayDays int, decayAmount, dormantThreshold float64) (decayed, archived int, err error) {
	if decayDays <= 0 {
		decayDays = 1
	}
	lambda := decayAmount / float64(decayDays)
	nowStr := formatTime(now)
	err = s.withTx(func(tx *sql.Tx) error {
		// Self-heal rows whose baseline is missing (e.g. stats rows created
		// before v2): the baseline is the rule's current confidence.
		if _, err := tx.Exec(`
UPDATE rule_stats SET base_confidence = (
  SELECT confidence FROM rules WHERE rules.id = rule_stats.rule_id
) WHERE base_confidence < 0`); err != nil {
			return err
		}

		// idle expression: days since the later of explicit confirmation
		// and last recall hit (retrieval counts as activity — testing effect).
		const idle = `(julianday(?) - julianday(MAX(st.last_confirmed_at, COALESCE(st.last_recalled_at, st.last_confirmed_at))))`
		const target = `MAX(0, st.base_confidence - ? * ` + idle + `)`
		// updated_at is deliberately NOT bumped: decay is background
		// maintenance, not user activity. Bumping it would poison
		// `list --since` ("touched since") and the recency tiebreaks in
		// find/Show with rules the user never touched.
		rows, err := tx.Query(`
UPDATE rules
SET confidence = `+target+`,
    status = CASE WHEN `+target+` < ? THEN ? ELSE rules.status END
FROM rule_stats st
WHERE st.rule_id = rules.id
  AND rules.status = ?
  AND `+idle+` >= ?
  AND `+target+` < rules.confidence
RETURNING rules.id, rules.status`,
			lambda, nowStr,
			lambda, nowStr, dormantThreshold, string(model.StatusDormant),
			string(model.StatusActive),
			nowStr, minIdleDays,
			lambda, nowStr,
		)
		if err != nil {
			return err
		}
		var dormantIDs []string
		for rows.Next() {
			var id, status string
			if err := rows.Scan(&id, &status); err != nil {
				_ = rows.Close()
				return err
			}
			decayed++
			if status == string(model.StatusDormant) {
				archived++
				dormantIDs = append(dormantIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}

		rows, err = tx.Query(`
UPDATE rules
SET status = ?
WHERE status = ? AND confidence < ?
RETURNING id`,
			string(model.StatusDormant),
			string(model.StatusActive), dormantThreshold,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			archived++
			dormantIDs = append(dormantIDs, id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range dormantIDs {
			if err := insertRuleEvent(tx, id, now, "sweep_dormant", ""); err != nil {
				return err
			}
		}
		return nil
	})
	return decayed, archived, err
}

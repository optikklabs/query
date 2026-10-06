package expr

import (
	"cmp"
	"database/sql"
	"time"

	models "github.com/optikklabs/query/internal/modules/alerting/shared/models"
)

type Decision struct {
	NewStatus    string
	Transition   bool
	ShouldNotify bool
	IsRecovery   bool
	// NoDataSince is when the current data gap began; invalid while data flows.
	NoDataSince sql.NullTime
}

func Decide(prev models.MonitorStateRow, cond models.Conditions, value float64, hasData bool, renotifyEverySec int64, now time.Time) Decision {
	prevStatus := prev.Status
	noDataSince := sql.NullTime{}
	if !hasData {
		noDataSince = prev.NoDataSince
		if !noDataSince.Valid {
			noDataSince = sql.NullTime{Valid: true, Time: now}
		}
	}
	// A data gap shorter than NoDataAfterSec holds the current status instead
	// of applying NoDataAs, so a late or sparse series does not flap.
	newStatus := prevStatus
	if hasData || now.Sub(noDataSince.Time) >= time.Duration(cond.NoDataAfterSec)*time.Second {
		newStatus = classify(prevStatus, cond, value, hasData)
	}
	transition := newStatus != prevStatus

	notify := false
	isRecovery := false
	switch {
	case transition && models.IsFiring(newStatus):
		notify = true
	case transition && newStatus == models.StatusOK && models.IsFiring(prevStatus):
		notify = true
		isRecovery = true
	case !transition && newStatus == models.StatusAlert && renotifyEverySec > 0 && prev.LastNotifiedAt.Valid:
		elapsed := now.Sub(prev.LastNotifiedAt.Time)
		if elapsed >= time.Duration(renotifyEverySec)*time.Second {
			notify = true
		}
	}
	return Decision{
		NewStatus:    newStatus,
		Transition:   transition,
		ShouldNotify: notify,
		IsRecovery:   isRecovery,
		NoDataSince:  noDataSince,
	}
}

func classify(prev string, cond models.Conditions, value float64, hasData bool) string {
	if !hasData {
		return cond.NoDataAs
	}
	hit := func(threshold *float64) bool {
		return threshold != nil && breaches(cond.Comparator, value, *threshold)
	}
	if hit(cond.AlertThreshold) {
		return models.StatusAlert
	}
	if hit(cond.WarnThreshold) {
		return models.StatusWarn
	}
	// A firing monitor stays firing until the value clears the recovery
	// threshold, which defaults to the warn, then the alert threshold.
	if models.IsFiring(prev) && breaches(cond.Comparator, value, *cmp.Or(cond.RecoveryThreshold, cond.WarnThreshold, cond.AlertThreshold)) {
		return prev
	}
	return models.StatusOK
}

// breaches reports whether value is on the alerting side of threshold.
func breaches(comparator string, value, threshold float64) bool {
	switch comparator {
	case models.ComparatorAbove:
		return value > threshold
	case models.ComparatorBelow:
		return value < threshold
	default: // models.ComparatorEqual
		return value == threshold
	}
}

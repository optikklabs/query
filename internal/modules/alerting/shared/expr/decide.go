package expr

import (
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
	if prevStatus == "" {
		prevStatus = "no_data"
	}
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
	case transition && newStatus == "alert":
		notify = true
	case transition && newStatus == "warn":
		notify = true
	case transition && newStatus == "ok" && (prevStatus == "alert" || prevStatus == "warn"):
		notify = true
		isRecovery = true
	case !transition && newStatus == "alert" && renotifyEverySec > 0 && prev.LastNotifiedAt.Valid:
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
		switch cond.NoDataAs {
		case "alert":
			return "alert"
		case "ok":
			return "ok"
		default:
			return "no_data"
		}
	}
	cmp := cond.Comparator
	if cmp == "" {
		cmp = "above"
	}
	hit := func(threshold *float64) bool {
		if threshold == nil {
			return false
		}
		switch cmp {
		case "above":
			return value > *threshold
		case "below":
			return value < *threshold
		case "equal":
			return value == *threshold
		}
		return false
	}
	hitRecovery := func() bool {
		t := cond.RecoveryThreshold
		if t == nil {
			t = cond.WarnThreshold
		}
		if t == nil {
			t = cond.AlertThreshold
		}
		if t == nil {
			return true
		}
		switch cmp {
		case "above":
			return value <= *t
		case "below":
			return value >= *t
		case "equal":
			return value != *t
		}
		return true
	}

	switch prev {
	case "alert", "warn":
		if hit(cond.AlertThreshold) {
			return "alert"
		}
		if hit(cond.WarnThreshold) {
			return "warn"
		}
		if hitRecovery() {
			return "ok"
		}
		return prev
	default:
		if hit(cond.AlertThreshold) {
			return "alert"
		}
		if hit(cond.WarnThreshold) {
			return "warn"
		}
		return "ok"
	}
}

package snapshot

import (
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
)

// ReadyForSnapshot returns true if enough time has passed to create a new snapshot.
func ReadyForSnapshot(crd *cosmosv1.HostedSnapshot, now time.Time) bool {
	history := crd.Status.JobHistory
	if len(history) == 0 {
		return true
	}

	dur := crd.Spec.Interval.Duration
	if dur <= 0 {
		dur = 24 * time.Hour
	}

	// JobHistory should always have most recent first.
	status := history[0]
	return now.Sub(status.StartTime.Time) >= dur
}

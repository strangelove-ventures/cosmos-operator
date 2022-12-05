package volsnapshot

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
)

// DurationUntilNext the duration until it's time to take the next snapshot.
// A zero duration without an error indicates a VolumeSnapshot should be created.
// TODO(nix): Take into account any running snapshots or completed snapshots.
func DurationUntilNext(crd *cosmosalpha.ScheduledVolumeSnapshot, now time.Time) (time.Duration, error) {
	sched, err := cron.ParseStandard(crd.Spec.Schedule)
	if err != nil {
		return 0, fmt.Errorf("invalid spec.schedule: %w", err)
	}
	next := sched.Next(crd.Status.CreatedAt.Time)
	return lo.Max([]time.Duration{next.Sub(now), 0}), nil
}

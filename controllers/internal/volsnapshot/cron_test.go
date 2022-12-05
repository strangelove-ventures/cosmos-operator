package volsnapshot

import (
	"testing"
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDurationUntilNext(t *testing.T) {
	t.Parallel()

	t.Run("happy path - first snapshot", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		createdAt := time.Date(2022, time.December, 1, 0, 0, 0, 0, time.UTC)
		crd.Status.CreatedAt = metav1.NewTime(createdAt)

		for _, tt := range []struct {
			Schedule     string
			Now          time.Time
			WantDuration time.Duration
		}{
			// Wait
			{
				"0 * * * *", // hourly
				createdAt,
				time.Hour,
			},
			{
				"0 * * * *", // hourly
				createdAt.Add(30 * time.Minute),
				30 * time.Minute,
			},
			{
				"0 0 * * *", // daily at midnight
				createdAt.Add(1 * time.Hour),
				23 * time.Hour,
			},
			{
				"* * * * *", // every minute
				createdAt,
				time.Minute,
			},
			{
				"0 */3 * * *", // At minute 0 past every 3rd hour
				createdAt,
				3 * time.Hour,
			},

			// Ready
			{
				"0 * * * *", // hourly
				createdAt.Add(1 * time.Hour),
				0,
			},
			{
				"0 * * * *", // hourly
				createdAt.Add(1 * time.Hour),
				0,
			},
			{
				"0 0 * * *", // daily at midnight
				createdAt.Add(24*time.Hour + time.Minute),
				0,
			},
		} {
			crd.Spec.Schedule = tt.Schedule
			got, err := DurationUntilNext(&crd, tt.Now)

			require.NoError(t, err, tt)
			require.Equal(t, tt.WantDuration, got, tt)
		}
	})

	t.Run("invalid schedule", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Spec.Schedule = "bogus"
		_, err := DurationUntilNext(&crd, time.Now())

		require.Error(t, err)
	})
}

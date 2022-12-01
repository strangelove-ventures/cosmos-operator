package fullnode

import (
	"testing"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResetStatus(t *testing.T) {
	t.Parallel()

	t.Run("basic happy path", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Generation = 123
		crd.Status.StatusMessage = ptr("should not see me")
		crd.Status.Phase = "should not see me"
		crd.Status.VolumeSnapshot = new(cosmosv1.VolumeSnapshotStatus)
		ResetStatus(&crd)

		require.EqualValues(t, 123, crd.Status.ObservedGeneration)
		require.Nil(t, crd.Status.StatusMessage)
		require.Equal(t, cosmosv1.FullNodePhaseProgressing, crd.Status.Phase)
		require.Nil(t, crd.Status.VolumeSnapshot)
	})

	t.Run("volume snapshot", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.VolumeSnapshot = &cosmosv1.VolumeSnapshotSpec{
			Schedule: "@daily",
		}
		ResetStatus(&crd)

		require.NotNil(t, crd.Status.VolumeSnapshot)
		require.WithinDuration(t, time.Now(), crd.Status.VolumeSnapshot.ActivatedAt.Time, 10*time.Second)

		existing := time.Date(2022, time.December, 1, 0, 0, 0, 0, time.UTC)
		crd.Status.VolumeSnapshot.ActivatedAt = metav1.NewTime(existing)
		ResetStatus(&crd)

		require.Equal(t, existing, crd.Status.VolumeSnapshot.ActivatedAt.Time)
	})
}

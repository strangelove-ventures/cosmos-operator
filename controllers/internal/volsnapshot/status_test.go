package volsnapshot

import (
	"testing"
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResetStatus(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Generation = 456
		crd.Status.StatusMessage = ptr("should not see me")
		createdAt := metav1.NewTime(time.Now())
		crd.Status.CreatedAt = createdAt

		ResetStatus(&crd)

		require.EqualValues(t, 456, crd.Status.ObservedGeneration)
		require.Nil(t, crd.Status.StatusMessage)
		require.Equal(t, createdAt, crd.Status.CreatedAt)
	})

	t.Run("createAt not set", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		ResetStatus(&crd)

		require.WithinDuration(t, time.Now(), crd.Status.CreatedAt.Time, 10*time.Second)
	})
}

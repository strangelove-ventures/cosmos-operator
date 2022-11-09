package snapshot

import (
	"testing"
	"time"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReadyForSnapshot(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		const duration = time.Hour
		now := time.Now()
		crd := cosmosalpha.StatefulJob{
			Spec: cosmosalpha.StatefulJobSpec{
				Interval: metav1.Duration{Duration: duration},
			},
			Status: cosmosalpha.StatefulJobStatus{
				JobHistory: []batchv1.JobStatus{
					{StartTime: ptr(metav1.NewTime(now))},
					{StartTime: ptr(metav1.NewTime(now.Add(-2 * duration)))},
				},
			},
		}

		require.True(t, ReadyForSnapshot(&crd, now.Add(duration)))
		require.True(t, ReadyForSnapshot(&crd, now.Add(duration+1)))
		require.False(t, ReadyForSnapshot(&crd, now.Add(duration-1)))
		require.False(t, ReadyForSnapshot(&crd, now))
	})

	t.Run("default", func(t *testing.T) {
		const duration = 24 * time.Hour
		now := time.Now()
		crd := cosmosalpha.StatefulJob{
			Status: cosmosalpha.StatefulJobStatus{
				JobHistory: []batchv1.JobStatus{
					{StartTime: ptr(metav1.NewTime(now))},
					{StartTime: ptr(metav1.NewTime(now.Add(-duration)))},
				},
			},
		}

		require.True(t, ReadyForSnapshot(&crd, now.Add(duration)))
		require.True(t, ReadyForSnapshot(&crd, now.Add(duration+1)))
		require.False(t, ReadyForSnapshot(&crd, now))
	})

	t.Run("zero state", func(t *testing.T) {
		now := time.Now()
		var crd cosmosalpha.StatefulJob

		require.True(t, ReadyForSnapshot(&crd, now))
		require.True(t, ReadyForSnapshot(&crd, now.Add(24*time.Hour)))
		require.True(t, ReadyForSnapshot(&crd, now.Add(-24*time.Hour)))
	})
}

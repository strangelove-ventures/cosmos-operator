package volsnapshot

import (
	"context"
	"testing"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockGet func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

func (fn mockGet) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic("got unexpected opts")
	}
	return fn(ctx, key, obj)
}

var panicGetter = mockGet(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	panic("should not be called")
})

func TestScheduler_CalcNext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

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
			sched := NewScheduler(panicGetter)
			sched.now = func() time.Time {
				return tt.Now
			}
			got, err := sched.CalcNext(ctx, &crd)

			require.NoError(t, err, tt)
			require.Equal(t, tt.WantDuration, got, tt)
		}
	})

	t.Run("happy path - completed snapshot", func(t *testing.T) {
		now := time.Date(2022, time.January, 0, 0, 0, 0, 0, time.Local)

		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Namespace = "strangelove"
		crd.Spec.Schedule = "0 * * * *"
		// Should not happen but proves test uses status.lastSnapshot.
		crd.Status.CreatedAt = metav1.NewTime(now)
		crd.Status.LastSnapshot = &cosmosalpha.VolumeSnapshotStatus{
			Name:      "my-snapshot-123",
			StartedAt: metav1.NewTime(now.Add(-time.Hour)),
			Status: &snapshotv1.VolumeSnapshotStatus{
				// Proves we use the result from Get.
				ReadyToUse: ptr(false),
			},
		}

		readyStatus := &snapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr(true)}
		sched := NewScheduler(mockGet(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			require.Equal(t, "strangelove", key.Namespace)
			require.Equal(t, "my-snapshot-123", key.Name)

			ref := obj.(*snapshotv1.VolumeSnapshot)
			*ref = snapshotv1.VolumeSnapshot{
				Status: readyStatus,
			}
			return nil
		}))
		sched.now = func() time.Time { return now }

		got, err := sched.CalcNext(ctx, &crd)
		require.NoError(t, err)
		require.Zero(t, got)
		require.Equal(t, readyStatus, crd.Status.LastSnapshot.Status)
	})

	t.Run("happy path - currently running snapshot", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("get error", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("get error - does not exist", func(t *testing.T) {
		// This would only happen if something or someone deleted the snapshot while it's running.
	})

	t.Run("invalid schedule", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Spec.Schedule = "bogus"
		sched := NewScheduler(panicGetter)
		_, err := sched.CalcNext(ctx, &crd)

		require.Error(t, err)
		require.False(t, err.IsTransient())
	})
}

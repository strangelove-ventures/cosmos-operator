package volsnapshot

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	t.Run("happy path - first snapshot", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		refDate := time.Date(2022, time.December, 1, 0, 0, 0, 0, time.UTC)

		for _, scenario := range []struct {
			Status cosmosalpha.ScheduledVolumeSnapshotStatus
			Name   string
		}{
			{cosmosalpha.ScheduledVolumeSnapshotStatus{CreatedAt: metav1.NewTime(refDate)}, "createdAt"},
			{cosmosalpha.ScheduledVolumeSnapshotStatus{LastSnapshot: &cosmosalpha.VolumeSnapshotStatus{StartedAt: metav1.NewTime(refDate)}}, "lastSnapshot.startedAt"},
		} {
			for _, tt := range []struct {
				Schedule     string
				Now          time.Time
				WantDuration time.Duration
			}{
				// Wait
				{
					"0 * * * *", // hourly
					refDate,
					time.Hour,
				},
				{
					"0 * * * *", // hourly
					refDate.Add(30 * time.Minute),
					30 * time.Minute,
				},
				{
					"0 0 * * *", // daily at midnight
					refDate.Add(1 * time.Hour),
					23 * time.Hour,
				},
				{
					"* * * * *", // every minute
					refDate,
					time.Minute,
				},
				{
					"0 */3 * * *", // At minute 0 past every 3rd hour
					refDate,
					3 * time.Hour,
				},

				// Ready
				{
					"0 * * * *", // hourly
					refDate.Add(1 * time.Hour),
					0,
				},
				{
					"0 * * * *", // hourly
					refDate.Add(1 * time.Hour),
					0,
				},
				{
					"0 0 * * *", // daily at midnight
					refDate.Add(24*time.Hour + time.Minute),
					0,
				},
			} {
				crd.Status = scenario.Status
				crd.Spec.Schedule = tt.Schedule
				sched := NewScheduler(panicGetter)
				sched.now = func() time.Time {
					return tt.Now
				}
				got, err := sched.CalcNext(&crd)

				msg := fmt.Sprintf("%s: %+v", scenario.Name, tt)
				require.NoError(t, err, scenario.Name, msg)
				require.Equal(t, tt.WantDuration, got, msg)
			}
		}
	})
}

func TestScheduler_IsSnapshotReady(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path - not ready", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Namespace = "strangelove"
		crd.Status.LastSnapshot = &cosmosalpha.VolumeSnapshotStatus{
			Name: "my-snapshot-123",
		}

		notReadyStatus := &snapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr(false)}
		sched := NewScheduler(mockGet(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			require.Equal(t, "strangelove", key.Namespace)
			require.Equal(t, "my-snapshot-123", key.Name)

			ref := obj.(*snapshotv1.VolumeSnapshot)
			*ref = snapshotv1.VolumeSnapshot{
				Status: notReadyStatus,
			}
			return nil
		}))

		got, err := sched.IsSnapshotReady(ctx, &crd)

		require.NoError(t, err)
		require.False(t, got)

		require.Equal(t, notReadyStatus, crd.Status.LastSnapshot.Status)
	})

	t.Run("happy path - ready for use", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Status.LastSnapshot = new(cosmosalpha.VolumeSnapshotStatus)

		readyStatus := &snapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr(true)}
		sched := NewScheduler(mockGet(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			ref := obj.(*snapshotv1.VolumeSnapshot)
			*ref = snapshotv1.VolumeSnapshot{
				Status: readyStatus,
			}
			return nil
		}))

		got, err := sched.IsSnapshotReady(ctx, &crd)

		require.NoError(t, err)
		require.True(t, got)

		require.Equal(t, readyStatus, crd.Status.LastSnapshot.Status)
	})

	t.Run("not found error", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		status := new(cosmosalpha.VolumeSnapshotStatus)
		crd.Status.LastSnapshot = status

		sched := NewScheduler(mockGet(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			return &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		}))

		got, err := sched.IsSnapshotReady(ctx, &crd)

		require.NoError(t, err)
		require.True(t, got)

		require.Same(t, status, crd.Status.LastSnapshot)
	})

	t.Run("error", func(t *testing.T) {
		var crd cosmosalpha.ScheduledVolumeSnapshot
		crd.Status.LastSnapshot = new(cosmosalpha.VolumeSnapshotStatus)

		sched := NewScheduler(mockGet(func(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			return errors.New("boom")
		}))

		_, err := sched.IsSnapshotReady(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}

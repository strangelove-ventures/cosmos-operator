package snapshot

import (
	"context"
	"errors"
	"testing"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockLister func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

func (fn mockLister) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fn(ctx, list, opts...)
}

func TestRecentVolumeSnapshot(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		crd = &cosmosv1.HostedSnapshot{
			Spec: cosmosv1.HostedSnapshotSpec{
				Selector: map[string]string{"test": "selector"},
			},
		}
	)
	crd.Namespace = "testns"

	t.Run("happy path", func(t *testing.T) {
		now := metav1.Now()
		var snap1 snapshotv1.VolumeSnapshot
		snap1.Name = "snap1"
		snap1.Status = &snapshotv1.VolumeSnapshotStatus{
			CreationTime: ptr(now),
			ReadyToUse:   ptr(true),
		}

		snap2 := *snap1.DeepCopy()
		snap2.Name = "snap2"
		snap2.Status.CreationTime = ptr(metav1.NewTime(now.Add(-time.Hour)))

		snap3 := *snap1.DeepCopy()
		snap3.Name = "snap3"
		snap3.Status = nil

		snap4 := *snap1.DeepCopy()
		snap4.Name = "snap4"
		snap4.Status = &snapshotv1.VolumeSnapshotStatus{} // empty

		var list snapshotv1.VolumeSnapshotList
		list.Items = append(list.Items, snap2, snap1, snap4, snap3)

		lister := mockLister(func(ctx context.Context, inList client.ObjectList, opts ...client.ListOption) error {
			require.NotNil(t, ctx)
			require.NotNil(t, list)
			require.Equal(t, 2, len(opts))
			require.Equal(t, client.InNamespace("testns"), opts[0])
			require.Equal(t, client.MatchingLabels{"test": "selector"}, opts[1])

			ref := inList.(*snapshotv1.VolumeSnapshotList)
			*ref = list
			return nil
		})

		got, err := RecentVolumeSnapshot(ctx, lister, crd)
		require.NoError(t, err)
		require.Equal(t, snap1, *got)
	})

	t.Run("none found", func(t *testing.T) {
		var list snapshotv1.VolumeSnapshotList
		lister := mockLister(func(ctx context.Context, inList client.ObjectList, opts ...client.ListOption) error {
			ref := inList.(*snapshotv1.VolumeSnapshotList)
			*ref = list
			return nil
		})

		_, err := RecentVolumeSnapshot(ctx, lister, crd)
		require.Error(t, err)
		require.EqualError(t, err, "no VolumeSnapshots found")
	})

	t.Run("error", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("boom")
		})

		_, err := RecentVolumeSnapshot(ctx, lister, crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})

	t.Run("not ready", func(t *testing.T) {
		for _, tt := range []struct {
			Status *snapshotv1.VolumeSnapshotStatus
		}{
			{nil},
			{&snapshotv1.VolumeSnapshotStatus{}},
			{&snapshotv1.VolumeSnapshotStatus{ReadyToUse: ptr(false)}},
		} {
			var snap1 snapshotv1.VolumeSnapshot
			snap1.Name = "not-ready-test"
			snap1.Status = tt.Status

			var list snapshotv1.VolumeSnapshotList
			list.Items = append(list.Items, snap1)

			lister := mockLister(func(ctx context.Context, inList client.ObjectList, opts ...client.ListOption) error {
				ref := inList.(*snapshotv1.VolumeSnapshotList)
				*ref = list
				return nil
			})

			_, err := RecentVolumeSnapshot(ctx, lister, crd)
			require.Error(t, err, tt)
			require.ErrorIs(t, err, ErrNotReady, tt)
			require.Contains(t, err.Error(), "not-ready-test")
		}
	})
}

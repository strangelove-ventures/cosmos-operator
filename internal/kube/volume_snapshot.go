package kube

import (
	"context"
	"errors"
	"sort"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VolumeSnapshotIsReady returns true if the snapshot is ready to use.
func VolumeSnapshotIsReady(status *snapshotv1.VolumeSnapshotStatus) bool {
	if status == nil {
		return false
	}
	if status.ReadyToUse == nil {
		return false
	}
	return *status.ReadyToUse
}

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

// RecentVolumeSnapshot finds the most recent, ready to use VolumeSnapshot.
// This function may not work well given very large lists and therefore assumes a reasonable number of VolumeSnapshots.
// If you must search among many VolumeSnapshots, consider refactoring to use Limit and Continue features of listing.
func RecentVolumeSnapshot(ctx context.Context, lister Lister, namespace string, selector map[string]string) (*snapshotv1.VolumeSnapshot, error) {
	var snapshots snapshotv1.VolumeSnapshotList
	err := lister.List(ctx,
		&snapshots,
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	)
	if err != nil {
		return nil, err
	}

	filtered := lo.Filter(snapshots.Items, func(s snapshotv1.VolumeSnapshot, _ int) bool {
		return VolumeSnapshotIsReady(s.Status)
	})
	if len(filtered) == 0 {
		return nil, errors.New("no ready to use VolumeSnapshots found")
	}

	sort.Slice(filtered, func(i, j int) bool {
		lhs := statusCreationTime(filtered[i].Status)
		rhs := statusCreationTime(filtered[j].Status)
		return !lhs.Before(rhs)
	})

	found := &filtered[0]
	return found, nil
}

func statusCreationTime(status *snapshotv1.VolumeSnapshotStatus) (zero time.Time) {
	if status == nil {
		return zero
	}
	if status.CreationTime == nil {
		return zero
	}
	return status.CreationTime.Time
}

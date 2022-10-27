package snapshot

import (
	"context"
	"errors"
	"sort"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

var (
	// ErrNotFound indicates the VolumeSnapshot does not exist.
	ErrNotFound = errors.New("not found")
	// ErrNotReady indicates VolumeSnapshot was found but is not ready for use.
	ErrNotReady = errors.New("not ready")

	isNotFoundErr = apierrors.IsNotFound
)

// RecentVolumeSnapshot finds the most recent, ready to use VolumeSnapshot.
// This function may not work well given very large lists and therefore assumes a reasonable number of VolumeSnapshots.
// If you must search among many VolumeSnapshots, consider refactoring to use Limit and Continue features of listing.
func RecentVolumeSnapshot(ctx context.Context, lister Lister, crd *cosmosv1.HostedSnapshot) (*snapshotv1.VolumeSnapshot, error) {
	var snapshots snapshotv1.VolumeSnapshotList
	err := lister.List(ctx,
		&snapshots,
		client.InNamespace(crd.Namespace),
		client.MatchingLabels(crd.Spec.Selector),
	)

	switch {
	case isNotFoundErr(err):
		return nil, multierr.Append(err, ErrNotFound)
	case err != nil:
		return nil, err
	}

	sort.Slice(snapshots.Items, func(i, j int) bool {
		lhs := statusCreationTime(snapshots.Items[i].Status)
		rhs := statusCreationTime(snapshots.Items[j].Status)
		return !lhs.Before(rhs)
	})

	found := &snapshots.Items[0]
	if !statusIsReady(found.Status) {
		return nil, ErrNotReady
	}

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

func statusIsReady(status *snapshotv1.VolumeSnapshotStatus) bool {
	if status == nil {
		return false
	}
	if status.ReadyToUse == nil {
		return false
	}
	return *status.ReadyToUse
}

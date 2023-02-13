package volsnapshot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/samber/lo"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const cosmosSourceLabel = "cosmos.strange.love/source"

// Client is a subset of client.Client.
type Client interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

type PodFinder interface {
	LargestHeight(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)
}

// VolumeSnapshotControl manages VolumeSnapshots
type VolumeSnapshotControl struct {
	client Client
	finder PodFinder
	now    func() time.Time
}

func NewVolumeSnapshotControl(client Client, finder PodFinder) *VolumeSnapshotControl {
	return &VolumeSnapshotControl{
		client: client,
		finder: finder,
		now:    time.Now,
	}
}

type Candidate = cosmosalpha.SnapshotCandidate

// FindCandidate finds a suitable candidate for creating a volume snapshot.
// Any errors returned can be treated as transient; worth a retry.
func (control VolumeSnapshotControl) FindCandidate(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot) (Candidate, error) {
	var pods corev1.PodList
	if err := control.client.List(ctx, &pods,
		client.InNamespace(crd.Spec.FullNodeRef.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Spec.FullNodeRef.Name},
	); err != nil {
		return Candidate{}, err
	}

	if len(pods.Items) == 0 {
		return Candidate{}, errors.New("list operation returned no pods")
	}

	avail := int32(len(kube.AvailablePods(ptrSlice(pods.Items), 0, time.Now())))
	minAvail := crd.Spec.MinAvailable
	if minAvail <= 0 {
		minAvail = 2
	}

	if avail < minAvail {
		return Candidate{}, fmt.Errorf("%d or more pods must be ready to prevent downtime, found %d ready", minAvail, avail)
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pod, err := control.finder.LargestHeight(cctx, ptrSlice(pods.Items))
	if err != nil {
		return Candidate{}, err
	}

	return Candidate{
		PodLabels: pod.Labels,
		PodName:   pod.Name,
		PVCName:   fullnode.PVCName(pod),
	}, nil
}

// CreateSnapshot creates VolumeSnapshot from the Candidate.PVCName and updates crd.status to reflect the created VolumeSnapshot.
// CreateSnapshot does not set an owner reference to avoid garbage collection if the CRD is deleted.
// Any error returned is considered transient and can be retried.
func (control VolumeSnapshotControl) CreateSnapshot(ctx context.Context, crd *cosmosalpha.ScheduledVolumeSnapshot, candidate Candidate) error {
	snapshot := snapshotv1.VolumeSnapshot{
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: ptr(candidate.PVCName),
			},
			VolumeSnapshotClassName: ptr(crd.Spec.VolumeSnapshotClassName),
		},
	}
	snapshot.Namespace = crd.Namespace
	ts := control.now().UTC().Format("200601021504")
	name := kube.ToName(fmt.Sprintf("%s-%s", crd.Name, ts))
	snapshot.Name = name

	snapshot.Labels = lo.Assign(candidate.PodLabels)
	snapshot.Labels[kube.ComponentLabel] = "ScheduledVolumeSnapshot"
	snapshot.Labels[kube.ControllerLabel] = "cosmos-operator"
	snapshot.Labels[cosmosSourceLabel] = crd.Name

	if err := control.client.Create(ctx, &snapshot); err != nil {
		return err
	}

	crd.Status.LastSnapshot = &cosmosalpha.VolumeSnapshotStatus{
		Name:      name,
		StartedAt: metav1.NewTime(control.now()),
	}

	return nil
}

// DeleteOldSnapshots deletes old VolumeSnapshots given crd's spec.limit.
// If limit not set, defaults to keeping the 3 most recent.
func (control VolumeSnapshotControl) DeleteOldSnapshots(ctx context.Context, log logr.Logger, crd *cosmosalpha.ScheduledVolumeSnapshot) error {
	limit := int(crd.Spec.Limit)
	if limit <= 0 {
		limit = 3
	}
	var snapshots snapshotv1.VolumeSnapshotList
	err := control.client.List(ctx,
		&snapshots,
		client.InNamespace(crd.Namespace),
		client.MatchingLabels(map[string]string{cosmosSourceLabel: crd.Name}),
	)
	if err != nil {
		return fmt.Errorf("list volume snapshots: %w", err)
	}

	filtered := lo.Filter(snapshots.Items, func(item snapshotv1.VolumeSnapshot, _ int) bool {
		return item.Status != nil && item.Status.CreationTime != nil
	})

	if len(filtered) <= limit {
		return nil
	}

	// Sort by time descending
	sort.Slice(filtered, func(i, j int) bool {
		lhs := filtered[i].Status.CreationTime
		rhs := filtered[j].Status.CreationTime
		return !lhs.Before(rhs)
	})

	toDelete := filtered[limit:]

	var merr error
	for _, vs := range toDelete {
		vs := vs
		log.Info("Deleting volume snapshot", "volumeSnapshotName", vs.Name, "limit", limit)
		if err := control.client.Delete(ctx, &vs); kube.IgnoreNotFound(err) != nil {
			multierr.AppendInto(&merr, fmt.Errorf("delete %s: %w", vs.Name, err))
		}
	}
	return merr
}

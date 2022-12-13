package volsnapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a subset of client.Client.
type Client interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

type PodFinder interface {
	SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error)
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
		client.InNamespace(crd.Spec.SourceRef.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Spec.SourceRef.Name},
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

	pod, err := control.finder.SyncedPod(ctx, ptrSlice(pods.Items))
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
	snapshot.Labels = candidate.PodLabels
	if snapshot.Labels == nil {
		snapshot.Labels = make(map[string]string)
	}
	snapshot.Labels[kube.ComponentLabel] = "ScheduledVolumeSnapshot"
	snapshot.Labels[kube.ControllerLabel] = "cosmos-operator"

	if err := control.client.Create(ctx, &snapshot); err != nil {
		return err
	}

	crd.Status.LastSnapshot = &cosmosalpha.VolumeSnapshotStatus{
		Name:      name,
		StartedAt: metav1.NewTime(control.now()),
	}

	return nil
}
